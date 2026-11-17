package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestQualityBugLifecycleWritesSemanticNotificationOutbox(t *testing.T) {
	fix := setupQualityPermissionFixtureWithConfig(t, enableTestNotifications)
	reporterID := factUserID(fix.users["reporter"])
	assigneeID := factUserID(fix.users["assignee"])

	createdResponse := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs",
		fix.app.token(t, fix.users["reporter"]), fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement.ID, "title": "登录页白屏",
			"severity_key": "p1", "assignee_user_id": assigneeID,
		})
	requireStatus(t, createdResponse, http.StatusCreated)
	createdID := uint(decodeData(t, createdResponse).(map[string]any)["id"].(float64))
	createdEvents := notificationEventsForBug(t, fix, createdID)
	if len(createdEvents) != 2 {
		t.Fatalf("created notification events=%d want=2", len(createdEvents))
	}
	createdRecipients := intentRecipients(t, createdEvents)
	if strings.Join(createdRecipients, ",") != uintStrings(assigneeID, reporterID) {
		t.Fatalf("created recipients=%v", createdRecipients)
	}
	for _, event := range createdEvents {
		intent := decodeNotificationIntent(t, event)
		if intent.Scene != notification.SceneQualityBugCreated || intent.WorkspaceID != fix.workspaceID ||
			intent.TeamID != fix.project.TeamID || intent.ProjectID != fix.plan.ID ||
			intent.Vars["bugKey"] != "BUG-"+uintString(createdID) ||
			intent.Vars["severity"] != "严重" ||
			!strings.HasSuffix(intent.Vars["url"], "/workspace/quality-management/bugs/"+uintString(createdID)) {
			t.Fatalf("created intent=%+v", intent)
		}
		for _, genericKey := range []string{
			"teamName", "projectName", "env", "resourceId", "changeSummary", "content", "summary",
		} {
			if _, exists := intent.Vars[genericKey]; exists {
				t.Fatalf("created intent unexpectedly contains Hub template key %q: %+v", genericKey, intent.Vars)
			}
		}
	}

	assignedBug := fix.createBug(t, 920, "triage", false)
	newAssigneeID := factUserID(fix.users["quality_owner"])
	ccID := factUserID(fix.users["reporter"])
	assigned := fix.request(t, "manager", http.MethodPatch, assignedBug.ID, map[string]any{
		"assignee_user_id": newAssigneeID, "cc_user_ids": []uint{ccID},
		"assignment_note": `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"今日优先处理"}]}]}`,
	})
	requireStatus(t, assigned, http.StatusOK)
	assignedEvents := notificationEventsForBug(t, fix, assignedBug.ID)
	if len(assignedEvents) != 2 ||
		strings.Join(intentRecipients(t, assignedEvents), ",") != uintStrings(ccID, newAssigneeID) {
		t.Fatalf("assigned events=%+v", assignedEvents)
	}
	assignedIntent := decodeNotificationIntent(t, assignedEvents[0])
	if assignedIntent.Scene != notification.SceneQualityBugAssigned ||
		assignedIntent.Vars["assigneeUserId"] != uintString(newAssigneeID) ||
		assignedIntent.Vars["note"] != "今日优先处理" ||
		assignedIntent.Vars["ccUsers"] != fix.users["reporter"].DisplayName {
		t.Fatalf("assigned intent=%+v", assignedIntent)
	}

	reviewBug := fix.createBug(t, 921, "in_progress", false)
	review := fix.request(t, "assignee", http.MethodPatch, reviewBug.ID, map[string]any{
		"status": "review", "resolution_reason": "fixed",
	})
	requireStatus(t, review, http.StatusOK)
	reviewEvents := notificationEventsForBug(t, fix, reviewBug.ID)
	if len(reviewEvents) != 1 {
		t.Fatalf("review notification events=%d want=1", len(reviewEvents))
	}
	reviewIntent := decodeNotificationIntent(t, reviewEvents[0])
	if reviewIntent.Scene != notification.SceneQualityBugReviewRequested ||
		reviewIntent.RecipientUserIDs[0] != factUserID(fix.users["reporter"]) {
		t.Fatalf("review intent=%+v", reviewIntent)
	}

	returnedBug := fix.createBug(t, 922, "review", false)
	returned := fix.request(t, "reviewer", http.MethodPatch, returnedBug.ID, map[string]any{
		"status": "reopened", "regression_note": "首页仍然白屏",
	})
	requireStatus(t, returned, http.StatusOK)
	returnedEvents := notificationEventsForBug(t, fix, returnedBug.ID)
	if len(returnedEvents) != 1 {
		t.Fatalf("return notification events=%d want=1", len(returnedEvents))
	}
	returnedIntent := decodeNotificationIntent(t, returnedEvents[0])
	if returnedIntent.Scene != notification.SceneQualityBugReturned ||
		returnedIntent.RecipientUserIDs[0] != factUserID(fix.users["assignee"]) ||
		returnedIntent.Vars["note"] != "首页仍然白屏" {
		t.Fatalf("returned intent=%+v", returnedIntent)
	}

	directReopenedBug := fix.createBug(t, 925, "in_progress", false)
	directReopened := fix.request(t, "manager", http.MethodPatch, directReopenedBug.ID, map[string]any{
		"status": "reopened",
	})
	requireStatus(t, directReopened, http.StatusOK)
	directReopenedEvents := notificationEventsForBug(t, fix, directReopenedBug.ID)
	if len(directReopenedEvents) != 1 {
		t.Fatalf("direct reopened notification events=%d want=1", len(directReopenedEvents))
	}
	directReopenedIntent := decodeNotificationIntent(t, directReopenedEvents[0])
	if directReopenedIntent.Scene != notification.SceneQualityBugReturned ||
		directReopenedIntent.RecipientUserIDs[0] != factUserID(fix.users["assignee"]) {
		t.Fatalf("direct reopened intent=%+v", directReopenedIntent)
	}
}

