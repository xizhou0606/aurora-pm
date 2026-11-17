// Package identityhttp adapts the identity access use cases to the PM HTTP
// contract. It has no GORM dependency; all access changes go through
// identity.AccessService.
package identityhttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type AccessHandler struct {
	access        *identity.AccessService
	personnel     *identity.PersonnelService
	workspace     *identity.WorkspaceAdminService
	inventory     identity.ServiceInterfaceInventory
	feishuOrg     *identity.FeishuOrgSyncService
	passwordReset *identity.PasswordResetService
}

func (handler *AccessHandler) SetPasswordResetService(service *identity.PasswordResetService) {
	handler.passwordReset = service
}

func (handler *AccessHandler) SetPersonnelService(personnel *identity.PersonnelService) {
	handler.personnel = personnel
}

func (handler *AccessHandler) SetWorkspaceAdminService(workspace *identity.WorkspaceAdminService) {
	handler.workspace = workspace
}

// SetFeishuOrg attaches the optional Feishu contact org sync; without it the
// sync endpoints fail closed with a dedicated conflict response.
func (handler *AccessHandler) SetFeishuOrg(feishuOrg *identity.FeishuOrgSyncService) {
	handler.feishuOrg = feishuOrg
}

func NewAccessHandler(access *identity.AccessService, inventory identity.ServiceInterfaceInventory) *AccessHandler {
	return &AccessHandler{access: access, inventory: inventory}
}

// RegisterRoutes mounts access administration below an already authenticated
// core group. Transport owns authentication and the entry authorization gate.
func (handler *AccessHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/access/me/action-permissions", handler.listMyActionPermissions)
	admin := group.Group("/access/admin")
	admin.GET("/permission-tree", handler.permissionTree)
	admin.GET("/service-interface-candidates", handler.listServiceInterfaceCandidates)
	admin.GET("/service-interfaces", handler.listServiceInterfaces)
	admin.POST("/service-interfaces", handler.createServiceInterface)
	admin.POST("/service-interfaces/bulk-register", handler.bulkCreateServiceInterfaces)
	admin.PATCH("/service-interfaces/:id", handler.patchServiceInterface)
	admin.DELETE("/service-interfaces/:id", handler.deleteServiceInterface)

	admin.GET("/roles", handler.listRoles)
	admin.POST("/roles", handler.createRole)
	admin.GET("/roles/:id", handler.getRole)
	admin.PATCH("/roles/:id", handler.patchRole)
	admin.DELETE("/roles/:id", handler.deleteRole)
	admin.PUT("/roles/:id/menu-permissions", handler.replaceRoleMenus)
	admin.PUT("/roles/:id/service-interface-permissions", handler.replaceRoleServiceInterfaces)
	admin.GET("/roles/:id/action-permissions", handler.listRoleActionCodes)
	admin.PUT("/roles/:id/action-permissions", handler.replaceRoleActionCodes)

	admin.GET("/users", handler.listUsers)
	admin.GET("/users/:user_id/roles", handler.listUserRoles)
	admin.PUT("/users/:user_id/roles", handler.replaceUserRoles)
	admin.PUT("/users/:user_id/group", handler.replaceUserGroup)
	admin.PUT("/users/:user_id/status", handler.replaceUserStatus)
	admin.PATCH("/users/:user_id/status", handler.replaceUserStatus)
	admin.POST("/users/:user_id/approve", handler.approveUser)
	admin.POST("/users/:user_id/reject", handler.rejectUser)
	admin.POST("/users/:user_id/ban", handler.banUser)
	admin.POST("/users/:user_id/unban", handler.unbanUser)
	admin.PUT("/users/:user_id/access", handler.replaceUserAccess)
	admin.POST("/users/:user_id/password-reset-links", handler.createPasswordResetLink)

	admin.GET("/groups", handler.listGroups)
	admin.POST("/groups/feishu-org/sync", handler.syncFeishuOrg)
	admin.GET("/groups/feishu-org/status", handler.feishuOrgStatus)
	admin.POST("/groups", handler.createGroup)
	admin.PATCH("/groups/:id", handler.patchGroup)
	admin.DELETE("/groups/:id", handler.deleteGroup)
	admin.GET("/groups/:id/service-interface-permissions", handler.listGroupServiceInterfaces)
	admin.PUT("/groups/:id/service-interface-permissions", handler.replaceGroupServiceInterfaces)
	admin.GET("/groups/:id/action-permissions", handler.listGroupActionCodes)
	admin.PUT("/groups/:id/action-permissions", handler.replaceGroupActionCodes)
	admin.GET("/groups/:id/menu-permissions", handler.listGroupMenus)
	admin.PUT("/groups/:id/menu-permissions", handler.replaceGroupMenus)

	group.GET("/workspaces", handler.listWorkspaces)
	group.POST("/workspaces", handler.createWorkspace)
	group.GET("/workspaces/:id", handler.getWorkspace)
	group.PATCH("/workspaces/:id", handler.patchWorkspace)
	group.DELETE("/workspaces/:id", handler.deleteWorkspace)
	group.GET("/workspaces/:id/members", handler.listWorkspaceMembers)
	group.POST("/workspaces/:id/members", handler.createWorkspaceMember)
	group.PATCH("/workspaces/:id/members/:user_id", handler.patchWorkspaceMember)
	group.DELETE("/workspaces/:id/members/:user_id", handler.deleteWorkspaceMember)
}

