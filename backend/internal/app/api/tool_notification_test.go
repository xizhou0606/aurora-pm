package api

import (
	"context"
	"encoding/json"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

func TestTaskToolAssignmentLocalizesTaskEnums(t *testing.T) {
	db := newToolNotificationFixture(t)
	if err := db.AutoMigrate(&taskgorm.WorkTask{}); err != nil {
		t.Fatal(err)
	}
	assignee := uint(2)
	if err := db.Create(&taskgorm.WorkTask{
		ID: 81, WorkspaceID: 7, TeamID: 11, ProjectSetID: 21, ProjectID: 31, RequirementID: 41,
		Kind: "subtask", TaskKey: "PM17-61", Title: "WS-18 构建、本地服务与验收",
		Status: "draft", Priority: "normal", AssigneeID: &assignee, MutationState: "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	service, err := task.NewReadService(taskgorm.NewReadRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	writer := taskToolAssignmentWriter{tasks: service, config: enabledToolNotificationConfig(), tx: db}
	if err := writer.RecordTaskAssignment(context.Background(), task.AssignmentNotification{
		WorkspaceID: 7, PrincipalID: 1, TaskID: 81, AssigneeUserID: assignee,
	}); err != nil {
		t.Fatal(err)
	}
	events := toolNotificationEvents(t, db)
	if len(events) != 1 {
		t.Fatalf("events=%d want=1", len(events))
	}
	intent := toolNotificationIntent(t, events[0])
	if intent.Vars["kind"] != "子任务" || intent.Vars["priority"] != "普通" || intent.Vars["status"] != "待处理" {
		t.Fatalf("localized task intent=%+v", intent)
	}
	for _, rawLine := range []string{"类型：subtask", "优先级：normal", "当前状态：draft"} {
		if strings.Contains(intent.FallbackPlainText, rawLine) {
			t.Fatalf("task notification text=%q contains raw enum line %q", intent.FallbackPlainText, rawLine)
		}
	}
}

func TestQualityToolMutationEnqueuesCreatedNotifications(t *testing.T) {
	db := newToolNotificationFixture(t)
	actor, assignee := uint(1), uint(2)
	writer := qualityToolMutationEffects{tx: db, config: enabledToolNotificationConfig()}
	err := writer.RecordBugMutation(context.Background(), quality.BugMutationEffect{
		Operation: quality.BugCreate, WorkspaceID: 7, PrincipalID: actor,
		After: quality.BugSnapshot{
			ID: 71, WorkspaceID: 7, TeamID: 11, ProjectSetID: 21, PlanUnitID: 31,
			BugKey: "BUG-71", Title: "CLI 创建缺陷", Status: "triage", SeverityKey: "p0",
			ReporterUserID: &actor, AssigneeUserID: &assignee,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := toolNotificationEvents(t, db)
	if len(events) != 2 {
		t.Fatalf("events=%d want=2", len(events))
	}
	recipients := map[uint]bool{}
	for _, event := range events {
		intent := toolNotificationIntent(t, event)
		if event.EventType != notification.EventType(notification.SceneQualityBugCreated) || len(intent.RecipientUserIDs) != 1 {
			t.Fatalf("event=%+v intent=%+v", event, intent)
		}
		recipients[intent.RecipientUserIDs[0]] = true
		if intent.Vars["operator"] != "铁奥" || intent.Vars["assignee"] != "艾伦" || intent.Vars["assigneeUserId"] != "2" || intent.FallbackPlainText == "" {
			t.Fatalf("intent=%+v", intent)
		}
	}
	if !recipients[actor] || !recipients[assignee] {
		t.Fatalf("recipients=%v", recipients)
	}
}

func TestTaskToolCommentEnqueuesAllExpectedRecipients(t *testing.T) {
	db := newToolNotificationFixture(t)
	if err := db.AutoMigrate(&taskgorm.WorkTaskWatcher{}, &taskgorm.WorkTaskComment{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&taskgorm.WorkTaskWatcher{WorkspaceID: 7, WorkTaskID: 81, UserID: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]taskgorm.WorkTaskComment{
		{WorkspaceID: 7, WorkTaskID: 81, UserID: 1, Body: "当前评论"},
		{WorkspaceID: 7, WorkTaskID: 81, UserID: 4, Body: "历史评论"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	writer := taskToolCollaborationNotifications{tx: db, config: enabledToolNotificationConfig()}
	err := writer.RecordTaskComment(context.Background(), task.CommentNotification{
		CommentID:   901,
		WorkspaceID: 7, PrincipalID: 1, TeamID: 11, ProjectSetID: 21, PlanUnitID: 31,
		TaskID: 81, AssigneeUserID: 2, ReporterUserID: 1,
		TaskKey: "TASK-81", Title: "CLI 评论任务", Body: "请 @five 看一下",
	})
	if err != nil {
		t.Fatal(err)
	}
	events := toolNotificationEvents(t, db)
	if len(events) != 4 {
		t.Fatalf("events=%d want=4", len(events))
	}
	recipients := map[uint]bool{}
	for _, event := range events {
		intent := toolNotificationIntent(t, event)
		if (event.EventType != notification.EventType(notification.SceneWorkTaskCommented) && event.EventType != notification.EventType(notification.SceneWorkTaskCommentMentioned)) || len(intent.RecipientUserIDs) != 1 {
			t.Fatalf("event=%+v intent=%+v", event, intent)
		}
		recipients[intent.RecipientUserIDs[0]] = true
	}
	for _, userID := range []uint{2, 3, 4, 5} {
		if !recipients[userID] {
			t.Fatalf("recipient %d missing from %v", userID, recipients)
		}
	}
	if recipients[1] {
		t.Fatalf("actor must not receive own comment notification: %v", recipients)
	}
}

func TestWorkflowToolMutationsEnqueueAssignmentsAndInvitations(t *testing.T) {
	db := newToolNotificationFixture(t)
	cfg := enabledToolNotificationConfig()
	cfg.Enabled = false // In-app delivery must not depend on Feishu.
	assignmentWriter := planWorkflowAssignments{tx: db, config: cfg}
	if err := assignmentWriter.RecordSubjectAssignment(context.Background(), workflow.SubjectAssignment{
		WorkspaceID: 7, PrincipalID: 1, ProjectSetID: 21, TeamID: 11,
		InstanceID: 91, SubjectID: 41, SubjectType: "requirement", SubjectTitle: "招聘台",
		NodeID: "development", NodeName: "研发", AssigneeUserID: 2,
	}); err != nil {
		t.Fatal(err)
	}
	invitationWriter := workflowToolInvitations{tx: db, config: cfg}
	if err := invitationWriter.RecordPreReviewInvitation(context.Background(), workflow.PreReviewInvitation{
		WorkspaceID: 7, PrincipalID: 1, ProjectSetID: 21, TeamID: 11,
		InstanceID: 91, SubjectID: 41, SubjectType: "requirement", SubjectTitle: "招聘台",
		NodeID: "pre-review", NodeName: "产品预审", InviteUserIDs: []uint{1, 2, 3},
	}); err != nil {
		t.Fatal(err)
	}
	events := toolNotificationEvents(t, db)
	if len(events) != 3 {
		t.Fatalf("events=%d want=3", len(events))
	}
	counts := map[string]int{}
	for _, event := range events {
		counts[event.EventType]++
		intent := toolNotificationIntent(t, event)
		if !intent.SkipExternal || intent.ResourceType != "requirement" || intent.ResourceID != "41" || intent.Vars["nodeId"] == "" {
			t.Fatalf("missing independent inbox route: %+v", intent)
		}
	}
	if counts[notification.EventType(notification.SceneWorkflowNodeAssigned)] != 1 ||
		counts[notification.EventType(notification.SceneWorkflowPreReviewInvited)] != 2 {
		t.Fatalf("event counts=%v", counts)
	}
}

func newToolNotificationFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&records.InboxReceipt{}, &identitygorm.User{}, &identitygorm.Workspace{}, &identitygorm.WorkspaceMember{}, &auditgorm.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.Workspace{ID: 7, Name: "PM", Slug: "pm", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	names := []struct{ username, displayName string }{
		{"teo", "铁奥"}, {"alan", "艾伦"}, {"watcher", "关注人"}, {"participant", "评论参与人"}, {"five", "被提及人"},
	}
	for index, name := range names {
		userID := uint(index + 1)
		if err := db.Create(&identitygorm.User{
			ID: userID, ExternalSubject: "local:" + name.username, Username: name.username,
			DisplayName: name.displayName, Active: true, Status: string(identity.AccountActive),
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 7, UserID: userID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func enabledToolNotificationConfig() config.Notification {
	return config.Notification{Enabled: true, PublicBaseURL: "https://pm.example.test"}
}

func toolNotificationEvents(t *testing.T, db *gorm.DB) []auditgorm.OutboxEvent {
	t.Helper()
	var events []auditgorm.OutboxEvent
	if err := db.Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

func toolNotificationIntent(t *testing.T, event auditgorm.OutboxEvent) notification.Intent {
	t.Helper()
	var intent notification.Intent
	if err := json.Unmarshal([]byte(event.Payload), &intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestToolNotificationDisplayResolvesReferencedNonMember(t *testing.T) {
	db := newToolNotificationFixture(t)
	admin := identitygorm.User{ID: 109, Username: "admin", DisplayName: "Administrator", Active: true}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	users, err := toolNotificationUsers(context.Background(), db, 7, 109)
	if err != nil || toolNotificationUserLabel(users, 109) != "Administrator" {
		t.Fatalf("users=%+v err=%v", users, err)
	}
	if _, ok := users[77]; ok {
		t.Fatal("legacy ID must not be added to the canonical directory")
	}
}
