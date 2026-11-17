package notificationhttp

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type readerStub struct{ workspace, user uint }

func (r *readerStub) ListMentions(_ context.Context, w, u, b uint) (notification.MentionPage, error) {
	r.workspace = w
	r.user = u
	return notification.MentionPage{Items: []notification.Mention{}}, nil
}

type verifierStub struct{ err error }

func (v verifierStub) VerifyWorkspaceMembership(context.Context, uint, uint) error { return v.err }

func TestMentionEndpointUsesAuthenticatedRecipient(t *testing.T) {
	for _, test := range []struct {
		name   string
		user   uint
		denied bool
		status int
	}{{"member", 8, false, 200}, {"anonymous", 0, false, 401}, {"foreign", 8, true, 403}} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			reader := &readerStub{}
			verifier := verifierStub{}
			if test.denied {
				verifier.err = errors.New("denied")
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(pmauth.PrincipalContextKey, pmauth.Principal{UserID: test.user})
				c.Set(pmauth.WorkspaceIDContextKey, uint(7))
			})
			NewMentionHandler(reader, verifier).RegisterRoutes(router.Group(""))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/notifications/mentions?user_id=99&workspace_id=99", nil))
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if test.status == 200 && (reader.workspace != 7 || reader.user != 8) {
				t.Fatalf("reader=%+v", reader)
			}
			if test.status != 200 && reader.user != 0 {
				t.Fatal("unauthorized query")
			}
		})
	}
}
