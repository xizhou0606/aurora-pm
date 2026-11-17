package notificationhttp

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"net/http"
	"strconv"
	"time"
)

type InboxHandler struct {
	store      notification.InboxStore
	membership notification.MembershipVerifier
}

func NewInboxHandler(store notification.InboxStore, membership notification.MembershipVerifier) *InboxHandler {
	return &InboxHandler{store, membership}
}
func (h *InboxHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/notifications/inbox", h.list)
	group.POST("/notifications/inbox/:id/read", h.read)
	group.POST("/notifications/inbox/read-all", h.readAll)
}
func (h *InboxHandler) principal(c *gin.Context) (uint, uint, bool) {
	value, _ := c.Get(pmauth.PrincipalContextKey)
	p, ok := value.(pmauth.Principal)
	scope, _ := c.Get(pmauth.WorkspaceIDContextKey)
	w, valid := scope.(uint)
	if !ok || !valid || w == 0 || p.UserID == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return 0, 0, false
	}
	if h.membership.VerifyWorkspaceMembership(c.Request.Context(), p.UserID, w) != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return 0, 0, false
	}
	return w, p.UserID, true
}
func (h *InboxHandler) list(c *gin.Context) {
	w, u, ok := h.principal(c)
	if !ok {
		return
	}
	category := c.Query("category")
	if !notification.ValidInboxCategory(category) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	before := uint64(0)
	if raw := c.Query("before"); raw != "" {
		var err error
		before, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}
	page, err := h.store.ListInbox(c.Request.Context(), w, u, notification.InboxFilter{Before: uint(before), Category: category, UnreadOnly: c.Query("unread") == "true"})
	if err != nil {
		c.JSON(500, gin.H{"code": "INBOX_READ_FAILED"})
		return
	}
	c.JSON(200, gin.H{"data": page})
}
func (h *InboxHandler) read(c *gin.Context) {
	w, u, ok := h.principal(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.AbortWithStatus(400)
		return
	}
	if err = h.store.MarkRead(c.Request.Context(), w, u, uint(id), time.Now().UTC()); err != nil {
		if errors.Is(err, notification.ErrInboxNotFound) {
			c.JSON(404, gin.H{"code": "NOTIFICATION_NOT_FOUND"})
		} else {
			c.JSON(500, gin.H{"code": "INBOX_UPDATE_FAILED"})
		}
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"read": true}})
}
func (h *InboxHandler) readAll(c *gin.Context) {
	w, u, ok := h.principal(c)
	if !ok {
		return
	}
	var body struct {
		Through  uint   `json:"through_id"`
		Category string `json:"category"`
	}
	if c.ShouldBindJSON(&body) != nil || body.Through == 0 || !notification.ValidInboxCategory(body.Category) {
		c.AbortWithStatus(400)
		return
	}
	if err := h.store.MarkAllRead(c.Request.Context(), w, u, body.Through, body.Category, time.Now().UTC()); err != nil {
		c.JSON(500, gin.H{"code": "INBOX_UPDATE_FAILED"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"read": true}})
}
