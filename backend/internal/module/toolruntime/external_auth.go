package toolruntime

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrExternalTokenInvalid                = errors.New("external tool access token is invalid")
	ErrExternalWorkspaceMembershipInactive = errors.New("external tool workspace membership is inactive")
)

const ExternalWorkspaceMembershipMessage = "current user is not an active member of the grant workspace"

// ExternalWorkspaceMembershipError preserves only the bound workspace fact
// needed by protocol adapters. It deliberately excludes principal, Grant, and
// credential identifiers so an authorization failure cannot leak them.
type ExternalWorkspaceMembershipError struct {
	WorkspaceID uint
}

func (err ExternalWorkspaceMembershipError) Error() string {
	return ErrExternalWorkspaceMembershipInactive.Error()
}

func (err ExternalWorkspaceMembershipError) Is(target error) bool {
	return target == ErrExternalWorkspaceMembershipInactive
}

func ExternalWorkspaceID(err error) (uint, bool) {
	var membershipError ExternalWorkspaceMembershipError
	if !errors.As(err, &membershipError) || membershipError.WorkspaceID == 0 {
		return 0, false
	}
	return membershipError.WorkspaceID, true
}

// ExternalTokenVerifier is the protocol-neutral authentication port consumed
// by both external delivery adapters. Implementations must verify the opaque
// token against a trusted credential source, re-check that the principal is a
// current admitted member of the bound workspace, and return only
// server-established execution facts. Raw tokens must never be returned,
// logged, or persisted by an adapter.
type ExternalTokenVerifier interface {
	VerifyExternalToken(
		context.Context,
		string,
		DeliveryChannel,
		TokenAudience,
	) (ExecutionAuthorization, error)
}

type ExternalTokenVerifierFunc func(
	context.Context,
	string,
	DeliveryChannel,
	TokenAudience,
) (ExecutionAuthorization, error)

func (function ExternalTokenVerifierFunc) VerifyExternalToken(
	ctx context.Context,
	rawToken string,
	channel DeliveryChannel,
	audience TokenAudience,
) (ExecutionAuthorization, error) {
	return function(ctx, rawToken, channel, audience)
}

// ValidateExternalAuthorization rejects incomplete facts and any verifier
// result that does not exactly match the adapter-owned channel and audience.
func ValidateExternalAuthorization(
	authorization ExecutionAuthorization,
	channel DeliveryChannel,
	audience TokenAudience,
) error {
	if channel != DeliveryMCP && channel != DeliveryCLI {
		return ErrExternalTokenInvalid
	}
	expectedAudience, err := AudienceForChannel(channel)
	if err != nil || audience != expectedAudience {
		return ErrExternalTokenInvalid
	}
	if authorization.PrincipalID == 0 || authorization.WorkspaceID == 0 || authorization.GrantID == 0 ||
		strings.TrimSpace(authorization.ClientID) == "" || authorization.Channel != channel ||
		authorization.Audience != audience {
		return ErrExternalTokenInvalid
	}
	return nil
}
