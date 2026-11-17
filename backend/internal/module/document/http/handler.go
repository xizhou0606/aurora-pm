package documenthttp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type Handler struct {
	service  *document.Service
	displays identity.UserDisplayReader
}

func NewHandler(service *document.Service) (*Handler, error) {
	if service == nil {
		return nil, document.ErrInvalidInput
	}
	return &Handler{service: service}, nil
}

func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/documents/access-scope", handler.getAccessScope)
	group.GET("/document-spaces/default", handler.getDefaultSpace)
	group.GET("/document-nodes/tree", handler.getTree)
	group.POST("/document-nodes", handler.createNode)
	group.POST("/document-nodes/:id/copies", handler.copyNode)
	group.POST("/document-nodes/:id/shortcuts", handler.createShortcut)
	group.PUT("/document-nodes/:id/preferences", handler.updateNodePreference)
	group.PATCH("/document-nodes/:id", handler.renameNode)
	group.POST("/document-nodes/:id/move", handler.moveNode)
	group.GET("/document-nodes/:id/deletion-impact", handler.getArchiveImpact)
	group.DELETE("/document-nodes/:id", handler.archiveNode)
	group.GET("/personal-document-spaces/default", handler.getDefaultSpace)
	group.GET("/personal-document-nodes/tree", handler.getTree)
	group.POST("/personal-document-nodes", handler.createNode)
	group.POST("/personal-document-nodes/:id/copies", handler.copyNode)
	group.POST("/personal-document-nodes/:id/shortcuts", handler.createShortcut)
	group.PUT("/personal-document-nodes/:id/preferences", handler.updateNodePreference)
	group.PATCH("/personal-document-nodes/:id", handler.renameNode)
	group.POST("/personal-document-nodes/:id/move", handler.moveNode)
	group.GET("/personal-document-nodes/:id/deletion-impact", handler.getArchiveImpact)
	group.DELETE("/personal-document-nodes/:id", handler.archiveNode)
	group.GET("/documents/:id", handler.getDocument)
	group.GET("/documents/:id/export", handler.exportDocument)
	group.PATCH("/documents/:id", handler.updateDocument)
	group.PUT("/documents/:id/owner", handler.transferDocumentOwner)
	group.GET("/documents/:id/assets", handler.listAssets)
	group.POST("/documents/:id/assets", handler.uploadAsset)
	group.GET("/documents/:id/assets/:assetId/content", handler.downloadAsset)
	group.DELETE("/documents/:id/assets/:assetId", handler.deleteAsset)
	group.GET("/documents/:id/comment-threads", handler.listCommentThreads)
	group.POST("/documents/:id/comment-threads", handler.createCommentThread)
	group.POST("/documents/:id/comment-threads/:threadId/comments", handler.replyToCommentThread)
	group.PATCH("/documents/:id/comments/:commentId", handler.editOwnComment)
	group.DELETE("/documents/:id/comments/:commentId", handler.deleteOwnComment)
	group.POST("/documents/:id/comment-threads/:threadId/resolve", handler.resolveCommentThread)
	group.POST("/documents/:id/comment-threads/:threadId/reopen", handler.reopenCommentThread)
	group.GET("/documents/:id/bindings", handler.listBindings)
	group.POST("/documents/:id/bindings/preview", handler.previewBinding)
	group.POST("/documents/:id/bindings", handler.bindRequirement)
	group.DELETE("/documents/:id/bindings/:bindingId", handler.unbindRequirement)
}

func principal(c *gin.Context) (document.Principal, bool) {
	principalValue, principalOK := c.Get(pmauth.PrincipalContextKey)
	identity, principalTyped := principalValue.(pmauth.Principal)
	workspaceValue, workspaceOK := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, workspaceTyped := workspaceValue.(uint)
	participantValue, participantOK := c.Get(pmauth.ActorFactUserIDContextKey)
	participantID, participantTyped := participantValue.(uint)
	if !principalOK || !principalTyped || identity.UserID == 0 || !workspaceOK || !workspaceTyped || workspaceID == 0 {
		return document.Principal{}, false
	}
	if !participantOK || !participantTyped || participantID == 0 {
		participantID = identity.UserID
	}
	linkAccessSecret := strings.TrimSpace(c.GetHeader("X-PM-Resource-Access"))
	if len(linkAccessSecret) > 256 {
		return document.Principal{}, false
	}
	return document.Principal{
		UserID: identity.UserID, ParticipantUserID: participantID, WorkspaceID: workspaceID,
		LinkAccessSecret: linkAccessSecret,
	}, true
}

func resourceID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || value == 0 {
		writeError(c, document.ErrInvalidInput)
		return 0, false
	}
	return uint(value), true
}

func requestContext(c *gin.Context) (document.Principal, bool) {
	value, ok := principal(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
	}
	return value, ok
}

