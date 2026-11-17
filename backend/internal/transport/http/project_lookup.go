package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

func pageNumber(c *gin.Context) int {
	value, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if value < 1 {
		return 1
	}
	return value
}

func pageSize(c *gin.Context) int {
	value, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "20")))
	if value < 1 {
		return 20
	}
	if value > 200 {
		return 200
	}
	return value
}

func (s *Server) projectByQuery(c *gin.Context, projectID uint) (projectgorm.ProjectSet, bool) {
	if projectID == 0 {
		bad(c, "project_new_id required")
		return projectgorm.ProjectSet{}, false
	}
	workspace, _ := currentWorkspace(c)
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", projectID, workspace.ID).
		First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return projectgorm.ProjectSet{}, false
	}
	return project, true
}

// currentProjectAccessUserID returns the canonical PM user ID used by project
// membership and ownership. External identity bindings do not participate.
func currentProjectAccessUserID(c *gin.Context) uint {
	return currentFactUserID(c)
}

// userCanAccessProject applies the project data-access gate. It is a
// separate layer from functional RBAC (RBACMode) — project data access is
// always enforced. A nil checker fails closed.
func (s *Server) userCanAccessProject(c *gin.Context, workspaceID, projectID uint) bool {
	if s.projectAccess == nil {
		return false
	}
	userID := currentProjectAccessUserID(c)
	if userID == 0 {
		return false
	}
	allowed, err := s.projectAccess.CanAccessProject(c.Request.Context(), workspaceID, projectID, userID)
	if err != nil {
		return false
	}
	return allowed
}

// requireProjectAccess fails with 404 when the project is missing OR the user
// has no data access; an inaccessible project is indistinguishable from a
// missing one.
func (s *Server) requireProjectAccess(c *gin.Context, workspaceID, projectID uint) bool {
	if !s.userCanAccessProject(c, workspaceID, projectID) {
		fail(c, http.StatusNotFound, "project set not found")
		return false
	}
	return true
}

// applyProjectDataFilter restricts a *gorm.DB query to the user's effective
// project scope in SQL. It is always enforced — RBACMode does not gate project
// data access. A nil checker fails closed (no rows).
func (s *Server) applyProjectDataFilter(c *gin.Context, query *gorm.DB, table, column string) bool {
	userID := currentProjectAccessUserID(c)
	if userID == 0 {
		*query = *query.Where("1 = 0")
		return true
	}
	workspace, _ := currentWorkspace(c)
	filter, ok := s.buildProjectAccessFilter(c, table, column, workspace.ID, userID)
	if !ok {
		*query = *query.Where("1 = 0")
		return true
	}
	if filter.SQL == "" {
		return true
	}
	*query = *query.Where(filter.SQL, filter.Args...)
	return true
}

// projectAccessScope resolves the caller's project membership filter as a raw
// SQL fragment for repositories that accept an AccessScope injection. An empty
// SQL is the admin fast path; ok=false fails closed.
func (s *Server) projectAccessScope(c *gin.Context, table, column string) (project.AccessFilter, bool) {
	userID := currentProjectAccessUserID(c)
	if userID == 0 {
		return project.AccessFilter{}, false
	}
	workspace, _ := currentWorkspace(c)
	return s.buildProjectAccessFilter(c, table, column, workspace.ID, userID)
}

// buildProjectAccessFilter centralizes the fact-user-ID access-filter
// resolution shared by applyProjectDataFilter and projectAccessScope.
func (s *Server) buildProjectAccessFilter(
	c *gin.Context, table, column string, workspaceID, userID uint,
) (project.AccessFilter, bool) {
	if s.projectAccess == nil {
		return project.AccessFilter{}, false
	}
	builder, ok := s.projectAccess.(project.AccessFilterBuilder)
	if !ok {
		return project.AccessFilter{}, false
	}
	filter, err := builder.BuildMemberFilter(c.Request.Context(), table, column, workspaceID, userID)
	if err != nil {
		return project.AccessFilter{}, false
	}
	return filter, true
}
