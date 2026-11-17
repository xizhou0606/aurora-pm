package toolruntimehttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func (handler *CredentialHandler) startDeviceAuthorization(c *gin.Context) {
	if handler == nil || handler.device == nil || handler.oauth == nil {
		oauthProtocolError(c, http.StatusServiceUnavailable, "temporarily_unavailable", "device authorization is unavailable")
		return
	}
	if !strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/x-www-form-urlencoded") {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "device authorization request is invalid")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialBodyBytes)
	if err := c.Request.ParseForm(); err != nil || c.Request.Form.Get("client_secret") != "" {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "device authorization request is invalid")
		return
	}
	clientID := strings.TrimSpace(c.Request.Form.Get("client_id"))
	resource := strings.TrimSpace(c.Request.Form.Get("resource"))
	channel, err := handler.oauth.ResolveClientChannel(c.Request.Context(), clientID, resource)
	workspaceID, parseErr := strconv.ParseUint(strings.TrimSpace(c.Request.Form.Get("workspace_id")), 10, 64)
	if err != nil || channel != toolruntime.DeliveryCLI || parseErr != nil || workspaceID == 0 {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "device authorization request is invalid")
		return
	}
	started, err := handler.device.Start(c.Request.Context(), toolruntime.StartDeviceAuthorizationInput{
		ClientID: clientID, WorkspaceID: uint(workspaceID), Resource: resource, Scope: c.Request.Form.Get("scope"),
	})
	if err != nil {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "device authorization request is invalid")
		return
	}
	noStore(c)
	c.JSON(http.StatusOK, gin.H{
		"device_code": started.DeviceCode, "user_code": started.UserCode,
		"verification_uri": started.VerificationURI, "verification_uri_complete": started.VerificationURIComplete,
		"expires_in": int64(started.ExpiresIn.Seconds()), "interval": int64(started.Interval.Seconds()),
	})
}

func (handler *CredentialHandler) deviceAuthorization(c *gin.Context) {
	if handler == nil || handler.device == nil {
		managementError(c, http.StatusServiceUnavailable, "DEVICE_AUTHORIZATION_UNAVAILABLE", "device authorization is unavailable")
		return
	}
	principal, ok := handler.managementPrincipal(c)
	if !ok {
		return
	}
	review, err := handler.device.Review(c.Request.Context(), principal, c.Query("user_code"))
	if err != nil {
		deviceManagementFailure(c, err)
		return
	}
	toolOK(c, deviceAuthorizationReview(review))
}

func (handler *CredentialHandler) decideDeviceAuthorization(c *gin.Context) {
	if handler == nil || handler.device == nil {
		managementError(c, http.StatusServiceUnavailable, "DEVICE_AUTHORIZATION_UNAVAILABLE", "device authorization is unavailable")
		return
	}
	principal, ok := handler.managementPrincipal(c)
	if !ok {
		return
	}
	var body struct {
		UserCode string                            `json:"user_code"`
		Decision toolruntime.AuthorizationDecision `json:"decision"`
	}
	if !credentialJSON(c, &body) {
		return
	}
	review, err := handler.device.Decide(c.Request.Context(), principal, body.UserCode, body.Decision)
	if err != nil {
		deviceManagementFailure(c, err)
		return
	}
	toolOK(c, deviceAuthorizationReview(review))
}

func deviceAuthorizationReview(review toolruntime.DeviceAuthorizationReview) gin.H {
	tools := make([]gin.H, 0, len(review.Tools))
	for _, definition := range review.Tools {
		tools = append(tools, managementTool(definition))
	}
	var effectiveFrom, expiresAt any
	if !review.EffectiveFrom.IsZero() && !review.GrantExpiresAt.IsZero() {
		effectiveFrom, expiresAt = review.EffectiveFrom, review.GrantExpiresAt
	}
	return gin.H{
		"user_code": review.UserCode, "status": review.Status,
		"client": gin.H{"id": review.ClientID, "name": review.ClientName}, "channel": toolruntime.DeliveryCLI,
		"workspace": gin.H{"id": review.WorkspaceID, "name": review.WorkspaceName},
		"mode":      review.Mode, "tools": tools, "effective_from": effectiveFrom, "expires_at": expiresAt,
		"request_expires_at": review.RequestExpiresAt,
	}
}

func deviceTokenError(c *gin.Context, err error) bool {
	code := ""
	description := ""
	switch {
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationPending):
		code, description = "authorization_pending", "authorization is pending"
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationSlowDown):
		code, description = "slow_down", "polling is too frequent"
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationDenied):
		code, description = "access_denied", "authorization was denied"
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationExpired):
		code, description = "expired_token", "device authorization expired"
	}
	if code == "" {
		return false
	}
	oauthProtocolError(c, http.StatusBadRequest, code, description)
	return true
}

func deviceManagementFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationExpired):
		managementError(c, http.StatusGone, "DEVICE_AUTHORIZATION_EXPIRED", "device authorization expired")
	case errors.Is(err, toolruntime.ErrDeviceAuthorizationInvalid):
		managementError(c, http.StatusNotFound, "DEVICE_AUTHORIZATION_NOT_FOUND", "device authorization not found")
	default:
		managementFailure(c, err)
	}
}
