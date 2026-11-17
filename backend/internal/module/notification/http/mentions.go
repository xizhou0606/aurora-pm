package notificationhttp

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type MentionHandler struct {
	reader     notification.MentionReader
	membership notification.MembershipVerifier
}

func NewMentionHandler(reader notification.MentionReader, membership notification.MembershipVerifier) *MentionHandler {
	return &MentionHandler{reader: reader, membership: membership}
}

func (h *MentionHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/notifications/mentions", h.list)
}

func (h *MentionHandler) list(c *gin.Context) {
	principalValue, _ := c.Get(pmauth.PrincipalContextKey)
	principal, ok := principalValue.(pmauth.Principal)
	workspaceValue, _ := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, valid := workspaceValue.(uint)
	if !ok || !valid || principal.UserID == 0 || workspaceID == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if err := h.membership.VerifyWorkspaceMembership(c.Request.Context(), principal.UserID, workspaceID); err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var before uint64
	if raw := c.Query("before"); raw != "" {
		var err error
		before, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}
	page, err := h.reader.ListMentions(c.Request.Context(), workspaceID, principal.UserID, uint(before))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "MENTION_LIST_FAILED", "error": "Unable to load mentions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": page})
}
