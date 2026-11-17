package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// requireWorkflowRequirementCreatorAction evaluates the workflow act grant
// first and falls back to immutable requirement-creator evidence when the
// catalog declares requirement.creator. It does not replace workspace,
// project, or workflow-state validation.
func (s *Server) requireWorkflowRequirementCreatorAction(
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
) bool {
	if s.cfg.RBACMode != config.RBACModeEnforce {
		return true
	}
	if s.authorizer == nil {
		failCode(c, http.StatusForbidden, apiErrorAccessBootstrapRequired, "access authorization is not initialized")
		return false
	}
	principal, exists := currentPrincipal(c)
	if !exists {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return false
	}
	registered := false
	for _, grantingAction := range accesscatalog.GrantingActions(accesscatalog.ActionWorkflowsInstancesAct) {
		authorizationRoute, exists := accesscatalog.AuthorizationRouteForAction(grantingAction)
		if !exists {
			continue
		}
		result, err := s.authorizer.Authorize(
			c.Request.Context(), principal.UserID,
			authorizationRoute.HTTPMethod, authorizationRoute.RouteTemplate,
		)
		if err != nil {
			fail(c, http.StatusInternalServerError, "access authorization is unavailable")
			return false
		}
		if result == identity.AuthorizationAllowed {
			return true
		}
		if result != identity.AuthorizationInterfaceUnregistered {
			registered = true
		}
	}
	if !registered {
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceUnregistered, "service interface is not registered")
		return false
	}
	policy, configured := accesscatalog.ObjectPolicyForAction(accesscatalog.ActionWorkflowsInstancesAct)
	if !configured || policy.Code != accesscatalog.ObjectPolicyRequirementCreator || policy.Kind != "creator_override" {
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
		return false
	}
	actorID := currentFactUserID(c)
	if actorID == 0 {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return false
	}
	if instance.SubjectType != "requirement" || instance.StartedByUserID == 0 || instance.StartedByUserID != actorID {
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
		return false
	}
	return true
}
