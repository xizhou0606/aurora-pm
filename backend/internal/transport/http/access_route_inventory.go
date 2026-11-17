package httpapi

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

// RouteInventory is the completed Gin route catalog used for permission
// resource registration. It stores only canonical, non-system routes, so a
// compatibility alias can never become an independent permission key.
type RouteInventory struct {
	mu         sync.RWMutex
	candidates map[string]identity.ServiceInterfaceCandidate
}

func NewRouteInventory() *RouteInventory {
	return &RouteInventory{candidates: map[string]identity.ServiceInterfaceCandidate{}}
}

// Load snapshots routes after the server has mounted every adapter. Startup
// completes this before the engine serves requests; the mutex also keeps test
// setup and future reloads race-safe.
func (inventory *RouteInventory) Load(routes []gin.RouteInfo) {
	candidates := make(map[string]identity.ServiceInterfaceCandidate)
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		template := strings.TrimSpace(route.Path)
		canonical := CanonicalRouteTemplate(template)
		if template != canonical || !isAccessCandidate(method, canonical) {
			continue
		}
		candidate := identity.ServiceInterfaceCandidate{HTTPMethod: method, RouteTemplate: canonical}
		candidates[routeKey(method, canonical)] = candidate
	}
	inventory.mu.Lock()
	inventory.candidates = candidates
	inventory.mu.Unlock()
}

// ValidateServiceInterface accepts only a current canonical route candidate.
// In particular, callers cannot register /core or /platform compatibility
// aliases as separate interface resources.
func (inventory *RouteInventory) ValidateServiceInterface(_ context.Context, method, routeTemplate string) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	routeTemplate = strings.TrimSpace(routeTemplate)
	if routeTemplate != CanonicalRouteTemplate(routeTemplate) {
		return identity.ErrAccessRouteNotRegistered
	}
	inventory.mu.RLock()
	_, ok := inventory.candidates[routeKey(method, routeTemplate)]
	inventory.mu.RUnlock()
	if !ok {
		return identity.ErrAccessRouteNotRegistered
	}
	return nil
}

func (inventory *RouteInventory) ServiceInterfaceCandidates(_ context.Context) []identity.ServiceInterfaceCandidate {
	inventory.mu.RLock()
	result := make([]identity.ServiceInterfaceCandidate, 0, len(inventory.candidates))
	for _, candidate := range inventory.candidates {
		result = append(result, candidate)
	}
	inventory.mu.RUnlock()
	sort.Slice(result, func(left, right int) bool {
		if result[left].RouteTemplate == result[right].RouteTemplate {
			return result[left].HTTPMethod < result[right].HTTPMethod
		}
		return result[left].RouteTemplate < result[right].RouteTemplate
	})
	return result
}

// CanonicalRouteTemplate converts only the two documented compatibility route
// prefixes. It works on Gin templates, never on a caller's raw URL path.
func CanonicalRouteTemplate(routeTemplate string) string {
	routeTemplate = strings.TrimSpace(routeTemplate)
	switch {
	case routeTemplate == "/core" || strings.HasPrefix(routeTemplate, "/core/"):
		return "/api/v1" + routeTemplate
	case routeTemplate == "/platform" || strings.HasPrefix(routeTemplate, "/platform/"):
		return "/api/v1" + routeTemplate
	default:
		return routeTemplate
	}
}

func routeKey(method, routeTemplate string) string {
	return method + "\x00" + routeTemplate
}

func isAccessCandidate(method, routeTemplate string) bool {
	if !accessRegisterableMethod(method) || isPublicAccessRoute(method, routeTemplate) || isExternalToolRoute(routeTemplate) {
		return false
	}
	return !isAuthenticatedSystemRoute(method, routeTemplate)
}

// External Tool Runtime routes use their own audience-bound token plus exact
// Grant gate. They must not be registered as PM page/action RBAC interfaces.
func isExternalToolRoute(routeTemplate string) bool {
	return routeTemplate == "/api/v1/mcp" || routeTemplate == "/api/v1/tool-runtime" || routeTemplate == "/api/v1/tool-auth" ||
		strings.HasPrefix(routeTemplate, "/api/v1/tool-auth/") ||
		strings.HasPrefix(routeTemplate, "/api/v1/tool-runtime/")
}

func accessRegisterableMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
		return true
	default:
		return false
	}
}

func isPublicAccessRoute(method, routeTemplate string) bool {
	switch routeKey(method, routeTemplate) {
	case routeKey("GET", "/health"),
		routeKey("GET", "/health/live"),
		routeKey("GET", "/health/ready"),
		routeKey("GET", "/internal/health"),
		routeKey("GET", "/api/v1/public/runtime"),
		routeKey("GET", "/api/v1/core/public/runtime"),
		routeKey("GET", "/api/v1/core/public/modules"),
		routeKey("POST", "/api/v1/core/auth/login"),
		routeKey("GET", "/api/v1/core/auth/feishu/start"),
		routeKey("GET", "/api/v1/core/auth/feishu/callback"),
		routeKey("GET", "/api/v1/core/auth/google/start"),
		routeKey("GET", "/api/v1/core/auth/google/callback"),
		routeKey("GET", "/api/v1/core/auth/telegram/start"),
		routeKey("GET", "/api/v1/core/auth/telegram/callback"),
		routeKey("GET", "/api/v1/core/auth/sso/start"),
		routeKey("GET", "/api/v1/core/auth/sso/callback"),
		routeKey("POST", "/api/v1/core/auth/external-registration/complete"),
		routeKey("POST", "/api/v1/core/auth/session/exchange"):
		return true
	default:
		return false
	}
}

// These authenticated endpoints are deliberately usable by an admitted user
// with no role. Account admission still blocks pending, rejected, and banned
// accounts before these self-service routes are reached.
func isAuthenticatedSystemRoute(method, routeTemplate string) bool {
	_, ok := accesscatalog.BaselineCapabilityForRoute(method, routeTemplate)
	return ok
}

var _ identity.ServiceInterfaceInventory = (*RouteInventory)(nil)
