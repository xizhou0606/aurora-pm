package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestBITeamMetricSettingsConcurrentInitialCreateReturnsConflict(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "metric-admin", "super_admin", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "One", Slug: "metric-concurrent", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	template := createMetricRequirementWorkflow(t, app, 1, admin.ID, 10, "review")
	body, err := json.Marshal(map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": template.ID, "approval_node_id": "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	token := app.token(t, admin)
	responses := make([]*httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range responses {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			req := httptest.NewRequest(http.MethodPut,
				"/api/v1/platform/executive-dashboard/bi-screen/team-metric-settings", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Workspace-ID", "1")
			responses[index] = httptest.NewRecorder()
			app.engine.ServeHTTP(responses[index], req)
		}(index)
	}
	close(start)
	group.Wait()

	counts := map[int]int{}
	for _, response := range responses {
		counts[response.Code]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("concurrent status counts=%v bodies=%q / %q", counts, responses[0].Body.String(), responses[1].Body.String())
	}
	for _, response := range responses {
		if response.Code == http.StatusConflict {
			assertResponseCode(t, response, "BI_TEAM_METRIC_SETTINGS_VERSION_CONFLICT")
		}
	}
}

func TestLoadBITeamMetricSettingsOnlyTreatsBusinessInvalidityAsInvalid(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "metric-admin", "super_admin", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "One", Slug: "metric-errors", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	template := createMetricRequirementWorkflow(t, app, 1, admin.ID, 10, "review")
	if err := app.db.Create(&workbenchgorm.BITeamMetricSettings{
		WorkspaceID: 1, Version: 1, RequirementWorkflowTemplateID: template.ID,
		ApprovalNodeID: "review", UpdatedByUserID: admin.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := app.db.Model(&template).Update("published_version", 2).Error; err != nil {
		t.Fatal(err)
	}
	settings, err := (&Server{db: app.db}).loadBITeamMetricSettingsState(1)
	if err != nil || !settings.Configured || settings.Valid {
		t.Fatalf("missing published version should be business-invalid, settings=%+v err=%v", settings, err)
	}

	if err := app.db.Migrator().DropTable(&workflowgorm.WorkflowTemplateVersion{}); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Server{db: app.db}).loadBITeamMetricSettingsState(1); err == nil {
		t.Fatal("workflow version database failure must propagate")
	}
	response := app.request(t, http.MethodPut,
		"/api/v1/platform/executive-dashboard/bi-screen/team-metric-settings",
		app.token(t, admin), 1, map[string]any{
			"expected_version": 1, "requirement_workflow_template_id": template.ID, "approval_node_id": "review",
		})
	requireStatus(t, response, http.StatusInternalServerError)
}

func TestBITeamMetricSettingsAreWorkspaceScopedVersionedAndAudited(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "metric-admin", "super_admin", "")
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "One", Slug: "metric-one", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "Two", Slug: "metric-two", Enabled: true},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	template := createMetricRequirementWorkflow(t, app, 1, admin.ID, 10, "review")
	createMetricRequirementWorkflow(t, app, 2, admin.ID, 20, "foreign-review")
	path := "/api/v1/platform/executive-dashboard/bi-screen/team-metric-settings"

	empty := app.request(t, http.MethodGet, path, app.token(t, admin), 1, nil)
	requireStatus(t, empty, http.StatusOK)
	emptyData := decodeData(t, empty).(map[string]any)
	if emptyData["version"] != float64(0) || emptyData["configured"] != false ||
		len(emptyData["options"].([]any)) != 1 {
		t.Fatalf("empty settings=%s", empty.Body.String())
	}
	if _, found := emptyData["can_manage"]; found {
		t.Fatalf("legacy BI authorization leaked into settings=%s", empty.Body.String())
	}
	option := emptyData["options"].([]any)[0].(map[string]any)
	if option["template_id"] != float64(template.ID) ||
		option["approval_nodes"].([]any)[0].(map[string]any)["node_id"] != "review" {
		t.Fatalf("settings options=%s", empty.Body.String())
	}

	created := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": template.ID, "approval_node_id": "review",
	})
	requireStatus(t, created, http.StatusOK)
	data := decodeData(t, created).(map[string]any)
	if data["version"] != float64(1) || data["configured"] != true || data["valid"] != true ||
		data["approval_node_name"] != "产品评审" || data["requirement_workflow_published_version"] != float64(1) {
		t.Fatalf("created settings=%s", created.Body.String())
	}

	conflict := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": template.ID, "approval_node_id": "review",
	})
	requireStatus(t, conflict, http.StatusConflict)
	assertResponseCode(t, conflict, "BI_TEAM_METRIC_SETTINGS_VERSION_CONFLICT")

	foreign := app.request(t, http.MethodGet, path, app.token(t, admin), 2, nil)
	requireStatus(t, foreign, http.StatusOK)
	foreignData := decodeData(t, foreign).(map[string]any)
	if foreignData["configured"] != false || len(foreignData["options"].([]any)) != 1 {
		t.Fatalf("settings leaked across workspace: %s", foreign.Body.String())
	}

	var settingsCount, auditCount, outboxCount int64
	app.db.Model(&workbenchgorm.BITeamMetricSettings{}).Where("workspace_id = ?", 1).Count(&settingsCount)
	app.db.Model(&auditgorm.AuditLog{}).Where("workspace_id = ? AND resource = ?", 1, "bi_team_metric_settings").Count(&auditCount)
	app.db.Model(&auditgorm.OutboxEvent{}).Where("workspace_id = ? AND aggregate_type = ?", 1, "bi_team_metric_settings").Count(&outboxCount)
	if settingsCount != 1 || auditCount != 1 || outboxCount != 1 {
		t.Fatalf("settings=%d audit=%d outbox=%d", settingsCount, auditCount, outboxCount)
	}
}

