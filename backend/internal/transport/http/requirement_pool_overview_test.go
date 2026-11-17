package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestRequirementPoolOverviewLoadsCoverStatesInOneWorkspaceScopedBatch(t *testing.T) {
	app := newTestApp(t, nil)
	rows := []requirementgorm.Requirement{
		{WorkspaceID: 1, TeamID: 1, ProjectSetID: 1, Title: "待生成", Priority: "normal", Status: "backlog", Version: 1},
		{WorkspaceID: 1, TeamID: 1, ProjectSetID: 1, Title: "已生成", Priority: "normal", Status: "backlog", Version: 1},
	}
	for index := range rows {
		if err := app.db.Create(&rows[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	for _, cover := range []requirementgorm.RequirementCover{
		{WorkspaceID: 1, RequirementID: rows[0].ID, SourceHash: "pending", Generation: 2, Status: requirementcover.StatusPending, NextAttemptAt: now},
		{WorkspaceID: 1, RequirementID: rows[1].ID, SourceHash: "ready", Generation: 3, Status: requirementcover.StatusReady, NextAttemptAt: now},
		{WorkspaceID: 2, RequirementID: rows[1].ID, SourceHash: "foreign", Generation: 9, Status: requirementcover.StatusFailed, NextAttemptAt: now},
	} {
		if err := app.db.Create(&cover).Error; err != nil {
			t.Fatal(err)
		}
	}

	overviews, err := (&Server{db: app.db}).loadRequirementPoolOverviews(1, rows, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		id         uint
		status     string
		generation uint
	}{
		{id: rows[0].ID, status: requirementcover.StatusPending, generation: 2},
		{id: rows[1].ID, status: requirementcover.StatusReady, generation: 3},
	} {
		cover := overviews[want.id].Deliverables.RequirementCover
		if cover == nil || cover.Status != want.status || cover.Generation != want.generation {
			t.Fatalf("requirement %d cover=%+v want status=%s generation=%d", want.id, cover, want.status, want.generation)
		}
	}
}

func TestRequirementPoolOverdueRequiresPersistedPlanUnitSchedule(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(
		t, app, token, workspace.ID, "需求池过期判定", scheduleSingleDefinition(),
	)
	created := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID,
		"workflow_template_id": templateID, "title": "排期边界验证",
	})
	requireStatus(t, created, http.StatusCreated)
	createdData := decodeData(t, created).(map[string]any)
	requirementID := uint(createdData["id"].(float64))
	instanceID := uint(createdData["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"delivery":{"planned_end_at":"2000-01-02T00:00:00Z"}}`).Error; err != nil {
		t.Fatal(err)
	}

	readOverdue := func(targetRequirementID uint) bool {
		t.Helper()
		listed := app.request(t, http.MethodGet,
			"/api/v1/platform/requirements?project_new_id="+uintString(project.ID),
			token, workspace.ID, nil,
		)
		requireStatus(t, listed, http.StatusOK)
		for _, row := range decodeRequirementPoolList(t, listed).Data {
			if row["id"] != float64(targetRequirementID) {
				continue
			}
			workflow, ok := row["workflow_overview"].(map[string]any)
			if !ok {
				t.Fatalf("workflow overview missing: %v", row)
			}
			nodes, ok := workflow["current_nodes"].([]any)
			if !ok || len(nodes) != 1 {
				t.Fatalf("current workflow nodes=%v, want one", workflow["current_nodes"])
			}
			overdue, ok := nodes[0].(map[string]any)["overdue"].(bool)
			if !ok {
				t.Fatalf("current workflow overdue missing: %v", nodes[0])
			}
			return overdue
		}
		t.Fatalf("requirement %d missing from pool", targetRequirementID)
		return false
	}

	projectStart := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	projectEnd := time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := app.db.Model(&projectgorm.ProjectSet{}).Where("id = ? AND workspace_id = ?", project.ID, workspace.ID).
		Updates(map[string]any{"planned_start_at": projectStart, "planned_end_at": projectEnd}).Error; err != nil {
		t.Fatal(err)
	}
	if readOverdue(requirementID) {
		t.Fatal("inherited project dates must not make an unscheduled plan unit overdue")
	}

	unitStart := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
		Updates(map[string]any{"planned_start_at": unitStart, "planned_end_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if readOverdue(requirementID) {
		t.Fatal("one-sided plan unit dates must suppress overdue")
	}

	unitEnd := time.Date(2998, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
		Update("planned_end_at", unitEnd).Error; err != nil {
		t.Fatal(err)
	}
	if !readOverdue(requirementID) {
		t.Fatal("a past active node in a fully scheduled plan unit must be overdue")
	}

	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"delivery":{"planned_start_at":"2001-01-02T00:00:00Z","planned_end_at":"2000-01-02T00:00:00Z"}}`).Error; err != nil {
		t.Fatal(err)
	}
	if readOverdue(requirementID) {
		t.Fatal("a conflicting subject schedule must suppress overdue")
	}

	unassigned := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "workflow_template_id": templateID, "title": "未分配版本需求",
	})
	requireStatus(t, unassigned, http.StatusCreated)
	unassignedData := decodeData(t, unassigned).(map[string]any)
	unassignedRequirementID := uint(unassignedData["id"].(float64))
	clearRequirementPlanUnit(t, app, unassignedRequirementID)
	unassignedInstanceID := uint(unassignedData["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", unassignedInstanceID, workspace.ID).
		Update("node_plans_json", `{"delivery":{"planned_end_at":"2000-01-02T00:00:00Z"}}`).Error; err != nil {
		t.Fatal(err)
	}
	if readOverdue(unassignedRequirementID) {
		t.Fatal("an unassigned requirement must suppress overdue even when its project has dates")
	}
}

func TestRequirementPoolNodeOverdueContract(t *testing.T) {
	businessDayStart := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	subjectStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	subjectEnd := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	past := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	plan := workflowNodePlan{PlannedEndAt: &past}
	valid := workflowSubjectSchedule{
		State:   "partial",
		Subject: workflowScheduleBoundary{PlannedStartAt: &subjectStart, PlannedEndAt: &subjectEnd},
	}
	sameBusinessDay := workflowNodePlan{PlannedEndAt: &businessDayStart}
	if requirementPoolNodeOverdue(sameBusinessDay, "active", valid, businessDayStart) {
		t.Fatal("deadline on the current business day must not be overdue")
	}

	tests := []struct {
		name     string
		status   string
		schedule workflowSubjectSchedule
		want     bool
	}{
		{name: "blank subject", status: "active", schedule: workflowSubjectSchedule{State: "partial"}},
		{name: "start only", status: "active", schedule: workflowSubjectSchedule{State: "partial", Subject: workflowScheduleBoundary{PlannedStartAt: &subjectStart}}},
		{name: "end only", status: "active", schedule: workflowSubjectSchedule{State: "partial", Subject: workflowScheduleBoundary{PlannedEndAt: &subjectEnd}}},
		{name: "inherited constraints only", status: "active", schedule: workflowSubjectSchedule{State: "partial", Constraints: &workflowScheduleBoundary{PlannedStartAt: &subjectStart, PlannedEndAt: &subjectEnd}}},
		{name: "valid active", status: "active", schedule: valid, want: true},
		{name: "completed", status: "completed", schedule: valid},
		{name: "skipped", status: "skipped", schedule: valid},
		{name: "conflict", status: "active", schedule: workflowSubjectSchedule{State: "conflict", Subject: valid.Subject}},
		{name: "reversed subject", status: "active", schedule: workflowSubjectSchedule{State: "partial", Subject: workflowScheduleBoundary{PlannedStartAt: &subjectEnd, PlannedEndAt: &subjectStart}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := requirementPoolNodeOverdue(plan, test.status, test.schedule, businessDayStart); got != test.want {
				t.Fatalf("overdue=%v want=%v", got, test.want)
			}
		})
	}
}

func TestRequirementPoolOverdueUsesTimezoneBusinessDay(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(
		t, app, token, workspace.ID, "需求池业务日", scheduleSingleDefinition(),
	)
	created := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID,
		"workflow_template_id": templateID, "title": "业务日逾期验证",
	})
	requireStatus(t, created, http.StatusCreated)
	createdData := decodeData(t, created).(map[string]any)
	requirementID := uint(createdData["id"].(float64))
	instanceID := uint(createdData["workflow_instance_id"].(float64))

	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
		Updates(map[string]any{
			"planned_start_at": time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			"planned_end_at":   time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC),
		}).Error; err != nil {
		t.Fatal(err)
	}

	readOverdue := func(timezone string) bool {
		t.Helper()
		response := requestWithTimezone(t, app, http.MethodGet,
			"/api/v1/platform/requirements?project_new_id="+uintString(project.ID),
			token, workspace.ID, timezone, nil,
		)
		requireStatus(t, response, http.StatusOK)
		for _, row := range decodeRequirementPoolList(t, response).Data {
			if row["id"] != float64(requirementID) {
				continue
			}
			nodes := row["workflow_overview"].(map[string]any)["current_nodes"].([]any)
			return nodes[0].(map[string]any)["overdue"].(bool)
		}
		t.Fatalf("requirement %d missing", requirementID)
		return false
	}
	for _, timezone := range []string{"America/Los_Angeles", "UTC", "Asia/Shanghai"} {
		t.Run(timezone, func(t *testing.T) {
			businessDate := businessDateForTimezone(timezone)
			for _, test := range []struct {
				name     string
				deadline time.Time
				want     bool
			}{
				{name: "due today", deadline: businessDate},
				{name: "due yesterday", deadline: businessDate.AddDate(0, 0, -1), want: true},
			} {
				t.Run(test.name, func(t *testing.T) {
					plans, err := json.Marshal(map[string]workflowNodePlan{
						"delivery": {PlannedEndAt: &test.deadline},
					})
					if err != nil {
						t.Fatal(err)
					}
					if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
						Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
						Update("node_plans_json", string(plans)).Error; err != nil {
						t.Fatal(err)
					}
					if got := readOverdue(timezone); got != test.want {
						t.Fatalf(
							"timezone=%s deadline=%s overdue=%v want=%v",
							timezone, test.deadline.Format(time.DateOnly), got, test.want,
						)
					}
				})
			}
		})
	}
}

func businessDateForTimezone(timezone string) time.Time {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	context.Request.Header.Set("X-Timezone", timezone)
	return requirementPoolBusinessDate(context)
}

func requestWithTimezone(
	t *testing.T,
	app *testApp,
	method, path, token string,
	workspaceID uint,
	timezone string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", fmt.Sprint(workspaceID))
	request.Header.Set("X-Timezone", timezone)
	response := httptest.NewRecorder()
	app.engine.ServeHTTP(response, request)
	return response
}

func TestRequirementPoolIncludesWorkflowAndDeliverableOverview(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审"},
			{"id": "design", "type": "human_task", "name": "UI/UX设计", "config": map[string]any{"require_design_image": true}},
			{"id": "test-cases", "type": "human_task", "name": "测试用例", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "design"},
			{"id": "e4", "source": "design", "target": "test-cases"},
			{"id": "e5", "source": "test-cases", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "需求交付流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID,
		"workflow_template_id": templateID, "title": "结算体验升级",
		"product_doc_url": "https://example.feishu.cn/wiki/checkout",
		"design_url":      "https://www.figma.com/design/checkout/PM",
	})
	requireStatus(t, createdRequirement, http.StatusCreated)
	createdData := decodeData(t, createdRequirement).(map[string]any)
	requirementID := uint(createdData["id"].(float64))
	instanceID := uint(createdData["workflow_instance_id"].(float64))
	createdInstance := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, createdInstance, http.StatusOK)
	instanceData := decodeData(t, createdInstance).(map[string]any)
	currentNodes, ok := instanceData["current_nodes"].([]any)
	if !ok || len(currentNodes) != 1 || currentNodes[0].(map[string]any)["id"] != "input" {
		t.Fatalf("created workflow current nodes=%v, want input", instanceData["current_nodes"])
	}
	artifact := workflowgorm.WorkflowNodeArtifact{
		WorkspaceID: workspace.ID, InstanceID: instanceID, NodeID: "test-cases",
		Kind: "uploaded", Name: "checkout.csv", ContentType: "text/csv", Size: 24,
		ObjectKey: "tests/requirement-pool/checkout.csv", URL: "https://materials.example.test/checkout.csv",
		UploaderUserID: 1,
	}
	if err := app.db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}

	listed := app.request(t, http.MethodGet, "/api/v1/platform/requirements?project_new_id=11", token, workspace.ID, nil)
	requireStatus(t, listed, http.StatusOK)
	page := decodeRequirementPoolList(t, listed)
	if len(page.Data) != 1 {
		t.Fatalf("requirement rows=%d, want 1", len(page.Data))
	}
	if page.Data[0]["id"] != float64(requirementID) {
		t.Fatalf("requirement id=%v, want %d", page.Data[0]["id"], requirementID)
	}
	workflow, ok := page.Data[0]["workflow_overview"].(map[string]any)
	if !ok {
		t.Fatalf("workflow overview missing: %v", page.Data[0])
	}
	current, ok := workflow["current_node_names"].([]any)
	if !ok || len(current) != 1 || current[0] != "需求录入" {
		t.Fatalf("current workflow nodes=%v", workflow["current_node_names"])
	}
	if workflow["completed_steps"] != float64(0) || workflow["total_steps"] != float64(4) {
		t.Fatalf("workflow progress=%v/%v, want 0/4", workflow["completed_steps"], workflow["total_steps"])
	}
	workflowNodes, ok := workflow["current_nodes"].([]any)
	if !ok || len(workflowNodes) != 1 {
		t.Fatalf("workflow current node evidence=%v", workflow["current_nodes"])
	}
	currentNode, ok := workflowNodes[0].(map[string]any)
	if !ok || currentNode["id"] != "input" || currentNode["name"] != "需求录入" {
		t.Fatalf("workflow current node evidence=%v", workflow["current_nodes"])
	}
	deliverables, ok := page.Data[0]["deliverable_overview"].(map[string]any)
	if !ok || deliverables["requirement_document"] != true || deliverables["design"] != true || deliverables["test_cases"] != true {
		t.Fatalf("deliverable overview=%v", page.Data[0]["deliverable_overview"])
	}
	if deliverables["requirement_document_url"] != "https://example.feishu.cn/wiki/checkout" ||
		deliverables["design_url"] != "https://www.figma.com/design/checkout/PM" {
		t.Fatalf("deliverable urls=%v", deliverables)
	}
	testCaseArtifact, ok := deliverables["test_case_artifact"].(map[string]any)
	if !ok || testCaseArtifact["id"] != float64(artifact.ID) ||
		testCaseArtifact["instance_id"] != float64(instanceID) ||
		testCaseArtifact["node_id"] != "test-cases" ||
		testCaseArtifact["kind"] != "uploaded" ||
		testCaseArtifact["name"] != "checkout.csv" ||
		testCaseArtifact["content_type"] != "text/csv" ||
		testCaseArtifact["size"] != float64(24) ||
		testCaseArtifact["ai_run_id"] != nil {
		t.Fatalf("test case artifact=%v", deliverables["test_case_artifact"])
	}
	if _, exposed := testCaseArtifact["url"]; exposed {
		t.Fatalf("requirement pool artifact must not expose object URL: %v", testCaseArtifact)
	}
	testCaseArtifacts, ok := deliverables["test_case_artifacts"].([]any)
	if !ok || len(testCaseArtifacts) != 1 || testCaseArtifacts[0].(map[string]any)["id"] != float64(artifact.ID) {
		t.Fatalf("test case artifacts=%v", deliverables["test_case_artifacts"])
	}

	projectList := app.request(t, http.MethodGet,
		"/api/v1/platform/product-requirements-new?project_new_id="+uintString(project.ID)+"&version_task_new_id="+uintString(unit.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, projectList, http.StatusOK)
	projectData := decodeData(t, projectList).([]any)
	projectRows := make([]map[string]any, 0, len(projectData))
	for _, item := range projectData {
		projectRows = append(projectRows, item.(map[string]any))
	}
	if len(projectRows) != 1 {
		t.Fatalf("project requirement rows=%d, want 1", len(projectRows))
	}
	projectWorkflow, ok := projectRows[0]["workflow_overview"].(map[string]any)
	if !ok || projectWorkflow["instance_id"] != float64(instanceID) {
		t.Fatalf("project requirement workflow overview=%v", projectRows[0]["workflow_overview"])
	}
	projectDeliverables, ok := projectRows[0]["deliverable_overview"].(map[string]any)
	if !ok || projectDeliverables["requirement_document"] != true ||
		projectDeliverables["design"] != true || projectDeliverables["test_cases"] != true {
		t.Fatalf("project requirement deliverable overview=%v", projectRows[0]["deliverable_overview"])
	}
}

func TestSelectRequirementPoolTestCaseArtifactPrefersUploadedAndFallsBackToAI(t *testing.T) {
	aiRunID := uint(91)
	definition := workflowDefinition{Nodes: []workflowNode{
		{ID: "cases", Type: "human_task", Name: "测试用例", Config: map[string]any{"test_case_artifact": true}},
		{ID: "other-cases", Type: "human_task", Name: "测试案例"},
		{ID: "design", Type: "human_task", Name: "UI/UX 设计"},
	}}
	selected := map[string]bool{"cases": true, "other-cases": false, "design": true}
	artifacts := []workflowgorm.WorkflowNodeArtifact{
		{ID: 2, NodeID: "cases", Kind: "uploaded", Name: "manual-old.csv"},
		{ID: 5, NodeID: "cases", Kind: "uploaded", Name: "manual-new.xlsx"},
		{ID: 6, NodeID: "cases", Kind: "ai_generated", Name: "generated.csv", AIRunID: &aiRunID},
		{ID: 7, NodeID: "other-cases", Kind: "uploaded", Name: "unselected.xlsx"},
		{ID: 8, NodeID: "design", Kind: "uploaded", Name: "wrong-node.xlsx"},
	}

	chosen := selectRequirementPoolTestCaseArtifact(definition, selected, artifacts)
	if chosen == nil || chosen.ID != 5 {
		t.Fatalf("chosen artifact=%v, want latest uploaded id 5", chosen)
	}
	listed := selectRequirementPoolTestCaseArtifacts(definition, selected, artifacts)
	if len(listed) != 2 || listed[0].ID != 2 || listed[1].ID != 5 {
		t.Fatalf("listed artifacts=%v, want uploaded ids 2 and 5", listed)
	}

	withoutUploaded := []workflowgorm.WorkflowNodeArtifact{artifacts[2], artifacts[3], artifacts[4]}
	chosen = selectRequirementPoolTestCaseArtifact(definition, selected, withoutUploaded)
	if chosen == nil || chosen.ID != 6 || chosen.AIRunID == nil || *chosen.AIRunID != aiRunID {
		t.Fatalf("chosen artifact=%v, want AI fallback id 6", chosen)
	}

	chosen = selectRequirementPoolTestCaseArtifact(definition, map[string]bool{"cases": false}, artifacts)
	if chosen != nil {
		t.Fatalf("chosen artifact=%v, want nil outside the selected path", chosen)
	}
	if listed = selectRequirementPoolTestCaseArtifacts(definition, map[string]bool{"cases": false}, artifacts); len(listed) != 0 {
		t.Fatalf("listed artifacts=%v, want none outside the selected path", listed)
	}
}

func TestSelectRequirementPoolDesignURLsKeepsNodeAttribution(t *testing.T) {
	definition := workflowDefinition{Nodes: []workflowNode{
		{ID: "interaction", Type: "human_task", Name: "交互设计", Config: map[string]any{"require_design_image": true}},
		{ID: "visual", Type: "human_task", Name: "视觉设计", Config: map[string]any{"require_design_image": true}},
		{ID: "skipped", Type: "human_task", Name: "未选中设计", Config: map[string]any{"require_design_image": true}},
	}}
	selected := map[string]bool{"interaction": true, "visual": true, "skipped": false}
	runs := []workflowgorm.WorkflowNodeRun{
		{ID: 1, NodeID: "interaction", OutputJSON: encodeWorkflowRunMaterial(1, "", "https://design.example.test/interaction.png")},
		{ID: 2, NodeID: "visual", OutputJSON: encodeWorkflowRunMaterial(1, "", "https://design.example.test/visual-draft.png")},
		{ID: 3, NodeID: "visual", OutputJSON: encodeWorkflowRunMaterial(1, "", "https://design.example.test/visual.png")},
		{ID: 4, NodeID: "skipped", OutputJSON: encodeWorkflowRunMaterial(1, "", "https://design.example.test/skipped.png")},
	}

	urls := selectRequirementPoolDesignURLs(definition, selected, runs, "https://design.example.test/global.png")
	if len(urls) != 3 || urls[0] != "https://design.example.test/interaction.png" ||
		urls[1] != "https://design.example.test/visual-draft.png" || urls[2] != "https://design.example.test/visual.png" {
		t.Fatalf("design urls=%v", urls)
	}

	urls = selectRequirementPoolDesignURLs(definition, selected, nil, "https://design.example.test/global.png")
	if urls != nil {
		t.Fatalf("ambiguous fallback urls=%v, want none", urls)
	}

	single := workflowDefinition{Nodes: []workflowNode{
		{ID: "design", Type: "human_task", Name: "UI/UX设计", Config: map[string]any{"require_design_image": true}},
	}}
	urls = selectRequirementPoolDesignURLs(single, map[string]bool{"design": true}, nil, "https://design.example.test/legacy.png")
	if len(urls) != 1 || urls[0] != "https://design.example.test/legacy.png" {
		t.Fatalf("legacy fallback urls=%v", urls)
	}
}
