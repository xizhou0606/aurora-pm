package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestApplyTaskPlanCreatesTreeOnlyOnce(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusOK)
	applied := decodeData(t, response).(map[string]any)
	if applied["status"] != "applied" {
		t.Fatalf("task plan status=%v", applied["status"])
	}
	assertTaskPlanTree(t, app, requirement, 4)
	requireModelCount(t, app.db, "task plan activities", &taskgorm.WorkTaskActivity{}, "workspace_id = ?", 4, workspace.ID)
	requireModelCount(t, app.db, "task plan task outbox", &auditgorm.OutboxEvent{}, "workspace_id = ? AND event_type = ?", 4, workspace.ID, "pm.work_task.created")
	requireModelCount(t, app.db, "task plan application", &aigorm.AIArtifactApplication{}, "run_id = ?", 1, run.ID)

	repeated := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, repeated, http.StatusConflict)
	assertTaskPlanTree(t, app, requirement, 4)
	requireModelCount(t, app.db, "single task plan application", &aigorm.AIArtifactApplication{}, "run_id = ?", 1, run.ID)
}

func TestApplyTaskPlanNotificationsUseDisplayNames(t *testing.T) {
	app, token, workspace, project, unit := newNotifyingAIWorkflowFixture(t)
	frontend := app.createUser(t, 33, "xizhou", "developer", "")
	if err := app.db.Model(&frontend).Update("display_name", "西洲").Error; err != nil {
		t.Fatal(err)
	}
	frontend.DisplayName = "西洲"
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: frontend.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	requirement.FrontendOwnerUserID = &frontend.ID
	requirement.BackendOwnerUserID = &frontend.ID
	if err := app.db.Model(&requirement).Updates(map[string]any{
		"frontend_owner_user_id": frontend.ID, "backend_owner_user_id": frontend.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, map[string]any{
		"assignees": map[string]any{"frontend": frontend.ID, "backend": frontend.ID},
	})
	requireStatus(t, response, http.StatusOK)

	var events []auditgorm.OutboxEvent
	if err := app.db.Where(
		"workspace_id = ? AND aggregate_type = ? AND event_type = ?",
		workspace.ID, "work_task_notification", notification.EventType(notification.SceneWorkTaskAssigned),
	).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected task assignment notifications after applying a task plan")
	}
	for _, event := range events {
		intent := decodeNotificationIntent(t, event)
		if intent.Vars["assignee"] != "西洲" || intent.Vars["operator"] != "owner" {
			t.Fatalf("task plan notification used placeholder names: %+v", intent.Vars)
		}
		if strings.Contains(intent.FallbackPlainText, "用户#") {
			t.Fatalf("task plan notification text still has user ids: %q", intent.FallbackPlainText)
		}
	}
}

func TestApplyTaskPlanNotificationsAllowUnscheduledRequirement(t *testing.T) {
	app, token, workspace, project, unit := newNotifyingAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	if err := app.db.Model(&requirement).Updates(map[string]any{
		"version_task_new_id": nil, "frontend_owner_user_id": 1, "backend_owner_user_id": 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&requirement, requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.PlanUnitID != 0 {
		t.Fatalf("requirement still scheduled to plan unit %d", requirement.PlanUnitID)
	}
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, map[string]any{
		"assignees": map[string]any{"frontend": 1, "backend": 1},
	})
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, app, requirement, 4)

	var events []auditgorm.OutboxEvent
	if err := app.db.Where(
		"workspace_id = ? AND aggregate_type = ? AND event_type = ?",
		workspace.ID, "work_task_notification", notification.EventType(notification.SceneWorkTaskAssigned),
	).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected task assignment notifications for an unscheduled requirement")
	}
	for _, event := range events {
		intent := decodeNotificationIntent(t, event)
		if intent.ProjectID != project.ID {
			t.Fatalf("unscheduled task notification project=%d want project set %d", intent.ProjectID, project.ID)
		}
	}
}

func TestApplyTaskPlanRejectsStaleDraftWithoutSideEffects(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", requirement.ID).Update("version", 4).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusConflict)
	assertTaskPlanTree(t, app, requirement, 0)
	requireModelCount(t, app.db, "stale task plan application", &aigorm.AIArtifactApplication{}, "run_id = ?", 0, run.ID)
	var current aigorm.AIRun
	if err := app.db.First(&current, run.ID).Error; err != nil || current.Status != "completed" {
		t.Fatalf("stale task plan run=%+v err=%v", current, err)
	}
}

