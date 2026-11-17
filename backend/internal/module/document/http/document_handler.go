package documenthttp

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func (handler *Handler) getDocument(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	body, err := handler.service.GetDocument(c.Request.Context(), principal, documentID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, body)
}

func (handler *Handler) exportDocument(c *gin.Context) {
	principal, ok := requestContext(c)
	if !ok {
		return
	}
	documentID, idOK := resourceID(c, "id")
	if !idOK {
		return
	}
	body, err := handler.service.ExportDocument(c.Request.Context(), principal, documentID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	writeData(c, http.StatusOK, body)
}

func (handler *Handler) updateDocument(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request struct {
		Content          json.RawMessage `json:"content" binding:"required"`
		ExpectedRevision uint            `json:"expected_revision" binding:"required"`
		SchemaVersion    uint            `json:"schema_version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	body, err := handler.service.UpdateDocument(
		c.Request.Context(), principal, documentID, request.ExpectedRevision, request.SchemaVersion, request.Content,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, body)
}

func (handler *Handler) listBindings(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	bindings, err := handler.service.ListBindings(c.Request.Context(), principal, documentID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"items": bindings})
}

type bindingRequest struct {
	RequirementID uint `json:"requirement_id" binding:"required"`
	Confirmed     bool `json:"confirmed"`
}

func (handler *Handler) previewBinding(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request bindingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	preview, err := handler.service.PreviewBinding(
		c.Request.Context(), principal, documentID, request.RequirementID,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, preview)
}

func (handler *Handler) bindRequirement(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request bindingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	binding, err := handler.service.BindRequirement(
		c.Request.Context(), principal, documentID, request.RequirementID, request.Confirmed,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusCreated, binding)
}

func (handler *Handler) unbindRequirement(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	bindingID, bindingOK := resourceID(c, "bindingId")
	if !ok || !documentOK || !bindingOK {
		return
	}
	if err := handler.service.UnbindRequirement(
		c.Request.Context(), principal, documentID, bindingID,
	); err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"deleted_id": bindingID})
}
