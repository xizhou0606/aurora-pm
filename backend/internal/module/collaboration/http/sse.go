package collaborationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type PrincipalResolver interface {
	ResolveCollaborationPrincipal(*gin.Context) (collaboration.Principal, error)
}

type RoomResolver interface {
	ResolveCollaborationRoom(*gin.Context, collaboration.Principal) (collaboration.Room, error)
}

type Streamer interface {
	Stream(context.Context, collaboration.Principal, collaboration.Room, uint64, func(collaboration.StreamItem) error) error
}

// Handler exposes a registration seam but is not mounted by this module. The
// owning app must attach canonical action middleware and shared resolvers.
type Handler struct {
	principals PrincipalResolver
	rooms      RoomResolver
	streamer   Streamer
}

func NewHandler(principals PrincipalResolver, rooms RoomResolver, streamer Streamer) (*Handler, error) {
	if principals == nil || rooms == nil || streamer == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &Handler{principals: principals, rooms: rooms, streamer: streamer}, nil
}

func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/document-resources/:resourceId/collaboration/events", handler.Stream)
}

func (handler *Handler) Stream(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	after, err := parseCursor(c.GetHeader("Last-Event-ID"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()
	err = handler.streamer.Stream(c.Request.Context(), principal, room, after, func(item collaboration.StreamItem) error {
		for index := range item.Presence {
			item.Presence[index] = publicPresence(item.Presence[index])
		}
		payload, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			return marshalErr
		}
		if item.Event != nil {
			_, err = c.Writer.WriteString("id: " + strconv.FormatUint(item.Event.Sequence, 10) + "\n")
			if err != nil {
				return err
			}
		}
		if _, err = c.Writer.WriteString("event: " + item.Kind + "\ndata: " + string(payload) + "\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	})
	if errors.Is(err, collaboration.ErrForbidden) {
		_, _ = c.Writer.WriteString("event: access_revoked\ndata: {}\n\n")
		c.Writer.Flush()
	}
}

func parseCursor(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}