func TestApplyLegacyTaskPlanTreatsCreatedTasksAsHistoricalReceipt(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 4)
	refs := seedLegacyTaskPlanTree(t, app, requirement)
	runRequirement := requirement
	runRequirement.Version = 3
	run := seedCompletedTaskPlanRun(t, app, runRequirement, taskPlanOutput(refs))

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, app, requirement, 4)
	requireModelCount(t, app.db, "legacy task plan does not recreate activities", &taskgorm.WorkTaskActivity{}, "workspace_id = ?", 0, workspace.ID)

	if err := app.db.Model(&taskgorm.WorkTask{}).Where("id = ?", refs[1].ID).Updates(map[string]any{
		"title": "用户调整后的任务标题", "parent_id": refs[3].ParentID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	second := seedCompletedTaskPlanRun(t, app, runRequirement, taskPlanOutput(refs))
	edited := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(second.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, edited, http.StatusOK)
	var current aigorm.AIRun
	if err := app.db.First(&current, second.ID).Error; err != nil || current.Status != "applied" {
		t.Fatalf("edited legacy run=%+v err=%v", current, err)
	}
	assertTaskPlanTree(t, app, requirement, 4)
}

func TestTaskPlanApplyRollsBackTasksWhenRunCASFails(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))
	callback := "test:remove_task_plan_run_before_apply_cas"
	if err := app.db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AIArtifactApplication" {
			tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Where("id = ?", run.ID).Delete(&aigorm.AIRun{}).Error)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Create().Remove(callback) })

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusConflict)
	assertTaskPlanTree(t, app, requirement, 0)
	requireModelCount(t, app.db, "rolled back task activities", &taskgorm.WorkTaskActivity{}, "workspace_id = ?", 0, workspace.ID)
	requireModelCount(t, app.db, "rolled back task outbox", &auditgorm.OutboxEvent{}, "event_type = ?", 0, "pm.work_task.created")
	requireModelCount(t, app.db, "rolled back task plan application", &aigorm.AIArtifactApplication{}, "run_id = ?", 0, run.ID)
	requireModelCount(t, app.db, "restored completed task plan", &aigorm.AIRun{}, "id = ? AND status = ?", 1, run.ID, "completed")
}

