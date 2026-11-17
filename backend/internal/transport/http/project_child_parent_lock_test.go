package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type projectParentMutationProbe struct {
	mu    sync.Mutex
	fired bool
}

func installProjectParentMutation(
	t *testing.T,
	db *gorm.DB,
	triggerProjectID uint,
	mutation func(*gorm.DB) error,
) *projectParentMutationProbe {
	t.Helper()
	probe := &projectParentMutationProbe{}
	callbackName := "test:mutate_project_child_after_project_lock"
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "ProjectSet" {
			return
		}
		if _, locked := tx.Statement.Clauses["FOR"]; !locked {
			return
		}
		project, ok := tx.Statement.Dest.(*projectgorm.ProjectSet)
		if !ok || project.ID != triggerProjectID {
			return
		}
		probe.mu.Lock()
		if probe.fired {
			probe.mu.Unlock()
			return
		}
		probe.fired = true
		probe.mu.Unlock()
		tx.AddError(mutation(tx.Session(&gorm.Session{NewDB: true})))
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
	return probe
}

func (probe *projectParentMutationProbe) assertFired(t *testing.T) {
	t.Helper()
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if !probe.fired {
		t.Fatal("project parent mutation callback did not fire")
	}
}

func moveRequirementForProjectGateTest(tx *gorm.DB, requirementID, projectID, planUnitID uint) error {
	return tx.Model(&requirementgorm.Requirement{}).Where("id = ? AND workspace_id = ?", requirementID, 1).
		Updates(map[string]any{
			"project_new_id": projectID, "version_task_new_id": planUnitID, "team_id": 10,
		}).Error
}

func moveTaskForProjectGateTest(tx *gorm.DB, taskID, projectID, planUnitID, requirementID uint) error {
	return tx.Model(&taskgorm.WorkTask{}).Where("id = ? AND workspace_id = ?", taskID, 1).
		Updates(map[string]any{
			"project_set_id": projectID, "project_id": planUnitID,
			"requirement_id": requirementID, "team_id": 10,
		}).Error
}

func moveQualityBugForProjectGateTest(tx *gorm.DB, bugID, projectID, planUnitID, requirementID uint) error {
	return tx.Model(&qualitygorm.QualityBug{}).Where("id = ? AND workspace_id = ?", bugID, 1).
		Updates(map[string]any{
			"project_set_id": projectID, "project_id": planUnitID,
			"requirement_id": requirementID, "team_id": 10,
		}).Error
}

