package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestProjectCRUDAndBoardContract(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	viewer := app.createUser(t, 2, "viewer", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "平台"}
	for _, row := range []any{&workspace, &team,
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: viewer.ID, Role: "member", Enabled: true},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: team.ID, UserID: admin.ID, Role: "owner"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: team.ID, UserID: viewer.ID, Role: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	adminToken := app.token(t, admin)
	projectRec := app.request(t, http.MethodPost, "/api/v1/platform/project-sets", adminToken, 1, map[string]any{
		"team_id": 10, "name": "PM 独立化", "owner_user_id": 1,
	})
	requireStatus(t, projectRec, http.StatusCreated)
	project := decodeData(t, projectRec).(map[string]any)
	projectID := uint(project["id"].(float64))
	// Owner changes are rejected on plain PATCH and go through the dedicated
	// transfer endpoint instead (removal checks + audit + consistency).
	ownerPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(projectID), adminToken, 1,
		map[string]any{"owner_user_id": viewer.ID})
	requireStatus(t, ownerPatch, http.StatusBadRequest)
	ownerReset := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(projectID), adminToken, 1,
		map[string]any{"owner_user_id": admin.ID})
	requireStatus(t, ownerReset, http.StatusBadRequest)

	planTemplateID := createPublishedPlanUnitWorkflow(t, app, adminToken, workspace.ID)
	planRec := app.request(t, http.MethodPost, "/api/v1/platform/project-new", adminToken, 1, map[string]any{
		"project_new_id": projectID, "title": "首个版本", "type": "version", "workflow_template_id": planTemplateID,
	})
	requireStatus(t, planRec, http.StatusCreated)
	planID := uint(decodeData(t, planRec).(map[string]any)["id"].(float64))

	requirementTemplateID := createPublishedRequirementWorkflow(t, app, adminToken, workspace.ID)
	requirementRec := app.request(t, http.MethodPost,
		"/api/v1/platform/product-requirements-new", adminToken, 1, map[string]any{
			"version_task_new_id": planID, "title": "完整迁移", "priority": "p1",
			"workflow_template_id": requirementTemplateID,
		})
	requireStatus(t, requirementRec, http.StatusCreated)
	requirementID := uint(decodeData(t, requirementRec).(map[string]any)["id"].(float64))

	taskRec := app.request(t, http.MethodPost, "/api/v1/platform/tasks", adminToken, 1, map[string]any{
		"requirement_id": requirementID, "kind": "task", "title": "验证看板",
	})
	requireStatus(t, taskRec, http.StatusCreated)
	task := decodeData(t, taskRec).(map[string]any)
	if task["task_key"] == "" {
		t.Fatalf("task key missing: %v", task)
	}
	if _, exists := task["story_points"]; exists {
		t.Fatalf("task contract still exposes retired story points: %v", task)
	}

	boardRec := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id="+uintString(projectID),
		adminToken, 1, nil)
	requireStatus(t, boardRec, http.StatusOK)
	board := decodeData(t, boardRec).(map[string]any)
	if len(board["version_tasks"].([]any)) != 1 || len(board["requirements"].([]any)) != 1 ||
		len(board["tasks"].([]any)) != 1 {
		t.Fatalf("board contract incomplete: %v", board)
	}
	scopeRec := app.request(t, http.MethodGet,
		"/api/v1/platform/project-requirement-scope-options", adminToken, 1, nil)
	requireStatus(t, scopeRec, http.StatusOK)
	scope := decodeData(t, scopeRec).(map[string]any)
	if len(scope["project_sets"].([]any)) != 1 || len(scope["projects"].([]any)) != 1 ||
		len(scope["requirements"].([]any)) != 1 {
		t.Fatalf("scope contract incomplete: %v", scope)
	}

	viewerToken := app.token(t, viewer)
	viewerBoard := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id="+uintString(projectID),
		viewerToken, 1, nil)
	requireStatus(t, viewerBoard, http.StatusOK)
	viewerProject := decodeData(t, viewerBoard).(map[string]any)["project"].(map[string]any)
	if _, exists := viewerProject["can_update"]; exists {
		t.Fatalf("retired project authorization field leaked: %v", viewerProject)
	}
	updated := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(projectID), viewerToken, 1,
		map[string]any{"name": "workspace member update"})
	requireStatus(t, updated, http.StatusOK)
}

func TestScopeOptionsCORSPreflight(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.CORSOrigins = []string{"https://one2all.example"}
	})
	req := httptest.NewRequest(http.MethodOptions,
		"/api/v1/platform/project-requirement-scope-options", nil)
	req.Header.Set("Origin", "https://one2all.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	app.engine.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusNoContent)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://one2all.example" {
		t.Fatalf("allow origin=%q", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-PM-Cover-Offset-X") || !strings.Contains(got, "X-PM-Cover-Offset-Y") {
		t.Fatalf("expose headers=%q", got)
	}
}

func uintString(value uint) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