func (handler *AccessHandler) createPasswordResetLink(c *gin.Context) {
	if handler.passwordReset == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "password reset service is unavailable"})
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request struct {
		Notify bool `json:"notify"`
	}
	if !accessJSON(c, &request) {
		return
	}
	workspaceID, _ := strconv.ParseUint(strings.TrimSpace(c.GetHeader("X-Workspace-ID")), 10, 64)
	result, err := handler.passwordReset.Create(c.Request.Context(), actorUserID, userID, uint(workspaceID), request.Notify)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, result)
}

func (handler *AccessHandler) permissionTree(c *gin.Context) {
	pages := accesscatalog.PermissionTree()
	items := make([]permissionTreeNode, 0, len(pages))
	for _, page := range pages {
		pageNode := permissionTreeNode{Code: page.Code, Name: page.Name, Type: "page"}
		pageNode.Children = make([]permissionTreeNode, 0, len(page.Views))
		for _, view := range page.Views {
			viewNode := permissionTreeViewNode(view)
			pageNode.Grantable = pageNode.Grantable || viewNode.Grantable
			pageNode.Children = append(pageNode.Children, viewNode)
		}
		items = append(items, pageNode)
	}
	accessOK(c, gin.H{"items": items})
}

func permissionTreeViewNode(view accesscatalog.PermissionView) permissionTreeNode {
	result := permissionTreeNode{Code: view.Code, Name: view.Name, Type: "view"}
	result.Children = make([]permissionTreeNode, 0, len(view.Actions)+len(view.Children))
	for _, action := range view.Actions {
		grantable := len(action.Routes) > 0
		actionNode := permissionTreeNode{
			Code: string(action.Code), Name: action.Name, Type: "action", Grantable: grantable,
			Tools: accesscatalog.ToolNamesForAction(action.Code),
		}
		if policy, configured := accesscatalog.ObjectPolicyForAction(action.Code); configured {
			actionNode.ObjectPolicy = &permissionTreeObjectPolicy{
				Code: string(policy.Code), Kind: policy.Kind,
			}
		}
		result.Children = append(result.Children, actionNode)
		result.Grantable = result.Grantable || grantable
	}
	for _, child := range view.Children {
		childNode := permissionTreeViewNode(child)
		result.Children = append(result.Children, childNode)
		result.Grantable = result.Grantable || childNode.Grantable
	}
	return result
}

