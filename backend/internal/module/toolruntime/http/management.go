package toolruntimehttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const maxManagementBodyBytes = 64 << 10

type ManagementPrincipalResolver interface {
	ResolveManagementPrincipal(*gin.Context) (toolruntime.ManagementPrincipal, error)
}

type ManagementPrincipalResolverFunc func(*gin.Context) (toolruntime.ManagementPrincipal, error)

func (function ManagementPrincipalResolverFunc) ResolveManagementPrincipal(c *gin.Context) (toolruntime.ManagementPrincipal, error) {
	return function(c)
}

type ManagementHandler struct {
	service  *toolruntime.AuthorizationService
	resolver ManagementPrincipalResolver
	oauth    *toolruntime.OAuthService
}

func NewManagementHandler(service *toolruntime.AuthorizationService, resolver ManagementPrincipalResolver, oauth ...*toolruntime.OAuthService) *ManagementHandler {
	var oauthService *toolruntime.OAuthService
	if len(oauth) > 0 {
		oauthService = oauth[0]
	}
	return &ManagementHandler{service: service, resolver: resolver, oauth: oauthService}
}

// RegisterRoutes mounts PM-session management endpoints below an already
// authenticated platform group. It is not an external-token adapter.
func (handler *ManagementHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/tool-runtime/catalog", handler.catalog)
	group.GET("/external-tooling/effective-access", handler.effectiveAccess)
	group.GET("/external-tooling/configuration", handler.configuration)
	group.PUT("/external-tooling/configuration", handler.saveConfiguration)
	group.GET("/external-tooling/grants", handler.grants)
	group.POST("/external-tooling/authorization-requests", handler.start)
	group.GET("/external-tooling/authorization-requests/:id", handler.request)
	group.POST("/external-tooling/authorization-requests/:id/decision", handler.decision)
}

func (handler *ManagementHandler) effectiveAccess(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	access, err := handler.service.GetEffectiveAccess(c.Request.Context(), principal)
	if err != nil {
		managementFailure(c, err)
		return
	}
	tools := make([]gin.H, 0, len(access.Tools))
	for _, definition := range access.Tools {
		tools = append(tools, managementTool(definition))
	}
	var effectiveFrom, expiresAt any
	if access.Mode == toolruntime.WorkspaceConfigurationWindow {
		effectiveFrom, expiresAt = access.EffectiveFrom, access.ExpiresAt
	}
	toolOK(c, gin.H{
		"configured":          access.Configured,
		"mode":                access.Mode,
		"status":              access.Status,
		"tools":               tools,
		"effective_from":      effectiveFrom,
		"expires_at":          expiresAt,
		"cli_grant_ttl_hours": access.CLIGrantTTLHours,
	})
}

func (handler *ManagementHandler) configuration(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	configuration, err := handler.service.GetWorkspaceConfiguration(c.Request.Context(), principal)
	if err != nil {
		managementFailure(c, err)
		return
	}
	toolOK(c, managementConfiguration(configuration))
}

func (handler *ManagementHandler) saveConfiguration(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	var body struct {
		Mode             toolruntime.WorkspaceConfigurationMode `json:"mode"`
		ToolScopes       []string                               `json:"tool_scopes"`
		EffectiveFrom    *time.Time                             `json:"effective_from"`
		ExpiresAt        *time.Time                             `json:"expires_at"`
		CLIGrantTTLHours uint16                                 `json:"cli_grant_ttl_hours"`
	}
	if !managementJSONWithError(c, &body, toolruntime.ErrWorkspaceConfigurationInvalid) {
		return
	}
	configuration, err := handler.service.SaveWorkspaceConfiguration(c.Request.Context(), principal, toolruntime.WorkspaceConfigurationInput{
		Mode: body.Mode, ToolNames: body.ToolScopes,
		EffectiveFrom: timeValue(body.EffectiveFrom), ExpiresAt: timeValue(body.ExpiresAt),
		CLIGrantTTLHours: body.CLIGrantTTLHours,
	})
	if err != nil {
		managementFailure(c, err)
		return
	}
	toolOK(c, managementConfiguration(configuration))
}

func (handler *ManagementHandler) catalog(c *gin.Context) {
	if _, ok := handler.principal(c); !ok {
		return
	}
	definitions := handler.service.Catalog()
	tools := make([]gin.H, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, managementTool(definition))
	}
	toolOK(c, gin.H{"tools": tools})
}

