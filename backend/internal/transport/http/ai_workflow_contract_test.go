package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestRequirementPoolWorkflowAndAIRunContract(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	initialTemplateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)

	create := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "AI 原生需求", "description": "初始描述",
		"workflow_template_id": initialTemplateID, "priority": "p1",
	})
	requireStatus(t, create, http.StatusCreated)
	requirement := decodeData(t, create).(map[string]any)
	requirementID := uint(requirement["id"].(float64))
	clearRequirementPlanUnit(t, app, requirementID)
	unscheduled := app.request(t, http.MethodGet, "/api/v1/platform/requirements/"+uintString(requirementID), token, workspace.ID, nil)
	requireStatus(t, unscheduled, http.StatusOK)
	requirement = decodeData(t, unscheduled).(map[string]any)
	if requirement["version_task_new_id"] != nil {
		t.Fatalf("unscheduled requirement contract=%v", requirement)
	}

	list := app.request(t, http.MethodGet, "/api/v1/platform/requirements?version_task_new_id=none", token, workspace.ID, nil)
	requireStatus(t, list, http.StatusOK)
	if len(decodeData(t, list).([]any)) != 1 {
		t.Fatalf("pool did not return unscheduled requirement")
	}

	schedule := app.request(t, http.MethodPost, "/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule", token, workspace.ID, map[string]any{
		"version_task_new_id": unit.ID, "expected_version": 2,
	})
	requireStatus(t, schedule, http.StatusOK)
	if got := decodeData(t, schedule).(map[string]any)["version"]; got != float64(3) {
		t.Fatalf("scheduled version=%v", got)
	}
	conflict := app.request(t, http.MethodPatch, "/api/v1/platform/requirements/"+uintString(requirementID), token, workspace.ID, map[string]any{
		"expected_version": 1, "description": "过期修改",
	})
	requireStatus(t, conflict, http.StatusConflict)

	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "人工评审", "config": map[string]any{"critical_gate": true, "subject_status": "grooming"}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateCreate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "需求评审流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, templateCreate, http.StatusCreated)
	templateID := uint(decodeData(t, templateCreate).(map[string]any)["id"].(float64))
	publish := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil)
	requireStatus(t, publish, http.StatusOK)
	workflowRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "AI 原生流程需求", "description": "初始描述",
	})
	requirementID = uint(workflowRequirement["id"].(float64))
	instanceID := uint(workflowRequirement["workflow_instance_id"].(float64))
	start := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, start, http.StatusOK)
	currentNodes := decodeData(t, start).(map[string]any)["current_nodes"].([]any)
	if len(currentNodes) != 1 || currentNodes[0].(map[string]any)["id"] != "review" {
		t.Fatalf("workflow did not auto-advance start node: %v", currentNodes)
	}
	startData := decodeData(t, start).(map[string]any)
	if startData["template_name"] != "需求评审流" || startData["progress"] != float64(0) || len(startData["timeline_nodes"].([]any)) != 3 {
		t.Fatalf("workflow execution card contract=%v", startData)
	}
	var activeRequirement requirementgorm.Requirement
	if err := app.db.First(&activeRequirement, requirementID).Error; err != nil || activeRequirement.Status != "grooming" {
		t.Fatalf("workflow did not synchronize active requirement stage: %+v err=%v", activeRequirement, err)
	}
	approve := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "approve", "node_id": "review", "comment": "评审通过"})
	requireStatus(t, approve, http.StatusOK)
	if status := decodeData(t, approve).(map[string]any)["status"]; status != "completed" {
		t.Fatalf("workflow did not auto-complete end node: %v", status)
	}
	if progress := decodeData(t, approve).(map[string]any)["progress"]; progress != float64(100) {
		t.Fatalf("completed workflow progress=%v", progress)
	}

	aiCreate := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "requirement.acceptance_criteria", "subject_type": "requirement", "subject_id": requirementID,
	})
	requireStatus(t, aiCreate, http.StatusCreated)
	runID := uint(decodeData(t, aiCreate).(map[string]any)["id"].(float64))
	output, _ := json.Marshal(map[string]any{"summary": "已生成", "changes": []map[string]any{{"field": "acceptance_criteria", "value": "Given/When/Then", "reason": "可验证"}}})
	if err := app.db.Model(&aigorm.AIRun{}).Where("id = ?", runID).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&aigorm.AIArtifact{WorkspaceID: workspace.ID, RunID: runID, Kind: "structured_result", Title: "已生成", ContentJSON: string(output)}).Error; err != nil {
		t.Fatal(err)
	}
	apply := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(runID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, apply, http.StatusOK)
	var updated requirementgorm.Requirement
	if err := app.db.First(&updated, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.AcceptanceCriteria != "Given/When/Then" || updated.Status != "done" || updated.Version != 4 {
		t.Fatalf("AI changes not safely applied: %+v", updated)
	}

	dailyBrief := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "inbox.daily_brief", "subject_type": "workspace", "subject_id": 999,
	})
	requireStatus(t, dailyBrief, http.StatusCreated)
	dailyData := decodeData(t, dailyBrief).(map[string]any)
	if dailyData["subject_id"] != float64(workspace.ID) || dailyData["model"] != "qwen3.6-max-preview" {
		t.Fatalf("workspace brief did not normalize subject/model: %v", dailyData)
	}
}