func TestWorkflowTaskPlanCompleteAppliesAndAdvancesAtomically(t *testing.T) {
	fixture := newWorkflowTaskPlanApplyFixture(t)

	response := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan",
	})
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 4)
	requireModelCount(t, fixture.app.db, "workflow task plan application", &aigorm.AIArtifactApplication{}, "run_id = ?", 1, fixture.run.ID)

	var currentRun aigorm.AIRun
	if err := fixture.app.db.First(&currentRun, fixture.run.ID).Error; err != nil || currentRun.Status != "applied" {
		t.Fatalf("workflow task plan run=%+v err=%v", currentRun, err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := fixture.app.db.First(&instance, fixture.instance.ID).Error; err != nil || instance.Status != "completed" || len(currentWorkflowNodes(instance.CurrentNodeIDs)) != 0 {
		t.Fatalf("workflow task plan instance=%+v err=%v", instance, err)
	}
	requireModelCount(t, fixture.app.db, "completed workflow plan token", &workflowgorm.WorkflowToken{}, "instance_id = ? AND node_id = ? AND status = ?", 1, fixture.instance.ID, "plan", "completed")
	requireModelCount(t, fixture.app.db, "completed workflow plan node run", &workflowgorm.WorkflowNodeRun{}, "instance_id = ? AND node_id = ? AND status = ?", 1, fixture.instance.ID, "plan", "completed")
	requireModelCount(t, fixture.app.db, "completed workflow plan job", &workflowgorm.WorkflowJob{}, "instance_id = ? AND status = ?", 1, fixture.instance.ID, "completed")
}

func TestWorkflowTaskPlanCompleteRollsBackWhenInstanceCASFails(t *testing.T) {
	fixture := newWorkflowTaskPlanApplyFixture(t)
	callback := "test:change_workflow_instance_before_task_plan_cas"
	if err := fixture.app.db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "AIArtifactApplication" {
			return
		}
		result := tx.Session(&gorm.Session{NewDB: true}).Model(&workflowgorm.WorkflowInstance{}).
			Where("id = ? AND workspace_id = ?", fixture.instance.ID, fixture.workspace.ID).
			UpdateColumn("current_node_ids", `["raced"]`)
		tx.AddError(result.Error)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fixture.app.db.Callback().Create().Remove(callback) })

	response := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan",
	})
	requireStatus(t, response, http.StatusConflict)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 0)
	requireModelCount(t, fixture.app.db, "rolled back workflow task activity", &taskgorm.WorkTaskActivity{}, "workspace_id = ?", 0, fixture.workspace.ID)
	requireModelCount(t, fixture.app.db, "rolled back workflow task outbox", &auditgorm.OutboxEvent{}, "workspace_id = ? AND event_type = ?", 0, fixture.workspace.ID, "pm.work_task.created")
	requireModelCount(t, fixture.app.db, "rolled back workflow application", &aigorm.AIArtifactApplication{}, "run_id = ?", 0, fixture.run.ID)

	var currentRun aigorm.AIRun
	if err := fixture.app.db.First(&currentRun, fixture.run.ID).Error; err != nil || currentRun.Status != "completed" || currentRun.OutputJSON != fixture.run.OutputJSON {
		t.Fatalf("workflow rollback run=%+v err=%v", currentRun, err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := fixture.app.db.First(&instance, fixture.instance.ID).Error; err != nil || instance.Status != fixture.instance.Status || instance.CurrentNodeIDs != fixture.instance.CurrentNodeIDs {
		t.Fatalf("workflow rollback instance=%+v want=%+v err=%v", instance, fixture.instance, err)
	}
	requireModelCount(t, fixture.app.db, "restored workflow plan token", &workflowgorm.WorkflowToken{}, "instance_id = ? AND node_id = ? AND status = ?", 1, fixture.instance.ID, "plan", "active")
	requireModelCount(t, fixture.app.db, "restored workflow plan node run", &workflowgorm.WorkflowNodeRun{}, "instance_id = ? AND node_id = ? AND status = ?", 1, fixture.instance.ID, "plan", "waiting")
	requireModelCount(t, fixture.app.db, "restored workflow plan job", &workflowgorm.WorkflowJob{}, "instance_id = ? AND status = ?", 1, fixture.instance.ID, "pending")
}

func TestStaleWorkflowTaskPlanCanBeRecheckedAndThenConfirmed(t *testing.T) {
	fixture := newWorkflowTaskPlanApplyFixture(t)
	if err := fixture.app.db.Model(&requirementgorm.Requirement{}).
		Where("id = ? AND workspace_id = ?", fixture.requirement.ID, fixture.workspace.ID).
		Update("version", gorm.Expr("version + 1")).Error; err != nil {
		t.Fatal(err)
	}

	stale := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan",
	})
	requireStatus(t, stale, http.StatusConflict)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 0)

	detail := fixture.app.request(t, http.MethodGet, strings.TrimSuffix(fixture.actionPath, "/actions"), fixture.token, fixture.workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	current := decodeData(t, detail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if !containsString(actionStrings(current["allowed_actions"]), "recheck_ai") {
		t.Fatalf("stale task plan cannot be regenerated: %v", current["allowed_actions"])
	}

	rechecked := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "recheck_ai", "node_id": "plan",
	})
	requireStatus(t, rechecked, http.StatusOK)
	bypass := fixture.app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(fixture.run.ID)+"/apply", fixture.token, fixture.workspace.ID, nil)
	requireStatus(t, bypass, http.StatusConflict)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 0)
	var replacement aigorm.AIRun
	if err := fixture.app.db.Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ? AND id <> ?",
		fixture.workspace.ID, "requirement", fixture.requirement.ID, taskplan.Capability, fixture.run.ID,
	).Order("id DESC").First(&replacement).Error; err != nil {
		t.Fatal(err)
	}
	if replacement.Status != "queued" || replacement.SubjectVersion != fixture.run.SubjectVersion+1 {
		t.Fatalf("replacement task plan run=%+v", replacement)
	}
	replacement.OutputJSON = taskPlanOutput(nil)
	now := time.Now().UTC()
	if err := fixture.app.db.Model(&replacement).Updates(map[string]any{
		"status": "completed", "output_json": replacement.OutputJSON, "completed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.db.Create(&aigorm.AIArtifact{
		WorkspaceID: fixture.workspace.ID, RunID: replacement.ID, Kind: "structured_result",
		Title: "已重新拆解任务", ContentJSON: replacement.OutputJSON,
	}).Error; err != nil {
		t.Fatal(err)
	}

	confirmed := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan",
	})
	requireStatus(t, confirmed, http.StatusOK)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 4)
}

