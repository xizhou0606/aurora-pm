package requirementhttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type ResourceLinksHandler struct {
	service *requirement.ResourceLinkService
}

func NewResourceLinksHandler(service *requirement.ResourceLinkService) *ResourceLinksHandler {
	return &ResourceLinksHandler{service: service}
}
func (h *ResourceLinksHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/requirements/:id/document-resources", h.list)
	group.GET("/requirements/:id/document-resources/candidates", h.candidates)
	group.POST("/requirements/:id/document-resources", h.attach)
	group.DELETE("/requirements/:id/document-resources/:resourceId", h.detach)
}
func resourceLinkScope(c *gin.Context) (uint, uint, uint, bool) {
	principalValue, _ := c.Get(pmauth.PrincipalContextKey)
	p, ok := principalValue.(pmauth.Principal)
	workspaceValue, _ := c.Get(pmauth.WorkspaceIDContextKey)
	w, valid := workspaceValue.(uint)
	if !ok || !valid || p.UserID == 0 || w == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return 0, 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return 0, 0, 0, false
	}
	return w, p.UserID, uint(id), true
}
func resourceLinkResult(c *gin.Context, page requirement.ResourceLinkPage, err error) {
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"data": page})
		return
	}
	status, code := http.StatusInternalServerError, "REQUIREMENT_RESOURCE_UNAVAILABLE"
	switch {
	case errors.Is(err, requirement.ErrResourceLinkInvalid), errors.Is(err, resourceaccess.ErrInvalidInput):
		status, code = http.StatusBadRequest, "REQUIREMENT_RESOURCE_INVALID"
	case errors.Is(err, requirement.ErrResourceLinkConflict), errors.Is(err, resourceaccess.ErrConflict):
		status, code = http.StatusConflict, "REQUIREMENT_RESOURCE_CONFLICT"
	case errors.Is(err, requirement.ErrResourceLinkNotFound), errors.Is(err, documentlibrary.ErrNotFound), errors.Is(err, resourceaccess.ErrNotFound):
		status, code = http.StatusNotFound, "REQUIREMENT_RESOURCE_NOT_FOUND"
	case errors.Is(err, resourceaccess.ErrForbidden):
		status, code = http.StatusForbidden, "REQUIREMENT_RESOURCE_FORBIDDEN"
	}
	c.JSON(status, gin.H{"code": code, "error": http.StatusText(status)})
}
func (h *ResourceLinksHandler) list(c *gin.Context)       { h.read(c, false) }
func (h *ResourceLinksHandler) candidates(c *gin.Context) { h.read(c, true) }
func (h *ResourceLinksHandler) read(c *gin.Context, candidates bool) {
	w, u, id, ok := resourceLinkScope(c)
	if !ok {
		return
	}
	page, err := h.service.Read(c.Request.Context(), w, u, id, candidates)
	resourceLinkResult(c, page, err)
}
func (h *ResourceLinksHandler) attach(c *gin.Context) { h.apply(c, false) }
func (h *ResourceLinksHandler) detach(c *gin.Context) { h.apply(c, true) }
func (h *ResourceLinksHandler) apply(c *gin.Context, remove bool) {
	w, u, id, ok := resourceLinkScope(c)
	if !ok {
		return
	}
	var body struct {
		ResourceID             string  `json:"resource_id"`
		ExpectedVersion        uint    `json:"expected_version"`
		ExpectedPolicyRevision uint64  `json:"expected_policy_revision"`
		OperationID            string  `json:"operation_id"`
		ProductDocURL          *string `json:"product_doc_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resourceLinkResult(c, requirement.ResourceLinkPage{}, requirement.ErrResourceLinkInvalid)
		return
	}
	if remove {
		body.ResourceID = c.Param("resourceId")
	}
	page, err := h.service.Apply(c.Request.Context(), requirement.ResourceLinkCommand{WorkspaceID: w, UserID: u, RequirementID: id, ResourceID: body.ResourceID, ExpectedVersion: body.ExpectedVersion, ExpectedPolicyRevision: body.ExpectedPolicyRevision, OperationID: body.OperationID, Remove: remove, ProductDocURL: body.ProductDocURL})
	resourceLinkResult(c, page, err)
}
