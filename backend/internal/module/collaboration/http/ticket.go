package collaborationhttp

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type ConnectionTicket struct {
	Ticket       string    `json:"ticket"`
	WebSocketURL string    `json:"websocket_url"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type ConnectionTicketIssuer interface {
	IssueDocumentConnectionTicket(context.Context, collaboration.Principal, collaboration.Room) (ConnectionTicket, error)
}

type TicketHandler struct {
	principals PrincipalResolver
	rooms      RoomResolver
	issuer     ConnectionTicketIssuer
}

func NewTicketHandler(
	principals PrincipalResolver,
	rooms RoomResolver,
	issuer ConnectionTicketIssuer,
) (*TicketHandler, error) {
	if principals == nil || rooms == nil || issuer == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &TicketHandler{principals: principals, rooms: rooms, issuer: issuer}, nil
}

func (handler *TicketHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/document-resources/:resourceId/collaboration/ticket", handler.IssueTicket)
}

func (handler *TicketHandler) IssueTicket(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	ticket, err := handler.issuer.IssueDocumentConnectionTicket(c.Request.Context(), principal, room)
	if err != nil {
		writeCollaborationError(c, err)
		return
	}
	if ticket.Ticket == "" || ticket.WebSocketURL == "" || !ticket.ExpiresAt.After(time.Now()) {
		writeCollaborationError(c, collaboration.ErrDataCorrupt)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, ticket)
}

var _ interface{ RegisterRoutes(*gin.RouterGroup) } = (*TicketHandler)(nil)
