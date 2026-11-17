package notificationgorm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"gorm.io/gorm"
)

func TestBusinessMessagesReceiptProjectionAndReadScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	scenes := []string{notification.SceneQualityBugCreated, notification.SceneQualityBugAssigned, notification.SceneQualityBugReviewRequested, notification.SceneQualityBugReturned, notification.SceneQualityBugCommented, notification.SceneWorkTaskAssigned, notification.SceneWorkTaskCommented, notification.SceneWorkTaskStatusChanged, notification.SceneWorkflowNodeAssigned, notification.SceneWorkflowPreReviewInvited, notification.ScenePlanUnitMemberAssigned}
	add := func(scene string, workspace, recipient uint) {
		t.Helper()
		err := db.Transaction(func(tx *gorm.DB) error {
			return notification.Enqueue(tx, "business", 42, notification.Intent{WorkspaceID: workspace, TeamID: 11, ProjectID: 21, ActorUserID: 1, RecipientUserIDs: []uint{recipient}, Scene: scene, ResourceType: "work_task", ResourceID: "42", SkipExternal: true, Vars: map[string]string{"operator": "铁奥", "title": "测试任务", "url": "SECRET-URL", "comment": "PRIVATE-BODY", "fromStatusKey": "todo", "toStatusKey": "done"}, FallbackPlainText: "notification"})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, scene := range scenes {
		add(scene, 7, 8)
	}
	add(notification.SceneWorkTaskAssigned, 8, 8)
	add(notification.SceneWorkTaskAssigned, 7, 9)
	add(notification.SceneWorkTaskCommentMentioned, 7, 8)
	add("unknown", 7, 8)
	repo := NewMentionRepository(db)
	page, err := repo.ListInbox(context.Background(), 7, 8, notification.InboxFilter{Category: "message"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 11 || page.UnreadByCategory["message"] != 11 || page.UnreadByCategory["mention"] != 1 {
		t.Fatalf("page=%+v", page)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "PRIVATE") {
		t.Fatal("unsafe payload projected")
	}
	for _, item := range page.Items {
		if item.ActorName != "铁奥" || item.ResourceID != "42" {
			t.Fatalf("missing identity/route: %+v", item)
		}
		if item.Scene == notification.SceneWorkTaskStatusChanged && (item.PreviousStatus != "todo" || item.Status != "done") {
			t.Fatalf("status=%+v", item)
		}
	}
	add(notification.SceneWorkTaskAssigned, 7, 8)
	if err := repo.MarkAllRead(context.Background(), 7, 8, page.HighWatermark, "message", time.Now()); err != nil {
		t.Fatal(err)
	}
	next, err := repo.ListInbox(context.Background(), 7, 8, notification.InboxFilter{})
	if err != nil || next.UnreadByCategory["message"] != 1 || next.UnreadByCategory["mention"] != 1 {
		t.Fatalf("read scope: %+v %v", next, err)
	}
	if notification.InboxCategory(notification.SceneIdentityPasswordResetRequested) != "" || notification.InboxCategory("unknown") != "" {
		t.Fatal("unsafe scene included")
	}
}

func TestQualityAssignmentProjectsOnlyCurrentRecipientRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return notification.Enqueue(tx, "quality_bug_notification", 42, notification.Intent{WorkspaceID: 7, TeamID: 11, ProjectID: 21, Scene: notification.SceneQualityBugAssigned, ResourceType: "quality_bug", ResourceID: "42", ActorUserID: 1, RecipientUserIDs: []uint{8, 9}, Vars: map[string]string{"assigneeUserId": "8", "ccUsers": "Private CC name"}, FallbackPlainText: "指派"})
	}); err != nil {
		t.Fatal(err)
	}
	repo := NewMentionRepository(db)
	for user, role := range map[uint]string{8: "assignee", 9: "cc"} {
		page, err := repo.ListInbox(context.Background(), 7, user, notification.InboxFilter{Category: "message"})
		if err != nil || len(page.Items) != 1 || page.Items[0].RecipientRole != role {
			t.Fatalf("user %d page=%+v err=%v", user, page, err)
		}
		raw, _ := json.Marshal(page)
		for _, secret := range []string{"assigneeUserId", "recipient_user_ids", "Private CC name"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("leaked %s", secret)
			}
		}
	}
}
