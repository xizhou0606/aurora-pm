package documenthttp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

type contextParentRequest struct {
	ParentID *uint `json:"parent_id"`
}

func (handler *Handler) copyNode(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request contextParentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.CopyNode
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalCopyNode
	}
	result, err := operation(c.Request.Context(), principal, nodeID, document.CopyNodeInput{ParentID: request.ParentID})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusCreated, result)
}

func (handler *Handler) createShortcut(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request contextParentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.CreateShortcut
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalCreateShortcut
	}
	node, err := operation(c.Request.Context(), principal, nodeID, document.CreateShortcutInput{ParentID: request.ParentID})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusCreated, node)
}

func (handler *Handler) updateNodePreference(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request struct {
		Pinned    *bool `json:"pinned"`
		Favorited *bool `json:"favorited"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	preference, err := handler.service.UpdateNodePreference(c.Request.Context(), principal, nodeID, document.UpdateNodePreferenceInput{
		Pinned: request.Pinned, Favorited: request.Favorited,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, preference)
}

func (handler *Handler) transferDocumentOwner(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request struct {
		OwnerUserID uint `json:"owner_user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	body, err := handler.service.TransferDocumentOwner(c.Request.Context(), principal, documentID, request.OwnerUserID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, body)
}
