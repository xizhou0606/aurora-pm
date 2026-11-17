package resourceaccesshttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type policyApplication interface {
	AuthorizeOperation(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, documentlibrary.Operation) error
	GetPolicy(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) (resourceaccess.PolicyView, error)
	ReplacePolicy(
		context.Context, resourceaccess.Principal, documentlibrary.ResourceID, resourceaccess.ReplacePolicyInput,
	) (resourceaccess.PolicyView, error)
}

type linkApplication interface {
	Create(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, resourceaccess.CreateShareLinkInput) (resourceaccess.CreatedShareLink, error)
	List(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) ([]resourceaccess.ShareLink, error)
	Revoke(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, string) error
	Rotate(context.Context, resourceaccess.Principal, documentlibrary.ResourceID, string) (resourceaccess.CreatedShareLink, error)
	Exchange(context.Context, resourceaccess.Principal, string) (resourceaccess.ExchangeResult, error)
}

type Handler struct {
	requests requestApplication
	policies policyApplication
	links    linkApplication
	subjects identity.SubjectDirectory
}

func NewHandler(policies policyApplication, links linkApplication, subjects identity.SubjectDirectory) (*Handler, error) {
	if policies == nil || links == nil || subjects == nil {
		return nil, resourceaccess.ErrUnavailable
	}
	return &Handler{policies: policies, links: links, subjects: subjects}, nil
}

// RegisterRoutes mounts the catalog-owned routes below an authenticated,
// workspace-scoped API group. The composition root remains responsible for
// registering canonical Action ownership before calling this method.
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/documents/:id/access-request-context", handler.documentRequestContext)
	group.GET("/bitables/:resourceRef/access-request-context", handler.bitableRequestContext)
	group.GET("/document-resources/:resourceId/access-requests", handler.listRequests)
	group.POST("/document-resources/:resourceId/access-requests", handler.createRequest)
	group.PATCH("/document-resources/:resourceId/access-requests/:requestId", handler.updateRequest)
	group.GET("/document-resources/:resourceId/access-policy", handler.getPolicy)
	group.PUT("/document-resources/:resourceId/access-policy", handler.replacePolicy)
	group.GET("/document-resource-subjects", handler.searchSubjects)
	group.GET("/document-resources/:resourceId/share-links", handler.listLinks)
	group.POST("/document-resources/:resourceId/share-links", handler.createLink)
	group.DELETE("/document-resources/:resourceId/share-links/:linkId", handler.revokeLink)
	group.POST("/document-resources/:resourceId/share-links/:linkId/rotate", handler.rotateLink)
	group.POST("/document-share-links/exchange", handler.exchangeLink)
}

func (handler *Handler) getPolicy(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationViewAccess)
	if !ok {
		return
	}
	policy, err := handler.policies.GetPolicy(c.Request.Context(), principal, resourceID)
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, policy)
}

