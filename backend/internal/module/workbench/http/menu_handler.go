// Package workbenchhttp adapts workbench menu use cases to the authenticated
// PM HTTP contract. It deliberately receives no database handle.
package workbenchhttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type MenuHandler struct {
	menus *workbench.MenuService
}

func NewMenuHandler(menus *workbench.MenuService) *MenuHandler {
	return &MenuHandler{menus: menus}
}

// RegisterRoutes mounts menu endpoints below an already authenticated core
// group. The transport authorization middleware controls who may use admin
// endpoints; GET me/menus is an explicit authenticated system exemption.
func (handler *MenuHandler) RegisterRoutes(group *gin.RouterGroup) {
	access := group.Group("/access")
	access.GET("/me/menus", handler.currentUserMenus)
	admin := access.Group("/admin")
	admin.GET("/menus", handler.listMenus)
	admin.POST("/menus", handler.createMenu)
	admin.PATCH("/menus/:id", handler.patchMenu)
	admin.DELETE("/menus/:id", handler.deleteMenu)
}

func (handler *MenuHandler) currentUserMenus(c *gin.Context) {
	principal, ok := menuPrincipal(c)
	if !ok {
		menuFailure(c, http.StatusUnauthorized, "SESSION_EXPIRED", "authentication required")
		return
	}
	items, err := handler.menus.MenusForUser(c.Request.Context(), principal.UserID, c.Query("host_key"))
	if err != nil {
		menuError(c, err)
		return
	}
	menuOK(c, gin.H{"items": items})
}

func (handler *MenuHandler) listMenus(c *gin.Context) {
	includeDisabled, ok := menuIncludeDisabled(c)
	if !ok {
		return
	}
	items, err := handler.menus.List(c.Request.Context(), includeDisabled)
	if err != nil {
		menuError(c, err)
		return
	}
	menuOK(c, gin.H{"items": items})
}

func (handler *MenuHandler) createMenu(c *gin.Context) {
	var request createMenuRequest
	if !menuJSON(c, &request) {
		return
	}
	if request.Enabled == nil {
		menuBad(c, "enabled is required")
		return
	}
	result, err := handler.menus.Create(c.Request.Context(), workbench.MenuInput{
		Code: request.Code, ParentID: request.ParentID, Name: request.Name, MenuType: request.MenuType,
		Path: request.Path, Icon: request.Icon, HostKey: request.HostKey, SortOrder: request.SortOrder, Enabled: *request.Enabled,
	})
	if err != nil {
		menuError(c, err)
		return
	}
	menuCreated(c, result)
}

func (handler *MenuHandler) patchMenu(c *gin.Context) {
	id, ok := menuPathID(c, "id")
	if !ok {
		return
	}
	var request patchMenuRequest
	if !menuJSON(c, &request) {
		return
	}
	if request.Code != nil {
		menuBad(c, "code is immutable")
		return
	}
	result, err := handler.menus.Update(c.Request.Context(), id, workbench.MenuUpdate{
		ParentID: request.ParentID.Value, ParentSet: request.ParentID.Set,
		Name: request.Name, MenuType: request.MenuType, Path: request.Path, Icon: request.Icon,
		HostKey: request.HostKey, SortOrder: request.SortOrder, Enabled: request.Enabled,
	})
	if err != nil {
		menuError(c, err)
		return
	}
	menuOK(c, result)
}

func (handler *MenuHandler) deleteMenu(c *gin.Context) {
	id, ok := menuPathID(c, "id")
	if !ok {
		return
	}
	if err := handler.menus.Delete(c.Request.Context(), id); err != nil {
		menuError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func menuPrincipal(c *gin.Context) (pmauth.Principal, bool) {
	value, found := c.Get(pmauth.PrincipalContextKey)
	if !found {
		return pmauth.Principal{}, false
	}
	principal, ok := value.(pmauth.Principal)
	return principal, ok && principal.UserID > 0
}

func menuIncludeDisabled(c *gin.Context) (bool, bool) {
	raw, found := c.GetQuery("include_disabled")
	if !found || strings.TrimSpace(raw) == "" {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		menuBad(c, "include_disabled must be a boolean")
		return false, false
	}
	return value, true
}

func menuPathID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value == 0 {
		menuBad(c, name+" is invalid")
		return 0, false
	}
	return uint(value), true
}

func menuJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		menuBad(c, "invalid body")
		return false
	}
	return true
}

func menuOK(c *gin.Context, value any) {
	c.JSON(http.StatusOK, gin.H{"data": value})
}

func menuCreated(c *gin.Context, value any) {
	c.JSON(http.StatusCreated, gin.H{"data": value})
}

func menuBad(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": message})
}

func menuFailure(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": message})
}

func menuError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, workbench.ErrMenuNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, workbench.ErrMenuInUse):
		menuFailure(c, http.StatusConflict, "ACCESS_RESOURCE_IN_USE", err.Error())
	case errors.Is(err, workbench.ErrMenuSystemManaged), errors.Is(err, workbench.ErrMenuCodeConflict):
		menuFailure(c, http.StatusConflict, "ACCESS_RESOURCE_CONFLICT", err.Error())
	case errors.Is(err, workbench.ErrMenuCodeInvalid), errors.Is(err, workbench.ErrMenuNameInvalid),
		errors.Is(err, workbench.ErrMenuTypeInvalid), errors.Is(err, workbench.ErrMenuPathInvalid),
		errors.Is(err, workbench.ErrMenuHostInvalid), errors.Is(err, workbench.ErrMenuParentInvalid),
		errors.Is(err, workbench.ErrMenuDuplicateID):
		menuBad(c, err.Error())
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "menu operation failed"})
	}
}