func (handler *AccessHandler) listUsers(c *gin.Context) {
	if handler.personnel != nil {
		page, err := handler.personnel.ListUsers(c.Request.Context(), identity.PersonnelUserFilter{
			Status:  identity.AccountStatus(strings.TrimSpace(c.Query("status"))),
			Query:   strings.TrimSpace(c.Query("q")),
			GroupID: accessUintQuery(c, "group_id"), RoleID: accessUintQuery(c, "role_id"),
			Page: accessIntQuery(c, "page"), PageSize: accessIntQuery(c, "page_size"),
		})
		if err != nil {
			accessFailure(c, err)
			return
		}
		accessOK(c, page)
		return
	}
	items, err := handler.access.ListActiveUsers(c.Request.Context())
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"items": items})
}

func accessIntQuery(c *gin.Context, name string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(c.Query(name)))
	return value
}

func accessUintQuery(c *gin.Context, name string) uint {
	value, _ := strconv.ParseUint(strings.TrimSpace(c.Query(name)), 10, 64)
	return uint(value)
}

func (handler *AccessHandler) listServiceInterfaceCandidates(c *gin.Context) {
	if handler.inventory == nil {
		accessFailure(c, errors.New("route inventory is unavailable"))
		return
	}
	registered, err := handler.access.ListServiceInterfaces(c.Request.Context(), true)
	if err != nil {
		accessFailure(c, err)
		return
	}
	byRoute := make(map[string]identity.ServiceInterface, len(registered))
	for _, serviceInterface := range registered {
		byRoute[accessRouteKey(serviceInterface.HTTPMethod, serviceInterface.RouteTemplate)] = serviceInterface
	}
	candidates := handler.inventory.ServiceInterfaceCandidates(c.Request.Context())
	items := make([]serviceInterfaceCandidateResponse, 0, len(candidates))
	for _, candidate := range candidates {
		item := serviceInterfaceCandidateResponse{
			HTTPMethod: candidate.HTTPMethod, RouteTemplate: candidate.RouteTemplate, ActionCode: candidate.ActionCode,
		}
		if serviceInterface, ok := byRoute[accessRouteKey(candidate.HTTPMethod, candidate.RouteTemplate)]; ok {
			item.Registered = true
			item.ServiceInterfaceID = serviceInterface.ID
			item.ServiceInterfaceCode = serviceInterface.Code
			item.ActionCode = serviceInterface.ActionCode
		}
		items = append(items, item)
	}
	accessOK(c, gin.H{"items": items})
}

func (handler *AccessHandler) listServiceInterfaces(c *gin.Context) {
	includeDisabled, ok := accessIncludeDisabled(c)
	if !ok {
		return
	}
	items, err := handler.access.ListServiceInterfaces(c.Request.Context(), includeDisabled)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"items": items})
}

func (handler *AccessHandler) createServiceInterface(c *gin.Context) {
	var request createServiceInterfaceRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Enabled == nil {
		accessBad(c, "enabled is required")
		return
	}
	result, err := handler.access.CreateServiceInterface(c.Request.Context(), serviceInterfaceInput(request))
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, result)
}

func (handler *AccessHandler) bulkCreateServiceInterfaces(c *gin.Context) {
	var request bulkCreateServiceInterfaceRequest
	if !accessJSON(c, &request) {
		return
	}
	inputs := make([]identity.ServiceInterfaceInput, 0, len(request.Items))
	for _, item := range request.Items {
		if item.Enabled == nil {
			accessBad(c, "enabled is required")
			return
		}
		inputs = append(inputs, serviceInterfaceInput(item))
	}
	items, err := handler.access.CreateServiceInterfaces(c.Request.Context(), inputs)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, gin.H{"items": items})
}

func (handler *AccessHandler) patchServiceInterface(c *gin.Context) {
	id, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var request patchServiceInterfaceRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Code != nil || request.ActionCode != nil || request.HTTPMethod != nil || request.RouteTemplate != nil {
		accessBad(c, "code, action_code, http_method, and route_template are immutable")
		return
	}
	result, err := handler.access.UpdateServiceInterface(c.Request.Context(), id, identity.ServiceInterfaceUpdate{
		Name: request.Name, Description: request.Description, Enabled: request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, result)
}

func (handler *AccessHandler) deleteServiceInterface(c *gin.Context) {
	id, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	if err := handler.access.DeleteServiceInterface(c.Request.Context(), id); err != nil {
		accessFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccessHandler) listRoles(c *gin.Context) {
	includeDisabled, ok := accessIncludeDisabled(c)
	if !ok {
		return
	}
	items, err := handler.access.ListRoles(c.Request.Context(), includeDisabled)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"items": items})
}

