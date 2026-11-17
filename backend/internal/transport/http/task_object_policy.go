package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// taskCreatorBatchAccess evaluates the action grant first and only falls back
// to immutable task creator evidence when the catalog declares task.creator.
// It does not replace workspace, project, hierarchy, or lifecycle validation.
func (s *Server) taskCreatorBatchAccess(
	c *gin.Context,
	actionCode accesscatalog.ActionCode,
	tasks []taskgorm.WorkTask,
) (map[uint]bool, bool) {
	allowed := make(map[uint]bool, len(tasks))
	allowAll := func() (map[uint]bool, bool) {
		for _, task := range tasks {
			allowed[task.ID] = true
		}
		return allowed, true
	}
	if s.cfg.RBACMode != config.RBACModeEnforce {
		return allowAll()
	}
	if len(tasks) == 0 {
		return allowed, true
	}
	if s.authorizer == nil {
		failCode(c, http.StatusForbidden, apiErrorAccessBootstrapRequired, "access authorization is not initialized")
		return nil, false
	}
	principal, exists := currentPrincipal(c)
	if !exists {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return nil, false
	}
	registered := false
	for _, grantingAction := range accesscatalog.GrantingActions(actionCode) {
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
			return nil, false
		}
		if result == identity.AuthorizationAllowed {
			return allowAll()
		}
		if result != identity.AuthorizationInterfaceUnregistered {
			registered = true
		}
	}
	if !registered {
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceUnregistered, "service interface is not registered")
		return nil, false
	}
	policy, configured := accesscatalog.ObjectPolicyForAction(actionCode)
	if !configured || policy.Code != accesscatalog.ObjectPolicyTaskCreator || policy.Kind != "creator_override" {
		failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
		return nil, false
	}
	actorID := currentFactUserID(c)
	if actorID == 0 {
		failCode(c, http.StatusUnauthorized, apiErrorSessionExpired, "authentication required")
		return nil, false
	}
	ids := make([]uint, 0, len(tasks))
	workspaceID := uint(0)
	for _, task := range tasks {
		if workspaceID == 0 {
			workspaceID = task.WorkspaceID
		}
		if task.WorkspaceID == 0 || task.WorkspaceID != workspaceID {
			fail(c, http.StatusInternalServerError, "task creator authorization is unavailable")
			return nil, false
		}
		ids = append(ids, task.ID)
	}
	if s.taskCreatorPolicy == nil {
		failCode(c, http.StatusForbidden, apiErrorAccessBootstrapRequired, "task creator authorization is not initialized")
		return nil, false
	}
	creatorAllowed, err := s.taskCreatorPolicy.AllowedCreatedBy(
		c.Request.Context(), workspaceID, actorID, ids,
	)
	if err != nil {
		fail(c, http.StatusInternalServerError, "task creator authorization is unavailable")
		return nil, false
	}
	for _, task := range tasks {
		allowed[task.ID] = creatorAllowed[task.ID]
	}
	return allowed, true
}

func (s *Server) requireTaskCreatorAction(
	c *gin.Context,
	actionCode accesscatalog.ActionCode,
	tasks ...taskgorm.WorkTask,
) bool {
	allowed, valid := s.taskCreatorBatchAccess(c, actionCode, tasks)
	if !valid {
		return false
	}
	for _, task := range tasks {
		if !allowed[task.ID] {
			failCode(c, http.StatusForbidden, apiErrorAccessInterfaceForbidden, "service interface access is forbidden")
			return false
		}
	}
	return true
}
