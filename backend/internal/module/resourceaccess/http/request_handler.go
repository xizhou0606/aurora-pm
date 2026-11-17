package resourceaccesshttp

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"net/http"
	"strconv"
)

type requestApplication interface {
	BitableContext(context.Context, resourceaccess.Principal, string) (resourceaccess.RequestContext, error)
	DocumentContext(context.Context, resourceaccess.Principal, uint) (resourceaccess.RequestContext, error)
	Context(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) (resourceaccess.RequestContext, error)
	Create(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, string, bool) (resourceaccess.AccessRequest, error)
	Update(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, uint, resourceaccess.RequestReview) (resourceaccess.AccessRequest, error)
}

func (h *Handler) ConfigureRequests(service requestApplication) { h.requests = service }
func (h *Handler) listRequests(c *gin.Context) {
	_, result, ok := h.requireDocumentResourceRequestPolicy(c)
	if !ok {
		return
	}
	writeData(c, http.StatusOK, result)
}
func (h *Handler) createRequest(c *gin.Context) {
	p, view, ok := h.requireDocumentResourceRequestPolicy(c)
	if !ok {
		return
	}
	if h.requests == nil {
		writeError(c, resourceaccess.ErrUnavailable)
		return
	}
	var body struct {
		Reason      string `json:"reason"`
		NotifyOwner *bool  `json:"notify_owner"`
	}
	if !bindJSON(c, &body) {
		return
	}
	notify := body.NotifyOwner == nil || *body.NotifyOwner
	result, err := h.requests.Create(c.Request.Context(), p, view.ResourceID, body.Reason, notify)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, result)
}

func (h *Handler) requireDocumentResourceRequestPolicy(c *gin.Context) (resourceaccess.Principal, resourceaccess.RequestContext, bool) {
	p, ok := requestPrincipal(c)
	if !ok {
		return p, resourceaccess.RequestContext{}, false
	}
	if h.requests == nil {
		writeError(c, resourceaccess.ErrUnavailable)
		return p, resourceaccess.RequestContext{}, false
	}
	view, err := h.requests.Context(c.Request.Context(), p, documentlibrary.ResourceID(c.Param("resourceId")))
	if err != nil {
		writeError(c, err)
		return p, resourceaccess.RequestContext{}, false
	}
	if expected := c.Query("resource_type"); expected != "" && string(view.ResourceType) != expected {
		writeError(c, resourceaccess.ErrNotFound)
		return p, resourceaccess.RequestContext{}, false
	}
	return p, view, true
}
func (h *Handler) documentRequestContext(c *gin.Context) {
	view, ok := h.requireDocumentRequestContext(c)
	if ok {
		writeData(c, http.StatusOK, view)
	}
}
func (h *Handler) bitableRequestContext(c *gin.Context) {
	view, ok := h.requireBitableRequestContext(c)
	if ok {
		writeData(c, http.StatusOK, view)
	}
}
func (h *Handler) requireBitableRequestContext(c *gin.Context) (resourceaccess.RequestContext, bool) {
	p, ok := requestPrincipal(c)
	if !ok {
		return resourceaccess.RequestContext{}, false
	}
	if h.requests == nil {
		writeError(c, resourceaccess.ErrUnavailable)
		return resourceaccess.RequestContext{}, false
	}
	view, err := h.requests.BitableContext(c.Request.Context(), p, c.Param("resourceRef"))
	if err != nil {
		writeError(c, err)
		return resourceaccess.RequestContext{}, false
	}
	return view, true
}
func (h *Handler) requireDocumentRequestContext(c *gin.Context) (resourceaccess.RequestContext, bool) {
	p, ok := requestPrincipal(c)
	if !ok {
		return resourceaccess.RequestContext{}, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(c, resourceaccess.ErrInvalidInput)
		return resourceaccess.RequestContext{}, false
	}
	if h.requests == nil {
		writeError(c, resourceaccess.ErrUnavailable)
		return resourceaccess.RequestContext{}, false
	}
	view, err := h.requests.DocumentContext(c.Request.Context(), p, uint(id))
	if err != nil {
		writeError(c, err)
		return resourceaccess.RequestContext{}, false
	}
	return view, true
}
func (h *Handler) updateRequest(c *gin.Context) {
	p, id, ok := h.requireDocumentResourcePolicy(c, documentlibrary.OperationManageAccess)
	if !ok {
		return
	}
	if h.requests == nil {
		writeError(c, resourceaccess.ErrUnavailable)
		return
	}
	requestID, err := strconv.ParseUint(c.Param("requestId"), 10, 64)
	if err != nil || requestID == 0 {
		writeError(c, resourceaccess.ErrInvalidInput)
		return
	}
	var body struct {
		Decision      resourceaccess.RequestStatus `json:"decision"`
		GrantLevel    resourceaccess.Level         `json:"grant_level"`
		GrantDuration resourceaccess.LinkDuration  `json:"grant_duration"`
		Read          *bool                        `json:"read"`
	}
	if !bindJSON(c, &body) {
		return
	}
	result, err := h.requests.Update(c.Request.Context(), p, id, uint(requestID), resourceaccess.RequestReview{Decision: body.Decision, Read: body.Read, GrantLevel: body.GrantLevel, GrantDuration: body.GrantDuration})
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, result)
}