func (handler *AccessHandler) createRole(c *gin.Context) {
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	var request createRoleRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Enabled == nil {
		accessBad(c, "enabled is required")
		return
	}
	result, err := handler.access.CreateRoleAudited(c.Request.Context(), actorUserID, identity.RoleInput{
		Name: request.Name, Description: request.Description, Enabled: *request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, result)
}

func (handler *AccessHandler) getRole(c *gin.Context) {
	id, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	role, err := handler.access.Role(c.Request.Context(), id)
	if err != nil {
		accessFailure(c, err)
		return
	}
	menuIDs, err := handler.access.RoleMenuIDs(c.Request.Context(), id)
	if err != nil {
		accessFailure(c, err)
		return
	}
	serviceInterfaceIDs, err := handler.access.RoleServiceInterfaceIDs(c.Request.Context(), id)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{
		"role": role, "menu_ids": menuIDs, "service_interface_ids": serviceInterfaceIDs,
	})
}

func (handler *AccessHandler) patchRole(c *gin.Context) {
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	id, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var request patchRoleRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Code != nil {
		accessBad(c, "code is immutable")
		return
	}
	result, err := handler.access.UpdateRoleAudited(c.Request.Context(), actorUserID, id, identity.RoleUpdate{
		Name: request.Name, Description: request.Description, Enabled: request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, result)
}

func (handler *AccessHandler) deleteRole(c *gin.Context) {
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	id, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	if err := handler.access.DeleteRoleAudited(c.Request.Context(), actorUserID, id); err != nil {
		accessFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccessHandler) replaceRoleMenus(c *gin.Context) {
	accessGone(c, "role menu grants are replaced by action permissions")
}

func (handler *AccessHandler) replaceRoleServiceInterfaces(c *gin.Context) {
	accessGone(c, "role interface grants are replaced by action permissions")
}

func (handler *AccessHandler) listRoleActionCodes(c *gin.Context) {
	roleID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	codes, err := handler.access.RoleActionCodes(c.Request.Context(), roleID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"role_id": roleID, "action_codes": codes})
}

func (handler *AccessHandler) replaceRoleActionCodes(c *gin.Context) {
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	roleID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var request replaceActionCodesRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.ActionCodes == nil {
		accessBad(c, "action_codes is required")
		return
	}
	if err := handler.access.ReplaceRoleActionCodes(c.Request.Context(), actorUserID, roleID, *request.ActionCodes); err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"role_id": roleID, "action_codes": *request.ActionCodes})
}

func (handler *AccessHandler) listUserRoles(c *gin.Context) {
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	roleIDs, err := handler.access.UserRoleIDs(c.Request.Context(), userID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"user_id": userID, "role_ids": roleIDs})
}

func (handler *AccessHandler) replaceUserRoles(c *gin.Context) {
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request replaceIDsRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.RoleIDs == nil {
		accessBad(c, "role_ids is required")
		return
	}
	var err error
	if handler.personnel != nil {
		actorUserID, ok := accessActorUserID(c)
		if !ok {
			return
		}
		err = handler.personnel.ReplaceUserRoles(c.Request.Context(), actorUserID, userID, *request.RoleIDs)
	} else {
		err = handler.access.ReplaceUserRoles(c.Request.Context(), userID, *request.RoleIDs)
	}
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"user_id": userID, "role_ids": *request.RoleIDs})
}

