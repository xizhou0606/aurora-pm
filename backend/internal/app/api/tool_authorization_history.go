package api

import (
	"context"
	"net/http"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type historyActionAccess interface {
	Authorize(context.Context, uint, string, string) (identity.AuthorizationResult, error)
}

// Domain composition only: identity owns route permissions and historical
// user profiles; toolruntime owns workspace/object filtering and revocation.
type toolAuthorizationHistoryAccess struct {
	actions historyActionAccess
	users   identity.UserDisplayReader
}

func (a toolAuthorizationHistoryAccess) CanManage(ctx context.Context, p toolruntime.ManagementPrincipal) (bool, error) {
	decision, err := a.actions.Authorize(ctx, p.PrincipalID, http.MethodPut, "/api/v1/platform/external-tooling/configuration")
	return err == nil && decision == identity.AuthorizationAllowed, err
}

func (a toolAuthorizationHistoryAccess) CanRevoke(ctx context.Context, p toolruntime.ManagementPrincipal) (bool, error) {
	decision, err := a.actions.Authorize(ctx, p.PrincipalID, http.MethodPost, "/api/v1/platform/external-tooling/authorization-records/:id/revoke")
	return err == nil && decision == identity.AuthorizationAllowed, err
}

func (a toolAuthorizationHistoryAccess) Names(ctx context.Context, ids []uint) (map[uint]toolruntime.AuthorizationUser, error) {
	profiles, err := a.users.ReadUserDisplays(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]toolruntime.AuthorizationUser, len(profiles))
	for id, user := range profiles {
		out[id] = toolruntime.AuthorizationUser{Name: user.Name, Username: user.Username}
	}
	return out, nil
}
