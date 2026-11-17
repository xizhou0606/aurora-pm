package collaboration

import (
	"context"
	"errors"
	"testing"
	"time"
)

type presenceStoreFake struct {
	heartbeats int
	leaves     int
}

type presenceAccessFake struct{ decision AccessDecision }

func (fake *presenceAccessFake) EvaluateCollaborationAccess(context.Context, Principal, Room) (AccessDecision, error) {
	return fake.decision, nil
}

func (store *presenceStoreFake) Heartbeat(_ context.Context, principal Principal, room Room, state []byte) (Presence, error) {
	store.heartbeats++
	return Presence{Room: room, ConnectionID: principal.ConnectionID, State: state}, nil
}
func (*presenceStoreFake) ListActive(context.Context, Room) ([]Presence, error) { return nil, nil }
func (store *presenceStoreFake) Leave(context.Context, Principal, Room) error {
	store.leaves++
	return nil
}
func (*presenceStoreFake) DeleteExpired(context.Context) (int64, error) { return 0, nil }

func TestPresenceServiceAllowsEditorsAndKeepsViewersReadOnly(t *testing.T) {
	store := &presenceStoreFake{}
	access := &presenceAccessFake{decision: AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: 3, ExpiresAt: time.Now().Add(time.Minute),
	}}
	service, err := NewPresenceService(store, access)
	if err != nil {
		t.Fatal(err)
	}
	principal := Principal{WorkspaceID: 7, UserID: 5, ConnectionID: "connection"}
	room := Room{WorkspaceID: 7, ResourceID: "resource"}
	if _, err := service.Heartbeat(context.Background(), principal, room, []byte(`{"cell":"a1"}`)); err != nil {
		t.Fatal(err)
	}
	access.decision.CanEdit = false
	if _, err := service.Heartbeat(context.Background(), principal, room, []byte(`{"cell":"a2"}`)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer heartbeat error=%v", err)
	}
	if store.heartbeats != 1 {
		t.Fatalf("heartbeats=%d", store.heartbeats)
	}
	if err := service.Leave(context.Background(), principal, room); err != nil || store.leaves != 1 {
		t.Fatalf("leave error=%v count=%d", err, store.leaves)
	}
}