func (handler *Handler) replacePolicy(c *gin.Context) {
	principal, resourceID, ok := handler.requireDocumentResourcePolicy(c, documentlibrary.OperationManageAccess)
	if !ok {
		return
	}
	var request replacePolicyRequest
	if !bindJSON(c, &request) {
		return
	}
	if request.ExpectedPolicyRevision == nil {
		writeError(c, resourceaccess.ErrInvalidInput)
		return
	}
	policy, err := handler.policies.ReplacePolicy(c.Request.Context(), principal, resourceID, resourceaccess.ReplacePolicyInput{
		Mode: resourceaccess.Mode(request.Mode), WorkspaceLevel: levelPointer(request.WorkspaceLevel),
		Rules: request.rules(), ExpectedPolicyRevision: *request.ExpectedPolicyRevision,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	writeData(c, http.StatusOK, policy)
}

func (handler *Handler) searchSubjects(c *gin.Context) {
	principal, ok := requestPrincipal(c)
	if !ok {
		return
	}
	limit, offset, ok := subjectPagination(c)
	if !ok {
		return
	}
	kind := identity.SubjectKind(strings.TrimSpace(c.Query("kind")))
	if !kind.Valid() {
		writeError(c, resourceaccess.ErrInvalidInput)
		return
	}
	page, err := handler.subjects.SearchSubjects(c.Request.Context(), principal.WorkspaceID, identity.SubjectSearch{
		Kind: kind, Query: c.Query("q"), Page: offset/limit + 1, PageSize: limit,
	})
	if err != nil {
		if errors.Is(err, identity.ErrSubjectKindUnavailable) {
			writeError(c, resourceaccess.ErrUnavailable)
		} else {
			writeError(c, resourceaccess.ErrInvalidInput)
		}
		return
	}
	writeData(c, http.StatusOK, gin.H{
		"items": page.Items, "limit": limit, "offset": offset, "total": page.Total, "has_more": page.HasMore,
		"subject_capabilities": resourceaccess.SubjectCapabilities(handler.subjects),
	})
}

// requireDocumentResourcePolicy is the stable object-policy marker consumed
// by route governance. It authorizes read-only policy visibility without revealing whether an
// inaccessible resource exists.
func (handler *Handler) requireDocumentResourcePolicy(
	c *gin.Context, operation documentlibrary.Operation,
) (resourceaccess.Principal, documentlibrary.ResourceID, bool) {
	principal, ok := requestPrincipal(c)
	if !ok {
		return resourceaccess.Principal{}, "", false
	}
	resourceID := documentlibrary.ResourceID(strings.TrimSpace(c.Param("resourceId")))
	if resourceID == "" {
		writeError(c, resourceaccess.ErrInvalidInput)
		return resourceaccess.Principal{}, "", false
	}
	if err := handler.policies.AuthorizeOperation(c.Request.Context(), principal, resourceID, operation); err != nil {
		writeError(c, err)
		return resourceaccess.Principal{}, "", false
	}
	return principal, resourceID, true
}

func requestPrincipal(c *gin.Context) (resourceaccess.Principal, bool) {
	value, exists := c.Get(pmauth.PrincipalContextKey)
	principal, typed := value.(pmauth.Principal)
	workspaceValue, workspaceExists := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, workspaceTyped := workspaceValue.(uint)
	if !exists || !typed || principal.UserID == 0 || !workspaceExists || !workspaceTyped || workspaceID == 0 {
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "RESOURCE_ACCESS_AUTHENTICATION_REQUIRED", "error": http.StatusText(http.StatusUnauthorized)})
		return resourceaccess.Principal{}, false
	}
	actorFactUserID := principal.UserID
	if value, exists := c.Get(pmauth.ActorFactUserIDContextKey); exists {
		if typedValue, typed := value.(uint); typed && typedValue > 0 {
			actorFactUserID = typedValue
		}
	}
	return resourceaccess.Principal{
		WorkspaceID: workspaceID, UserID: principal.UserID, ActorFactUserID: actorFactUserID,
	}, true
}

func bindJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(c, resourceaccess.ErrInvalidInput)
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeError(c, resourceaccess.ErrInvalidInput)
		return false
	}
	return true
}

func subjectPagination(c *gin.Context) (int, int, bool) {
	limit, offset := 20, 0
	var err error
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeError(c, resourceaccess.ErrInvalidInput)
			return 0, 0, false
		}
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			writeError(c, resourceaccess.ErrInvalidInput)
			return 0, 0, false
		}
	}
	if limit < 1 || limit > identity.MaximumSubjectPageSize || offset < 0 || offset%limit != 0 {
		writeError(c, resourceaccess.ErrInvalidInput)
		return 0, 0, false
	}
	return limit, offset, true
}

func writeData(c *gin.Context, status int, data any) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"data": data})
}

func writeError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "RESOURCE_ACCESS_INTERNAL_ERROR"
	switch {
	case errors.Is(err, resourceaccess.ErrInvalidInput):
		status, code = http.StatusBadRequest, "RESOURCE_ACCESS_INVALID_INPUT"
	case errors.Is(err, resourceaccess.ErrNotFound), errors.Is(err, resourceaccess.ErrForbidden), errors.Is(err, resourceaccess.ErrLinkInvalid):
		status, code = http.StatusNotFound, "RESOURCE_ACCESS_NOT_FOUND"
	case errors.Is(err, resourceaccess.ErrConflict):
		status, code = http.StatusConflict, "RESOURCE_ACCESS_REVISION_CONFLICT"
	case errors.Is(err, resourceaccess.ErrIdempotencyConflict):
		status, code = http.StatusConflict, "RESOURCE_ACCESS_IDEMPOTENCY_CONFLICT"
	case errors.Is(err, resourceaccess.ErrRateLimited):
		status, code = http.StatusTooManyRequests, "RESOURCE_ACCESS_RATE_LIMITED"
	case errors.Is(err, resourceaccess.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "RESOURCE_ACCESS_UNAVAILABLE"
	}
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": http.StatusText(status)})
}
