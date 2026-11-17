package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

// TestProjectAccessSubresourceDenial verifies that sub-resource detail/mutation
// routes return 404 when the user has no project access (deny-all checker).
func TestProjectAccessSubresourceDenial(t *testing.T) {
	t.Run("task detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		task := fx.createTask(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/tasks/"+fmt.Sprint(task.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("bug detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		bug := fx.createBug(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/quality/bugs/"+fmt.Sprint(bug.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("plan unit detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		unit := fx.createPlanUnit(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/plan-units/"+fmt.Sprint(unit.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("requirement detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		req := fx.createRequirement(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/project-new/"+fmt.Sprint(req.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("ai run detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		run := fx.createAIRun(t)
		if run.ID == 0 {
			t.Skip("ai_runs table not available")
		}
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/ai/runs/"+fmt.Sprint(run.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("workflow instance detail 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		inst := fx.createWorkflowInstance(t)
		if inst.ID == 0 {
			t.Skip("workflow_instances table not available")
		}
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+fmt.Sprint(inst.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("board 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/board?project_set_id="+fmt.Sprint(fx.project.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("project news 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/project-sets/"+fmt.Sprint(fx.project.ID)+"/news", fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("clone task 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		task := fx.createTask(t)
		resp := fx.request(t, http.MethodPost, "/api/v1/platform/tasks/"+fmt.Sprint(task.ID)+"/clone", fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("batch move requirements 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		req := fx.createRequirement(t)
		unit := fx.createPlanUnit(t)
		resp := fx.request(t, http.MethodPost, "/api/v1/platform/project-new/batch-move", fx.tok, fx.ws.ID, map[string]any{
			"requirement_ids":     []uint{req.ID},
			"target_plan_unit_id": unit.ID,
		})
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("batch delete tasks 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		task := fx.createTask(t)
		resp := fx.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete", fx.tok, fx.ws.ID, map[string]any{"ids": []uint{task.ID}})
		requireStatus(t, resp, http.StatusNotFound)
	})
	t.Run("bi screen 404", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/executive-dashboard/bi-screen?project_set_ids="+fmt.Sprint(fx.project.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusNotFound)
	})
}

// TestProjectAccessSubresourceFiltered verifies that list/aggregate routes
// return 200 with empty data for non-members (SQL-level filtering).
func TestProjectAccessSubresourceFiltered(t *testing.T) {
	t.Run("workspace tasks empty", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/tasks?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
		items := decodeData(t, resp).(map[string]any)["items"].([]any)
		if len(items) != 0 {
			t.Fatalf("expected empty items, got %d", len(items))
		}
	})
	t.Run("workspace task stats ok", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/tasks/stats?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
	})
	t.Run("bug list empty", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/quality/bugs?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
		items := decodeData(t, resp).(map[string]any)["items"].([]any)
		if len(items) != 0 {
			t.Fatalf("expected empty items, got %d", len(items))
		}
	})
	t.Run("bug dashboard ok", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/quality/bugs/dashboard?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
	})
	t.Run("ai run list empty", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/ai/runs?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
	})
	t.Run("workbench todo summary ok", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/workbench/todos/summary?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
	})
	t.Run("workbench todo feed ok", func(t *testing.T) {
		t.Parallel()
		fx := newAccessDenialFixture(t)
		resp := fx.request(t, http.MethodGet, "/api/v1/platform/workbench/todos/feed?workspace_id="+fmt.Sprint(fx.ws.ID), fx.tok, fx.ws.ID, nil)
		requireStatus(t, resp, http.StatusOK)
	})
}

// accessDenialFixture creates a deny-all test app with a project, user, and
// common sub-resources already seeded.
type accessDenialFixture struct {
	*testApp
	tok     string
	ws      accessDenialWS
	project projectgorm.ProjectSet
}

type accessDenialWS struct {
	ID   uint
	Name string
	Slug string
}

func newAccessDenialFixture(t *testing.T) *accessDenialFixture {
	t.Helper()
	app := newTestAppDenyAll(t, nil)
	user := app.createUser(t, 9900, "deny-user", "user", "")
	wsRow := identitygorm.Workspace{ID: 9900, Name: "DenyWS", Slug: "denyws", Enabled: true}
	if err := app.db.Create(&wsRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: wsRow.ID, UserID: user.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ws := accessDenialWS{ID: wsRow.ID, Name: wsRow.Name, Slug: wsRow.Slug}
	proj := projectgorm.ProjectSet{
		WorkspaceID: ws.ID, TeamID: 9900, Name: "Denied Project",
		OwnerUserID: &user.ID, Status: "active",
	}
	if err := app.db.Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	return &accessDenialFixture{
		testApp: app,
		tok:     app.token(t, user),
		ws:      ws,
		project: proj,
	}
}

func (f *accessDenialFixture) createTask(t *testing.T) taskgorm.WorkTask {
	t.Helper()
	req := f.createRequirement(t)
	task := taskgorm.WorkTask{
		WorkspaceID: f.ws.ID, TeamID: f.project.TeamID, ProjectSetID: f.project.ID,
		ProjectID: req.PlanUnitID, RequirementID: req.ID, Kind: "task",
		Title: "deny-test-task", Status: "todo", Priority: "medium", TaskKey: "TASK-1",
	}
	if err := f.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func (f *accessDenialFixture) createBug(t *testing.T) accessDenialBug {
	t.Helper()
	req := f.createRequirement(t)
	bug := accessDenialBug{}
	now := time.Now()
	if err := f.db.Exec(
		"INSERT INTO quality_bugs (workspace_id, team_id, project_set_id, project_id, requirement_id, bug_key, title, status, priority, severity_key, bug_type, found_environment, status_changed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		f.ws.ID, f.project.TeamID, f.project.ID, req.PlanUnitID, req.ID, "BUG-1", "deny-test-bug", "open", "medium", "medium", "defect", "test", now, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	f.db.Raw("SELECT id FROM quality_bugs WHERE workspace_id = ? AND title = ?", f.ws.ID, "deny-test-bug").Scan(&bug)
	return bug
}

type accessDenialBug struct{ ID uint }

func (f *accessDenialFixture) createPlanUnit(t *testing.T) projectgorm.PlanUnit {
	t.Helper()
	unit := projectgorm.PlanUnit{
		WorkspaceID: f.ws.ID, TeamID: f.project.TeamID, ProjectSetID: f.project.ID,
		Title: "deny-test-unit", Status: "active",
	}
	if err := f.db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	return unit
}

func (f *accessDenialFixture) createRequirement(t *testing.T) requirementgorm.Requirement {
	t.Helper()
	unit := f.createPlanUnit(t)
	req := requirementgorm.Requirement{
		WorkspaceID: f.ws.ID, TeamID: f.project.TeamID, ProjectSetID: f.project.ID,
		PlanUnitID: unit.ID, Title: "deny-test-req", Status: "draft",
	}
	if err := f.db.Create(&req).Error; err != nil {
		t.Fatal(err)
	}
	return req
}

func (f *accessDenialFixture) createWorkflowInstance(t *testing.T) accessDenialWFInstance {
	t.Helper()
	req := f.createRequirement(t)
	inst := accessDenialWFInstance{}
	now := time.Now()
	if err := f.db.Exec(
		"INSERT INTO workflow_instances (workspace_id, template_id, template_version_id, subject_type, subject_id, current_node_ids, status, started_by_user_id, started_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		f.ws.ID, 1, 1, "requirement", req.ID, "[]", "running", 0, now,
	).Error; err != nil {
		if !tableMissingErr(err) {
			t.Fatal(err)
		}
		return inst
	}
	f.db.Raw("SELECT id FROM workflow_instances WHERE workspace_id = ? AND subject_id = ?", f.ws.ID, req.ID).Scan(&inst)
	return inst
}

type accessDenialWFInstance struct{ ID uint }

func (f *accessDenialFixture) createAIRun(t *testing.T) accessDenialAIRun {
	t.Helper()
	req := f.createRequirement(t)
	run := accessDenialAIRun{}
	if err := f.db.Exec(
		"INSERT INTO ai_runs (workspace_id, capability, subject_type, subject_id, subject_version, status, model, input_json, requested_by_user, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		f.ws.ID, "qa.test_cases", "requirement", req.ID, 1, "pending", "test-model", "{}", 0, time.Now(), time.Now(),
	).Error; err != nil {
		if !tableMissingErr(err) {
			t.Fatal(err)
		}
		return run
	}
	f.db.Raw("SELECT id FROM ai_runs WHERE workspace_id = ? AND subject_id = ?", f.ws.ID, req.ID).Scan(&run)
	return run
}

type accessDenialAIRun struct{ ID uint }

func tableMissingErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}