func TestAIWorkflowTenantIsolation(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	other := identitygorm.Workspace{ID: 2, Name: "其他", Slug: "other", Enabled: true}
	if err := app.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	requirement := requirementgorm.Requirement{WorkspaceID: other.ID, TeamID: project.TeamID, ProjectSetID: project.ID, Title: "隔离需求", Version: 1}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	get := app.request(t, http.MethodGet, "/api/v1/platform/requirements/"+uintString(requirement.ID), token, workspace.ID, nil)
	requireStatus(t, get, http.StatusNotFound)
	run := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{"capability": "requirement.draft", "subject_type": "requirement", "subject_id": requirement.ID})
	requireStatus(t, run, http.StatusNotFound)
}

func TestApplyAIRunRollsBackWhenRunDisappearsBeforeCAS(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "AI 并发应用需求",
	})
	requirementID := uint(created["id"].(float64))
	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(map[string]any{
		"summary": "已生成",
		"changes": []map[string]any{{"field": "acceptance_criteria", "value": "Given/When/Then"}},
	})
	run := aigorm.AIRun{
		WorkspaceID: workspace.ID, Capability: "requirement.acceptance_criteria",
		SubjectType: "requirement", SubjectID: requirement.ID, SubjectVersion: requirement.Version,
		Status: "completed", Model: "test", InputJSON: `{}`, OutputJSON: string(output), RequestedByUser: 1,
	}
	if err := app.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&aigorm.AIArtifact{
		WorkspaceID: workspace.ID, RunID: run.ID, Kind: "structured_result", Title: "已生成", ContentJSON: string(output),
	}).Error; err != nil {
		t.Fatal(err)
	}

	callbackName := "test:delete_ai_run_before_apply_cas"
	if err := app.db.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "AIArtifactApplication" {
			return
		}
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).
			Where("id = ? AND workspace_id = ?", run.ID, workspace.ID).
			Delete(&aigorm.AIRun{}).Error)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Create().Remove(callbackName) })

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusConflict)
	requireModelCount(t, app.db, "rolled-back AI application", &aigorm.AIArtifactApplication{}, "run_id = ?", 0, run.ID)
	requireModelCount(t, app.db, "restored AI run", &aigorm.AIRun{}, "id = ? AND status = ?", 1, run.ID, "completed")
	var unchanged requirementgorm.Requirement
	if err := app.db.First(&unchanged, requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Version != requirement.Version || unchanged.AcceptanceCriteria != requirement.AcceptanceCriteria {
		t.Fatalf("failed AI apply committed requirement changes: before=%+v after=%+v", requirement, unchanged)
	}
}