func (handler *ManagementHandler) grants(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	summaries, err := handler.service.ListGrants(c.Request.Context(), principal)
	if err != nil {
		managementFailure(c, err)
		return
	}
	rows := make([]gin.H, 0, len(summaries))
	for _, summary := range summaries {
		rows = append(rows, gin.H{
			"id": strconv.FormatUint(uint64(summary.Grant.ID), 10), "client_name": summary.ClientName,
			"channel": summary.Grant.Channel, "status": summary.Status, "tool_scopes": summary.ToolNames,
			"effective_from": summary.Grant.EffectiveFrom, "expires_at": summary.Grant.ExpiresAt,
		})
	}
	toolOK(c, gin.H{"grants": rows})
}

func (handler *ManagementHandler) start(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	var body struct {
		Channel       toolruntime.DeliveryChannel `json:"channel"`
		ToolScopes    []string                    `json:"tool_scopes"`
		EffectiveFrom time.Time                   `json:"effective_from"`
		ExpiresAt     time.Time                   `json:"expires_at"`
		OAuth         *struct {
			ResponseType        string `json:"response_type"`
			ClientID            string `json:"client_id"`
			RedirectURI         string `json:"redirect_uri"`
			PKCEChallenge       string `json:"code_challenge"`
			PKCEChallengeMethod string `json:"code_challenge_method"`
			State               string `json:"state"`
			Resource            string `json:"resource"`
			Scope               string `json:"scope"`
		} `json:"oauth"`
	}
	if !managementJSON(c, &body) {
		return
	}
	var request toolruntime.AuthorizationRequest
	var err error
	if body.OAuth != nil {
		if handler.oauth == nil {
			managementError(c, http.StatusServiceUnavailable, "EXTERNAL_OAUTH_UNAVAILABLE", "external OAuth is unavailable")
			return
		}
		request, err = handler.oauth.StartAuthorization(c.Request.Context(), principal, toolruntime.OAuthAuthorizationStartInput{
			ResponseType: body.OAuth.ResponseType, ClientID: body.OAuth.ClientID,
			RedirectURI: body.OAuth.RedirectURI, PKCEChallenge: body.OAuth.PKCEChallenge,
			PKCEChallengeMethod: body.OAuth.PKCEChallengeMethod, State: body.OAuth.State,
			Resource: body.OAuth.Resource, Scope: body.OAuth.Scope, Channel: body.Channel,
		})
	} else {
		request, err = handler.service.Start(c.Request.Context(), principal, toolruntime.AuthorizationRequestInput{
			Channel: body.Channel, ToolNames: body.ToolScopes,
			EffectiveFrom: body.EffectiveFrom, ExpiresAt: body.ExpiresAt,
		})
	}
	if err != nil {
		managementFailure(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"request_id": request.ID}})
}

func managementConfiguration(configuration toolruntime.WorkspaceConfiguration) gin.H {
	effectiveFrom, expiresAt := configurationTimes(configuration)
	return gin.H{
		"workspace_id": configuration.WorkspaceID,
		"mode":         configuration.Mode,
		"tool_scopes":  configuration.AllowedTools, "grant_scopes": configuration.AllowedScopes,
		"effective_from": effectiveFrom, "expires_at": expiresAt,
		"cli_grant_ttl_hours": configuration.EffectiveCLIGrantTTLHours(),
		"updated_by":          configuration.UpdatedBy, "created_at": configuration.CreatedAt,
		"updated_at": configuration.UpdatedAt, "version": configuration.Version,
	}
}

func configurationTimes(configuration toolruntime.WorkspaceConfiguration) (any, any) {
	if configuration.Mode != toolruntime.WorkspaceConfigurationWindow {
		return nil, nil
	}
	return configuration.EffectiveFrom, configuration.ExpiresAt
}

func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

func (handler *ManagementHandler) request(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	request, err := handler.service.Get(c.Request.Context(), principal, c.Param("id"))
	if err != nil {
		managementFailure(c, err)
		return
	}
	response, err := handler.authorizationRequest(request)
	if err != nil {
		managementFailure(c, err)
		return
	}
	toolOK(c, response)
}

