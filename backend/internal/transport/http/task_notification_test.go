package httpapi

import (
	"net/http"
	"strings"
	"testing"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestTaskCreateAndReassignmentWritePMOwnedNotifications(t *testing.T) {
	app := newTestApp(t, enableTestNotifications)
	admin := app.createUser(t, 1, "task-admin", "super_admin", "")
	assignee := app.createUser(t, 2, "task-assignee", "developer", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []any{
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: assignee.ID, Role: "member", Enabled: true},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: assignee.ID, Role: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)

	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1, map[string]any{
		"requirement_id": 13, "title": "修复登录页", "priority": "high",
		"assignee_id": assignee.ID,
	})
	requireStatus(t, created, http.StatusCreated)
	createdID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	events := notificationEventsForTask(t, app, createdID)
	if len(events) != 1 {
		t.Fatalf("created task notification events=%d want=1", len(events))
	}
	intent := decodeNotificationIntent(t, events[0])
	if intent.Scene != notification.SceneWorkTaskAssigned ||
		len(intent.RecipientUserIDs) != 1 || intent.RecipientUserIDs[0] != assignee.ID ||
		intent.Vars["title"] != "修复登录页" || intent.Vars["taskKey"] != "A-1" ||
		intent.Vars["kind"] != "任务" || intent.Vars["priority"] != "高" || intent.Vars["status"] != "待处理" ||
		intent.Vars["assignee"] != assignee.DisplayName ||
		!strings.HasSuffix(intent.Vars["url"], "/workspace/work-items/"+uintString(createdID)) {
		t.Fatalf("created task intent=%+v", intent)
	}
	for _, forbidden := range []string{
		"团队：", "项目：", "环境：", "资源 ID：", "变更明细：", "摘要：",
	} {
		if strings.Contains(intent.FallbackPlainText, forbidden) {
			t.Fatalf("task notification text=%q contains %q", intent.FallbackPlainText, forbidden)
		}
	}

	unassigned := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1, map[string]any{
		"requirement_id": 13, "title": "稍后分配",
	})
	requireStatus(t, unassigned, http.StatusCreated)
	unassignedID := uint(decodeData(t, unassigned).(map[string]any)["id"].(float64))
	if got := len(notificationEventsForTask(t, app, unassignedID)); got != 0 {
		t.Fatalf("unassigned create queued %d notification events", got)
	}

	assigned := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(unassignedID), token, 1,
		map[string]any{"assignee_id": assignee.ID})
	requireStatus(t, assigned, http.StatusOK)
	if got := len(notificationEventsForTask(t, app, unassignedID)); got != 1 {
		t.Fatalf("reassignment queued %d notification events want=1", got)
	}

	unchanged := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(unassignedID), token, 1,
		map[string]any{"assignee_id": assignee.ID, "title": "负责人未变化"})
	requireStatus(t, unchanged, http.StatusOK)
	cleared := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(unassignedID), token, 1,
		map[string]any{"assignee_id": nil})
	requireStatus(t, cleared, http.StatusOK)
	selfAssigned := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(unassignedID), token, 1,
		map[string]any{"assignee_id": admin.ID})
	requireStatus(t, selfAssigned, http.StatusOK)
	selfEvents := notificationEventsForTask(t, app, unassignedID)
	if got := len(selfEvents); got != 2 {
		t.Fatalf("unchanged or cleared assignment changed event count; self assignment events=%d want=2", got)
	}
	selfIntent := decodeNotificationIntent(t, selfEvents[1])
	if len(selfIntent.RecipientUserIDs) != 1 || selfIntent.RecipientUserIDs[0] != admin.ID {
		t.Fatalf("self assignment recipients=%v want=[%d]", selfIntent.RecipientUserIDs, admin.ID)
	}
}

