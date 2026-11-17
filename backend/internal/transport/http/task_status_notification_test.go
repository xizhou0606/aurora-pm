package httpapi

import (
	"errors"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
	"net/http"
	"testing"
)

func TestTaskStatusAndCommentPersistWithExternalDisabled(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "status-admin", "super_admin", "")
	owner := app.createUser(t, 2, "status-owner", "developer", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []any{&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: owner.ID, Role: "member", Enabled: true}, &identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: owner.ID, Role: "member"}} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)
	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1, map[string]any{"requirement_id": 13, "title": "状态通知任务", "assignee_id": owner.ID})
	requireStatus(t, created, http.StatusCreated)
	id := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	path := "/api/v1/platform/tasks/" + uintString(id)
	for i := 0; i < 2; i++ {
		requireStatus(t, app.request(t, http.MethodPatch, path, token, 1, map[string]any{"status": "in_dev"}), http.StatusOK)
	}
	events := notificationEventsForTask(t, app, id)
	if len(events) != 2 {
		t.Fatalf("assignment+single transition expected, got %d", len(events))
	}
	intent := decodeNotificationIntent(t, events[1])
	if intent.Scene != notification.SceneWorkTaskStatusChanged || !intent.SkipExternal || len(intent.RecipientUserIDs) != 1 || intent.RecipientUserIDs[0] != owner.ID || intent.Vars["fromStatusKey"] != "draft" || intent.Vars["toStatusKey"] != "in_dev" {
		t.Fatalf("intent=%+v", intent)
	}
	requireStatus(t, app.request(t, http.MethodPost, path+"/comments", token, 1, map[string]any{"body": "普通评论"}), http.StatusCreated)
	events = notificationEventsForTask(t, app, id)
	comment := decodeNotificationIntent(t, events[len(events)-1])
	if comment.Scene != notification.SceneWorkTaskCommented || !comment.SkipExternal || comment.CommentID == 0 {
		t.Fatalf("comment=%+v", comment)
	}
	// Force the outbox insert to fail: the persisted status must roll back with it.
	if err := app.db.Callback().Create().Before("gorm:create").Register("reject_status_outbox", func(tx *gorm.DB) {
		if tx.Statement.Table == "outbox_events" {
			tx.AddError(errors.New("outbox unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer app.db.Callback().Create().Remove("reject_status_outbox")
	requireStatus(t, app.request(t, http.MethodPatch, path, token, 1, map[string]any{"status": "completed"}), http.StatusInternalServerError)
	var row taskgorm.WorkTask
	if err := app.db.First(&row, id).Error; err != nil || row.Status != "in_dev" {
		t.Fatalf("rollback status=%s err=%v", row.Status, err)
	}
	var count int64
	app.db.Model(&auditgorm.OutboxEvent{}).Where("workspace_id = ? AND aggregate_type = ? AND aggregate_id = ?", 1, "work_task_notification", id).Count(&count)
	if count != 3 {
		t.Fatalf("rollback events=%d", count)
	}
}

func TestTaskBatchStatusNotifiesChildAndAutoCompletedParent(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "batch-status-admin", "super_admin", "")
	owner := app.createUser(t, 2, "batch-status-owner", "developer", "")
	seedNativeWorkbenchScopes(t, app, admin)
	parent := taskgorm.WorkTask{ID: 901, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-901", Title: "父项", Status: "in_dev", Priority: "normal", AssigneeID: &owner.ID}
	child := parent
	child.ID = 902
	child.Kind = "subtask"
	child.TaskKey = "A-902"
	child.Title = "子项"
	child.ParentID = &parent.ID
	for _, row := range []any{&parent, &child} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-update", app.token(t, admin), 1, map[string]any{"ids": []uint{child.ID}, "atomic": true, "updates": map[string]any{"status": "completed"}}), http.StatusOK)
	}
	for _, id := range []uint{parent.ID, child.ID} {
		events := notificationEventsForTask(t, app, id)
		if len(events) != 1 {
			t.Fatalf("task %d events %d", id, len(events))
		}
		intent := decodeNotificationIntent(t, events[0])
		if intent.Scene != notification.SceneWorkTaskStatusChanged || intent.Vars["fromStatusKey"] != "in_dev" || intent.Vars["toStatusKey"] != "completed" || !intent.SkipExternal {
			t.Fatalf("intent %+v", intent)
		}
	}
}
