package collaborationhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type ticketIssuerStub struct {
	principal collaboration.Principal
	room      collaboration.Room
	value     ConnectionTicket
	err       error
}

func (stub *ticketIssuerStub) IssueDocumentConnectionTicket(
	_ context.Context, principal collaboration.Principal, room collaboration.Room,
) (ConnectionTicket, error) {
	stub.principal, stub.room = principal, room
	return stub.value, stub.err
}

func TestTicketHandlerIssuesNoStoreResourceScopedTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	issuer := &ticketIssuerStub{value: ConnectionTicket{
		Ticket: "opaque", WebSocketURL: "wss://collaboration.example.test", ExpiresAt: time.Now().Add(time.Minute),
	}}
	principal := collaboration.Principal{WorkspaceID: 11, UserID: 7, DisplayName: "Alice", ConnectionID: "auth-connection"}
	room := collaboration.Room{WorkspaceID: 11, ResourceID: "resource"}
	handler, err := NewTicketHandler(principalResolverFake{}, roomResolverFake{}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler.RegisterRoutes(router.Group(""))
	request := httptest.NewRequest(http.MethodPost, "/document-resources/resource/collaboration/ticket", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	var body ConnectionTicket
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Ticket != "opaque" {
		t.Fatalf("body=%+v err=%v", body, err)
	}
	if issuer.principal != principal || issuer.room != room {
		t.Fatalf("principal=%+v room=%+v", issuer.principal, issuer.room)
	}
}