func TestWorkflowAICanvasProposalInputAndPlanUnitInstanceLookup(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "负责人确认", "config": map[string]any{"critical_gate": true, "assignee_user_id": 999}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{"name": "版本交付流", "subject_type": "plan_unit", "definition": definition})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))

	validate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/validate-definition", token, workspace.ID, map[string]any{"definition": definition})
	requireStatus(t, validate, http.StatusOK)
	if decodeData(t, validate).(map[string]any)["valid"] != true {
		t.Fatalf("valid proposal was rejected: %v", decodeData(t, validate))
	}

	aiCreate := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "workflow.generate", "subject_type": "workflow_template", "subject_id": templateID,
		"user_prompt": "保留人工确认，并增加交付风险检查", "workflow_definition": definition,
	})
	requireStatus(t, aiCreate, http.StatusCreated)
	var run aigorm.AIRun
	if err := app.db.First(&run, uint(decodeData(t, aiCreate).(map[string]any)["id"].(float64))).Error; err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err != nil {
		t.Fatal(err)
	}
	if input["user_prompt"] != "保留人工确认，并增加交付风险检查" || input["workflow_definition"] == nil {
		t.Fatalf("workflow AI input omitted user canvas context: %v", input)
	}
	if strings.Contains(run.InputJSON, "assignee_user_id") {
		t.Fatalf("workflow AI input retained template assignee: %s", run.InputJSON)
	}

	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	started := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": unit.ProjectSetID, "title": "带流程版本", "type": "version", "workflow_template_id": templateID,
	})
	requireStatus(t, started, http.StatusCreated)
	startedData := decodeData(t, started).(map[string]any)
	instanceID := startedData["workflow_instance_id"]
	createdUnitID := uint(startedData["id"].(float64))
	listed := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances?subject_type=plan_unit&subject_id="+uintString(createdUnitID), token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	rows := decodeData(t, listed).([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != instanceID {
		t.Fatalf("plan-unit workflow lookup=%v", rows)
	}
}

func TestWorkflowCompilerRejectsUnsafeGraph(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "ai", "type": "ai_generate", "name": "自动发布", "capability": "requirement.draft", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "ai"}, {"id": "e2", "source": "ai", "target": "end", "condition": "subject.unknown >"}},
	})
	_, issues := compileWorkflowDefinition(string(raw))
	if len(issues) < 3 {
		t.Fatalf("unsafe graph issues=%v", issues)
	}
}

func TestWorkflowCompilerRejectsFakeOrUnpairedParallelGraphs(t *testing.T) {
	fake, _ := json.Marshal(map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "产品评审"},
			{"id": "plan", "type": "human_task", "name": "任务拆解"},
			{"id": "ui", "type": "condition", "name": "是否需要 UI"},
			{"id": "orphan", "type": "human_task", "name": "悬空任务"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "plan"},
			{"id": "e3", "source": "review", "target": "ui"},
			{"id": "e4", "source": "plan", "target": "end"},
			{"id": "e5", "source": "ui", "target": "end"},
		},
	})
	_, fakeIssues := compileWorkflowDefinition(string(fake))
	if !containsString(fakeIssues, "普通节点不能用多条默认路径伪装并行，请使用并行分支节点：review") || !containsString(fakeIssues, "非开始节点必须有入边：orphan") || !containsString(fakeIssues, "节点无法到达结束节点：orphan") {
		t.Fatalf("fake parallel issues=%v", fakeIssues)
	}

	unpaired, _ := json.Marshal(map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "split", "type": "parallel_split", "name": "并行启动"},
			{"id": "left", "type": "human_task", "name": "左支路"},
			{"id": "right", "type": "human_task", "name": "右支路"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "split"},
			{"id": "e2", "source": "split", "target": "left"},
			{"id": "e3", "source": "split", "target": "right"},
			{"id": "e4", "source": "left", "target": "end"},
			{"id": "e5", "source": "right", "target": "end"},
		},
	})
	_, unpairedIssues := compileWorkflowDefinition(string(unpaired))
	if !containsString(unpairedIssues, "并行分支的所有支路必须汇聚到同一个并行汇聚节点：split") {
		t.Fatalf("unpaired parallel issues=%v", unpairedIssues)
	}
}

