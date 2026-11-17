package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type projectGateFixture struct {
	app            *testApp
	admin          identitygorm.User
	token          string
	planWorkflowID uint
	store          *fakeTaskObjectStore
}

func setupProjectGateFixture(t *testing.T) projectGateFixture {
	t.Helper()
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "project-gate-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []any{
		&projectgorm.ProjectSet{ID: 14, WorkspaceID: 1, TeamID: 10, Name: "Project C", Code: "C", OwnerUserID: &admin.ID},
		&projectgorm.PlanUnit{ID: 15, WorkspaceID: 1, TeamID: 10, ProjectSetID: 14, Title: "Version C"},
		&requirementgorm.Requirement{ID: 16, WorkspaceID: 1, TeamID: 10, ProjectSetID: 14, PlanUnitID: 15, Title: "Requirement C"},
		&taskgorm.WorkTask{ID: 300, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-300", Title: "Parent task",
			Status: "draft", Priority: "normal", AssigneeID: &admin.ID},
		&taskgorm.WorkTask{ID: 301, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-301", Title: "Task A",
			Status: "draft", Priority: "normal", AssigneeID: &admin.ID},
		&qualitygorm.QualityBug{ID: 401, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, BugKey: "BUG-401", Title: "Bug A", Status: "triage",
			Priority: "medium", SeverityKey: "p2", ReporterUserID: &admin.ID},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return projectGateFixture{app: app, admin: admin, token: app.token(t, admin)}
}

type projectGateProbe struct {
	mu            sync.Mutex
	expectedLocks []string
	targetSchemas map[string]struct{}
	locksByConn   map[string][]string
	writes        int
	violations    []string
}

func installProjectGateProbe(
	t *testing.T,
	db *gorm.DB,
	expectedLocks []string,
	targetSchemas ...string,
) *projectGateProbe {
	t.Helper()
	probe := &projectGateProbe{
		expectedLocks: append([]string(nil), expectedLocks...),
		targetSchemas: make(map[string]struct{}, len(targetSchemas)),
		locksByConn:   map[string][]string{},
	}
	for _, schema := range targetSchemas {
		probe.targetSchemas[schema] = struct{}{}
	}
	if err := db.Callback().Query().After("gorm:query").Register(
		"test:record_project_gate", func(tx *gorm.DB) {
			if tx.Error != nil || tx.Statement.Schema == nil {
				return
			}
			if _, locked := tx.Statement.Clauses["FOR"]; !locked {
				return
			}
			id, ok := projectGateLockedRowID(tx.Statement.Dest)
			if !ok {
				return
			}
			probe.mu.Lock()
			defer probe.mu.Unlock()
			key := projectGateConnKey(tx)
			probe.locksByConn[key] = append(
				probe.locksByConn[key], fmt.Sprintf("%s:%d", tx.Statement.Schema.Name, id),
			)
		},
	); err != nil {
		t.Fatal(err)
	}
	verifyWrite := func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		if _, targeted := probe.targetSchemas[tx.Statement.Schema.Name]; !targeted {
			return
		}
		probe.mu.Lock()
		locks := append([]string(nil), probe.locksByConn[projectGateConnKey(tx)]...)
		probe.writes++
		if !reflect.DeepEqual(locks, probe.expectedLocks) {
			probe.violations = append(probe.violations, fmt.Sprintf(
				"%s write observed project locks %v, want %v",
				tx.Statement.Schema.Name, locks, probe.expectedLocks,
			))
		}
		probe.mu.Unlock()
		if !reflect.DeepEqual(locks, probe.expectedLocks) {
			_ = tx.AddError(fmt.Errorf("project transaction gate missing before %s write", tx.Statement.Schema.Name))
		}
	}
	if err := db.Callback().Create().Before("gorm:create").Register("test:verify_project_gate_create", verifyWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register("test:verify_project_gate_update", verifyWrite); err != nil {
		t.Fatal(err)
	}
	return probe
}

func projectGateLockedRowID(destination any) (uint, bool) {
	switch row := destination.(type) {
	case *projectgorm.ProjectSet:
		return row.ID, row.ID > 0
	case *requirementgorm.Requirement:
		return row.ID, row.ID > 0
	case *taskgorm.WorkTask:
		return row.ID, row.ID > 0
	case *qualitygorm.QualityBug:
		return row.ID, row.ID > 0
	case *projectgorm.PlanUnit:
		return row.ID, row.ID > 0
	default:
		return 0, false
	}
}

func projectGateConnKey(tx *gorm.DB) string {
	return fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
}

func (probe *projectGateProbe) assert(t *testing.T) {
	t.Helper()
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.writes == 0 {
		t.Fatal("project gate probe did not observe a target write")
	}
	if len(probe.violations) > 0 {
		t.Fatal(probe.violations)
	}
}

