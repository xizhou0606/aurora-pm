package collaborationhttp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type PresenceLifecycle interface {
	Heartbeat(context.Context, collaboration.Principal, collaboration.Room, []byte) (collaboration.Presence, error)
	Leave(context.Context, collaboration.Principal, collaboration.Room) error
}

type PresenceHandler struct {
	principals PrincipalResolver
	rooms      RoomResolver
	presence   PresenceLifecycle
}

func NewPresenceHandler(
	principals PrincipalResolver,
	rooms RoomResolver,
	presence PresenceLifecycle,
) (*PresenceHandler, error) {
	if principals == nil || rooms == nil || presence == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &PresenceHandler{principals: principals, rooms: rooms, presence: presence}, nil
}

func (handler *PresenceHandler) RegisterRoutes(group *gin.RouterGroup) {
	routes := group.Group("/document-resources/:resourceId/collaboration/presence")
	routes.POST("/heartbeat", handler.HeartbeatPresence)
	routes.DELETE("", handler.LeavePresence)
}

func (handler *PresenceHandler) HeartbeatPresence(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	var request struct {
		State json.RawMessage `json:"state" binding:"required"`
	}
	if c.ShouldBindJSON(&request) != nil {
		writeCollaborationError(c, collaboration.ErrInvalidInput)
		return
	}
	presence, err := handler.presence.Heartbeat(c.Request.Context(), principal, room, request.State)
	if err != nil {
		writeCollaborationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"presence": publicPresence(presence)})
}

func publicPresence(value collaboration.Presence) collaboration.Presence {
	// The storage connection id is an ownership factor for lease/presence
	// cleanup. Never broadcast it. The public digest remains stable enough for a
	// UI list key but cannot be replayed as X-PM-Collaboration-Connection.
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"%d\x00%s\x00%d\x00%s", value.Room.WorkspaceID, value.Room.ResourceID, value.ActorUserID, value.ConnectionID,
	)))
	value.ConnectionID = fmt.Sprintf("presence-%x", digest[:16])
	return value
}

func (handler *PresenceHandler) LeavePresence(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	if err := handler.presence.Leave(c.Request.Context(), principal, room); err != nil {
		writeCollaborationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
