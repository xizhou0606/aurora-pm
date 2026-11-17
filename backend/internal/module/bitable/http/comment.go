package bitablehttp

import (
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"strconv"
)

func (h *Handler) comments(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	offset, e := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	limit, e := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if e != nil {
		failure(c, bitable.ErrInvalidInput)
		return
	}
	result, e := h.service.Comments(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("recordId"), offset, limit)
	respond(c, result, e)
}
func (h *Handler) createComment(c *gin.Context) {
	p, ok := principal(c)
	if !ok {
		return
	}
	var in bitable.CommentInput
	if !bind(c, &in) {
		return
	}
	result, e := h.service.CreateComment(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Param("recordId"), in)
	respond(c, result, e)
}
