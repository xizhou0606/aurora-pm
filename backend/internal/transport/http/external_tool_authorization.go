package httpapi

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const maxExternalBearerBytes = 8 << 10

// ExternalToolAuthorizationResolver is shared by the CLI Tool API and MCP
// HTTP adapters. Channel and audience are fixed by composition, never inferred
// from request headers or bodies.
type ExternalToolAuthorizationResolver struct {
	verifier toolruntime.ExternalTokenVerifier
	channel  toolruntime.DeliveryChannel
	audience toolruntime.TokenAudience
}

func NewExternalToolAuthorizationResolver(
	verifier toolruntime.ExternalTokenVerifier,
	channel toolruntime.DeliveryChannel,
	audience toolruntime.TokenAudience,
) (*ExternalToolAuthorizationResolver, error) {
	if verifier == nil {
		return nil, toolruntime.ErrExternalTokenInvalid
	}
	expectedAudience, err := toolruntime.AudienceForChannel(channel)
	if err != nil || audience != expectedAudience {
		return nil, toolruntime.ErrExternalTokenInvalid
	}
	return &ExternalToolAuthorizationResolver{verifier: verifier, channel: channel, audience: audience}, nil
}

func (resolver *ExternalToolAuthorizationResolver) ResolveAuthorization(c *gin.Context) (toolruntime.ExecutionAuthorization, error) {
	if resolver == nil || resolver.verifier == nil || c == nil || c.Request == nil {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	}
	rawToken, ok := externalBearer(c.GetHeader("Authorization"))
	if !ok {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	}
	authorization, err := resolver.verifier.VerifyExternalToken(
		c.Request.Context(), rawToken, resolver.channel, resolver.audience,
	)
	if errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) {
		return toolruntime.ExecutionAuthorization{}, err
	}
	if err != nil || toolruntime.ValidateExternalAuthorization(authorization, resolver.channel, resolver.audience) != nil {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	}
	return authorization, nil
}

func externalBearer(header string) (string, bool) {
	if len(header) == 0 || len(header) > maxExternalBearerBytes {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