func TestProjectDependentWritesLockParentsBeforeWriting(t *testing.T) {
	tests := []struct {
		name       string
		locks      []string
		schemas    []string
		prepare    func(*testing.T, *projectGateFixture)
		request    func(*testing.T, projectGateFixture) *httptest.ResponseRecorder
		wantStatus int
	}{
		{
			name: "task create", locks: []string{"ProjectSet:11", "Requirement:13"}, schemas: []string{"WorkTask"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks", fix.token, 1,
					map[string]any{"requirement_id": 13, "title": "Created task"})
			},
		},
		{
			name: "subtask create locks current parent", locks: []string{"ProjectSet:11", "Requirement:13", "WorkTask:301"}, schemas: []string{"WorkTask"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks", fix.token, 1,
					map[string]any{"parent_id": 301, "title": "Created subtask"})
			},
		},
		{
			name: "task batch sorts parent locks", locks: []string{"ProjectSet:11", "ProjectSet:14", "Requirement:13", "Requirement:16"}, schemas: []string{"WorkTask"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create", fix.token, 1,
					map[string]any{"tasks": []map[string]any{
						{"client_id": "project-c", "requirement_id": 16, "title": "Project C task"},
						{"client_id": "project-a", "requirement_id": 13, "title": "Project A task"},
					}})
			},
		},
		{
			name: "quality create", locks: []string{"ProjectSet:11", "Requirement:13"}, schemas: []string{"QualityBug"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", fix.token, 1,
					map[string]any{"requirement_id": 13, "title": "Created bug"})
			},
		},
		{
			name: "project news", locks: []string{"ProjectSet:11"}, schemas: []string{"ProjectNews"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/project-sets/11/news", fix.token, 1,
					map[string]any{"title": "Project update"})
			},
		},
		{
			name: "delivery label", locks: []string{"ProjectSet:11"}, schemas: []string{"DeliveryNodeLabel"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/version-tasks-new/12/delivery-node-labels", fix.token, 1,
					map[string]any{"name": "Release"})
			},
		},
		{
			name: "task comment", locks: []string{"ProjectSet:11", "WorkTask:301"}, schemas: []string{"WorkTaskComment"}, wantStatus: http.StatusCreated,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/301/comments", fix.token, 1,
					map[string]any{"body": "Comment"})
			},
		},
		{
			name: "task attachment", locks: []string{"ProjectSet:11", "WorkTask:301"}, schemas: []string{"WorkTaskAttachment"}, wantStatus: http.StatusCreated,
			prepare: func(t *testing.T, fix *projectGateFixture) {
				store := &fakeTaskObjectStore{}
				fix.app.cfg.ObjectStorage = config.ObjectStorage{Enabled: true, Endpoint: "minio.test", Bucket: "pm"}
				fix.app.rebuildEngine(t, store, nil)
			},
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return taskAttachmentUploadRequest(t, fix.app, fix.token, 301, "proof.txt", []byte("proof"))
			},
		},
		{
			name: "task watch", locks: []string{"ProjectSet:11", "WorkTask:301"}, schemas: []string{"WorkTaskWatcher"}, wantStatus: http.StatusOK,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPut, "/api/v1/platform/tasks/301/watch", fix.token, 1,
					map[string]any{"watch": true})
			},
		},
		{
			name: "task patch sorts current and parent locks", locks: []string{"ProjectSet:11", "WorkTask:300", "WorkTask:301"}, schemas: []string{"WorkTaskActivity"}, wantStatus: http.StatusOK,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/301", fix.token, 1,
					map[string]any{"kind": "subtask", "parent_id": 300})
			},
		},
		{
			name: "task move locks source and target", locks: []string{"ProjectSet:11", "ProjectSet:14", "Requirement:16", "WorkTask:301"}, schemas: []string{"WorkTaskActivity"}, wantStatus: http.StatusOK,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/301", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "quality move locks source and target", locks: []string{"ProjectSet:11", "ProjectSet:14", "Requirement:16", "QualityBug:401"}, schemas: []string{"QualityBugActivity"}, wantStatus: http.StatusOK,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/quality/bugs/401", fix.token, 1,
					map[string]any{"requirement_id": 16})
			},
		},
		{
			name: "plan patch delivery nodes", locks: []string{"ProjectSet:11"}, schemas: []string{"DeliveryNode"}, wantStatus: http.StatusOK,
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPatch, "/api/v1/platform/project-new/12", fix.token, 1,
					map[string]any{"delivery_nodes": []map[string]any{{"label_name": "Deploy", "type": "release"}}})
			},
		},
		{
			name: "plan create initial labels", locks: []string{"ProjectSet:11"}, schemas: []string{"DeliveryNodeLabel"}, wantStatus: http.StatusCreated,
			prepare: func(t *testing.T, fix *projectGateFixture) {
				fix.planWorkflowID = createPublishedPlanUnitWorkflow(t, fix.app, fix.token, 1)
			},
			request: func(t *testing.T, fix projectGateFixture) *httptest.ResponseRecorder {
				return fix.app.request(t, http.MethodPost, "/api/v1/platform/project-new", fix.token, 1,
					map[string]any{"project_new_id": 11, "title": "Gated plan", "workflow_template_id": fix.planWorkflowID,
						"delivery_node_labels": []map[string]any{{"client_key": "release", "name": "Release"}}})
			},
		},
		{
			name: "requirement move locks source and target", locks: []string{"ProjectSet:11", "ProjectSet:14", "Requirement:13", "PlanUnit:12", "PlanUnit:15"}, schemas: []string{"Requirement"}, wantStatus: http.StatusOK,
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
			probe := installProjectGateProbe(t, fix.app.db, test.locks, test.schemas...)
			response := test.request(t, fix)
			requireStatus(t, response, test.wantStatus)
			probe.assert(t)
		})
	}
}