func TestTaskAssignmentNotificationsRespectDisabledSwitch(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-admin-disabled", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", app.token(t, admin), 1,
		map[string]any{"requirement_id": 13, "title": "自分配但通知关闭", "assignee_id": admin.ID})
	requireStatus(t, created, http.StatusCreated)
	taskID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	events := notificationEventsForTask(t, app, taskID)
	if len(events) != 1 || !decodeNotificationIntent(t, events[0]).SkipExternal {
		t.Fatalf("disabled external must retain inbox event: %+v", events)
	}
}

func TestTaskCommentNotifiesRelatedPeopleOnceAndExcludesCommenter(t *testing.T) {
	app := newTestApp(t, enableTestNotifications)
	admin := app.createUser(t, 1, "task-comment-admin", "super_admin", "")
	assignee := app.createUser(t, 2, "task-comment-assignee", "developer", "")
	participant := app.createUser(t, 3, "task-comment-participant", "developer", "")
	watcher := app.createUser(t, 4, "task-comment-watcher", "developer", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, user := range []identitygorm.User{assignee, participant, watcher} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 1, UserID: user.ID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", app.token(t, admin), 1,
		map[string]any{"requirement_id": 13, "title": "需要评论的任务", "assignee_id": assignee.ID})
	requireStatus(t, created, http.StatusCreated)
	taskID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	if err := app.db.Where("aggregate_type = ? AND aggregate_id = ?", "work_task_notification", taskID).
		Delete(&auditgorm.OutboxEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&taskgorm.WorkTaskComment{WorkspaceID: 1, WorkTaskID: taskID, UserID: participant.ID, Body: "之前评论"},
		&taskgorm.WorkTaskWatcher{WorkspaceID: 1, WorkTaskID: taskID, UserID: watcher.ID},
		&taskgorm.WorkTaskWatcher{WorkspaceID: 1, WorkTaskID: taskID, UserID: assignee.ID},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	commented := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/"+uintString(taskID)+"/comments", app.token(t, admin), 1,
		map[string]any{"body": "请复核这个修复"})
	requireStatus(t, commented, http.StatusCreated)
	events := notificationEventsForTask(t, app, taskID)
	if len(events) != 3 {
		t.Fatalf("task comment events=%d want=3", len(events))
	}
	gotRecipients := make([]uint, 0, len(events))
	for _, event := range events {
		intent := decodeNotificationIntent(t, event)
		if intent.Scene != notification.SceneWorkTaskCommented || intent.ActorUserID != admin.ID ||
			intent.Vars["comment"] != "请复核这个修复" ||
			!strings.Contains(intent.FallbackPlainText, "任务新增评论") {
			t.Fatalf("task comment intent=%+v", intent)
		}
		gotRecipients = append(gotRecipients, intent.RecipientUserIDs...)
	}
	want := map[uint]bool{assignee.ID: true, participant.ID: true, watcher.ID: true}
	if len(gotRecipients) != len(want) {
		t.Fatalf("task comment recipients=%v", gotRecipients)
	}
	for _, id := range gotRecipients {
		if !want[id] || id == admin.ID {
			t.Fatalf("task comment recipients=%v", gotRecipients)
		}
	}
}

func notificationEventsForTask(t *testing.T, app *testApp, taskID uint) []auditgorm.OutboxEvent {
	t.Helper()
	var events []auditgorm.OutboxEvent
	if err := app.db.Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ? AND event_type LIKE ?",
		1, "work_task_notification", taskID, notification.EventTypePrefix+"%",
	).Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

func TestTaskNotificationProjectIDFallsBackToProjectSet(t *testing.T) {
	if got := taskNotificationProjectID(taskgorm.WorkTask{ProjectID: 12, ProjectSetID: 11}); got != 12 {
		t.Fatalf("scheduled task project=%d", got)
	}
	if got := taskNotificationProjectID(taskgorm.WorkTask{ProjectID: 0, ProjectSetID: 11}); got != 11 {
		t.Fatalf("unscheduled task project=%d", got)
	}
}