func (handler *ManagementHandler) decision(c *gin.Context) {
	principal, ok := handler.principal(c)
	if !ok {
		return
	}
	var body struct {
		Decision toolruntime.AuthorizationDecision `json:"decision"`
		OAuth    bool                              `json:"oauth"`
	}
	if !managementJSON(c, &body) {
		return
	}
	if body.OAuth {
		if handler.oauth == nil {
			managementError(c, http.StatusServiceUnavailable, "EXTERNAL_OAUTH_UNAVAILABLE", "external OAuth is unavailable")
			return
		}
		completion, err := handler.oauth.CompleteAuthorization(c.Request.Context(), principal, c.Param("id"), body.Decision)
		if err != nil {
			managementFailure(c, err)
			return
		}
		toolOK(c, gin.H{"authorization_redirect": completion.RedirectURL})
		return
	}
	if _, err := handler.service.Decide(c.Request.Context(), principal, c.Param("id"), body.Decision); err != nil {
		managementFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *ManagementHandler) principal(c *gin.Context) (toolruntime.ManagementPrincipal, bool) {
	if handler.service == nil || handler.resolver == nil {
		managementError(c, http.StatusServiceUnavailable, "EXTERNAL_TOOLING_UNAVAILABLE", "external tooling is unavailable")
		return toolruntime.ManagementPrincipal{}, false
	}
	principal, err := handler.resolver.ResolveManagementPrincipal(c)
	if err != nil || principal.PrincipalID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(principal.WorkspaceName) == "" {
		managementError(c, http.StatusUnauthorized, "EXTERNAL_TOOLING_SESSION_REQUIRED", "authentication and workspace selection are required")
		return toolruntime.ManagementPrincipal{}, false
	}
	return principal, true
}

func (handler *ManagementHandler) authorizationRequest(request toolruntime.AuthorizationRequest) (gin.H, error) {
	definitions, err := handler.service.DefinitionsFor(request)
	if err != nil {
		return nil, err
	}
	tools := make([]gin.H, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, managementTool(definition))
	}
	var effectiveFrom, expiresAt any
	if request.ConfigurationMode == toolruntime.WorkspaceConfigurationWindow || request.Channel == toolruntime.DeliveryCLI {
		effectiveFrom, expiresAt = request.EffectiveFrom, request.ExpiresAt
	}
	return gin.H{
		"id":        request.ID,
		"client":    gin.H{"id": request.ClientID, "name": request.ClientName},
		"channel":   request.Channel,
		"workspace": gin.H{"id": request.WorkspaceID, "name": request.WorkspaceName},
		"mode":      request.ConfigurationMode, "tools": tools,
		"effective_from": effectiveFrom, "expires_at": expiresAt,
		"oauth": request.OAuth != nil,
	}, nil
}

func managementTool(definition toolruntime.Definition) gin.H {
	return gin.H{
		"name": definition.Name, "version": definition.Version, "summary": definition.Summary,
		"lifecycle": definition.Lifecycle, "risk": definition.Risk,
		"authorization": gin.H{"grant_scope": definition.Authorization.GrantScope},
	}
}

func managementJSON(c *gin.Context, target any) bool {
	return managementJSONWithError(c, target, toolruntime.ErrAuthorizationRequestInvalid)
}

func managementJSONWithError(c *gin.Context, target any, invalid error) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxManagementBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		managementFailure(c, invalid)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		managementFailure(c, invalid)
		return false
	}
	return true
}

func managementFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, toolruntime.ErrWorkspaceConfigurationNotFound):
		managementError(c, http.StatusNotFound, "EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND", "external tooling configuration not found")
	case errors.Is(err, toolruntime.ErrWorkspaceConfigurationInvalid):
		managementError(c, http.StatusBadRequest, "EXTERNAL_TOOLING_CONFIGURATION_INVALID", "external tooling configuration is invalid")
	case errors.Is(err, toolruntime.ErrAuthorizationRequestNotFound):
		managementError(c, http.StatusNotFound, "AUTHORIZATION_REQUEST_NOT_FOUND", "authorization request not found")
	case errors.Is(err, toolruntime.ErrAuthorizationRequestExpired):
		managementError(c, http.StatusGone, "AUTHORIZATION_REQUEST_EXPIRED", "authorization request expired")
	case errors.Is(err, toolruntime.ErrAuthorizationRequestDecided):
		managementError(c, http.StatusConflict, "AUTHORIZATION_REQUEST_DECIDED", "authorization request already decided")
	case errors.Is(err, toolruntime.ErrAuthorizationCatalogInvalid):
		managementError(c, http.StatusUnprocessableEntity, "AUTHORIZATION_SCOPE_INVALID", "requested tools are unavailable")
	case errors.Is(err, toolruntime.ErrAuthorizationRequestInvalid), errors.Is(err, toolruntime.ErrAuthorizationDecisionInvalid), errors.Is(err, toolruntime.ErrGrantInvalid):
		managementError(c, http.StatusBadRequest, "AUTHORIZATION_REQUEST_INVALID", "authorization request is invalid")
	case errors.Is(err, toolruntime.ErrOAuthAuthorizationInvalid), errors.Is(err, toolruntime.ErrOAuthClientInvalid):
		managementError(c, http.StatusBadRequest, "OAUTH_AUTHORIZATION_INVALID", "OAuth authorization request is invalid")
	default:
		managementError(c, http.StatusInternalServerError, "EXTERNAL_TOOLING_FAILED", "external tooling operation failed")
	}
}

func managementError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
