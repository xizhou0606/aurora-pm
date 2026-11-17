package collaborationhttp

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

type LeaseLifecycle interface {
	Acquire(context.Context, collaboration.Principal, collaboration.CellKey, string) (collaboration.CellLease, error)
	Renew(context.Context, collaboration.Principal, collaboration.CellLeaseProof) (collaboration.CellLease, error)
	Release(context.Context, collaboration.Principal, collaboration.CellLeaseProof) error
	Disconnect(context.Context, collaboration.Principal, collaboration.Room) (int64, error)
}

type LeaseHandler struct {
	principals PrincipalResolver
	rooms      RoomResolver
	leases     LeaseLifecycle
}

func NewLeaseHandler(principals PrincipalResolver, rooms RoomResolver, leases LeaseLifecycle) (*LeaseHandler, error) {
	if principals == nil || rooms == nil || leases == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &LeaseHandler{principals: principals, rooms: rooms, leases: leases}, nil
}

func (handler *LeaseHandler) RegisterRoutes(group *gin.RouterGroup) {
	routes := group.Group("/document-resources/:resourceId/collaboration/cell-leases")
	routes.POST("/acquire", handler.Acquire)
	routes.POST("/renew", handler.Renew)
	routes.POST("/release", handler.Release)
	routes.POST("/disconnect", handler.Disconnect)
}

type cellRequest struct {
	TableID          string `json:"table_id" binding:"required"`
	RecordID         string `json:"record_id" binding:"required"`
	FieldID          string `json:"field_id" binding:"required"`
	EditorInstanceID string `json:"editor_instance_id" binding:"required"`
}

type proofRequest struct {
	cellRequest
	LeaseID      string `json:"lease_id" binding:"required"`
	FencingToken uint64 `json:"fencing_token" binding:"required"`
}

type cellLeaseHolder struct {
	ActorUserID uint      `json:"actor_user_id"`
	ActorName   string    `json:"actor_display_name"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (handler *LeaseHandler) Acquire(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	var request cellRequest
	if c.ShouldBindJSON(&request) != nil {
		writeCollaborationError(c, collaboration.ErrInvalidInput)
		return
	}
	lease, err := handler.leases.Acquire(c.Request.Context(), principal, cellKey(room, request), request.EditorInstanceID)
	if err != nil {
		if lease.LeaseID != "" {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{"code": "lease_held"},
				"holder": cellLeaseHolder{
					ActorUserID: lease.ActorUserID, ActorName: lease.ActorName, ExpiresAt: lease.ExpiresAt,
				},
			})
			return
		}
		writeCollaborationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"lease": lease})
}

func (handler *LeaseHandler) Renew(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	request, ok := bindProof(c)
	if !ok {
		return
	}
	lease, err := handler.leases.Renew(c.Request.Context(), principal, proof(room, principal, request))
	if err != nil {
		writeCollaborationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"lease": lease})
}

func (handler *LeaseHandler) Release(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	request, ok := bindProof(c)
	if !ok {
		return
	}
	if err := handler.leases.Release(c.Request.Context(), principal, proof(room, principal, request)); err != nil {
		writeCollaborationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *LeaseHandler) Disconnect(c *gin.Context) {
	principal, room, ok := requireCollaborationResourcePolicy(c, handler.principals, handler.rooms)
	if !ok {
		return
	}
	if _, err := handler.leases.Disconnect(c.Request.Context(), principal, room); err != nil {
		writeCollaborationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func requireCollaborationResourcePolicy(
	c *gin.Context,
	principals PrincipalResolver,
	rooms RoomResolver,
) (collaboration.Principal, collaboration.Room, bool) {
	principal, err := principals.ResolveCollaborationPrincipal(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return collaboration.Principal{}, collaboration.Room{}, false
	}
	room, err := rooms.ResolveCollaborationRoom(c, principal)
	if err != nil || room.WorkspaceID != principal.WorkspaceID {
		if err == nil {
			err = collaboration.ErrForbidden
		}
		writeCollaborationError(c, err)
		return collaboration.Principal{}, collaboration.Room{}, false
	}
	return principal, room, true
}

func bindProof(c *gin.Context) (proofRequest, bool) {
	var request proofRequest
	if c.ShouldBindJSON(&request) != nil {
		writeCollaborationError(c, collaboration.ErrInvalidInput)
		return proofRequest{}, false
	}
	return request, true
}

func cellKey(room collaboration.Room, request cellRequest) collaboration.CellKey {
	return collaboration.CellKey{Room: room, TableID: request.TableID, RecordID: request.RecordID, FieldID: request.FieldID}
}

func proof(room collaboration.Room, principal collaboration.Principal, request proofRequest) collaboration.CellLeaseProof {
	return collaboration.CellLeaseProof{
		Key: cellKey(room, request.cellRequest), LeaseID: request.LeaseID, FencingToken: request.FencingToken,
		ConnectionID: principal.ConnectionID, EditorInstanceID: request.EditorInstanceID,
	}
}