func TestWorkflowAINodeWaitsForRunAndAllowsAuditedLowScoreOverride(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "ai", "type": "ai_check", "name": "AI 完整性", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "人工确认", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e-input-ai", "source": "input", "target": "ai", "loop": true},
			{"id": "e2", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e3", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e4", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{"name": "AI 检查流", "subject_type": "requirement", "definition": definition})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "AI 节点需求"})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	queued := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "complete", "node_id": "input"})
	requireStatus(t, queued, http.StatusOK)
	var aiRun aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", requirement.ID).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	queuedDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, queuedDetail, http.StatusOK)
	queuedExecution := workflowTimelineNode(t, decodeData(t, queuedDetail).(map[string]any), "ai")["ai_execution"].(map[string]any)
	if queuedExecution["run_id"] != float64(aiRun.ID) || queuedExecution["status"] != "queued" {
		t.Fatalf("queued AI execution was not exposed on workflow node: %v", queuedExecution)
	}
	waiting := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "complete", "node_id": "ai"})
	requireStatus(t, waiting, http.StatusConflict)
	output, _ := json.Marshal(workflowCompletenessOutput(false, requirement.Version, 42))
	if err := app.db.Model(&aiRun).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
		t.Fatal(err)
	}
	completedDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, completedDetail, http.StatusOK)
	completedExecution := workflowTimelineNode(t, decodeData(t, completedDetail).(map[string]any), "ai")["ai_execution"].(map[string]any)
	completedOutput := completedExecution["output"].(map[string]any)
	if completedExecution["status"] != "completed" || completedOutput["summary"] == "" || len(completedOutput["risks"].([]any)) != 1 {
		t.Fatalf("completed AI output was not exposed on workflow node: %v", completedExecution)
	}
	returned := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "complete", "node_id": "ai"})
	requireStatus(t, returned, http.StatusOK)
	returnedData := decodeData(t, returned).(map[string]any)
	current := returnedData["current_nodes"].([]any)
	if len(current) != 1 || current[0].(map[string]any)["id"] != "input" {
		t.Fatalf("incomplete report did not follow failure path: %v", current)
	}
	if workflowTimelineNode(t, returnedData, "ai")["ai_execution"].(map[string]any)["output"] == nil {
		t.Fatalf("AI result disappeared after workflow returned: %v", returnedData["timeline_nodes"])
	}

	reentered := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "complete", "node_id": "input"})
	requireStatus(t, reentered, http.StatusOK)
	reenteredExecution := workflowTimelineNode(t, decodeData(t, reentered).(map[string]any), "ai")["ai_execution"].(map[string]any)
	if reenteredExecution["run_id"] != float64(aiRun.ID) || reenteredExecution["status"] != "completed" {
		t.Fatalf("AI re-entry did not preserve previous result: %v", reenteredExecution)
	}
	var runCount int64
	if err := app.db.Model(&aigorm.AIRun{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", requirement.ID).Count(&runCount).Error; err != nil || runCount != 1 {
		t.Fatalf("AI re-entry queued unexpectedly: count=%d err=%v", runCount, err)
	}

	rechecked := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "recheck_ai", "node_id": "ai"})
	requireStatus(t, rechecked, http.StatusOK)
	var secondRun aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", requirement.ID).Order("id DESC").First(&secondRun).Error; err != nil {
		t.Fatal(err)
	}
	if secondRun.ID == aiRun.ID || secondRun.Status != "queued" {
		t.Fatalf("manual AI recheck did not queue a fresh run: first=%+v second=%+v", aiRun, secondRun)
	}
	recheckedExecution := workflowTimelineNode(t, decodeData(t, rechecked).(map[string]any), "ai")["ai_execution"].(map[string]any)
	if recheckedExecution["run_id"] != float64(secondRun.ID) || recheckedExecution["status"] != "queued" {
		t.Fatalf("manual AI recheck did not expose fresh run: %v", recheckedExecution)
	}
	prematureOverride := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "override_ai", "node_id": "ai"})
	requireStatus(t, prematureOverride, http.StatusConflict)
	secondOutput, _ := json.Marshal(workflowCompletenessOutput(false, secondRun.SubjectVersion, 35))
	if err := app.db.Model(&secondRun).Updates(map[string]any{"status": "completed", "output_json": string(secondOutput)}).Error; err != nil {
		t.Fatal(err)
	}
	continued := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "override_ai", "node_id": "ai", "comment": "产品负责人接受风险"})
	requireStatus(t, continued, http.StatusOK)
	continuedData := decodeData(t, continued).(map[string]any)
	continuedCurrent := continuedData["current_nodes"].([]any)
	if len(continuedCurrent) != 1 || continuedCurrent[0].(map[string]any)["id"] != "review" {
		t.Fatalf("manual low-score override did not follow success path: %v", continuedCurrent)
	}
	foundOverride := false
	for _, raw := range continuedData["transition_history"].([]any) {
		item := raw.(map[string]any)
		if item["action"] == "ai_overridden" {
			foundOverride = item["operator_type"] == "user" && item["operator_name"] == "owner" && item["comment"] == "产品负责人接受风险"
		}
	}
	if !foundOverride {
		t.Fatalf("manual override actor, comment, and action were not retained: %v", continuedData["transition_history"])
	}
}