func TestProjectChildWritesRejectCurrentParentScopeMismatch(t *testing.T) {
	tests := []struct {
		name             string
		triggerProjectID uint
		model            any
		prepare          func(*testing.T, *projectGateFixture)
		mutation         func(*gorm.DB) error
		request          func(*testing.T, projectGateFixture) *httptest.ResponseRecorder
		verify           func(*testing.T, projectGateFixture)
	}{
		{
			name: "task create requirement moved", triggerProjectID: 11, model: &taskgorm.WorkTask{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 13, 14, 15) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks", fix.token, 1,
					map[string]any{"requirement_id": 13, "title": "Stale task"})
			},
		},
		{
			name: "subtask create parent moved", triggerProjectID: 11, model: &taskgorm.WorkTask{},
			mutation: func(tx *gorm.DB) error { return moveTaskForProjectGateTest(tx, 301, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks", fix.token, 1,
					map[string]any{"parent_id": 301, "title": "Stale subtask"})
			},
		},
		{
			name: "task batch requirement moved", triggerProjectID: 14, model: &taskgorm.WorkTask{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 16, 11, 12) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create", fix.token, 1,
					map[string]any{"tasks": []map[string]any{
						{"client_id": "stale-c", "requirement_id": 16, "title": "Stale C"},
						{"client_id": "stable-a", "requirement_id": 13, "title": "Stable A"},
					}})
			},
		},
		{
			name: "quality create requirement moved", triggerProjectID: 11, model: &qualitygorm.QualityBug{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 13, 14, 15) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", fix.token, 1,
					map[string]any{"requirement_id": 13, "title": "Stale bug"})
			},
		},
		{
			name: "task comment parent moved", triggerProjectID: 11, model: &taskgorm.WorkTaskComment{},
			mutation: func(tx *gorm.DB) error { return moveTaskForProjectGateTest(tx, 301, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/301/comments", fix.token, 1,
					map[string]any{"body": "Stale comment"})
			},
		},
		{
			name: "task watch parent moved", triggerProjectID: 11, model: &taskgorm.WorkTaskWatcher{},
			mutation: func(tx *gorm.DB) error { return moveTaskForProjectGateTest(tx, 301, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPut, "/api/v1/platform/tasks/301/watch", fix.token, 1,
					map[string]any{"watch": true})
			},
		},
		{
			name: "task attachment parent moved", triggerProjectID: 11, model: &taskgorm.WorkTaskAttachment{},
			prepare: func(t *testing.T, fix *projectGateFixture) {
				store := &fakeTaskObjectStore{}
				fix.app.cfg.ObjectStorage = config.ObjectStorage{Enabled: true, Endpoint: "minio.test", Bucket: "pm"}
				fix.app.rebuildEngine(t, store, nil)
				fix.store = store
			},
			mutation: func(tx *gorm.DB) error { return moveTaskForProjectGateTest(tx, 301, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return taskAttachmentUploadRequest(t, fix.app, fix.token, 301, "stale.txt", []byte("stale"))
			},
			verify: func(t *testing.T, fix projectGateFixture) {
				if fix.store == nil || len(fix.store.objects) != 0 || len(fix.store.removed) != 1 {
					t.Fatalf("attachment cleanup objects=%v removed=%v", fix.store.objects, fix.store.removed)
				}
			},
		},
		{
			name: "task patch current row moved", triggerProjectID: 14, model: &taskgorm.WorkTask{},
			mutation: func(tx *gorm.DB) error { return moveTaskForProjectGateTest(tx, 301, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/301", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "task patch target requirement moved", triggerProjectID: 14, model: &taskgorm.WorkTask{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 16, 11, 12) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/301", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "quality patch current row moved", triggerProjectID: 14, model: &qualitygorm.QualityBug{},
			mutation: func(tx *gorm.DB) error { return moveQualityBugForProjectGateTest(tx, 401, 14, 15, 16) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/quality/bugs/401", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "quality patch target requirement moved", triggerProjectID: 14, model: &qualitygorm.QualityBug{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 16, 11, 12) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/quality/bugs/401", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "requirement move current row moved", triggerProjectID: 14, model: &requirementgorm.Requirement{},
			mutation: func(tx *gorm.DB) error { return moveRequirementForProjectGateTest(tx, 13, 14, 15) },
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/product-requirements-new/13", fix.token, 1,
					map[string]any{"version_task_new_id": 15, "expected_version": 1})
			},
		},
		{
			name: "requirement move target plan moved", triggerProjectID: 14, model: &requirementgorm.Requirement{},
			mutation: func(tx *gorm.DB) error {
				return tx.Model(&projectgorm.PlanUnit{}).Where("id = ? AND workspace_id = ?", 15, 1).
					Update("project_new_id", 11).Error
			},
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/product-requirements-new/13", fix.token, 1,
					map[string]any{"version_task_new_id": 15, "expected_version": 1})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fix := setupProjectGateFixture(t)
			if test.prepare != nil {
				test.prepare(t, &fix)
			}
			before := projectGateModelCount(t, fix.app.db, test.model)
			probe := installProjectParentMutation(
				t, fix.app.db, test.triggerProjectID, test.mutation,
			)
			response := test.request(t, fix)
			requireStatus(t, response, http.StatusConflict)
			probe.assertFired(t)
			after := projectGateModelCount(t, fix.app.db, test.model)
			if after != before {
				t.Fatalf("row count after scope conflict=%d, want %d", after, before)
			}
			if test.verify != nil {
				test.verify(t, fix)
			}
		})
	}
}

func projectGateModelCount(t *testing.T, db *gorm.DB, row any) int64 {
	t.Helper()
	var count int64
	if err := db.Model(row).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
