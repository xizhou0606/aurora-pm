package toolruntimehttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const maxCredentialBodyBytes = 64 << 10

type CredentialHandler struct {
	history  *toolruntime.AuthorizationHistoryService
	service  *toolruntime.CredentialService
	resolver ManagementPrincipalResolver
	oauth    *toolruntime.OAuthService
	device   *toolruntime.DeviceAuthorizationService
}

func NewCredentialHandler(
	service *toolruntime.CredentialService,
	resolver ManagementPrincipalResolver,
	oauth ...*toolruntime.OAuthService,
) *CredentialHandler {
	var oauthService *toolruntime.OAuthService
	if len(oauth) > 0 {
		oauthService = oauth[0]
	}
	return &CredentialHandler{service: service, resolver: resolver, oauth: oauthService}
}

func (handler *CredentialHandler) WithDeviceAuthorization(service *toolruntime.DeviceAuthorizationService) *CredentialHandler {
	if handler != nil {
		handler.device = service
	}
	return handler
}

// RegisterManagementRoutes mounts the authorization-code issuer below the
// PM-session protected platform group. Principal, workspace, grant, client,
// channel, and audience are resolved from persisted server facts.
func (handler *CredentialHandler) RegisterManagementRoutes(group *gin.RouterGroup) {
	group.POST("/external-tooling/authorization-requests/:id/code", handler.issueAuthorizationCode)
	group.GET("/external-tooling/authorization-records", handler.authorizationHistory)
	group.POST("/external-tooling/authorization-records/:id/revoke", handler.revokeAuthorizationHistory)
	group.GET("/external-tooling/device-authorizations", handler.deviceAuthorization)
	group.POST("/external-tooling/device-authorizations", handler.decideDeviceAuthorization)
}

// RegisterPublicRoutes mounts the external client's code exchange, refresh,
// and revocation endpoints below a dedicated unauthenticated-by-PM-session
// group such as /api/v1/tool-auth.
func (handler *CredentialHandler) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.POST("/device", handler.startDeviceAuthorization)
	group.POST("/token", handler.token)
	group.POST("/revoke", handler.revoke)
}

func (handler *CredentialHandler) issueAuthorizationCode(c *gin.Context) {
	principal, ok := handler.managementPrincipal(c)
	if !ok {
		return
	}
	var body struct {
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
		RedirectURI         string `json:"redirect_uri"`
	}
	if !credentialJSON(c, &body) {
		return
	}
	issued, err := handler.service.IssueAuthorizationCode(c.Request.Context(), toolruntime.IssueAuthorizationCodeInput{
		PrincipalID: principal.PrincipalID, WorkspaceID: principal.WorkspaceID, RequestID: c.Param("id"),
		PKCEMethod: body.CodeChallengeMethod, PKCEChallenge: body.CodeChallenge, RedirectURI: body.RedirectURI,
	})
	if err != nil {
		credentialError(c, http.StatusBadRequest, "AUTHORIZATION_CODE_REQUEST_INVALID", "authorization code request is invalid")
		return
	}
	noStore(c)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"code": issued.Code, "expires_at": issued.ExpiresAt}})
}

func (handler *CredentialHandler) token(c *gin.Context) {
	if handler == nil || handler.service == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_CREDENTIALS_UNAVAILABLE", "external credentials are unavailable")
		return
	}
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/x-www-form-urlencoded") {
		handler.oauthToken(c)
		return
	}
	var body struct {
		GrantType    string                      `json:"grant_type"`
		Code         string                      `json:"code"`
		CodeVerifier string                      `json:"code_verifier"`
		RedirectURI  string                      `json:"redirect_uri"`
		RefreshToken string                      `json:"refresh_token"`
		ClientID     string                      `json:"client_id"`
		Channel      toolruntime.DeliveryChannel `json:"channel"`
	}
	if !credentialJSON(c, &body) {
		return
	}
	var pair toolruntime.IssuedTokenPair
	var err error
	switch strings.TrimSpace(body.GrantType) {
	case "authorization_code":
		if body.RefreshToken != "" {
			err = toolruntime.ErrAuthorizationCodeInvalid
			break
		}
		pair, err = handler.service.ExchangeAuthorizationCode(c.Request.Context(), toolruntime.ExchangeAuthorizationCodeInput{
			Code: body.Code, PKCEVerifier: body.CodeVerifier, RedirectURI: body.RedirectURI,
			ClientID: body.ClientID, Channel: body.Channel,
		})
	case "refresh_token":
		if body.Code != "" || body.CodeVerifier != "" || body.RedirectURI != "" {
			err = toolruntime.ErrRefreshTokenInvalid
			break
		}
		pair, err = handler.service.RefreshAccessToken(c.Request.Context(), toolruntime.RefreshAccessTokenInput{
			RefreshToken: body.RefreshToken, ClientID: body.ClientID, Channel: body.Channel,
		})
	default:
		err = toolruntime.ErrAuthorizationCodeInvalid
	}
	if err != nil {
		credentialError(c, http.StatusUnauthorized, "EXTERNAL_CREDENTIAL_INVALID", "external credential is invalid")
		return
	}
	noStore(c)
	toolOK(c, gin.H{
		"token_type": "Bearer", "access_token": pair.AccessToken, "access_expires_at": pair.AccessExpiresAt,
		"refresh_token": pair.RefreshToken, "refresh_expires_at": pair.RefreshExpiresAt,
	})
}