func TestCompleteFormNodeSurfacesDisabledAIRuntime(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AI.Enabled = false
		cfg.AI.Model = "qwen3.6-max-preview"
		cfg.AI.InternalSecret = "test-internal-secret-at-least-32-characters"
	})
	user := app.createUser(t, 1, "owner", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "AI PM", OwnerUserID: &user.ID}
	unit := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "V1.1", Type: "version"}
	for _, row := range []any{&workspace, &team, &identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: "owner", Enabled: true}, &identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: "owner"}, &project, &unit} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, user)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "ai", "type": "ai_check", "name": "AI 完整性", "capability": "requirement.completeness"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai", "loop": true},
			{"id": "e3", "source": "ai", "target": "end", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "AI 关闭流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "AI 关闭需求"})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	completed := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	})
	requireStatus(t, completed, http.StatusServiceUnavailable)
	if !strings.Contains(completed.Body.String(), "AI runtime is disabled") {
		t.Fatalf("disabled AI complete response=%s", completed.Body.String())
	}
	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	current := decodeData(t, detail).(map[string]any)["current_nodes"].([]any)
	if len(current) != 1 || current[0].(map[string]any)["id"] != "input" {
		t.Fatalf("disabled AI complete moved the instance: %v", current)
	}
}

func TestWorkflowAIResultPath(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		canProceed bool
		want       string
	}{
		{name: "low score follows failure", canProceed: false, want: "failure"},
		{name: "passing score follows success", canProceed: true, want: "success"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := json.Marshal(workflowCompletenessOutput(testCase.canProceed, 1, 85))
			if err != nil {
				t.Fatal(err)
			}
			if got := workflowAIResultPath("requirement.completeness", string(output)); got != testCase.want {
				t.Fatalf("workflowAIResultPath() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestWorkflowCompletenessRejectsStaleSubjectVersionAndRechecks(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "ai", "type": "ai_check", "name": "AI 完整性", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "人工确认"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai", "loop": true},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e5", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "AI 版本一致性流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirementData := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "会变更的需求"})
	requirementID := uint(requirementData["id"].(float64))
	instanceID := uint(requirementData["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	var firstRun aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?", workspace.ID, "requirement", requirementID, "requirement.completeness").Order("id DESC").First(&firstRun).Error; err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(workflowCompletenessOutput(true, firstRun.SubjectVersion, 90))
	if err := app.db.Model(&firstRun).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).Update("version", firstRun.SubjectVersion+1).Error; err != nil {
		t.Fatal(err)
	}

	stale := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "ai",
	})
	requireWorkflowAIStale(t, stale)
	overridden := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "override_ai", "node_id": "ai", "comment": "产品负责人接受风险",
	})
	requireWorkflowAIStale(t, overridden)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "ai") {
		t.Fatalf("stale AI result advanced workflow: %+v", instance)
	}

	rechecked := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "recheck_ai", "node_id": "ai",
	})
	requireStatus(t, rechecked, http.StatusOK)
	var latest aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?", workspace.ID, "requirement", requirementID, "requirement.completeness").Order("id DESC").First(&latest).Error; err != nil {
		t.Fatal(err)
	}
	if latest.ID == firstRun.ID || latest.SubjectVersion != firstRun.SubjectVersion+1 {
		t.Fatalf("recheck run=%+v first=%+v", latest, firstRun)
	}
}

