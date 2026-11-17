package collaborationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type principalResolverFake struct{}

func (principalResolverFake) ResolveCollaborationPrincipal(*gin.Context) (collaboration.Principal, error) {
	return collaboration.Principal{WorkspaceID: 11, UserID: 7, DisplayName: "Alice", ConnectionID: "auth-connection"}, nil
}

type roomResolverFake struct{}

func (roomResolverFake) ResolveCollaborationRoom(c *gin.Context, principal collaboration.Principal) (collaboration.Room, error) {
	return collaboration.Room{WorkspaceID: principal.WorkspaceID, ResourceID: c.Param("resourceId")}, nil
}

type streamerFake struct{ t *testing.T }

func (fake streamerFake) Stream(
	_ context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	after uint64,
	emit func(collaboration.StreamItem) error,
) error {
	fake.t.Helper()
	if principal.UserID != 7 || room.WorkspaceID != 11 || room.ResourceID != "resource-1" || after != 4 {
		fake.t.Fatalf("unexpected stream arguments: principal=%#v room=%#v after=%d", principal, room, after)
	}
	if err := emit(collaboration.StreamItem{Kind: "committed", Event: &collaboration.Event{
		Room: room, Sequence: 5, OperationID: "op-5", ActorUserID: 7, Type: "record.updated",
		Payload: []byte(`{"record_id":"r1"}`), PolicyRevision: 3, CommittedAt: time.Unix(1, 0).UTC(),
	}}); err != nil {
		return err
	}
	return collaboration.ErrForbidden
}

func TestSSEUsesCursorAndSignalsRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, err := NewHandler(principalResolverFake{}, roomResolverFake{}, streamerFake{t: t})
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	handler.RegisterRoutes(engine.Group("/api/v1"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/document-resources/resource-1/collaboration/events?resource_id=forged", nil)
	request.Header.Set("Last-Event-ID", "4")
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("response status/headers = %d %#v", recorder.Code, recorder.Header())
	}
	body := recorder.Body.String()
	for _, expected := range []string{"id: 5", "event: committed", "event: access_revoked"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("SSE body missing %q: %s", expected, body)
		}
	}
}