// syncFeishuOrg handles POST /access/admin/groups/feishu-org/sync: it runs a
// foreground org sync and returns the fresh operator status. A sync already
// in progress fails fast with FEISHU_ORG_SYNC_IN_PROGRESS.
func (handler *AccessHandler) syncFeishuOrg(c *gin.Context) {
	if handler.feishuOrg == nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"code":  "FEISHU_ORG_SYNC_UNAVAILABLE",
			"error": "feishu org sync is unavailable; enable the Feishu capability first",
		})
		return
	}
	stats, err := handler.feishuOrg.SyncManually(c.Request.Context())
	if err != nil {
		if errors.Is(err, identity.ErrFeishuOrgSyncInProgress) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"code": "FEISHU_ORG_SYNC_IN_PROGRESS", "error": err.Error(),
			})
			return
		}
		accessFailure(c, err)
		return
	}
	status, err := handler.feishuOrg.Status(c.Request.Context())
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"stats": stats, "status": status})
}

// feishuOrgStatus handles GET /access/admin/groups/feishu-org/status.
func (handler *AccessHandler) feishuOrgStatus(c *gin.Context) {
	if handler.feishuOrg == nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"code":  "FEISHU_ORG_SYNC_UNAVAILABLE",
			"error": "feishu org sync is unavailable; enable the Feishu capability first",
		})
		return
	}
	status, err := handler.feishuOrg.Status(c.Request.Context())
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, status)
}

func (handler *AccessHandler) listGroups(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	includeDisabled, ok := accessIncludeDisabled(c)
	if !ok {
		return
	}
	items, err := handler.personnel.ListGroups(c.Request.Context(), includeDisabled)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"items": items})
}

func (handler *AccessHandler) createGroup(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	var request createUserGroupRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Enabled == nil {
		accessBad(c, "enabled is required")
		return
	}
	group, err := handler.personnel.CreateGroup(c.Request.Context(), actorUserID, identity.UserGroupInput{
		Code: request.Code, Name: request.Name, Description: request.Description, Enabled: *request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessCreated(c, group)
}

func (handler *AccessHandler) patchGroup(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var request patchUserGroupRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.Code != nil {
		accessBad(c, "code is immutable")
		return
	}
	group, err := handler.personnel.UpdateGroup(c.Request.Context(), actorUserID, groupID, identity.UserGroupUpdate{
		Name: request.Name, Description: request.Description, Enabled: request.Enabled,
	})
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, group)
}

func (handler *AccessHandler) deleteGroup(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	if err := handler.personnel.DeleteGroup(c.Request.Context(), actorUserID, groupID); err != nil {
		accessFailure(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccessHandler) listGroupServiceInterfaces(c *gin.Context) {
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	interfaceIDs, err := handler.access.GroupServiceInterfaceIDs(c.Request.Context(), groupID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"group_id": groupID, "service_interface_ids": interfaceIDs})
}

func (handler *AccessHandler) replaceGroupServiceInterfaces(c *gin.Context) {
	accessGone(c, "group interface grants are replaced by action permissions")
}

func (handler *AccessHandler) listGroupActionCodes(c *gin.Context) {
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	codes, err := handler.access.GroupActionCodes(c.Request.Context(), groupID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"group_id": groupID, "action_codes": codes})
}

func (handler *AccessHandler) replaceGroupActionCodes(c *gin.Context) {
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	var request replaceActionCodesRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.ActionCodes == nil {
		accessBad(c, "action_codes is required")
		return
	}
	if err := handler.access.ReplaceGroupActionCodes(c.Request.Context(), actorUserID, groupID, *request.ActionCodes); err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"group_id": groupID, "action_codes": *request.ActionCodes})
}

func (handler *AccessHandler) listGroupMenus(c *gin.Context) {
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	menuIDs, err := handler.access.GroupMenuIDs(c.Request.Context(), groupID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"group_id": groupID, "menu_ids": menuIDs})
}

func (handler *AccessHandler) replaceGroupMenus(c *gin.Context) {
	groupID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	var request replaceIDsRequest
	if !accessJSON(c, &request) {
		return
	}
	if request.MenuIDs == nil {
		accessBad(c, "menu_ids is required")
		return
	}
	if err := handler.access.ReplaceGroupMenus(c.Request.Context(), actorUserID, groupID, *request.MenuIDs); err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"group_id": groupID, "menu_ids": *request.MenuIDs})
}

