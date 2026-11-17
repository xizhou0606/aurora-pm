package notificationhttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type inboxStub struct {
	w, u, id uint
	category string
}

func (s *inboxStub) ListInbox(_ context.Context, w, u uint, f notification.InboxFilter) (notification.InboxPage, error) {
	s.w, s.u, s.category = w, u, f.Category
	return notification.InboxPage{Items: []notification.Mention{}}, nil
}
func (s *inboxStub) MarkRead(_ context.Context, w, u, id uint, _ time.Time) error {
	s.w, s.u, s.id = w, u, id
	return nil
}
func (s *inboxStub) MarkAllRead(ctx context.Context, w, u, id uint, category string, now time.Time) error {
	s.category = category
	return s.MarkRead(ctx, w, u, id, now)
}
func TestInboxEndpointsBindOnlySessionRecipient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/notifications/inbox?category=mention&user_id=99&workspace_id=99", "", 200},
		{"GET", "/notifications/inbox?category=message", "", 200},
		{"GET", "/notifications/inbox?category=unknown", "", 400},
		{"GET", "/notifications/inbox?before=-1", "", 400},
		{"POST", "/notifications/inbox/23/read", "", 200},
		{"POST", "/notifications/inbox/0/read", "", 400},
		{"POST", "/notifications/inbox/read-all", `{"through_id":23,"user_id":99}`, 200},
		{"POST", "/notifications/inbox/read-all", `{}`, 400},
		{"POST", "/notifications/inbox/read-all", `{"through_id":23,"category":"message"}`, 200},
		{"POST", "/notifications/inbox/read-all", `{"through_id":23,"category":"unknown"}`, 400},
	} {
		t.Run(test.path, func(t *testing.T) {
			store := &inboxStub{}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: 8})
				c.Set(pmauth.WorkspaceIDContextKey, uint(7))
			})
			NewInboxHandler(store, verifierStub{}).RegisterRoutes(router.Group(""))
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if test.status == 200 && (store.w != 7 || store.u != 8) {
				t.Fatalf("scope=%+v", store)
			}
			if test.status == 400 && store.w != 0 {
				t.Fatal("invalid input reached store")
			}
		})
	}
}
