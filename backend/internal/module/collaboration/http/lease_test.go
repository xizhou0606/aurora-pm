package collaborationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type leaseLifecycleFake struct {
	acquired        collaboration.CellKey
	proofs          []collaboration.CellLeaseProof
	disconnect      string
	disconnectRoom  collaboration.Room
	disconnectActor uint
	err             error
}

func (fake *leaseLifecycleFake) Acquire(
	_ context.Context,
	principal collaboration.Principal,
	key collaboration.CellKey,
	editor string,
) (collaboration.CellLease, error) {
	fake.acquired = key
	return collaboration.CellLease{
		Key: key, LeaseID: "00000000-0000-0000-0000-000000000040", FencingToken: 4,
		ConnectionID: principal.ConnectionID, EditorInstanceID: editor,
		ActorUserID: principal.UserID, ActorName: principal.DisplayName,
	}, fake.err
}

func (fake *leaseLifecycleFake) Renew(
	_ context.Context,
	_ collaboration.Principal,
	proof collaboration.CellLeaseProof,
) (collaboration.CellLease, error) {
	fake.proofs = append(fake.proofs, proof)
	return collaboration.CellLease{Key: proof.Key, LeaseID: proof.LeaseID, FencingToken: proof.FencingToken}, fake.err
}

func (fake *leaseLifecycleFake) Release(
	_ context.Context,
	_ collaboration.Principal,
	proof collaboration.CellLeaseProof,
) error {
	fake.proofs = append(fake.proofs, proof)
	return fake.err
}

func (fake *leaseLifecycleFake) Disconnect(_ context.Context, principal collaboration.Principal, room collaboration.Room) (int64, error) {
	fake.disconnect, fake.disconnectRoom, fake.disconnectActor = principal.ConnectionID, room, principal.UserID
	return 2, fake.err
}

func TestLeaseRoutesUseServerPrincipalRoomAndConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	leases := &leaseLifecycleFake{}
	handler, err := NewLeaseHandler(principalResolverFake{}, roomResolverFake{}, leases)
	if err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	handler.RegisterRoutes(engine.Group("/api/v1"))
	base := "/api/v1/document-resources/resource-authoritative/collaboration/cell-leases"
	cell := `"table_id":"00000000-0000-0000-0000-000000000010","record_id":"00000000-0000-0000-0000-000000000020","field_id":"00000000-0000-0000-0000-000000000030","editor_instance_id":"editor-1"`
	proof := cell + `,"lease_id":"00000000-0000-0000-0000-000000000040","fencing_token":4,"connection_id":"forged","workspace_id":99,"actor_user_id":999`

	for _, request := range []struct {
		path, body string
		status     int
	}{
		{base + "/acquire", "{" + cell + "}", http.StatusOK},
		{base + "/renew", "{" + proof + "}", http.StatusOK},
		{base + "/release", "{" + proof + "}", http.StatusNoContent},
		{base + "/disconnect", "", http.StatusNoContent},
	} {
		recorder := httptest.NewRecorder()
		httpRequest := httptest.NewRequest(http.MethodPost, request.path, strings.NewReader(request.body))
		httpRequest.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, httpRequest)
		if recorder.Code != request.status {
			t.Fatalf("%s status=%d body=%s", request.path, recorder.Code, recorder.Body.String())
		}
	}
	if leases.acquired.Room.WorkspaceID != 11 || leases.acquired.Room.ResourceID != "resource-authoritative" {
		t.Fatalf("acquire room = %#v", leases.acquired.Room)
	}
	for _, proof := range leases.proofs {
		if proof.ConnectionID != "auth-connection" {
			t.Fatalf("client forged connection accepted: %#v", proof)
		}
	}
	if leases.disconnect != "auth-connection" {
		t.Fatalf("disconnect connection = %q", leases.disconnect)
	}
	if leases.disconnectActor != 7 || leases.disconnectRoom.ResourceID != "resource-authoritative" {
		t.Fatalf("disconnect scope actor=%d room=%#v", leases.disconnectActor, leases.disconnectRoom)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, base+"/batch-acquire", strings.NewReader("{}")))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("batch must not be exposed as an HTTP loop, status=%d", recorder.Code)
	}
}

func TestLeaseHandlerMapsDomainConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	leases := &leaseLifecycleFake{err: collaboration.ErrStaleLease}
	handler, _ := NewLeaseHandler(principalResolverFake{}, roomResolverFake{}, leases)
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	body := `{"table_id":"a","record_id":"b","field_id":"c","editor_instance_id":"editor","lease_id":"lease","fencing_token":1}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/document-resources/resource/collaboration/cell-leases/renew", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "stale_lease") {
		t.Fatalf("mapped response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestLeaseHeldResponseDoesNotDiscloseLeaseProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	leases := &leaseLifecycleFake{err: collaboration.ErrLeaseHeld}
	handler, _ := NewLeaseHandler(principalResolverFake{}, roomResolverFake{}, leases)
	engine := gin.New()
	handler.RegisterRoutes(engine.Group(""))
	body := `{"table_id":"a","record_id":"b","field_id":"c","editor_instance_id":"editor"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/document-resources/resource/collaboration/cell-leases/acquire", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	response := recorder.Body.String()
	if recorder.Code != http.StatusConflict || !strings.Contains(response, "actor_user_id") ||
		strings.Contains(response, "lease_id") || strings.Contains(response, "fencing_token") ||
		strings.Contains(response, "connection_id") || strings.Contains(response, "editor_instance_id") ||
		strings.Contains(response, "policy_revision") {
		t.Fatalf("unsafe held response = %d %s", recorder.Code, response)
	}
}
