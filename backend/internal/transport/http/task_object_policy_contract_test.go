package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type taskObjectPolicyFixture struct {
	app      *testApp
	access   *identity.AccessService
	creators *task.CreatorPolicyService
	admin    identitygorm.User
	creator  identitygorm.User
	other    identitygorm.User
}

func TestTaskCreatorPolicyCoversSingleUpdateStatusAndDelete(t *testing.T) {
	fix := setupTaskObjectPolicyFixture(t)
	fix.createTask(t, 2101, fix.creator.ID)
	fix.createTask(t, 2102, fix.creator.ID)
	fix.createTask(t, 2103, fix.creator.ID)

	status := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2101/status",
		fix.app.token(t, fix.creator), 1, map[string]any{"status": "in_dev"})
	requireStatus(t, status, http.StatusOK)

	legacyStatus := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2102",
		fix.app.token(t, fix.creator), 1, map[string]any{"status": "in_dev"})
	requireStatus(t, legacyStatus, http.StatusOK)

	update := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2103",
		fix.app.token(t, fix.creator), 1, map[string]any{"title": "creator update"})
	requireStatus(t, update, http.StatusOK)

	denied := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/2103",
		fix.app.token(t, fix.other), 1, nil)
	requireStatus(t, denied, http.StatusForbidden)
	deleted := fix.app.request(t, http.MethodDelete, "/api/v1/platform/tasks/2103",
		fix.app.token(t, fix.creator), 1, nil)
	requireStatus(t, deleted, http.StatusOK)
}

func TestTaskCreatorPolicyBatchSemantics(t *testing.T) {
	t.Run("atomic status transition requires every selected task", func(t *testing.T) {
		fix := setupTaskObjectPolicyFixture(t)
		fix.createTask(t, 2201, fix.creator.ID)
		fix.createTask(t, 2202, fix.other.ID)

		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-status-update",
			fix.app.token(t, fix.creator), 1, map[string]any{
				"ids": []uint{2201, 2202}, "updates": map[string]any{"status": "in_dev"}, "atomic": true,
			})
		requireStatus(t, response, http.StatusForbidden)
		assertTaskStatus(t, fix.app, 2201, "draft")
		assertTaskStatus(t, fix.app, 2202, "draft")
	})

	t.Run("non-atomic delete reports forbidden items without mutating them", func(t *testing.T) {
		fix := setupTaskObjectPolicyFixture(t)
		fix.createTask(t, 2211, fix.creator.ID)
		fix.createTask(t, 2212, fix.other.ID)

		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete",
			fix.app.token(t, fix.creator), 1, map[string]any{"ids": []uint{2211, 2212}})
		requireStatus(t, response, http.StatusOK)
		data := decodeData(t, response).(map[string]any)
		if data["deleted"] != float64(1) || data["failed"] != float64(1) {
			t.Fatalf("batch result=%s", response.Body.String())
		}
		assertTaskExists(t, fix.app, 2211, false)
		assertTaskExists(t, fix.app, 2212, true)
	})
}

