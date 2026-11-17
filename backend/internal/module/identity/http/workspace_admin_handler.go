package identityhttp

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type createWorkspaceRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type patchWorkspaceRequest struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	IsDefault   *bool   `json:"is_default"`
}

type createWorkspaceMemberRequest struct {
	UserID  uint   `json:"user_id"`
	Role    string `json:"role"`
	Enabled *bool  `json:"enabled"`
}

type patchWorkspaceMemberRequest struct {
	UserID  *uint   `json:"user_id"`
	Role    *string `json:"role"`
	Enabled *bool   `json:"enabled"`
}

func (handler *AccessHandler) requireWorkspaceAdmin(c *gin.Context) bool {
	if handler.workspace != nil {
		return true
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "workspace administration is unavailable"})
	return false
}

func (handler *AccessHandler) listWorkspaces(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	includeDisabled, ok := accessIncludeDisabled(c)
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	items, err := handler.workspace.List(c.Request.Context(), principal.UserID, includeDisabled)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"items": items})
}

func (handler *AccessHandler) getWorkspace(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	workspace, err := handler.workspace.Workspace(c.Request.Context(), principal.UserID, workspaceID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, workspace)
}

func (handler *AccessHandler) createWorkspace(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	var request createWorkspaceRequest
	if !accessJSON(c, &request) {
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	workspace, err := handler.workspace.Create(c.Request.Context(), identity.WorkspaceInput{
		Name: request.Name, Slug: request.Slug, Description: request.Description,
		Enabled: enabled, IsDefault: false,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, workspace)
}

func (handler *AccessHandler) patchWorkspace(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	var request patchWorkspaceRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Slug != nil {
		accessBad(c, "slug is immutable")
		return
	}
	workspace, err := handler.workspace.Update(c.Request.Context(), principal.UserID, workspaceID, identity.WorkspaceUpdate{
		Name: request.Name, Description: request.Description, Enabled: request.Enabled, IsDefault: request.IsDefault,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, workspace)
}

func (handler *AccessHandler) deleteWorkspace(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	if err := handler.workspace.Delete(c.Request.Context(), principal.UserID, workspaceID); err != nil {
		accessFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccessHandler) listWorkspaceMembers(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	includeDisabled, ok := accessIncludeDisabled(c)
	if !ok {
		return
	}
	page, err := handler.workspace.ListMembers(c.Request.Context(), principal.UserID, workspaceID, identity.WorkspaceMemberFilter{
		Query: strings.TrimSpace(c.Query("q")), IncludeDisabled: includeDisabled,
		Page: accessIntQuery(c, "page"), PageSize: accessIntQuery(c, "page_size"),
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, page)
}

func (handler *AccessHandler) createWorkspaceMember(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	var request createWorkspaceMemberRequest
	if !accessJSON(c, &request) {
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	member, err := handler.workspace.AddMember(c.Request.Context(), principal.UserID, workspaceID, identity.WorkspaceMemberInput{
		UserID: request.UserID, Role: request.Role, Enabled: enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, member)
}

func (handler *AccessHandler) patchWorkspaceMember(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request patchWorkspaceMemberRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.UserID != nil {
		accessBad(c, "user_id is immutable")
		return
	}
	member, err := handler.workspace.UpdateMember(c.Request.Context(), principal.UserID, workspaceID, userID, identity.WorkspaceMemberUpdate{
		Role: request.Role, Enabled: request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, member)
}

func (handler *AccessHandler) deleteWorkspaceMember(c *gin.Context) {
	if !handler.requireWorkspaceAdmin(c) {
		return
	}
	workspaceID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	principal, ok := baselinePrincipal(c)
	if !ok {
		return
	}
	if err := handler.workspace.DeleteMember(c.Request.Context(), principal.UserID, workspaceID, userID); err != nil {
		accessFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
