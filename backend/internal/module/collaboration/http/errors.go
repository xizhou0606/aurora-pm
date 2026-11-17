package collaborationhttp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

func writeCollaborationError(c *gin.Context, err error) {
	status, code := collaborationError(err)
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code}})
}

func collaborationError(err error) (int, string) {
	status, code := http.StatusInternalServerError, "collaboration_unavailable"
	switch {
	case errors.Is(err, collaboration.ErrInvalidInput):
		status, code = http.StatusBadRequest, "invalid_input"
	case errors.Is(err, collaboration.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, collaboration.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, collaboration.ErrLeaseHeld):
		status, code = http.StatusConflict, "lease_held"
	case errors.Is(err, collaboration.ErrLeaseExpired):
		status, code = http.StatusConflict, "lease_expired"
	case errors.Is(err, collaboration.ErrStaleLease):
		status, code = http.StatusConflict, "stale_lease"
	case errors.Is(err, collaboration.ErrStalePolicy):
		status, code = http.StatusConflict, "stale_policy"
	case errors.Is(err, collaboration.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, collaboration.ErrDataCorrupt):
		status, code = http.StatusServiceUnavailable, "durable_data_corrupt"
	}
	return status, code
}