func TestTransitionActionGrantDoesNotGrantCoreTaskUpdate(t *testing.T) {
	fix := setupTaskObjectPolicyFixture(t)
	fix.createTask(t, 2301, fix.creator.ID)
	role, err := fix.access.CreateRole(context.Background(), identity.RoleInput{
		Code: "task-transitioner", Name: "Task transitioner", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceRoleActionCodes(
		context.Background(), fix.admin.ID, role.ID,
		[]string{string(accesscatalog.ActionWorkItemsTasksTransitionStatus)},
	); err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceUserRoles(context.Background(), fix.other.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	effective, err := fix.access.EffectiveActionCodes(context.Background(), fix.other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(effective) != 1 || effective[0] != string(accesscatalog.ActionWorkItemsTasksTransitionStatus) {
		t.Fatalf("effective actions=%v", effective)
	}

	transition := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2301/status",
		fix.app.token(t, fix.other), 1, map[string]any{"status": "in_dev"})
	requireStatus(t, transition, http.StatusOK)
	authorization, err := fix.access.Authorize(
		context.Background(), fix.other.ID, http.MethodPatch, "/api/v1/platform/tasks/:id",
	)
	if err != nil || authorization != identity.AuthorizationForbidden {
		t.Fatalf("core update authorization=%v err=%v", authorization, err)
	}
	creatorAllowed, err := fix.creators.AllCreatedBy(context.Background(), 1, fix.other.ID, []uint{2301})
	if err != nil || creatorAllowed {
		t.Fatalf("creator override=%t err=%v", creatorAllowed, err)
	}
	update := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2301",
		fix.app.token(t, fix.other), 1, map[string]any{"title": "not allowed"})
	requireStatus(t, update, http.StatusForbidden)
}

func TestBroadTaskUpdateGrantStillIncludesStatusTransitions(t *testing.T) {
	fix := setupTaskObjectPolicyFixture(t)
	fix.createTask(t, 2311, fix.creator.ID)
	fix.createTask(t, 2312, fix.creator.ID)
	role, err := fix.access.CreateRole(context.Background(), identity.RoleInput{
		Code: "task-editor", Name: "Task editor", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceRoleActionCodes(
		context.Background(), fix.admin.ID, role.ID,
		[]string{string(accesscatalog.ActionWorkItemsTasksUpdate)},
	); err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceUserRoles(context.Background(), fix.other.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}

	single := fix.app.request(t, http.MethodPatch, "/api/v1/platform/tasks/2311/status",
		fix.app.token(t, fix.other), 1, map[string]any{"status": "in_dev"})
	requireStatus(t, single, http.StatusOK)
	batch := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-status-update",
		fix.app.token(t, fix.other), 1, map[string]any{
			"ids": []uint{2312}, "updates": map[string]any{"status": "in_dev"}, "atomic": true,
		})
	requireStatus(t, batch, http.StatusOK)
}

func setupTaskObjectPolicyFixture(t *testing.T) taskObjectPolicyFixture {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) { cfg.RBACMode = config.RBACModeEnforce })
	admin := app.createUser(t, 1, "policy-admin", "super_admin", "")
	creator := app.createUser(t, 2, "policy-creator", "user", "")
	other := app.createUser(t, 3, "policy-other", "user", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, member := range []identitygorm.WorkspaceMember{
		{WorkspaceID: 1, UserID: creator.ID, Role: "member", Enabled: true},
		{WorkspaceID: 1, UserID: other.ID, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(&member).Error; err != nil {
			t.Fatal(err)
		}
	}
	access := identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil)
	creatorPolicy, err := task.NewCreatorPolicyService(taskgorm.NewCreatorPolicyRepository(app.db))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer: access, ProjectAccess: fakeAccessAllower{}, TaskCreatorPolicy: creatorPolicy,
	})
	inventory := NewRouteInventory()
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	policyActions := map[accesscatalog.ActionCode]struct{}{
		accesscatalog.ActionWorkItemsTasksUpdate:           {},
		accesscatalog.ActionWorkItemsTasksTransitionStatus: {},
		accesscatalog.ActionWorkItemsTasksBatchUpdate:      {},
		accesscatalog.ActionWorkItemsTasksDelete:           {},
	}
	for _, record := range accesscatalog.BootstrapActionRoutes() {
		if _, ok := policyActions[record.ActionCode]; !ok {
			continue
		}
		if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
			Code: record.InterfaceCode, ActionCode: string(record.ActionCode),
			HTTPMethod: record.HTTPMethod, RouteTemplate: record.RouteTemplate,
			Name: record.ActionName, Description: record.CopyKey, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	app.engine = engine
	return taskObjectPolicyFixture{
		app: app, access: access, creators: creatorPolicy,
		admin: admin, creator: creator, other: other,
	}
}

func (fix taskObjectPolicyFixture) createTask(t *testing.T, id, creatorID uint) {
	t.Helper()
	row := taskgorm.WorkTask{
		ID: id, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: fmt.Sprintf("POLICY-%d", id), Title: "Policy task",
		Status: "draft", Priority: "normal",
	}
	if err := fix.app.db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Create(&taskgorm.WorkTaskActivity{
		WorkspaceID: 1, WorkTaskID: id, UserID: creatorID, Action: "created",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func assertTaskStatus(t *testing.T, app *testApp, id uint, expected string) {
	t.Helper()
	var row taskgorm.WorkTask
	if err := app.db.First(&row, id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != expected {
		t.Fatalf("task %d status=%q want=%q", id, row.Status, expected)
	}
}
