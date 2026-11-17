package requirementhttp

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// PoolPage normalizes requirement-pool pagination while preserving the
// existing limit=all API contract.
func PoolPage(c *gin.Context) (limit int, offset int, all bool) {
	if strings.EqualFold(strings.TrimSpace(c.Query("limit")), "all") {
		return 0, 0, true
	}
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset, false
}
