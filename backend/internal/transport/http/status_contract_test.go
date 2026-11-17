package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"strings"
	"testing"
)

func TestProjectSetStatusStillUsesProjectStatusContract(t *testing.T) {
	app, token, workspace, _, project, _, _ := newBusinessStatusFixture(t)

	invalid := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID,
		map[string]any{"status": "done"})
	requireStatus(t, invalid, http.StatusBadRequest)

	if err := app.db.First(&project, project.ID).Error; err != nil || project.Status != "active" {
		t.Fatalf("invalid patch changed project status: status=%q err=%v", project.Status, err)
	}

	valid := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID,
		map[string]any{"status": "archived"})
	requireStatus(t, valid, http.StatusOK)

	invalidCreate := app.request(t, http.MethodPost, "/api/v1/platform/project-sets",
		token, workspace.ID, map[string]any{
			"team_id": project.TeamID, "name": "非法状态项目集", "status": "planning",
		})
	requireStatus(t, invalidCreate, http.StatusBadRequest)
}

func TestRequirementAndPlanUnitStatusRejectAllManualWriteEntrypoints(t *testing.T) {
	app, token, workspace, _, project, unit, requirement := newBusinessStatusFixture(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{
			name: "plan unit create", method: http.MethodPost, path: "/api/v1/platform/project-new",
			body: map[string]any{"project_new_id": project.ID, "title": "manual-plan", "status": "planning"},
		},
		{
			name: "plan unit patch", method: http.MethodPatch,
			path: "/api/v1/platform/project-new/" + uintString(unit.ID),
			body: map[string]any{"title": "manual-plan-patch", "status": "released"},
		},
		{
			name: "legacy requirement create", method: http.MethodPost,
			path: "/api/v1/platform/product-requirements-new",
			body: map[string]any{"version_task_new_id": unit.ID, "title": "manual-legacy-requirement", "status": "backlog", "priority": "p1"},
		},
		{
			name: "requirement pool create", method: http.MethodPost,
			path: "/api/v1/platform/requirements",
			body: map[string]any{"project_new_id": project.ID, "title": "manual-pool-requirement", "status": "grooming", "priority": "p1"},
		},
		{
			name: "legacy requirement patch", method: http.MethodPatch,
			path: "/api/v1/platform/product-requirements-new/" + uintString(requirement.ID),
			body: map[string]any{"title": "manual-legacy-patch", "status": "done", "expected_version": requirement.Version},
		},
		{
			name: "requirement pool patch rejects null presence", method: http.MethodPatch,
			path: "/api/v1/platform/requirements/" + uintString(requirement.ID),
			body: map[string]any{"title": "manual-pool-patch", "status": nil, "expected_version": requirement.Version},
		},
		{
			name: "requirement schedule", method: http.MethodPost,
			path: "/api/v1/platform/requirements/" + uintString(requirement.ID) + "/schedule",
			body: map[string]any{
				"version_task_new_id": unit.ID, "status": "acceptance", "expected_version": requirement.Version,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := app.request(t, test.method, test.path, token, workspace.ID, test.body)
			requireStatus(t, response, http.StatusBadRequest)
			if !strings.Contains(response.Body.String(), `"error":"`+workflowManagedStatusMessage+`"`) {
				t.Fatalf("manual status write returned ambiguous error: %s", response.Body.String())
			}
		})
	}

	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedUnit.Title != unit.Title || storedUnit.Status != unit.Status {
		t.Fatalf("rejected request changed plan unit: before=%+v after=%+v", unit, storedUnit)
	}
	var storedRequirement requirementgorm.Requirement
	if err := app.db.First(&storedRequirement, requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedRequirement.Title != requirement.Title || storedRequirement.Status != requirement.Status ||
		storedRequirement.Version != requirement.Version {
		t.Fatalf("rejected request changed requirement: before=%+v after=%+v", requirement, storedRequirement)
	}
	requireModelCount(t, app.db, "rejected plan create", &projectgorm.PlanUnit{},
		"workspace_id = ? AND title = ?", 0, workspace.ID, "manual-plan")
	requireModelCount(t, app.db, "rejected legacy requirement create", &requirementgorm.Requirement{},
		"workspace_id = ? AND title = ?", 0, workspace.ID, "manual-legacy-requirement")
	requireModelCount(t, app.db, "rejected pool requirement create", &requirementgorm.Requirement{},
		"workspace_id = ? AND title = ?", 0, workspace.ID, "manual-pool-requirement")
	requireModelCount(t, app.db, "rejected status audit", &auditgorm.AuditLog{},
		"workspace_id = ?", 0, workspace.ID)
	requireModelCount(t, app.db, "rejected status outbox", &auditgorm.OutboxEvent{},
		"workspace_id = ?", 0, workspace.ID)
	requireModelCount(t, app.db, "rejected status workflow", &workflowgorm.WorkflowInstance{},
		"workspace_id = ?", 0, workspace.ID)
}

func TestUnrelatedEditsPreserveHistoricalBusinessStatus(t *testing.T) {
	app, token, workspace, _, _, unit, requirement := newBusinessStatusFixture(t)

	planPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"title": "V1 保留历史状态"})
	requireStatus(t, planPatch, http.StatusOK)

	requirementPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(requirement.ID), token, workspace.ID,
		map[string]any{"title": "需求保留历史状态", "expected_version": requirement.Version})
	requireStatus(t, requirementPatch, http.StatusOK)

	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil || storedUnit.Status != unit.Status {
		t.Fatalf("unrelated edit changed historical plan status: status=%q err=%v", storedUnit.Status, err)
	}
	var storedRequirement requirementgorm.Requirement
	if err := app.db.First(&storedRequirement, requirement.ID).Error; err != nil ||
		storedRequirement.Status != requirement.Status {
		t.Fatalf("unrelated edit changed historical requirement status: status=%q err=%v", storedRequirement.Status, err)
	}
}

