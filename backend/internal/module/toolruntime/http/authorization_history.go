package toolruntimehttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func (h *CredentialHandler) WithAuthorizationHistory(s *toolruntime.AuthorizationHistoryService) *CredentialHandler {
	h.history = s
	return h
}
func (h *CredentialHandler) authorizationHistory(c *gin.Context) {
	p, ok := h.managementPrincipal(c)
	if !ok {
		return
	}
	if h.history == nil {
		credentialError(c, http.StatusServiceUnavailable, "AUTHORIZATION_HISTORY_UNAVAILABLE", "authorization history unavailable")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		historyError(c, toolruntime.ErrAuthorizationHistoryPage)
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		historyError(c, toolruntime.ErrAuthorizationHistoryPage)
		return
	}
	result, err := h.history.List(c.Request.Context(), p, page, size)
	if err != nil {
		historyError(c, err)
		return
	}
	noStore(c)
	toolOK(c, result)
}
func (h *CredentialHandler) revokeAuthorizationHistory(c *gin.Context) {
	p, ok := h.managementPrincipal(c)
	if !ok {
		return
	}
	if h.history == nil {
		credentialError(c, http.StatusServiceUnavailable, "AUTHORIZATION_HISTORY_UNAVAILABLE", "authorization history unavailable")
		return
	}
	if err := h.history.Revoke(c.Request.Context(), p, c.Param("id")); err != nil {
		historyError(c, err)
		return
	}
	noStore(c)
	c.Status(http.StatusNoContent)
}
func historyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, toolruntime.ErrAuthorizationHistoryPage):
		credentialError(c, 400, "AUTHORIZATION_HISTORY_PAGE_INVALID", "page must be positive and page_size between 1 and 100")
	case errors.Is(err, toolruntime.ErrAuthorizationHistoryForbidden):
		credentialError(c, 403, "AUTHORIZATION_HISTORY_FORBIDDEN", "authorization history access denied")
	case errors.Is(err, toolruntime.ErrAuthorizationRecordNotFound):
		credentialError(c, 404, "AUTHORIZATION_RECORD_NOT_FOUND", "authorization record not found")
	default:
		credentialError(c, 500, "AUTHORIZATION_HISTORY_UNAVAILABLE", "authorization history unavailable")
	}
}