func TestBITeamMetricSettingsRequirePublishedRequirementApprovalNode(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "metric-admin", "super_admin", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "One", Slug: "metric-validation", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	template := createMetricRequirementWorkflow(t, app, 1, admin.ID, 10, "review")
	path := "/api/v1/platform/executive-dashboard/bi-screen/team-metric-settings"

	badNode := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": template.ID, "approval_node_id": "input",
	})
	requireStatus(t, badNode, http.StatusUnprocessableEntity)
	assertResponseCode(t, badNode, "BI_TEAM_METRIC_APPROVAL_NODE_INVALID")

	planTemplate := createMetricWorkflow(t, app, 1, admin.ID, 11, "plan_unit", "review", true)
	badSubject := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": planTemplate.ID, "approval_node_id": "review",
	})
	requireStatus(t, badSubject, http.StatusUnprocessableEntity)
	assertResponseCode(t, badSubject, "BI_TEAM_METRIC_WORKFLOW_INVALID")

	unpublished := createMetricWorkflow(t, app, 1, admin.ID, 12, "requirement", "review", false)
	badPublished := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "requirement_workflow_template_id": unpublished.ID, "approval_node_id": "review",
	})
	requireStatus(t, badPublished, http.StatusUnprocessableEntity)
}

func TestLoadBITeamReviewDecisionsUsesConfiguredRequirementApprovalNode(t *testing.T) {
	app := newTestApp(t, nil)
	now := time.Date(2026, time.August, 13, 9, 0, 0, 0, time.UTC)
	requirement := requirementgorm.Requirement{ID: 41, WorkspaceID: 1, Title: "评审需求"}
	instances := []workflowgorm.WorkflowInstance{
		{ID: 51, WorkspaceID: 1, TemplateID: 10, SubjectType: "requirement", SubjectID: requirement.ID,
			Status: "completed", CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedAt: now},
		{ID: 52, WorkspaceID: 1, TemplateID: 10, SubjectType: "plan_unit", SubjectID: requirement.ID,
			Status: "completed", CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedAt: now},
	}
	runs := []workflowgorm.WorkflowNodeRun{
		{ID: 61, WorkspaceID: 1, InstanceID: 51, NodeID: "review", NodeType: "approval", Status: "completed", StartedAt: now},
		{ID: 62, WorkspaceID: 1, InstanceID: 51, NodeID: "other", NodeType: "approval", Status: "completed", StartedAt: now},
		{ID: 63, WorkspaceID: 1, InstanceID: 52, NodeID: "review", NodeType: "approval", Status: "completed", StartedAt: now},
	}
	decidedLater := now.Add(time.Hour)
	approvals := []workflowgorm.WorkflowApproval{
		{ID: 71, WorkspaceID: 1, InstanceID: 51, NodeRunID: 61, Status: "rejected", DecidedAt: &now, CreatedAt: now},
		{ID: 72, WorkspaceID: 1, InstanceID: 51, NodeRunID: 61, Status: "completed", DecidedAt: &decidedLater, CreatedAt: now},
		{ID: 73, WorkspaceID: 1, InstanceID: 51, NodeRunID: 62, Status: "completed", DecidedAt: &now, CreatedAt: now},
		{ID: 74, WorkspaceID: 1, InstanceID: 52, NodeRunID: 63, Status: "completed", DecidedAt: &now, CreatedAt: now},
	}
	for _, row := range []any{&requirement, &instances, &runs, &approvals} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := (&Server{db: app.db}).loadBITeamReviewDecisions(1, []requirementgorm.Requirement{requirement}, biTeamMetricSettingsDTO{
		Configured: true, Valid: true, RequirementWorkflowTemplateID: 10, ApprovalNodeID: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Status != "rejected" || rows[1].Status != "completed" ||
		rows[0].RequirementID != requirement.ID || !rows[1].DecidedAt.Equal(decidedLater) {
		t.Fatalf("review decisions=%+v", rows)
	}
}

func createMetricRequirementWorkflow(
	t *testing.T, app *testApp, workspaceID, userID, id uint, nodeID string,
) workflowgorm.WorkflowTemplate {
	t.Helper()
	return createMetricWorkflow(t, app, workspaceID, userID, id, "requirement", nodeID, true)
}

func createMetricWorkflow(
	t *testing.T,
	app *testApp,
	workspaceID, userID, id uint,
	subjectType, nodeID string,
	published bool,
) workflowgorm.WorkflowTemplate {
	t.Helper()
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start", Name: "开始"},
			{ID: "input", Type: "human_task", Name: "需求录入"},
			{ID: nodeID, Type: "approval", Name: "产品评审"},
			{ID: "end", Type: "end", Name: "结束"},
		},
		Edges: []workflowEdge{
			{ID: "e1", Source: "start", Target: "input"},
			{ID: "e2", Source: "input", Target: nodeID},
			{ID: "e3", Source: nodeID, Target: "end", Kind: "approve"},
		},
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	publishedVersion := uint(0)
	if published {
		publishedVersion = 1
	}
	template := workflowgorm.WorkflowTemplate{
		ID: id, WorkspaceID: workspaceID, Name: "需求评审流程", SubjectType: subjectType,
		DraftDefinition: string(raw), PublishedVersion: publishedVersion,
		CreatedByUserID: userID, UpdatedByUserID: userID,
	}
	if err := app.db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if published {
		version := workflowgorm.WorkflowTemplateVersion{
			WorkspaceID: workspaceID, TemplateID: template.ID, Version: 1,
			Definition: string(raw), DefinitionHash: "metric-test", PublishedByUser: userID,
		}
		if err := app.db.Create(&version).Error; err != nil {
			t.Fatal(err)
		}
	}
	return template
}

func assertResponseCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["code"] != expected {
		t.Fatalf("code=%v want=%s body=%s", envelope["code"], expected, response.Body.String())
	}
}