func TestQualityBugNotificationOutboxSkipsNonLifecycleAndDisabledPaths(t *testing.T) {
	enabled := setupQualityPermissionFixtureWithConfig(t, enableTestNotifications)
	bug := enabled.createBug(t, 923, "triage", false)
	response := enabled.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{"priority": "high"})
	requireStatus(t, response, http.StatusOK)
	if events := notificationEventsForBug(t, enabled, bug.ID); len(events) != 0 {
		t.Fatalf("normal edit unexpectedly queued %d notification events", len(events))
	}

	disabled := setupQualityPermissionFixture(t)
	disabledBug := disabled.createBug(t, 924, "review", false)
	response = disabled.request(t, "reviewer", http.MethodPatch, disabledBug.ID, map[string]any{
		"status": "reopened", "regression_note": "仍然失败",
	})
	requireStatus(t, response, http.StatusOK)
	events := notificationEventsForBug(t, disabled, disabledBug.ID)
	if len(events) != 1 || !decodeNotificationIntent(t, events[0]).SkipExternal {
		t.Fatalf("disabled external delivery must retain inbox event: %+v", events)
	}
}

func TestQualityBugCommentNotifiesRelatedPeopleOnceAndExcludesCommenter(t *testing.T) {
	fix := setupQualityPermissionFixtureWithConfig(t, enableTestNotifications)
	bug := fix.createBug(t, 926, "triage", false)
	if err := fix.app.db.Create(&qualitygorm.QualityBugComment{
		WorkspaceID: fix.workspaceID, QualityBugID: bug.ID,
		UserID: factUserID(fix.users["tester"]), Body: "之前的参与者",
	}).Error; err != nil {
		t.Fatal(err)
	}

	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/"+uintString(bug.ID)+"/comments",
		fix.app.token(t, fix.users["reporter"]), fix.workspaceID,
		map[string]any{"body": `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"请复核"}]}]}`})
	requireStatus(t, response, http.StatusCreated)

	events := notificationEventsForBug(t, fix, bug.ID)
	wantRecipients := uintStrings(
		factUserID(fix.users["assignee"]),
		factUserID(fix.users["reviewer"]),
		factUserID(fix.users["tester"]),
	)
	if got := strings.Join(intentRecipients(t, events), ","); got != wantRecipients {
		t.Fatalf("comment recipients=%s want=%s", got, wantRecipients)
	}
	for _, event := range events {
		intent := decodeNotificationIntent(t, event)
		if intent.Scene != notification.SceneQualityBugCommented ||
			intent.ActorUserID != factUserID(fix.users["reporter"]) ||
			intent.Vars["comment"] != "请复核" ||
			!strings.Contains(intent.FallbackPlainText, "缺陷新增评论") {
			t.Fatalf("comment intent=%+v", intent)
		}
	}
}