func (handler *AccessHandler) listMyActionPermissions(c *gin.Context) {
	userID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	codes, err := handler.access.EffectiveActionCodes(c.Request.Context(), userID)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"action_codes": codes})
}

func (handler *AccessHandler) replaceUserGroup(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request replaceUserGroupRequest
	if !accessJSON(c, &request) {
		return
	}
	if err := handler.personnel.ReplaceUserGroup(c.Request.Context(), actorUserID, userID, request.GroupID); err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"user_id": userID, "group_id": request.GroupID})
}

func (handler *AccessHandler) replaceUserStatus(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request replaceUserStatusRequest
	if !accessJSON(c, &request) {
		return
	}
	user, err := handler.personnel.TransitionUserStatus(
		c.Request.Context(), actorUserID, userID, identity.AccountStatus(request.Status), request.Reason,
	)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, user)
}

func (handler *AccessHandler) approveUser(c *gin.Context) {
	handler.transitionUserAction(c, identity.AccountActive)
}

func (handler *AccessHandler) rejectUser(c *gin.Context) {
	handler.transitionUserAction(c, identity.AccountRejected)
}

func (handler *AccessHandler) banUser(c *gin.Context) {
	handler.transitionUserAction(c, identity.AccountBanned)
}

func (handler *AccessHandler) unbanUser(c *gin.Context) {
	handler.transitionUserAction(c, identity.AccountActive)
}

func (handler *AccessHandler) transitionUserAction(c *gin.Context, status identity.AccountStatus) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request replaceUserStatusRequest
	if c.Request.ContentLength != 0 && !accessJSON(c, &request) {
		return
	}
	user, err := handler.personnel.TransitionUserStatus(c.Request.Context(), actorUserID, userID, status, request.Reason)
	if err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, user)
}

func (handler *AccessHandler) replaceUserAccess(c *gin.Context) {
	if !handler.requirePersonnel(c) {
		return
	}
	actorUserID, ok := accessActorUserID(c)
	if !ok {
		return
	}
	userID, ok := accessPathID(c, "user_id")
	if !ok {
		return
	}
	var request replaceUserAccessRequest
	if !accessJSON(c, &request) {
		return
	}
	if err := handler.personnel.ReplaceUserAccess(c.Request.Context(), actorUserID, userID, request.GroupID, request.RoleIDs); err != nil {
		accessFailure(c, err)
		return
	}
	accessOK(c, gin.H{"user_id": userID, "group_id": request.GroupID, "role_ids": request.RoleIDs})
}

func (handler *AccessHandler) requirePersonnel(c *gin.Context) bool {
	if handler.personnel != nil {
		return true
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "personnel service is unavailable"})
	return false
}

func accessActorUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(pmauth.PrincipalContextKey)
	principal, ok := value.(pmauth.Principal)
	if !exists || !ok || principal.UserID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authenticated PM principal is required"})
		return 0, false
	}
	return principal.UserID, true
}

func serviceInterfaceInput(request createServiceInterfaceRequest) identity.ServiceInterfaceInput {
	return identity.ServiceInterfaceInput{
		Code: request.Code, ActionCode: request.ActionCode, HTTPMethod: request.HTTPMethod, RouteTemplate: request.RouteTemplate,
		Name: request.Name, Description: request.Description, Enabled: *request.Enabled,
	}
}

func accessIncludeDisabled(c *gin.Context) (bool, bool) {
	raw, found := c.GetQuery("include_disabled")
	if !found || strings.TrimSpace(raw) == "" {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		accessBad(c, "include_disabled must be a boolean")
		return false, false
	}
	return value, true
}

func accessPathID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value == 0 {
		accessBad(c, name+" is invalid")
		return 0, false
	}
	return uint(value), true
}

func accessJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		accessBad(c, "invalid body")
		return false
	}
	return true
}

