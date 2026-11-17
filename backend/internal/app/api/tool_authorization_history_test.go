package api

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type historyAccessStub struct {
	result identity.AuthorizationResult
	err    error
	routes []string
}

func (s *historyAccessStub) Authorize(_ context.Context, id uint, method, route string) (identity.AuthorizationResult, error) {
	s.routes = append(s.routes, method+" "+route)
	if id != 47 {
		return identity.AuthorizationForbidden, nil
	}
	return s.result, s.err
}

func TestHistoryAccessChecksIndependentPermissionsAndFailsClosed(t *testing.T) {
	ctx := context.Background()
	p := toolruntime.ManagementPrincipal{PrincipalID: 47, WorkspaceID: 2}
	store := &historyAccessStub{result: identity.AuthorizationAllowed}
	a := toolAuthorizationHistoryAccess{actions: store}
	if ok, err := a.CanManage(ctx, p); err != nil || !ok {
		t.Fatalf("manage=%v %v", ok, err)
	}
	if ok, err := a.CanRevoke(ctx, p); err != nil || !ok {
		t.Fatalf("revoke=%v %v", ok, err)
	}
	if len(store.routes) != 2 || store.routes[0] != "PUT /api/v1/platform/external-tooling/configuration" || store.routes[1] != "POST /api/v1/platform/external-tooling/authorization-records/:id/revoke" {
		t.Fatalf("routes=%v", store.routes)
	}
	store.err = errors.New("permission store unavailable")
	if ok, err := a.CanManage(ctx, p); ok || err == nil {
		t.Fatalf("must fail closed: %v %v", ok, err)
	}
	if ok, err := a.CanRevoke(ctx, p); ok || err == nil {
		t.Fatalf("must fail closed: %v %v", ok, err)
	}
	store.err = nil
	store.result = identity.AuthorizationForbidden
	if ok, err := a.CanRevoke(ctx, p); ok || err != nil {
		t.Fatalf("deny=%v %v", ok, err)
	}
}
