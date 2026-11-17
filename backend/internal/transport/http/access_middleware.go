package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// authorizeAccess is the sole online HTTP authorization gate. It receives a
// PM principal plus Gin's matched route template. Catalog-declared object
// policies defer a denied global Action to the owning handler, which must
// enforce the registered object evidence before mutation.
func (s *Server) authorizeAccess(c *gin.Context) {
	if s.cfg.RBACMode != config.RBACModeEnforce {
		c.Next()
		return
	}

	method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
	routeTemplate := CanonicalRouteTemplate(c.FullPath())
	if isAuthenticatedSystemRoute(method, routeTemplate) {
		c.Next()
		return
	}
	if !s.requireAccessRoute(c, method, routeTemplate) {
		return
	}
	c.Next()
}

// requireAccessRoute applies the same authoritative route gate from handlers
// that retain a broad compatibility endpoint while dispatching to a narrower
// canonical action route.
func (s *Server) requireAccessRoute(c *gin.Context, method, routeTemplate string) bool {
	if s.cfg.RBACMode != config.RBACModeEnforce {
		return true
	}
	if s.authorizer == nil {
		failCode(c, http.StatusForbidden, apiErrorAccessBootstrapRequired, "access authorization is not initialized")
		return false
	}
	principal, ok := currentPrincipal(c)
	if !ok {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return false
	}
	result, err := s.authorizer.Authorize(
		c.Request.Context(),
		principal.UserID,
		strings.ToUpper(strings.TrimSpace(method)),
		CanonicalRouteTemplate(routeTemplate),
	)
	if err != nil {
		fail(c, http.StatusInternalServerError, "access authorization is unavailable")
		return false
	}
	switch result {
	case identity.AuthorizationAllowed:
		return true
	case identity.AuthorizationInterfaceUnregistered:
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceUnregistered, "service interface is not registered")
	default:
		if _, configured := accesscatalog.ObjectPolicyForRoute(method, routeTemplate); configured {
			return true
		}
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
	}
	return false
}
