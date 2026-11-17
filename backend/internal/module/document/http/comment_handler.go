package documenthttp

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

type commentBodyRequest struct {
	Body string `json:"body" binding:"required"`
}

type createCommentThreadRequest struct {
	Anchor document.CommentAnchor `json:"anchor" binding:"required"`
	Body   string                 `json:"body" binding:"required"`
}

type replyCommentRequest struct {
	ParentCommentID *uint  `json:"parent_comment_id"`
	Body            string `json:"body" binding:"required"`
}

func (handler *Handler) listCommentThreads(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	if !ok || !documentOK {
		return
	}
	limit, limitOK := optionalNonnegativeInt(c, "limit")
	offset, offsetOK := optionalNonnegativeInt(c, "offset")
	if !limitOK || !offsetOK {
		writeError(c, document.ErrInvalidInput)
		return
	}
	page, err := handler.service.ListCommentThreads(c.Request.Context(), principal, documentID, document.CommentThreadFilter{
		Status: document.CommentThreadStatus(strings.TrimSpace(c.Query("status"))), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, page)
}

func (handler *Handler) createCommentThread(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	if !ok || !documentOK {
		return
	}
	var request createCommentThreadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	thread, err := handler.service.CreateCommentThread(c.Request.Context(), principal, documentID, document.CreateCommentThreadInput{
		Anchor: request.Anchor, Body: request.Body,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusCreated, thread)
}

func (handler *Handler) replyToCommentThread(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	threadID, threadOK := resourceID(c, "threadId")
	if !ok || !documentOK || !threadOK {
		return
	}
	var request replyCommentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	comment, err := handler.service.ReplyToCommentThread(c.Request.Context(), principal, documentID, threadID, document.ReplyCommentInput{
		ParentCommentID: request.ParentCommentID, Body: request.Body,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusCreated, comment)
}

func (handler *Handler) editOwnComment(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	commentID, commentOK := resourceID(c, "commentId")
	if !ok || !documentOK || !commentOK {
		return
	}
	var request commentBodyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, document.ErrInvalidInput)
		return
	}
	comment, err := handler.service.EditOwnComment(c.Request.Context(), principal, documentID, commentID, request.Body)
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, comment)
}

func (handler *Handler) deleteOwnComment(c *gin.Context) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	commentID, commentOK := resourceID(c, "commentId")
	if !ok || !documentOK || !commentOK {
		return
	}
	if err := handler.service.DeleteOwnComment(c.Request.Context(), principal, documentID, commentID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *Handler) resolveCommentThread(c *gin.Context) {
	handler.setCommentThreadStatus(c, true)
}

func (handler *Handler) reopenCommentThread(c *gin.Context) {
	handler.setCommentThreadStatus(c, false)
}

func (handler *Handler) setCommentThreadStatus(c *gin.Context, resolve bool) {
	principal, ok := requestContext(c)
	documentID, documentOK := resourceID(c, "id")
	threadID, threadOK := resourceID(c, "threadId")
	if !ok || !documentOK || !threadOK {
		return
	}
	var (
		thread document.CommentThread
		err    error
	)
	if resolve {
		thread, err = handler.service.ResolveCommentThread(c.Request.Context(), principal, documentID, threadID)
	} else {
		thread, err = handler.service.ReopenCommentThread(c.Request.Context(), principal, documentID, threadID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, thread)
}

func optionalNonnegativeInt(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}
