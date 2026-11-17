package documenthttp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

type createNodeRequest struct {
	Collaborative bool              `json:"collaborative"`
	ParentID      *uint             `json:"parent_id"`
	Type          document.NodeType `json:"type" binding:"required"`
	Title         string            `json:"title" binding:"required"`
}

func (handler *Handler) createNode(c *gin.Context) {
	principal, ok := requestContext(c)
	if !ok {
		return
	}
	var request createNodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.CreateNode
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalCreateNode
	}
	result, err := operation(c.Request.Context(), principal, document.CreateNodeInput{
		ParentID: request.ParentID, Type: request.Type, Title: request.Title,
		Collaborative: request.Collaborative,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusCreated, result)
}

func (handler *Handler) renameNode(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.RenameNode
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalRenameNode
	}
	node, err := operation(c.Request.Context(), principal, nodeID, request.Title)
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, node)
}

func (handler *Handler) moveNode(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	var request struct {
		ParentID                  *uint   `json:"parent_id"`
		SortOrder                 *int64  `json:"sort_order"`
		BeforeNodeID              *uint   `json:"before_node_id"`
		AfterNodeID               *uint   `json:"after_node_id"`
		ExpectedDirectoryRevision *uint64 `json:"expected_directory_revision"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.MoveNode
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalMoveNode
	}
	node, err := operation(c.Request.Context(), principal, nodeID, document.MoveNodeInput{
		ParentID: request.ParentID, SortOrder: request.SortOrder,
		BeforeNodeID: request.BeforeNodeID, AfterNodeID: request.AfterNodeID, ExpectedDirectoryRevision: request.ExpectedDirectoryRevision,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, node)
}

func (handler *Handler) getArchiveImpact(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	operation := handler.service.ArchiveImpact
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalArchiveImpact
	}
	impact, err := operation(c.Request.Context(), principal, nodeID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, impact)
}

func (handler *Handler) archiveNode(c *gin.Context) {
	principal, ok := requestContext(c)
	nodeID, idOK := resourceID(c, "id")
	if !ok || !idOK {
		return
	}
	confirmed, _ := strconv.ParseBool(c.Query("confirm_non_empty"))
	expectedDescendants, descendantsOK := optionalNonnegativeInt64(c, "expected_descendant_count")
	expectedDocuments, documentsOK := optionalNonnegativeInt64(c, "expected_document_count")
	if !descendantsOK || !documentsOK {
		writeError(c, document.ErrInvalidInput)
		return
	}
	operation := handler.service.ArchiveNode
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalArchiveNode
	}
	if err := operation(c.Request.Context(), principal, nodeID, document.ArchiveConfirmation{
		ConfirmNonEmpty: confirmed, ExpectedDescendantCount: expectedDescendants,
		ExpectedDocumentCount: expectedDocuments,
	}); err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"archived_id": nodeID})
}

func optionalNonnegativeInt64(c *gin.Context, name string) (*int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, false
	}
	return &value, true
}
