package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

func (s *Server) authenticate(c *gin.Context) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		fail(c, http.StatusUnauthorized, "bearer token required")
		return
	}
	rawToken := strings.TrimSpace(header[7:])
	identity, err := s.authenticateToken(c, rawToken)
	if err != nil {
		if errors.Is(err, errAuthUpstreamUnavailable) {
			failCode(c, http.StatusServiceUnavailable, apiErrorAuthUpstreamUnavailable, "shared authentication is unavailable")
		} else if errors.Is(err, errIdentityNotProvisioned) {
			failCode(c, http.StatusForbidden, apiErrorIdentityNotProvisioned, "shared identity is not provisioned")
		} else {
			failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "invalid or expired token")
		}
		return
	}
	user, err := s.resolveIdentity(identity)
	if err != nil {
		failCode(c, http.StatusForbidden, apiErrorIdentityNotProvisioned, err.Error())
		return
	}
	a := actor{User: user, Source: identity.Source}
	c.Set(actorKey, a)
	c.Set(pmauth.PrincipalContextKey, pmauth.Principal{
		Issuer: identity.Issuer, Subject: identity.Subject, UserID: user.ID,
		Username: user.Username, DisplayName: user.DisplayName, Email: user.Email,
	})
	c.Set(pmauth.ActorFactUserIDContextKey, factUserID(user))
	c.Next()
}

func (s *Server) resolveIdentity(identity pmauth.Identity) (identitygorm.User, error) {
	// Every authentication mode resolves to an existing PM user. Shared bearer
	// identities are namespaced by issuer + subject and are never JIT-created.
	if identity.Source == "local" || identity.Source == "pm_session" {
		var user identitygorm.User
		if err := s.db.Where("id = ?", identity.UserID).First(&user).Error; err != nil {
			return identitygorm.User{}, errors.New("user is missing")
		}
		if !identityCanAuthenticate(user) {
			return identitygorm.User{}, errors.New("user is disabled")
		}
		return user, nil
	}
	if strings.TrimSpace(identity.Issuer) == "" || strings.TrimSpace(identity.Subject) == "" {
		return identitygorm.User{}, errors.New("shared identity issuer and subject are required")
	}
	var user identitygorm.User
	if err := s.db.Where("external_issuer = ? AND external_subject = ?",
		strings.TrimSpace(identity.Issuer), strings.TrimSpace(identity.Subject),
	).First(&user).Error; err != nil {
		return identitygorm.User{}, errors.New("shared identity is missing")
	}
	if !identityCanAuthenticate(user) {
		return identitygorm.User{}, errors.New("shared identity is disabled")
	}
	return user, nil
}

func identityCanAuthenticate(user identitygorm.User) bool {
	return user.Active || identity.EffectiveAccountStatus(user.Status, user.Active) == identity.AccountBanned
}

func effectiveAccountStatus(user identitygorm.User) identity.AccountStatus {
	return identity.EffectiveAccountStatus(user.Status, user.Active)
}

// enforceAccountAdmission is intentionally evaluated before RBAC. A valid PM
// identity may inspect its own state or log out, but non-active identities can
// never reach workspace or business handlers regardless of assigned grants.
func (s *Server) enforceAccountAdmission(c *gin.Context) {
	a, ok := currentActor(c)
	if !ok {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return
	}
	status := effectiveAccountStatus(a.User)
	systemRoute := isAccountAdmissionSystemRoute(
		strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		CanonicalRouteTemplate(c.FullPath()),
	)
	if systemRoute {
		c.Next()
		return
	}
	if status.AllowsBusinessAccess() {
		if a.User.PasswordResetRequired {
			if isRequiredPasswordChangeRoute(
				strings.ToUpper(strings.TrimSpace(c.Request.Method)),
				CanonicalRouteTemplate(c.FullPath()),
			) {
				c.Next()
				return
			}
			failCode(c, http.StatusForbidden, apiErrorPasswordResetRequired, "password reset is required")
			return
		}
		c.Next()
		return
	}
	switch status {
	case identity.AccountPendingApproval:
		failCode(c, http.StatusForbidden, apiErrorIdentityApprovalPending, "identity approval is pending")
	case identity.AccountRejected:
		failCode(c, http.StatusForbidden, apiErrorIdentityRejected, "identity approval was rejected")
	default:
		failCode(c, http.StatusForbidden, apiErrorIdentityBanned, "identity is banned")
	}
}

func isRequiredPasswordChangeRoute(method, routeTemplate string) bool {
	switch method + " " + routeTemplate {
	case "PUT /api/v1/core/auth/me/password", "PUT /core/auth/me/password":
		return true
	default:
		return false
	}
}

func isAccountAdmissionSystemRoute(method, routeTemplate string) bool {
	switch method + " " + routeTemplate {
	case "GET /api/v1/core/auth/me", "POST /api/v1/core/auth/logout",
		"GET /core/auth/me", "POST /core/auth/logout":
		return true
	default:
		return false
	}
}

func (s *Server) requireWorkspace(c *gin.Context) {
	principal, ok := currentPrincipal(c)
	if !ok {
		fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	workspaceID := headerWorkspaceID(c)
	if workspaceID == 0 {
		failCode(c, http.StatusBadRequest, apiErrorWorkspaceContextRequired, "X-Workspace-ID is required")
		return
	}
	var workspace identitygorm.Workspace
	err := s.db.Where("id = ? AND enabled = ?", workspaceID, true).First(&workspace).Error
	if err != nil {
		if isNotFound(err) {
			failCode(c, http.StatusNotFound, apiErrorWorkspaceNotFound, "workspace not found")
		} else {
			dbFail(c)
		}
		return
	}
	if s.workspaceMemberships == nil {
		failCode(c, http.StatusServiceUnavailable, apiErrorWorkspaceNotFound, "workspace access is unavailable")
		return
	}
	if err := s.workspaceMemberships.VerifyWorkspaceMembership(c.Request.Context(), principal.UserID, workspaceID); err != nil {
		if errors.Is(err, identity.ErrWorkspaceMembershipInactive) {
			failCode(c, http.StatusNotFound, apiErrorWorkspaceNotFound, "workspace not found")
		} else {
			dbFail(c)
		}
		return
	}
	c.Set(workspaceKey, workspace)
	c.Set(pmauth.WorkspaceIDContextKey, workspace.ID)
	c.Next()
}

func headerWorkspaceID(c *gin.Context) uint {
	value, _ := strconv.ParseUint(strings.TrimSpace(c.GetHeader("X-Workspace-ID")), 10, 64)
	return uint(value)
}

func (s *Server) cors(c *gin.Context) {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" && s.originAllowed(origin) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Workspace-ID, X-Timezone")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition, ETag, X-PM-Cover-Offset-X, X-PM-Cover-Offset-Y")
	}
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}

func (s *Server) originAllowed(origin string) bool {
	for _, configured := range s.cfg.CORSOrigins {
		if configured == "*" || strings.EqualFold(configured, origin) {
			return true
		}
	}
	return false
}