func writeError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "DOCUMENT_INTERNAL_ERROR"
	switch {
	case errors.Is(err, document.ErrDirectoryConflict):
		status, code = http.StatusConflict, "DOCUMENT_DIRECTORY_CONFLICT"
	case errors.Is(err, document.ErrInvalidInput):
		status, code = http.StatusBadRequest, "DOCUMENT_INVALID_INPUT"
	case errors.Is(err, document.ErrForbidden):
		status, code = http.StatusForbidden, "DOCUMENT_FORBIDDEN"
	case errors.Is(err, document.ErrSpaceNotFound), errors.Is(err, document.ErrNodeNotFound),
		errors.Is(err, document.ErrDocumentNotFound), errors.Is(err, document.ErrBindingNotFound),
		errors.Is(err, document.ErrAssetNotFound), errors.Is(err, document.ErrCommentThreadNotFound),
		errors.Is(err, document.ErrCommentNotFound):
		status, code = http.StatusNotFound, "DOCUMENT_NOT_FOUND"
	case errors.Is(err, document.ErrAssetStorageUnavailable):
		status, code = http.StatusServiceUnavailable, "DOCUMENT_ASSET_STORAGE_UNAVAILABLE"
	case errors.Is(err, document.ErrCommentsUnavailable):
		status, code = http.StatusServiceUnavailable, "DOCUMENT_COMMENTS_UNAVAILABLE"
	case errors.Is(err, document.ErrContextActionsUnavailable):
		status, code = http.StatusServiceUnavailable, "DOCUMENT_CONTEXT_ACTIONS_UNAVAILABLE"
	case errors.Is(err, document.ErrOwnerUserInvalid):
		status, code = http.StatusBadRequest, "DOCUMENT_OWNER_USER_INVALID"
	case errors.Is(err, document.ErrShortcutTargetInvalid):
		status, code = http.StatusBadRequest, "DOCUMENT_SHORTCUT_TARGET_INVALID"
	case errors.Is(err, document.ErrSystemManagedNode):
		status, code = http.StatusConflict, "DOCUMENT_SYSTEM_NODE_LOCKED"
	case errors.Is(err, document.ErrLegacyContentWriteDisabled):
		status, code = http.StatusConflict, "DOCUMENT_COLLABORATION_CUTOVER"
	case errors.Is(err, document.ErrParentNotFolder):
		status, code = http.StatusConflict, "DOCUMENT_PARENT_NOT_FOLDER"
	case errors.Is(err, document.ErrHierarchyCycle):
		status, code = http.StatusConflict, "DOCUMENT_HIERARCHY_CYCLE"
	case errors.Is(err, document.ErrArchiveConfirmationRequired):
		status, code = http.StatusConflict, "DOCUMENT_ARCHIVE_CONFIRMATION_REQUIRED"
	case errors.Is(err, document.ErrArchiveImpactChanged):
		status, code = http.StatusConflict, "DOCUMENT_ARCHIVE_IMPACT_CHANGED"
	case errors.Is(err, document.ErrBindingConfirmationRequired):
		status, code = http.StatusConflict, "DOCUMENT_BINDING_CONFIRMATION_REQUIRED"
	case errors.Is(err, document.ErrRequirementUnavailable):
		status, code = http.StatusNotFound, "DOCUMENT_REQUIREMENT_UNAVAILABLE"
	case errors.Is(err, document.ErrBindingConflict):
		status, code = http.StatusConflict, "DOCUMENT_BINDING_CONFLICT"
	case errors.Is(err, document.ErrRevisionConflict):
		status, code = http.StatusConflict, "DOCUMENT_REVISION_CONFLICT"
	case errors.Is(err, document.ErrCommentMentionInvalid):
		status, code = http.StatusBadRequest, "DOCUMENT_COMMENT_MENTION_INVALID"
	case errors.Is(err, document.ErrCommentThreadResolved):
		status, code = http.StatusConflict, "DOCUMENT_COMMENT_THREAD_RESOLVED"
	case errors.Is(err, document.ErrCommentTransitionConflict):
		status, code = http.StatusConflict, "DOCUMENT_COMMENT_TRANSITION_CONFLICT"
	}
	payload := gin.H{"code": code, "error": err.Error()}
	var conflict document.RevisionConflictError
	if errors.As(err, &conflict) {
		payload["details"] = gin.H{"latest_revision": conflict.LatestRevision}
	}
	var impactChanged document.ArchiveImpactChangedError
	if errors.As(err, &impactChanged) {
		payload["details"] = gin.H{"latest_impact": impactChanged.Latest}
	}
	c.AbortWithStatusJSON(status, payload)
}

func (handler *Handler) getAccessScope(c *gin.Context) {
	principal, ok := requestContext(c)
	if !ok {
		return
	}
	if err := handler.service.RequireAllScope(c.Request.Context(), principal); err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"scope": "all"})
}

func (handler *Handler) getDefaultSpace(c *gin.Context) {
	principal, ok := requestContext(c)
	if !ok {
		return
	}
	operation := handler.service.DefaultSpace
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalDefaultSpace
	}
	space, err := operation(c.Request.Context(), principal)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, space)
}

func (handler *Handler) getTree(c *gin.Context) {
	principal, ok := requestContext(c)
	if !ok {
		return
	}
	operation := handler.service.Tree
	if personalDirectoryRequest(c) {
		operation = handler.service.PersonalTree
	}
	tree, err := operation(c.Request.Context(), principal)
	if err != nil {
		writeError(c, err)
		return
	}
	handler.writeData(c, http.StatusOK, tree)
}

func personalDirectoryRequest(c *gin.Context) bool {
	return strings.Contains(c.FullPath(), "/personal-document-")
}