func TestWorkflowAIRecheckRejectsSkippedOrFutureNode(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "是否 AI 检查"},
			{"id": "ai", "type": "ai_check", "name": "AI 检查", "capability": "requirement.completeness"},
			{"id": "manual", "type": "human_task", "name": "人工处理"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "ai", "kind": "yes"},
			{"id": "e3", "source": "choice", "target": "manual", "kind": "no"},
			{"id": "e4", "source": "ai", "target": "end", "kind": "success"},
			{"id": "e5", "source": "ai", "target": "manual", "kind": "failure"},
			{"id": "e6", "source": "manual", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "可选 AI 流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "不选 AI"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_no", "node_id": "choice",
	}), http.StatusOK)

	recheck := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "recheck_ai", "node_id": "ai",
	})
	requireStatus(t, recheck, http.StatusConflict)
	var runs int64
	if err := app.db.Model(&aigorm.AIRun{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", uint(requirement["id"].(float64))).Count(&runs).Error; err != nil || runs != 0 {
		t.Fatalf("skipped AI run count=%d err=%v", runs, err)
	}
}

func TestRequirementWorkflowConditionDecisionAndDesignDeliverable(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "ui", "type": "condition", "name": "是否需要UI设计"},
			{"id": "design", "type": "human_task", "name": "UI/UX设计", "config": map[string]any{"require_design_image": true}},
			{"id": "develop", "type": "human_task", "name": "研发实现"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "ui"},
			{"id": "e2", "source": "ui", "target": "design", "condition": "subject.needs_ui == true"},
			{"id": "e3", "source": "ui", "target": "develop", "condition": "subject.needs_ui == false"},
			{"id": "e4", "source": "design", "target": "end"},
			{"id": "e5", "source": "develop", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "UI 条件流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)

	startInstance := func(title string) (uint, uint) {
		t.Helper()
		createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": title})
		return uint(createdRequirement["id"].(float64)), uint(createdRequirement["workflow_instance_id"].(float64))
	}

	yesRequirementID, yesInstanceID := startInstance("需要设计")
	yes := app.workflowAction(t, workspace.ID, token, yesInstanceID, map[string]any{"action": "condition_yes", "node_id": "ui"})
	requireStatus(t, yes, http.StatusOK)
	yesData := decodeData(t, yes).(map[string]any)
	if current := yesData["current_nodes"].([]any); len(current) != 1 || current[0].(map[string]any)["id"] != "design" {
		t.Fatalf("yes decision did not enter design branch: %v", current)
	}
	if status := workflowTimelineNode(t, yesData, "develop")["status"]; status != "skipped" {
		t.Fatalf("unselected non-design branch was not skipped: %v", status)
	}
	if edges := yesData["selected_edge_ids"].([]any); !containsAny(edges, "e2") || containsAny(edges, "e3") {
		t.Fatalf("yes decision display edges=%v, want only the design branch", edges)
	}
	var yesRequirement requirementgorm.Requirement
	if err := app.db.First(&yesRequirement, yesRequirementID).Error; err != nil || yesRequirement.DesignURL != "" {
		t.Fatalf("condition decision unexpectedly changed the requirement design material: %+v err=%v", yesRequirement, err)
	}
	if history := yesData["transition_history"].([]any); history[len(history)-1].(map[string]any)["action"] != "condition_yes" {
		t.Fatalf("yes decision was not retained in workflow history: %v", history)
	}
	missingImage := app.workflowAction(t, workspace.ID, token, yesInstanceID, map[string]any{"action": "complete", "node_id": "design"})
	requireStatus(t, missingImage, http.StatusConflict)
	invalidImage := app.workflowAction(t, workspace.ID, token, yesInstanceID, map[string]any{"action": "complete", "node_id": "design", "design_url": "javascript:alert(1)"})
	requireStatus(t, invalidImage, http.StatusConflict)
	designDone := app.workflowAction(t, workspace.ID, token, yesInstanceID, map[string]any{"action": "complete", "node_id": "design", "design_url": "https://www.figma.com/design/abc123/PM"})
	requireStatus(t, designDone, http.StatusOK)
	designHistory := decodeData(t, designDone).(map[string]any)["transition_history"].([]any)
	var designEntry map[string]any
	for _, rawEntry := range designHistory {
		entry := rawEntry.(map[string]any)
		if entry["node_id"] == "design" {
			designEntry = entry
		}
	}
	if designEntry == nil {
		t.Fatalf("design task transition was not retained: %v", designHistory)
	}
	materials, ok := designEntry["materials"].([]any)
	if !ok || len(materials) != 1 || materials[0].(map[string]any)["kind"] != "design_image" || materials[0].(map[string]any)["url"] != "https://www.figma.com/design/abc123/PM" {
		t.Fatalf("design task material was not attributed to its workflow node: %v", designEntry)
	}
	if err := app.db.First(&yesRequirement, yesRequirementID).Error; err != nil || yesRequirement.DesignURL != "https://www.figma.com/design/abc123/PM" || yesRequirement.Version <= 1 {
		t.Fatalf("design task material was not retained on requirement: %+v err=%v", yesRequirement, err)
	}

	noRequirementID, noInstanceID := startInstance("无需设计")
	no := app.workflowAction(t, workspace.ID, token, noInstanceID, map[string]any{"action": "condition_no", "node_id": "ui"})
	requireStatus(t, no, http.StatusOK)
	noData := decodeData(t, no).(map[string]any)
	if current := noData["current_nodes"].([]any); len(current) != 1 || current[0].(map[string]any)["id"] != "develop" {
		t.Fatalf("no decision did not enter non-design branch: %v", current)
	}
	if status := workflowTimelineNode(t, noData, "design")["status"]; status != "skipped" {
		t.Fatalf("unselected design branch was not skipped: %v", status)
	}
	if edges := noData["selected_edge_ids"].([]any); containsAny(edges, "e2") || !containsAny(edges, "e3") {
		t.Fatalf("no decision display edges=%v, want only the development branch", edges)
	}
	if noData["completed_steps"] != float64(1) || noData["total_steps"] != float64(2) || noData["progress"] != float64(50) {
		t.Fatalf("skipped design branch distorted workflow progress: %v/%v (%v)", noData["completed_steps"], noData["total_steps"], noData["progress"])
	}
	skippedPlan := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(noInstanceID)+"/node-plans", token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "design", "planned_start_at": "2026-08-10T00:00:00Z", "expected_planned_start_at": nil}},
	})
	requireStatus(t, skippedPlan, http.StatusConflict)
	skippedMaterial := app.workflowAction(t, workspace.ID, token, noInstanceID, map[string]any{
		"action": "supplement_design", "node_id": "design", "design_url": "https://www.figma.com/design/should-not-save/PM",
	})
	requireStatus(t, skippedMaterial, http.StatusConflict)
	var noRequirement requirementgorm.Requirement
	if err := app.db.First(&noRequirement, noRequirementID).Error; err != nil || noRequirement.DesignURL != "" {
		t.Fatalf("skipped design branch accepted a material: %+v err=%v", noRequirement, err)
	}
}

