package bitablehttp

import (
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func (h *Handler) field(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.FieldMutation
	if !bind(c, &in) {
		return
	}
	v, e := h.service.Field(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("fieldId"), in)
	respond(c, v, e)
}
func (h *Handler) view(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.ViewMutation
	if !bind(c, &in) {
		return
	}
	v, e := h.service.View(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("viewId"), in)
	respond(c, v, e)
}
