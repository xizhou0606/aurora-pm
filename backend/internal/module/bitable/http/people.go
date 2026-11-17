package bitablehttp

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func (h *Handler) people(c *gin.Context) {
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
	v, e := h.service.People(c.Request.Context(), p, c.Param("resourceRef"), c.Param("tableId"), c.Query("query"), offset, limit)
	respond(c, v, e)
}
