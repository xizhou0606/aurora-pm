package toolruntimehttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type OAuthHandler struct {
	service *toolruntime.OAuthService
}

func NewOAuthHandler(service *toolruntime.OAuthService) *OAuthHandler {
	return &OAuthHandler{service: service}
}

func (handler *OAuthHandler) RegisterDiscoveryRoutes(routes gin.IRoutes) {
	routes.GET("/.well-known/oauth-protected-resource", handler.mcpProtectedResource)
	routes.GET("/.well-known/oauth-protected-resource/api/v1/mcp", handler.mcpProtectedResource)
	routes.GET("/.well-known/oauth-protected-resource/api/v1/tool-runtime", handler.cliProtectedResource)
	routes.GET("/.well-known/oauth-authorization-server", handler.authorizationServer)
	routes.GET("/.well-known/oauth-authorization-server/api/v1/tool-auth", handler.authorizationServer)
}

func (handler *OAuthHandler) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.POST("/tool-auth/register", handler.register)
}

func (handler *OAuthHandler) mcpProtectedResource(c *gin.Context) {
	handler.protectedResource(c, toolruntime.DeliveryMCP)
}

func (handler *OAuthHandler) cliProtectedResource(c *gin.Context) {
	handler.protectedResource(c, toolruntime.DeliveryCLI)
}

func (handler *OAuthHandler) protectedResource(c *gin.Context, channel toolruntime.DeliveryChannel) {
	if handler == nil || handler.service == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_OAUTH_UNAVAILABLE", "external OAuth is unavailable")
		return
	}
	configuration := handler.service.Configuration()
	resource := configuration.MCPResource
	if channel == toolruntime.DeliveryCLI {
		resource = configuration.CLIResource
	}
	noStore(c)
	c.JSON(http.StatusOK, gin.H{
		"resource":              resource,
		"authorization_servers": []string{configuration.Issuer},
		"scopes_supported":      []string{toolruntime.OAuthScopeTools, toolruntime.OAuthScopeOfflineAccess},
	})
}

func (handler *OAuthHandler) authorizationServer(c *gin.Context) {
	if handler == nil || handler.service == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_OAUTH_UNAVAILABLE", "external OAuth is unavailable")
		return
	}
	configuration := handler.service.Configuration()
	noStore(c)
	c.JSON(http.StatusOK, gin.H{
		"issuer": configuration.Issuer,
		"authorization_response_iss_parameter_supported": true,
		"authorization_endpoint":                         configuration.AuthorizationEndpoint,
		"device_authorization_endpoint":                  configuration.DeviceAuthorizationEndpoint,
		"token_endpoint":                                 configuration.TokenEndpoint,
		"registration_endpoint":                          configuration.RegistrationEndpoint,
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token", toolruntime.DeviceAuthorizationGrantType},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"code_challenge_methods_supported":               []string{toolruntime.PKCEMethodS256},
		"scopes_supported":                               []string{toolruntime.OAuthScopeTools, toolruntime.OAuthScopeOfflineAccess},
	})
}

func (handler *OAuthHandler) register(c *gin.Context) {
	if handler == nil || handler.service == nil {
		credentialError(c, http.StatusServiceUnavailable, "EXTERNAL_OAUTH_UNAVAILABLE", "external OAuth is unavailable")
		return
	}
	var body struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
	}
	if !oauthRegistrationJSON(c, &body) {
		return
	}
	if (body.TokenEndpointAuthMethod != "" && body.TokenEndpointAuthMethod != "none") ||
		!subsetWithRequired(body.GrantTypes, []string{"authorization_code", "refresh_token"}, "authorization_code") ||
		!subsetWithRequired(body.ResponseTypes, []string{"code"}, "code") {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_client_metadata", "client metadata is invalid")
		return
	}
	client, err := handler.service.RegisterClient(c.Request.Context(), toolruntime.OAuthClientRegistrationInput{
		Name: body.ClientName, RedirectURIs: body.RedirectURIs,
	})
	if err != nil {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_redirect_uri", "redirect URI is invalid")
		return
	}
	noStore(c)
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  client.ID,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

// RFC 7591 requires authorization servers to ignore unrecognized client
// metadata. Keep the body bounded and syntactically strict, but do not apply
// the internal management API's unknown-field rejection to interoperable DCR.
func oauthRegistrationJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCredentialBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(target); err != nil {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_client_metadata", "client metadata is invalid")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		oauthProtocolError(c, http.StatusBadRequest, "invalid_client_metadata", "client metadata is invalid")
		return false
	}
	return true
}

func subsetWithRequired(actual, allowed []string, required string) bool {
	if len(actual) == 0 {
		return true
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	seen := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		value = strings.TrimSpace(value)
		if _, ok := allowedSet[value]; !ok {
			return false
		}
		seen[value] = struct{}{}
	}
	_, ok := seen[required]
	return ok
}

func oauthProtocolError(c *gin.Context, status int, code, description string) {
	noStore(c)
	c.AbortWithStatusJSON(status, gin.H{"error": code, "error_description": description})
}
