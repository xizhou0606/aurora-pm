package bitablehttp

import (
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func (h *Handler) resourceMutation(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := principal(c)
		if !ok {
			return
		}
		var in bitable.ResourceMutation
		if !bind(c, &in) {
			return
		}
		v, e := h.service.MutateResource(c.Request.Context(), p, c.Param("resourceRef"), kind, in)
		respond(c, v, e)
	}
}
func (h *Handler) shortcutMutation(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := principal(c)
		if !ok {
			return
		}
		var in bitable.ResourceMutation
		if !bind(c, &in) {
			return
		}
		id := c.Param("shortcutRef")
		if kind == "shortcut" {
			id = c.Param("resourceRef")
		}
		v, e := h.service.MutateShortcut(c.Request.Context(), p, id, kind, in)
		respond(c, v, e)
	}
}
func (h *Handler) shortcuts(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	v, e := h.service.Shortcuts(c.Request.Context(), p)
	respond(c, v, e)
}