func workflowCompletenessOutput(canProceed bool, version uint, score int) map[string]any {
	status, verdict, risks := "partial", "blocked", []string{"验收标准缺失"}
	if canProceed {
		status, verdict, risks = "pass", "pass", []string{}
	}
	keys := []string{"background", "goal", "scope", "boundaries", "exceptions", "dependencies", "acceptance_criteria"}
	labels := []string{"背景", "目标", "范围", "边界", "异常场景", "依赖", "验收标准"}
	weights := []int{15, 15, 15, 10, 15, 10, 20}
	dimensions := make([]map[string]any, 0, len(keys))
	for index, key := range keys {
		dimensionScore := weights[index]
		if !canProceed {
			dimensionScore = 0
		}
		dimensions = append(dimensions, map[string]any{"key": key, "label": labels[index], "status": status, "score": dimensionScore, "maxScore": weights[index], "findings": []string{"检查结论"}, "evidence": []string{}, "remediation": []string{"补充事实"}})
	}
	return map[string]any{
		"summary": "完整性检查完成", "changes": []any{}, "items": []any{}, "risks": risks, "humanReviewRequired": true, "workflowProposal": nil,
		"completenessReport": map[string]any{"verdict": verdict, "score": score, "threshold": 80, "canProceed": canProceed, "checkedSubjectVersion": version, "dimensions": dimensions, "missingRequiredFields": []string{}, "blockingIssues": risks, "recommendations": []string{"补充需求信息"}},
	}
}