func TestQualityNotificationPlainTextUsesOnlyPMBusinessFields(t *testing.T) {
	vars := map[string]string{
		"title": "登录页白屏", "bugKey": "BUG-42", "severity": "严重",
		"status": "处理中", "resolutionReason": "已修复", "assignee": "Bill",
		"reviewer": "Alice", "ccUsers": "Carol", "note": "请优先处理",
		"operator": "Administrator", "url": "https://pm.test/bugs/42",
	}
	tests := []struct {
		scene string
		want  []string
	}{
		{notification.SceneQualityBugCreated, []string{"缺陷已提交：登录页白屏", "编号：BUG-42", "严重级别：严重", "当前状态：处理中", "操作人：Administrator", "链接：https://pm.test/bugs/42"}},
		{notification.SceneQualityBugAssigned, []string{"缺陷待处理：登录页白屏", "指派给：Bill", "抄送：Carol", "备注：请优先处理"}},
		{notification.SceneQualityBugReviewRequested, []string{"缺陷待回归：登录页白屏", "解决方案：已修复", "处理人：Bill"}},
		{notification.SceneQualityBugReturned, []string{"缺陷已重新打开：登录页白屏", "重新打开说明：请优先处理", "负责人：Bill", "操作人：Administrator"}},
		{notification.SceneQualityBugCommented, []string{"缺陷新增评论：登录页白屏", "编号：BUG-42", "评论人：Administrator", "评论：-"}},
	}
	for _, test := range tests {
		text := qualityNotificationPlainText(test.scene, vars)
		for _, want := range test.want {
			if !strings.Contains(text, want) {
				t.Errorf("scene %s text=%q missing %q", test.scene, text, want)
			}
		}
		for _, forbidden := range []string{"团队：", "项目：", "环境：", "资源 ID：", "变更明细：", "摘要："} {
			if strings.Contains(text, forbidden) {
				t.Errorf("scene %s text=%q contains generic Hub field %q", test.scene, text, forbidden)
			}
		}
	}
}

func enableTestNotifications(cfg *config.Config) {
	cfg.Notification = config.Notification{
		Enabled: true, PublicBaseURL: "https://pm-main.example.invalid",
		PollInterval: time.Second, RequestTimeout: time.Second, ClaimTTL: time.Minute, BatchSize: 20,
	}
}

func notificationEventsForBug(
	t *testing.T, fix qualityPermissionFixture, bugID uint,
) []auditgorm.OutboxEvent {
	t.Helper()
	var events []auditgorm.OutboxEvent
	if err := fix.app.db.Where("workspace_id = ? AND aggregate_id = ? AND event_type LIKE ?",
		fix.workspaceID, bugID, notification.EventTypePrefix+"%").Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

func decodeNotificationIntent(t *testing.T, event auditgorm.OutboxEvent) notification.Intent {
	t.Helper()
	var intent notification.Intent
	if err := json.Unmarshal([]byte(event.Payload), &intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func intentRecipients(t *testing.T, events []auditgorm.OutboxEvent) []string {
	t.Helper()
	values := make([]string, 0, len(events))
	for _, event := range events {
		intent := decodeNotificationIntent(t, event)
		if len(intent.RecipientUserIDs) != 1 {
			t.Fatalf("event %d recipients=%v want exactly one", event.ID, intent.RecipientUserIDs)
		}
		values = append(values, uintString(intent.RecipientUserIDs[0]))
	}
	sort.Strings(values)
	return values
}

func uintStrings(values ...uint) string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, uintString(value))
	}
	sort.Strings(result)
	return strings.Join(result, ",")
}
