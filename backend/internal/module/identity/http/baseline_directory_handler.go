package identityhttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type BaselineDirectoryHandler struct {
	service *identity.BaselineDirectoryService
}

func NewBaselineDirectoryHandler(service *identity.BaselineDirectoryService) *BaselineDirectoryHandler {
	return &BaselineDirectoryHandler{service: service}
}

func (handler *BaselineDirectoryHandler) RegisterAuthenticatedRoutes(group *gin.RouterGroup) {
	group.GET("/auth/me/workspaces", handler.workspaces)
}

func (handler *BaselineDirectoryHandler) RegisterWorkspaceRoutes(group *gin.RouterGroup) {
	group.GET("/directory/users", handler.users)
	group.GET("/directory/teams", handler.teams)
	group.GET("/directory/teams/:id/members", handler.teamMembers)
}

func (handler *BaselineDirectoryHandler) workspaces(c *gin.Context) {
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	items, err := handler.service.ListAvailableWorkspaces(c.Request.Context(), principal.UserID)
	if err != nil {
		baselineDirectoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"items": items}})
}

func baselinePrincipal(c *gin.Context) (pmauth.Principal, bool) {
	value, exists := c.Get(pmauth.PrincipalContextKey)
	principal, ok := value.(pmauth.Principal)
	if !exists || !ok || principal.UserID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": "SESSION_EXPIRED", "error": "authentication required",
		})
		return pmauth.Principal{}, false
	}
	return principal, true
}

func (handler *BaselineDirectoryHandler) users(c *gin.Context) {
	workspaceID, ok := baselineWorkspaceID(c)
	if !ok {
		return
	}
	limit, err := baselineNonNegativeInt(c.Query("limit"), 100)
	if err != nil || limit < 1 {
		baselineDirectoryError(c, identity.ErrBaselineDirectoryInvalid)
		return
	}
	offset, err := baselineNonNegativeInt(c.Query("offset"), 0)
	if err != nil {
		baselineDirectoryError(c, identity.ErrBaselineDirectoryInvalid)
		return
	}
	exact := false
	if raw, exists := c.GetQuery("exact"); exists {
		exact, err = strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			baselineDirectoryError(c, identity.ErrBaselineDirectoryInvalid)
			return
		}
	}
	ids, err := baselineUintList(c.Query("ids"))
	if err != nil {
		baselineDirectoryError(c, identity.ErrBaselineDirectoryInvalid)
		return
	}
	page, err := handler.service.SearchUsers(c.Request.Context(), workspaceID, identity.DirectoryUserFilter{
		Query: c.Query("q"), Username: c.Query("username"), IDs: ids,
		Exact: exact, Limit: limit, Offset: offset,
	})
	if err != nil {
		baselineDirectoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": page})
}

func baselineUintList(raw string) ([]uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 100 {
		return nil, identity.ErrBaselineDirectoryInvalid
	}
	result := make([]uint, 0, len(parts))
	seen := make(map[uint]struct{}, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err != nil || value == 0 {
			return nil, identity.ErrBaselineDirectoryInvalid
		}
		id := uint(value)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func (handler *BaselineDirectoryHandler) teams(c *gin.Context) {
	workspaceID, ok := baselineWorkspaceID(c)
	if !ok {
		return
	}
	items, err := handler.service.ListTeams(c.Request.Context(), workspaceID)
	if err != nil {
		baselineDirectoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"items": items}})
}

func (handler *BaselineDirectoryHandler) teamMembers(c *gin.Context) {
	workspaceID, ok := baselineWorkspaceID(c)
	if !ok {
		return
	}
	teamID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || teamID == 0 {
		baselineDirectoryError(c, identity.ErrBaselineDirectoryInvalid)
		return
	}
	items, err := handler.service.ListTeamMembers(c.Request.Context(), workspaceID, uint(teamID))
	if err != nil {
		baselineDirectoryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"items": items}})
}

func baselineWorkspaceID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, ok := value.(uint)
	if !exists || !ok || workspaceID == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"code": "WORKSPACE_CONTEXT_REQUIRED", "error": "workspace context is required",
		})
		return 0, false
	}
	return workspaceID, true
}

func baselineNonNegativeInt(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, identity.ErrBaselineDirectoryInvalid
	}
	return value, nil
}

func baselineDirectoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrBaselineDirectoryInvalid):
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"code": "BASELINE_DIRECTORY_QUERY_INVALID", "error": "baseline directory query is invalid",
		})
	case errors.Is(err, identity.ErrBaselineDirectoryTeamMissing):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"code": "BASELINE_DIRECTORY_TEAM_NOT_FOUND", "error": "team not found",
		})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code": "BASELINE_DIRECTORY_UNAVAILABLE", "error": "baseline directory is unavailable",
		})
	}
}