type workflowTaskPlanApplyFixture struct {
	app         *testApp
	token       string
	workspace   identitygorm.Workspace
	requirement requirementgorm.Requirement
	instance    workflowgorm.WorkflowInstance
	run         aigorm.AIRun
	actionPath  string
}

func newWorkflowTaskPlanApplyFixture(t *testing.T) workflowTaskPlanApplyFixture {
	t.Helper()
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "plan", "type": "ai_generate", "name": "任务拆解", "capability": taskplan.Capability},
			{"id": "repair", "type": "human_task", "name": "人工补充"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "plan"},
			{"id": "e2", "source": "plan", "target": "end", "kind": "success"},
			{"id": "e3", "source": "plan", "target": "repair", "kind": "failure"},
			{"id": "e4", "source": "repair", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "任务拆解确认", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "结算优化", "frontend_owner_user_id": 1, "backend_owner_user_id": 1,
	})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))

	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	var run aigorm.AIRun
	if err := app.db.Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?",
		workspace.ID, "requirement", requirement.ID, taskplan.Capability,
	).Order("id DESC").First(&run).Error; err != nil {
		t.Fatal(err)
	}
	run.OutputJSON = taskPlanOutput(nil)
	now := time.Now().UTC()
	if err := app.db.Model(&run).Updates(map[string]any{
		"status": "completed", "output_json": run.OutputJSON, "completed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&aigorm.AIArtifact{
		WorkspaceID: workspace.ID, RunID: run.ID, Kind: "structured_result",
		Title: "已拆解任务", ContentJSON: run.OutputJSON,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	return workflowTaskPlanApplyFixture{
		app: app, token: token, workspace: workspace, requirement: requirement, instance: instance, run: run,
		actionPath: "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions",
	}
}

func seedTaskPlanRequirement(t *testing.T, app *testApp, workspace identitygorm.Workspace, project projectgorm.ProjectSet, unit projectgorm.PlanUnit, version uint) requirementgorm.Requirement {
	t.Helper()
	requirement := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Version: version, Title: "结算优化", Priority: "high",
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	return requirement
}

func seedCompletedTaskPlanRun(t *testing.T, app *testApp, requirement requirementgorm.Requirement, output string) aigorm.AIRun {
	t.Helper()
	run := aigorm.AIRun{
		WorkspaceID: requirement.WorkspaceID, Capability: taskplan.Capability,
		SubjectType: "requirement", SubjectID: requirement.ID, SubjectVersion: requirement.Version,
		Status: "completed", Model: "test", InputJSON: `{}`, OutputJSON: output, RequestedByUser: 1,
	}
	if err := app.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&aigorm.AIArtifact{
		WorkspaceID: requirement.WorkspaceID, RunID: run.ID, Kind: "structured_result",
		Title: "已拆解任务", ContentJSON: output,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return run
}

func taskPlanOutput(refs []taskplan.CreatedTaskRef) string {
	output := map[string]any{
		"summary": "已拆解前后端任务", "changes": []any{},
		"taskPlan": map[string]any{"chains": []any{
			map[string]any{"lane": "frontend", "summary": "交付需求页面", "tasks": []any{map[string]any{"title": "实现需求页面", "description": "完成交互和接口接入", "acceptanceCriteria": []string{"页面可操作"}}}},
			map[string]any{"lane": "backend", "summary": "交付需求接口", "tasks": []any{map[string]any{"title": "实现需求接口", "description": "完成租户隔离和持久化", "acceptanceCriteria": []string{"接口测试通过"}}}},
		}},
	}
	if refs != nil {
		output["createdTasks"] = refs
	}
	raw, _ := json.Marshal(output)
	return string(raw)
}

func seedLegacyTaskPlanTree(t *testing.T, app *testApp, requirement requirementgorm.Requirement) []taskplan.CreatedTaskRef {
	t.Helper()
	tasks := []taskgorm.WorkTask{
		{ID: 101, WorkspaceID: requirement.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID, ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID, Kind: "task", TaskKey: "LEGACY-1", Title: "前端研发 · 结算优化", Status: "draft", Priority: "high"},
		{ID: 103, WorkspaceID: requirement.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID, ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID, Kind: "task", TaskKey: "LEGACY-3", Title: "后端研发 · 结算优化", Status: "draft", Priority: "high"},
	}
	frontParent, backParent := uint(101), uint(103)
	tasks = append(tasks,
		taskgorm.WorkTask{ID: 102, WorkspaceID: requirement.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID, ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID, Kind: "subtask", ParentID: &frontParent, TaskKey: "LEGACY-2", Title: "实现需求页面", Status: "draft", Priority: "high"},
		taskgorm.WorkTask{ID: 104, WorkspaceID: requirement.WorkspaceID, TeamID: requirement.TeamID, ProjectSetID: requirement.ProjectSetID, ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID, Kind: "subtask", ParentID: &backParent, TaskKey: "LEGACY-4", Title: "实现需求接口", Status: "draft", Priority: "high"},
	)
	for index := range tasks {
		if err := app.db.Create(&tasks[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	return []taskplan.CreatedTaskRef{
		{ID: 101, TaskKey: "LEGACY-1", Title: tasks[0].Title, Lane: "frontend", Kind: "task"},
		{ID: 102, TaskKey: "LEGACY-2", Title: tasks[2].Title, Lane: "frontend", Kind: "subtask", ParentID: &frontParent},
		{ID: 103, TaskKey: "LEGACY-3", Title: tasks[1].Title, Lane: "backend", Kind: "task"},
		{ID: 104, TaskKey: "LEGACY-4", Title: tasks[3].Title, Lane: "backend", Kind: "subtask", ParentID: &backParent},
	}
}

func assertTaskPlanTree(t *testing.T, app *testApp, requirement requirementgorm.Requirement, want int64) {
	t.Helper()
	var count int64
	if err := app.db.Model(&taskgorm.WorkTask{}).Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).Count(&count).Error; err != nil || count != want {
		t.Fatalf("task plan tree count=%d want=%d err=%v", count, want, err)
	}
	if want > 0 {
		var rows []taskgorm.WorkTask
		if err := app.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ProjectSetID != requirement.ProjectSetID || row.ProjectID != requirement.PlanUnitID || row.TeamID != requirement.TeamID || strings.TrimSpace(row.TaskKey) == "" {
				t.Fatalf("task plan row escaped requirement scope: %+v", row)
			}
		}
	}
}

func TestCompleteTaskPlanNodeCanSkipApplyWhenTasksExist(t *testing.T) {
	fixture := newWorkflowTaskPlanApplyFixture(t)
	seedLegacyTaskPlanTree(t, fixture.app, fixture.requirement)

	response := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan", "skip_task_plan": true,
		"comment": "需求下已有任务，不使用 AI 方案，直接完成本节点。",
	})
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 4)

	var instance workflowgorm.WorkflowInstance
	if err := fixture.app.db.First(&instance, fixture.instance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "plan") {
		t.Fatalf("skipped task plan node is still current: %v", instance.CurrentNodeIDs)
	}
	var appliedRuns int64
	if err := fixture.app.db.Model(&aigorm.AIArtifactApplication{}).
		Where("workspace_id = ? AND run_id = ?", fixture.workspace.ID, fixture.run.ID).
		Count(&appliedRuns).Error; err != nil || appliedRuns != 0 {
		t.Fatalf("skipped task plan was applied: runs=%d err=%v", appliedRuns, err)
	}
}

func TestCompleteTaskPlanNodeSkipRequiresExistingTasks(t *testing.T) {
	fixture := newWorkflowTaskPlanApplyFixture(t)

	response := fixture.app.request(t, http.MethodPost, fixture.actionPath, fixture.token, fixture.workspace.ID, map[string]any{
		"action": "complete", "node_id": "plan", "skip_task_plan": true,
	})
	requireStatus(t, response, http.StatusConflict)
	assertTaskPlanTree(t, fixture.app, fixture.requirement, 0)

	var instance workflowgorm.WorkflowInstance
	if err := fixture.app.db.First(&instance, fixture.instance.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "plan") {
		t.Fatalf("failed skip advanced the workflow: %v", instance.CurrentNodeIDs)
	}
}