func TestAIApplyCannotWriteRequirementStatus(t *testing.T) {
	app, token, workspace, _, _, _, requirement := newBusinessStatusFixture(t)
	run := aigorm.AIRun{
		WorkspaceID: workspace.ID, Capability: "requirement.draft", SubjectType: "requirement",
		SubjectID: requirement.ID, SubjectVersion: requirement.Version, Status: "completed",
		Model: "test", InputJSON: `{}`, RequestedByUser: 1,
		OutputJSON: `{"summary":"attempted status change","changes":[{"field":"status","value":"done"}]}`,
	}
	if err := app.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusUnprocessableEntity)
	if !strings.Contains(response.Body.String(), "workflow-managed field") {
		t.Fatalf("AI status guard returned ambiguous error: %s", response.Body.String())
	}

	var storedRequirement requirementgorm.Requirement
	if err := app.db.First(&storedRequirement, requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedRequirement.Status != requirement.Status || storedRequirement.Version != requirement.Version {
		t.Fatalf("forbidden AI status apply changed requirement: before=%+v after=%+v", requirement, storedRequirement)
	}
	requireModelCount(t, app.db, "forbidden AI status application", &aigorm.AIArtifactApplication{},
		"run_id = ?", 0, run.ID)
	requireModelCount(t, app.db, "forbidden AI status audit", &auditgorm.AuditLog{},
		"workspace_id = ?", 0, workspace.ID)
	requireModelCount(t, app.db, "completed rejected AI run", &aigorm.AIRun{},
		"id = ? AND status = ?", 1, run.ID, "completed")
}

func newBusinessStatusFixture(t *testing.T) (*testApp, string, identitygorm.Workspace, identitygorm.Team, projectgorm.ProjectSet, projectgorm.PlanUnit, requirementgorm.Requirement) {
	t.Helper()
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "PM", Code: "PM", OwnerUserID: &admin.ID, Status: "active",
	}
	unit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1", Type: "version", Status: "acceptance",
	}
	requirement := requirementgorm.Requirement{
		ID: 40, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: "需求", Status: "development",
		Priority: "normal", Version: 7,
	}
	for _, row := range []any{
		&workspace, &team,
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: admin.ID, Role: "owner", Enabled: true},
		&project, &unit, &requirement,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return app, app.token(t, admin), workspace, team, project, unit, requirement
}
