package httpapi

import (
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type taskDeletePolicyFixture struct {
	app     *testApp
	creator identitygorm.User
	other   identitygorm.User
	manager identitygorm.User
}

func TestTaskDeleteUsesEntryAuthorizationOnly(t *testing.T) {
	t.Run("another authenticated principal may delete a creator task", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTask(t, 1001, "A-1001", fix.creator.ID, nil)

		deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/1001",
			fix.app.token(t, fix.other), 1, nil)
		requireStatus(t, deleted, http.StatusOK)
		assertTaskExists(t, fix.app, 1001, false)
	})

	t.Run("workspace administrator may delete another creator task", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTask(t, 1002, "A-1002", fix.creator.ID, nil)

		deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/1002",
			fix.app.token(t, fix.manager), 1, nil)
		requireStatus(t, deleted, http.StatusOK)
		assertTaskExists(t, fix.app, 1002, false)
	})

	t.Run("task without creator evidence remains deletable", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTask(t, 1003, "A-1003", 0, nil)

		deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/1003",
			fix.app.token(t, fix.creator), 1, nil)
		requireStatus(t, deleted, http.StatusOK)
		assertTaskExists(t, fix.app, 1003, false)
	})

	t.Run("clone activity identifies the clone creator", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTaskWithAction(t, 1004, "A-1004", fix.creator.ID, nil, "cloned")

		deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/1004",
			fix.app.token(t, fix.creator), 1, nil)
		requireStatus(t, deleted, http.StatusOK)
		assertTaskExists(t, fix.app, 1004, false)
	})
}

func TestTaskDeleteCascadeIgnoresDescendantCreator(t *testing.T) {
	fix := setupTaskDeletePolicyFixture(t)
	fix.createTask(t, 1010, "A-1010", fix.creator.ID, nil)
	parentID := uint(1010)
	fix.createTask(t, 1011, "A-1011", fix.other.ID, &parentID)

	deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/1010?cascade=true",
		fix.app.token(t, fix.creator), 1, nil)
	requireStatus(t, deleted, http.StatusOK)
	assertTaskExists(t, fix.app, 1010, false)
	assertTaskExists(t, fix.app, 1011, false)
}

func TestTaskBatchDeleteUsesEntryAuthorizationOnly(t *testing.T) {
	t.Run("partial mode deletes every valid task", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTask(t, 1020, "A-1020", fix.creator.ID, nil)
		fix.createTask(t, 1021, "A-1021", fix.other.ID, nil)

		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete",
			fix.app.token(t, fix.creator), 1, map[string]any{"ids": []uint{1020, 1021}})
		requireStatus(t, response, http.StatusOK)
		data := decodeData(t, response).(map[string]any)
		if data["deleted"] != float64(2) || data["failed"] != float64(0) {
			t.Fatalf("batch result=%s", response.Body.String())
		}
		assertTaskExists(t, fix.app, 1020, false)
		assertTaskExists(t, fix.app, 1021, false)
	})

	t.Run("atomic mode deletes a mixed-creator batch", func(t *testing.T) {
		fix := setupTaskDeletePolicyFixture(t)
		fix.createTask(t, 1030, "A-1030", fix.creator.ID, nil)
		fix.createTask(t, 1031, "A-1031", fix.other.ID, nil)

		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete",
			fix.app.token(t, fix.creator), 1,
			map[string]any{"ids": []uint{1030, 1031}, "atomic": true})
		requireStatus(t, response, http.StatusOK)
		assertTaskExists(t, fix.app, 1030, false)
		assertTaskExists(t, fix.app, 1031, false)
	})
}

func setupTaskDeletePolicyFixture(t *testing.T) taskDeletePolicyFixture {
	t.Helper()
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "delete-super-admin", "super_admin", "")
	creator := app.createUser(t, 2, "delete-creator", "user", "")
	other := app.createUser(t, 3, "delete-other", "user", "")
	manager := app.createUser(t, 4, "delete-manager", "user", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, member := range []identitygorm.WorkspaceMember{
		{WorkspaceID: 1, UserID: creator.ID, Role: "member", Enabled: true},
		{WorkspaceID: 1, UserID: other.ID, Role: "member", Enabled: true},
		{WorkspaceID: 1, UserID: manager.ID, Role: "admin", Enabled: true},
	} {
		if err := app.db.Create(&member).Error; err != nil {
			t.Fatal(err)
		}
	}
	return taskDeletePolicyFixture{
		app: app, creator: creator, other: other, manager: manager,
	}
}

func (fix taskDeletePolicyFixture) createTask(
	t *testing.T,
	id uint,
	key string,
	creatorID uint,
	parentID *uint,
) {
	fix.createTaskWithAction(t, id, key, creatorID, parentID, "created")
}

func (fix taskDeletePolicyFixture) createTaskWithAction(
	t *testing.T,
	id uint,
	key string,
	creatorID uint,
	parentID *uint,
	action string,
) {
	t.Helper()
	task := taskgorm.WorkTask{
		ID: id, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: key, Title: key,
		Status: "draft", Priority: "normal", ParentID: parentID,
	}
	if parentID != nil {
		task.Kind = taskgorm.WorkTaskKindSubtask
	}
	if err := fix.app.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if creatorID > 0 {
		activity := taskgorm.WorkTaskActivity{
			WorkspaceID: 1, WorkTaskID: task.ID, UserID: creatorID, Action: action,
		}
		if err := fix.app.db.Create(&activity).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func assertTaskExists(t *testing.T, app *testApp, id uint, expected bool) {
	t.Helper()
	var count int64
	if err := app.db.Model(&taskgorm.WorkTask{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if (count == 1) != expected {
		t.Fatalf("task %d exists=%t want=%t", id, count == 1, expected)
	}
}