func (handler *CredentialHandler) oauthToken(c *gin.Context) {
	if handler.oauth == nil {
		oauthProtocolError(c, http.StatusServiceUnavailable, "temporarily_unavailable", "external OAuth is unavailable")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialBodyBytes)
	if err := c.Request.ParseForm(); err != nil {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "token request is invalid")
		return
	}
	if c.Request.Form.Get("client_secret") != "" {
		oauthProtocolError(c, http.StatusUnauthorized, "invalid_client", "public clients do not use a client secret")
		return
	}
	clientID := strings.TrimSpace(c.Request.Form.Get("client_id"))
	resource := strings.TrimSpace(c.Request.Form.Get("resource"))
	channel, err := handler.oauth.ResolveClientChannel(c.Request.Context(), clientID, resource)
	if err != nil {
		oauthProtocolError(c, http.StatusUnauthorized, "invalid_client", "OAuth client is invalid")
		return
	}
	var pair toolruntime.IssuedTokenPair
	switch c.Request.Form.Get("grant_type") {
	case "authorization_code":
		pair, err = handler.service.ExchangeAuthorizationCode(c.Request.Context(), toolruntime.ExchangeAuthorizationCodeInput{
			Code: c.Request.Form.Get("code"), PKCEVerifier: c.Request.Form.Get("code_verifier"),
			RedirectURI: c.Request.Form.Get("redirect_uri"), ClientID: clientID, Channel: channel,
		})
	case "refresh_token":
		pair, err = handler.service.RefreshAccessToken(c.Request.Context(), toolruntime.RefreshAccessTokenInput{
			RefreshToken: c.Request.Form.Get("refresh_token"), ClientID: clientID, Channel: channel,
		})
	case toolruntime.DeviceAuthorizationGrantType:
		if handler.device == nil || channel != toolruntime.DeliveryCLI {
			err = toolruntime.ErrDeviceAuthorizationInvalid
			break
		}
		pair, err = handler.device.Exchange(c.Request.Context(), c.Request.Form.Get("device_code"), clientID)
	default:
		err = toolruntime.ErrAuthorizationCodeInvalid
	}
	if err != nil {
		if deviceTokenError(c, err) {
			return
		}
		oauthProtocolError(c, http.StatusBadRequest, "invalid_grant", "authorization grant is invalid")
		return
	}
	accessExpiresIn := max(int64(time.Until(pair.AccessExpiresAt).Seconds()), 1)
	noStore(c)
	c.JSON(http.StatusOK, gin.H{
		"token_type": "Bearer", "access_token": pair.AccessToken,
		"expires_in": accessExpiresIn, "refresh_token": pair.RefreshToken,
	})
}

func (handler *CredentialHandler) revoke(c *gin.Context) {
	if handler == nil || handler.service == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_CREDENTIALS_UNAVAILABLE", "external credentials are unavailable")
		return
	}
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "application/x-www-form-urlencoded") {
		handler.oauthRevoke(c)
		return
	}
	var body struct {
		RefreshToken string                      `json:"refresh_token"`
		ClientID     string                      `json:"client_id"`
		Channel      toolruntime.DeliveryChannel `json:"channel"`
	}
	if !credentialJSON(c, &body) {
		return
	}
	if err := handler.service.RevokeRefreshToken(c.Request.Context(), toolruntime.RevokeRefreshTokenInput{
		RefreshToken: body.RefreshToken, ClientID: body.ClientID, Channel: body.Channel,
	}); err != nil {
		credentialError(c, http.StatusUnauthorized, "EXTERNAL_CREDENTIAL_INVALID", "external credential is invalid")
		return
	}
	noStore(c)
	c.Status(http.StatusNoContent)
}

func (handler *CredentialHandler) oauthRevoke(c *gin.Context) {
	if handler.oauth == nil {
		oauthProtocolError(c, http.StatusServiceUnavailable, "temporarily_unavailable", "external OAuth is unavailable")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialBodyBytes)
	if err := c.Request.ParseForm(); err != nil {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_request", "revocation request is invalid")
		return
	}
	clientID := strings.TrimSpace(c.Request.Form.Get("client_id"))
	resource := strings.TrimSpace(c.Request.Form.Get("resource"))
	channel, err := handler.oauth.ResolveClientChannel(c.Request.Context(), clientID, resource)
	if err != nil {
		oauthProtocolError(c, http.StatusUnauthorized, "invalid_client", "OAuth client is invalid")
		return
	}
	if err := handler.service.RevokeRefreshToken(c.Request.Context(), toolruntime.RevokeRefreshTokenInput{
		RefreshToken: c.Request.Form.Get("token"), ClientID: clientID, Channel: channel,
	}); err != nil {
		// RFC 7009 intentionally makes revocation idempotent and non-enumerable.
		noStore(c)
		c.Status(http.StatusOK)
		return
	}
	noStore(c)
	c.Status(http.StatusOK)
}

func (handler *CredentialHandler) managementPrincipal(c *gin.Context) (toolruntime.ManagementPrincipal, bool) {
	if handler == nil || handler.service == nil || handler.resolver == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_CREDENTIALS_UNAVAILABLE", "external credentials are unavailable")
		return toolruntime.ManagementPrincipal{}, false
	}
	principal, err := handler.resolver.ResolveManagementPrincipal(c)
	if err != nil || principal.PrincipalID == 0 || principal.WorkspaceID == 0 {
		credentialError(c, http.StatusUnauthorized, "EXTERNAL_TOOLING_SESSION_REQUIRED", "authentication and workspace selection are required")
		return toolruntime.ManagementPrincipal{}, false
	}
	return principal, true
}

func credentialJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		credentialError(c, http.StatusBadRequest, "EXTERNAL_CREDENTIAL_REQUEST_INVALID", "external credential request is invalid")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		credentialError(c, http.StatusBadRequest, "EXTERNAL_CREDENTIAL_REQUEST_INVALID", "external credential request is invalid")
		return false
	}
	return true
}

func credentialError(c *gin.Context, status int, code, message string) {
	noStore(c)
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}