func accessRouteKey(method, routeTemplate string) string {
	return method + "\x00" + routeTemplate
}

func accessOK(c *gin.Context, value any) {
	c.JSON(http.StatusOK, gin.H{"data": value})
}

func accessCreated(c *gin.Context, value any) {
	c.JSON(http.StatusCreated, gin.H{"data": value})
}

func accessBad(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": message})
}

func accessGone(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusGone, gin.H{"error": message})
}

func accessFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrAccessRoleNotFound), errors.Is(err, identity.ErrAccessServiceInterfaceNotFound), errors.Is(err, identity.ErrAccessUserNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrPersonnelUserNotFound), errors.Is(err, identity.ErrUserGroupNotFound),
		errors.Is(err, identity.ErrWorkspaceAdminNotFound), errors.Is(err, identity.ErrWorkspaceMemberNotFound),
		errors.Is(err, identity.ErrWorkspaceMemberUserMissing):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, identity.ErrAccessResourceInUse):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "ACCESS_RESOURCE_IN_USE", "error": err.Error()})
	case errors.Is(err, identity.ErrAccessSystemManaged), errors.Is(err, identity.ErrAccessCodeConflict),
		errors.Is(err, identity.ErrAccessRoleNameConflict), errors.Is(err, identity.ErrAccessRouteConflict):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "ACCESS_RESOURCE_CONFLICT", "error": err.Error()})
	case errors.Is(err, identity.ErrUserGroupCodeConflict), errors.Is(err, identity.ErrUserGroupInUse),
		errors.Is(err, identity.ErrDefaultUserGroupProtected), errors.Is(err, identity.ErrLastAdministrator):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "PERSONNEL_RESOURCE_CONFLICT", "error": err.Error()})
	case errors.Is(err, identity.ErrWorkspaceSlugConflict), errors.Is(err, identity.ErrWorkspaceDefaultProtected),
		errors.Is(err, identity.ErrWorkspaceInUse), errors.Is(err, identity.ErrWorkspaceMemberConflict):
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"code": "WORKSPACE_RESOURCE_CONFLICT", "error": err.Error()})
	case errors.Is(err, identity.ErrAccessRoleCodeInvalid), errors.Is(err, identity.ErrAccessServiceInterfaceCodeInvalid),
		errors.Is(err, identity.ErrAccessActionCodeInvalid), errors.Is(err, identity.ErrAccessActionNotFound),
		errors.Is(err, identity.ErrAccessNameInvalid), errors.Is(err, identity.ErrAccessDescriptionInvalid),
		errors.Is(err, identity.ErrAccessRouteInvalid), errors.Is(err, identity.ErrAccessMethodInvalid),
		errors.Is(err, identity.ErrAccessDuplicateID), errors.Is(err, identity.ErrAccessRouteNotRegistered),
		errors.Is(err, identity.ErrAccessBulkEmpty):
		accessBad(c, err.Error())
	case errors.Is(err, identity.ErrUserGroupCodeInvalid), errors.Is(err, identity.ErrUserGroupNameInvalid),
		errors.Is(err, identity.ErrUserGroupDescriptionInvalid), errors.Is(err, identity.ErrInvalidAccountTransition),
		errors.Is(err, identity.ErrAccountReasonRequired), errors.Is(err, identity.ErrPersonnelActorRequired),
		errors.Is(err, identity.ErrPersonnelQueryInvalid):
		accessBad(c, err.Error())
	case errors.Is(err, identity.ErrWorkspaceNameInvalid), errors.Is(err, identity.ErrWorkspaceSlugInvalid),
		errors.Is(err, identity.ErrWorkspaceDescriptionInvalid), errors.Is(err, identity.ErrWorkspaceMemberRoleInvalid),
		errors.Is(err, identity.ErrWorkspaceMemberQueryInvalid):
		accessBad(c, err.Error())
	case errors.Is(err, identity.ErrPasswordResetScope):
		accessBad(c, err.Error())
	case errors.Is(err, identity.ErrPasswordResetNotificationUnavailable):
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "access operation failed"})
	}
}
