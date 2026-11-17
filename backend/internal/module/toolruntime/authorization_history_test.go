package toolruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type historyAccessStub struct{ manage, revoke bool }

func (a historyAccessStub) CanManage(context.Context, ManagementPrincipal) (bool, error) {
	return a.manage, nil
}
func (a historyAccessStub) CanRevoke(context.Context, ManagementPrincipal) (bool, error) {
	return a.revoke, nil
}
func (a historyAccessStub) Names(context.Context, []uint) (map[uint]AuthorizationUser, error) {
	return map[uint]AuthorizationUser{7: {Name: "Admin", Username: "admin"}}, nil
}

type historyStoreStub struct {
	scope   AuthorizationHistoryScope
	revoked bool
}

func (s *historyStoreStub) ListHistory(_ context.Context, scope AuthorizationHistoryScope, p, n int, _ time.Time) (AuthorizationHistoryPage, error) {
	s.scope = scope
	id := uint(7)
	return AuthorizationHistoryPage{Items: []AuthorizationHistoryRecord{{ID: "r_test", PrincipalID: &id, CanRevoke: true, Status: "approved"}}, Total: 1, Page: p, PageSize: n}, nil
}
func (s *historyStoreStub) RevokeHistory(_ context.Context, scope AuthorizationHistoryScope, _ string, _ time.Time) error {
	s.scope = scope
	s.revoked = true
	return nil
}
func TestAuthorizationHistoryRequiresActionAndAuthoritativeScope(t *testing.T) {
	ctx := context.Background()
	p := ManagementPrincipal{PrincipalID: 7, WorkspaceID: 2, WorkspaceName: "test"}
	store := &historyStoreStub{}
	service := NewAuthorizationHistoryService(store, historyAccessStub{})
	page, err := service.List(ctx, p, 1, 20)
	if err != nil || page.Items[0].CanRevoke || store.scope.All || store.scope.PrincipalID != 7 {
		t.Fatalf("self scope %+v %v", page, err)
	}
	if page.Items[0].PrincipalUsername == nil || *page.Items[0].PrincipalUsername != "admin" {
		t.Fatal("name hydration missing")
	}
	if err := service.Revoke(ctx, p, "r_test"); !errors.Is(err, ErrAuthorizationHistoryForbidden) || store.revoked {
		t.Fatal("missing action allowed revoke")
	}
	service = NewAuthorizationHistoryService(store, historyAccessStub{manage: true, revoke: true})
	if err := service.Revoke(ctx, p, "r_test"); err != nil || !store.scope.All || !store.revoked {
		t.Fatal("admin scope missing")
	}
	for _, args := range [][2]int{{0, 20}, {1, 101}, {1000001, 20}} {
		if _, err := service.List(ctx, p, args[0], args[1]); !errors.Is(err, ErrAuthorizationHistoryPage) {
			t.Fatalf("accepted invalid page %v", args)
		}
	}
	if _, err := service.List(ctx, ManagementPrincipal{}, 1, 20); !errors.Is(err, ErrAuthorizationHistoryForbidden) {
		t.Fatal("missing principal allowed")
	}
}
