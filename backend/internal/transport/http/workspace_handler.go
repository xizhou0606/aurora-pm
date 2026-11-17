package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"gorm.io/gorm"
)

var workspaceSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}[a-z0-9]$`)

func (s *Server) listWorkspaces(c *gin.Context) {
	includeDisabled, err := strconv.ParseBool(strings.TrimSpace(c.DefaultQuery("include_disabled", "false")))
	if err != nil {
		bad(c, "include_disabled must be a boolean")
		return
	}
	query := s.db.Table("workspaces AS w")
	if !includeDisabled {
		query = query.Where("w.enabled = ?", true)
	}
	var items []identity.ManagedWorkspace
	if err := query.Select(`w.id, w.name, w.slug, w.description, w.enabled, w.is_default,
        w.created_at, w.updated_at,
        (SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = w.id) AS member_count`).
		Order("w.is_default DESC, w.name ASC, w.id ASC").Scan(&items).Error; err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"items": items})
}

func (s *Server) createWorkspace(c *gin.Context) {
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Slug = strings.ToLower(strings.TrimSpace(body.Slug))
	if body.Name == "" || !workspaceSlugPattern.MatchString(body.Slug) {
		bad(c, "name and valid slug required")
		return
	}
	workspace := identitygorm.Workspace{Name: body.Name, Slug: body.Slug, Description: body.Description, Enabled: true}
	if err := s.db.Create(&workspace).Error; err != nil {
		bad(c, "workspace could not be created")
		return
	}
	created(c, workspaceJSON(workspace))
}

func (s *Server) getWorkspace(c *gin.Context) {
	workspace, allowed := s.workspaceByParam(c)
	if !allowed {
		return
	}
	ok(c, workspaceJSON(workspace))
}

func (s *Server) patchWorkspace(c *gin.Context) {
	workspace, allowed := s.workspaceByParam(c)
	if !allowed {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Enabled     *bool   `json:"enabled"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	updates := map[string]any{}
	if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		updates["description"] = strings.TrimSpace(*body.Description)
	}
	if body.Enabled != nil {
		updates["enabled"] = *body.Enabled
	}
	if len(updates) > 0 && s.db.Model(&workspace).Updates(updates).Error != nil {
		dbFail(c)
		return
	}
	_ = s.db.First(&workspace, workspace.ID).Error
	ok(c, workspaceJSON(workspace))
}

func (s *Server) workspaceByParam(c *gin.Context) (identitygorm.Workspace, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return identitygorm.Workspace{}, false
	}
	var workspace identitygorm.Workspace
	if err := s.db.First(&workspace, id).Error; err != nil {
		fail(c, http.StatusNotFound, "workspace not found")
		return identitygorm.Workspace{}, false
	}
	return workspace, true
}

func workspaceJSON(workspace identitygorm.Workspace) gin.H {
	return gin.H{
		"id": workspace.ID, "name": workspace.Name, "slug": workspace.Slug,
		"description": workspace.Description, "enabled": workspace.Enabled,
		"is_default": workspace.IsDefault,
		"created_at": workspace.CreatedAt, "updated_at": workspace.UpdatedAt,
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