func requireWorkflowAIStale(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	requireStatus(t, rec, http.StatusConflict)
	var envelope map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stale response: %v body=%s", err, rec.Body.String())
	}
	if envelope["code"] != workflowAIStaleCode {
		t.Fatalf("stale code=%v want=%s body=%s", envelope["code"], workflowAIStaleCode, rec.Body.String())
	}
}

func workflowTimelineNode(t *testing.T, data map[string]any, nodeID string) map[string]any {
	t.Helper()
	for _, raw := range data["timeline_nodes"].([]any) {
		node := raw.(map[string]any)
		if node["id"] == nodeID {
			return node
		}
	}
	t.Fatalf("workflow timeline node %q not found: %v", nodeID, data["timeline_nodes"])
	return nil
}

func createRequirementWithWorkflow(
	t *testing.T,
	app *testApp,
	token string,
	workspaceID uint,
	unitID uint,
	templateID uint,
	body map[string]any,
) map[string]any {
	t.Helper()
	data := createUnassignedRequirementWithWorkflow(t, app, token, workspaceID, unitID, templateID, body)
	assignWorkflowStarterToDefinitionNodes(t, app, token, workspaceID, uint(data["workflow_instance_id"].(float64)))
	return data
}

func createUnassignedRequirementWithWorkflow(
	t *testing.T,
	app *testApp,
	token string,
	workspaceID uint,
	unitID uint,
	templateID uint,
	body map[string]any,
) map[string]any {
	t.Helper()
	payload := map[string]any{
		"version_task_new_id":  unitID,
		"workflow_template_id": templateID,
		"title":                "流程需求",
		"priority":             "p1",
	}
	for key, value := range body {
		payload[key] = value
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, workspaceID, payload)
	requireStatus(t, created, http.StatusCreated)
	data := decodeData(t, created).(map[string]any)
	if data["workflow_instance_id"] == nil {
		t.Fatalf("requirement creation omitted workflow instance: %v", data)
	}
	return data
}

func clearRequirementPlanUnit(t *testing.T, app *testApp, requirementID uint) {
	t.Helper()
	if err := app.db.Model(&requirementgorm.Requirement{}).
		Where("id = ?", requirementID).
		Update("version_task_new_id", nil).Error; err != nil {
		t.Fatal(err)
	}
}

func assignWorkflowStarterToDefinitionNodes(t *testing.T, app *testApp, token string, workspaceID, instanceID uint) {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspaceID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if instance.StartedByUserID == 0 {
		t.Fatal("workflow instance missing starter")
	}
	var version workflowgorm.WorkflowTemplateVersion
	if err := app.db.Select("definition").Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, workspaceID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		t.Fatalf("published workflow is invalid: %s", strings.Join(issues, "；"))
	}
	var runs []workflowgorm.WorkflowNodeRun
	if err := app.db.Where("workspace_id = ? AND instance_id = ?", workspaceID, instanceID).Order("id").Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	selected, _, err := workflowScheduleDisplayTopology(app.db, definition, instance, runs)
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]map[string]any, 0, len(definition.Nodes))
	for _, node := range definition.Nodes {
		if !workflowNodeTakesAssignee(node) || !selected[node.ID] {
			continue
		}
		nodes = append(nodes, map[string]any{"node_id": node.ID, "assignee_user_id": instance.StartedByUserID})
	}
	if len(nodes) == 0 {
		return
	}
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspaceID, map[string]any{"nodes": nodes},
	), http.StatusOK)
}

func newAIWorkflowFixture(t *testing.T) (*testApp, string, identitygorm.Workspace, projectgorm.ProjectSet, projectgorm.PlanUnit) {
	t.Helper()
	app := newTestAppWithProjectAccess(t, func(cfg *config.Config) {
		cfg.AI.Enabled = true
		cfg.AI.Model = "qwen3.6-max-preview"
		cfg.AI.InternalSecret = "test-internal-secret-at-least-32-characters"
	})
	user := app.createUser(t, 1, "owner", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "AI PM", OwnerUserID: &user.ID}
	unit := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "V1.1", Type: "version"}
	for _, row := range []any{&workspace, &team, &identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: "owner", Enabled: true}, &identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: "owner"}, &project, &unit} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return app, app.token(t, user), workspace, project, unit
}
