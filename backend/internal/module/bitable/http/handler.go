package bitablehttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type Handler struct{ service *bitable.Service }

func NewHandler(s *bitable.Service) (*Handler, error) {
	if s == nil {
		return nil, bitable.ErrInvalidInput
	}
	return &Handler{s}, nil
}
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/bitables", h.list)
	group.POST("/bitables", h.create)
	group.GET("/bitables/:resourceRef", h.get)
	group.PATCH("/bitables/:resourceRef/name", h.resourceMutation("name"))
	group.PATCH("/bitables/:resourceRef/location", h.resourceMutation("location"))
	group.PATCH("/bitables/:resourceRef/owner", h.resourceMutation("owner"))
	group.PATCH("/bitables/:resourceRef/pin", h.resourceMutation("pin"))
	group.PATCH("/bitables/:resourceRef/favorite", h.resourceMutation("favorite"))
	group.POST("/bitables/:resourceRef/delete", h.resourceMutation("delete"))
	group.POST("/bitables/:resourceRef/duplicate", h.resourceMutation("duplicate"))
	group.POST("/bitables/:resourceRef/shortcuts", h.shortcutMutation("shortcut"))
	group.GET("/bitable-shortcuts", h.shortcuts)
	group.PATCH("/bitable-shortcuts/:shortcutRef/location", h.shortcutMutation("location"))
	group.POST("/bitable-shortcuts/:shortcutRef/delete", h.shortcutMutation("delete"))
	group.GET("/bitables/:resourceRef/tables/:tableId/bootstrap", h.bootstrap)
	group.POST("/bitables/:resourceRef/tables/:tableId/records/query", h.query)
	group.POST("/bitables/:resourceRef/tables/:tableId/records/search", h.search)
	group.POST("/bitables/:resourceRef/tables/:tableId/commands", h.commands)
	group.POST("/bitables/:resourceRef/tables/:tableId/imports", h.imports)
	group.GET("/bitables/:resourceRef/tables/:tableId/operations/:operationId", h.operation)
	group.POST("/bitables/:resourceRef/tables/:tableId/export-snapshot", h.export)
	group.POST("/bitables/:resourceRef/tables/:tableId/fields", h.field)
	group.PATCH("/bitables/:resourceRef/tables/:tableId/fields/:fieldId", h.field)
	group.POST("/bitables/:resourceRef/tables/:tableId/views", h.view)
	group.PATCH("/bitables/:resourceRef/tables/:tableId/views/:viewId", h.view)
	group.GET("/bitables/:resourceRef/tables/:tableId/people", h.people)
	group.GET("/bitables/:resourceRef/tables/:tableId/records/:recordId/comments", h.comments)
	group.POST("/bitables/:resourceRef/tables/:tableId/records/:recordId/comments", h.createComment)
	group.POST("/bitables/:resourceRef/tables/:tableId/records/:recordId/fields/:fieldId/attachments", h.upload)
	group.GET("/bitables/:resourceRef/tables/:tableId/records/:recordId/fields/:fieldId/attachments/:assetId/preview", h.preview)
	group.GET("/bitables/:resourceRef/tables/:tableId/records/:recordId/fields/:fieldId/attachments/:assetId/download", h.download)
}
func principal(c *gin.Context) (bitable.Principal, bool) {
	v, ok := c.Get(pmauth.PrincipalContextKey)
	p, typed := v.(pmauth.Principal)
	w, wok := c.Get(pmauth.WorkspaceIDContextKey)
	workspace, wtyped := w.(uint)
	if !ok || !typed || !wok || !wtyped || p.UserID == 0 || workspace == 0 {
		failure(c, bitable.ErrForbidden)
		return bitable.Principal{}, false
	}
	actorFactUserID := p.UserID
	if value, exists := c.Get(pmauth.ActorFactUserIDContextKey); exists {
		if typedValue, typed := value.(uint); typed && typedValue > 0 {
			actorFactUserID = typedValue
		}
	}
	linkAccessSecret := strings.TrimSpace(c.GetHeader("X-PM-Resource-Access"))
	if len(linkAccessSecret) > 256 {
		failure(c, bitable.ErrInvalidInput)
		return bitable.Principal{}, false
	}
	return bitable.Principal{UserID: p.UserID, WorkspaceID: workspace, ActorFactUserID: actorFactUserID, LinkAccessSecret: linkAccessSecret}, true
}
func bind(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<20)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(target); e != nil {
		failure(c, bitable.ErrInvalidInput)
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		failure(c, bitable.ErrInvalidInput)
		return false
	}
	return true
}
func respond(c *gin.Context, v any, e error) {
	if e != nil {
		failure(c, e)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": v})
}
func failure(c *gin.Context, e error) {
	status, code := http.StatusInternalServerError, "BITABLE_INTERNAL_ERROR"
	switch {
	case errors.Is(e, bitable.ErrInvalidInput):
		status, code = 400, "BITABLE_INVALID_INPUT"
	case errors.Is(e, bitable.ErrForbidden):
		status, code = 403, "BITABLE_FORBIDDEN"
	case errors.Is(e, bitable.ErrNotFound):
		status, code = 404, "BITABLE_NOT_FOUND"
	case errors.Is(e, bitable.ErrConflict):
		status, code = 409, "BITABLE_CONFLICT"
	case errors.Is(e, bitable.ErrCellLeaseLost):
		status, code = 409, "BITABLE_CELL_LEASE_LOST"
	case errors.Is(e, bitable.ErrCellLeaseConflict):
		status, code = 409, "BITABLE_CELL_LEASE_CONFLICT"
	case errors.Is(e, bitable.ErrRecordFilteredOut):
		status, code = 409, "BITABLE_RECORD_FILTERED_OUT"
	case errors.Is(e, bitable.ErrBudgetExceeded):
		status, code = 413, "BITABLE_BUDGET_EXCEEDED"
	case errors.Is(e, bitable.ErrUnknownOutcome):
		status, code = 503, "BITABLE_UNKNOWN_OUTCOME"
	case errors.Is(e, bitable.ErrUnavailable):
		status, code = 503, "BITABLE_UNAVAILABLE"
	}
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": http.StatusText(status)})
}
func (h *Handler) list(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	v, e := h.service.List(c.Request.Context(), p)
	respond(c, v, e)
}
func (h *Handler) get(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	v, e := h.service.Get(c.Request.Context(), p, c.Param("resourceRef"))
	respond(c, v, e)
}
func (h *Handler) create(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Create(c.Request.Context(), p, in.Name, c.GetHeader("Idempotency-Key"))
	respond(c, v, e)
}
func (h *Handler) bootstrap(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	v, e := h.service.Bootstrap(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"))
	respond(c, v, e)
}
func (h *Handler) query(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.Query
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Query(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), in)
	respond(c, v, e)
}
func (h *Handler) search(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.Search
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Search(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), in)
	respond(c, v, e)
}
func (h *Handler) commands(c *gin.Context) { h.execute(c, false) }
func (h *Handler) imports(c *gin.Context)  { h.execute(c, true) }
func (h *Handler) execute(c *gin.Context, importing bool) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.CommandBatch
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Execute(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), in, importing)
	respond(c, v, e)
}
func (h *Handler) operation(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	v, e := h.service.Operation(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("operationId"))
	respond(c, v, e)
}
func (h *Handler) export(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.ExportQuery
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Export(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), in)
	respond(c, v, e)
}
