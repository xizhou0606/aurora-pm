package httpapi

import (
	"fmt"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"net/http"
	"strings"
	"testing"
)

func TestCommentMentionsReachUnrelatedWorkspaceMember(t *testing.T) {
	for _, resource := range []string{"quality", "task"} {
		for _, rich := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/rich=%v", resource, rich), func(t *testing.T) {
				fix := setupQualityPermissionFixtureWithConfig(t, enableTestNotifications)
				member, actor := fix.users["readonly"], fix.users["system_admin"]
				outsider := fix.app.createUser(t, 100, "mention-outsider", "user", "")
				body := fmt.Sprintf("@%s @%s", member.Username, outsider.Username)
				if rich {
					body = fmt.Sprintf(`{"type":"doc","content":[{"type":"mention","attrs":{"id":"%d"}},{"type":"mention","attrs":{"id":"%d"}},{"type":"mention","attrs":{"id":"%d"}}]}`, factUserID(member), factUserID(member), factUserID(outsider))
				}
				bug := fix.createBug(t, 990, "triage", false)
				route := fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID)
				scene := notification.SceneQualityCommentMentioned
				if resource == "task" {
					created := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks", fix.app.token(t, actor), fix.workspaceID, map[string]any{"requirement_id": fix.requirement.ID, "title": "mention regression"})
					requireStatus(t, created, http.StatusCreated)
					route = fmt.Sprintf("/api/v1/platform/tasks/%d/comments", uint(decodeData(t, created).(map[string]any)["id"].(float64)))
					scene = notification.SceneWorkTaskCommentMentioned
				}
				response := fix.app.request(t, http.MethodPost, route, fix.app.token(t, actor), fix.workspaceID, map[string]any{"body": body})
				requireStatus(t, response, http.StatusCreated)
				var events []auditgorm.OutboxEvent
				if err := fix.app.db.Where("event_type = ?", notification.EventType(scene)).Find(&events).Error; err != nil {
					t.Fatal(err)
				}
				if len(events) != 1 {
					t.Fatalf("mention events=%d want 1", len(events))
				}
				intent := decodeNotificationIntent(t, events[0])
				if len(intent.RecipientUserIDs) != 1 || intent.RecipientUserIDs[0] != factUserID(member) || intent.SkipExternal {
					t.Fatalf("intent=%+v", intent)
				}
				title, area := bug.Title, "质量工单"
				if resource == "task" {
					title, area = "mention regression", "任务"
				}
				name := actor.DisplayName
				if name == "" {
					name = actor.Username
				}
				want := name + "在" + area + "「" + title + "」评论中提到了你\n"
				if !strings.HasPrefix(intent.FallbackPlainText, want) {
					t.Fatalf("mention text %q want prefix %q", intent.FallbackPlainText, want)
				}
				var receipts []records.InboxReceipt
				if err := fix.app.db.Where("event_id = ?", events[0].ID).Find(&receipts).Error; err != nil {
					t.Fatal(err)
				}
				if len(receipts) != 1 || receipts[0].RecipientUserID != factUserID(member) {
					t.Fatalf("receipts=%+v", receipts)
				}
				if err := fix.app.db.Model(&identitygorm.WorkspaceMember{}).Where("workspace_id = ? AND user_id = ?", fix.workspaceID, member.ID).Update("enabled", false).Error; err != nil {
					t.Fatal(err)
				}
				response = fix.app.request(t, http.MethodPost, route, fix.app.token(t, actor), fix.workspaceID, map[string]any{"body": body})
				requireStatus(t, response, http.StatusCreated)
				var count int64
				fix.app.db.Model(&auditgorm.OutboxEvent{}).Where("event_type = ?", notification.EventType(scene)).Count(&count)
				if count != 1 {
					t.Fatalf("disabled member received another event: %d", count)
				}
			})
		}
	}
}
