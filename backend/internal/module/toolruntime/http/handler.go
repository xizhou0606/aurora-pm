// Package toolruntimehttp adapts the protocol-neutral Tool Runtime application
// contract to the CLI Tool API. It deliberately receives no database handle.
package toolruntimehttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const maxInvocationBodyBytes = 1 << 20

const (
	ErrorToolNotFound            = "TOOL_NOT_FOUND"
	ErrorToolDraftUnavailable    = "TOOL_DRAFT_UNAVAILABLE"
	ErrorToolUATBlocked          = "TOOL_UAT_BLOCKED"
	ErrorToolAuthorizationDenied = "TOOL_AUTHORIZATION_DENIED"
	ErrorToolWorkspaceMembership = "TOOL_WORKSPACE_MEMBERSHIP_REQUIRED"
	ErrorToolChangeSetRequired   = "CHANGE_SET_REQUIRED"
	ErrorToolInputInvalid        = "TOOL_INPUT_INVALID"
	ErrorToolInputTooLarge       = "TOOL_INPUT_TOO_LARGE"
	ErrorToolUnavailable         = "TOOL_UNAVAILABLE"
	ErrorToolExecutionFailed     = "TOOL_EXECUTION_FAILED"
	ErrorChangeSetNotFound       = "CHANGE_SET_NOT_FOUND"
	ErrorChangeSetConflict       = "CHANGE_SET_CONFLICT"
	ErrorChangeSetInProgress     = "CHANGE_SET_IN_PROGRESS"
	ErrorChangeSetInvalid        = "CHANGE_SET_INVALID"
	ErrorChangeSetSensitive      = "CHANGE_SET_SENSITIVE_INPUT"
)

// AuthorizationResolver converts authenticated HTTP middleware state into
// server-established execution facts. Implementations must not read these
// values from the request body.
type AuthorizationResolver interface {
	ResolveAuthorization(*gin.Context) (toolruntime.ExecutionAuthorization, error)
}

type AuthorizationResolverFunc func(*gin.Context) (toolruntime.ExecutionAuthorization, error)

func (function AuthorizationResolverFunc) ResolveAuthorization(c *gin.Context) (toolruntime.ExecutionAuthorization, error) {
	return function(c)
}

type Handler struct {
	executor toolruntime.Executor
	resolver AuthorizationResolver
}

func NewHandler(executor toolruntime.Executor, resolver AuthorizationResolver) *Handler {
	return &Handler{executor: executor, resolver: resolver}
}

// RegisterRoutes mounts endpoints below an externally authenticated
// /api/v1/tool-runtime group.
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/catalog", handler.catalog)
	group.POST("/preflight", handler.preflight)
	group.GET("/tools/:name", handler.describe)
	group.POST("/tools/:name/invoke", handler.invoke)
	group.GET("/changesets/:id", handler.showChangeSet)
	group.POST("/changesets/:id/confirm", handler.confirmChangeSet)
}

func (handler *Handler) preflight(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	executor, ok := handler.executor.(toolruntime.ToolPreflightExecutor)
	if !ok {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	var input struct {
		Tools []string `json:"tools"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.Tools) == 0 || len(input.Tools) > 100 {
		toolFailure(c, toolruntime.ErrToolInputInvalid)
		return
	}
	items, err := executor.Preflight(c.Request.Context(), input.Tools, authorization)
	if err != nil {
		toolFailure(c, err)
		return
	}
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		status := item.Status
		entry := gin.H{"tool": item.Tool}
		if item.Status == toolruntime.ToolPreflightConditional {
			// Keep the stable status enum compatible with older CLI releases.
			// New clients can explain that the final decision is object-scoped.
			status = toolruntime.ToolPreflightAvailable
			entry["conditional"] = true
		}
		entry["status"] = status
		if item.DenialReason != "" {
			entry["denial_reason"] = item.DenialReason
		}
		if action, found := accesscatalog.ActionForTool(item.Tool); found {
			entry["action_code"] = action
			if policy, conditional := accesscatalog.ObjectPolicyForAction(action); conditional {
				entry["object_policy"] = policy.Code
			}
		}
		result = append(result, entry)
	}
	toolOK(c, gin.H{
		"items": result, "management_path": "/workspace/mcp-cli",
		"action_management_path": "/workspace/pm/access-management",
	})
}

func (handler *Handler) catalog(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	if handler.executor == nil {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	definitions, err := handler.executor.Catalog(c.Request.Context(), authorization)
	if err != nil {
		toolFailure(c, err)
		return
	}
	published := make([]toolruntime.Definition, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Lifecycle != toolruntime.LifecycleDraft {
			published = append(published, definition)
		}
	}
	toolOK(c, gin.H{"tools": published})
}

func (handler *Handler) describe(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	if handler.executor == nil {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	definition, err := handler.executor.Describe(c.Request.Context(), c.Param("name"), authorization)
	if err != nil {
		toolFailure(c, err)
		return
	}
	if definition.Lifecycle == toolruntime.LifecycleDraft {
		toolFailure(c, toolruntime.ErrToolUnknown)
		return
	}
	toolOK(c, gin.H{"tool": definition})
}

func (handler *Handler) invoke(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	if handler.executor == nil {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	input, err := invocationInput(c)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			toolError(c, http.StatusRequestEntityTooLarge, ErrorToolInputTooLarge, "tool input exceeds the size limit")
		} else {
			toolFailure(c, toolruntime.ErrToolInputInvalid)
		}
		return
	}
	result, err := handler.executor.Invoke(c.Request.Context(), toolruntime.Invocation{
		Name: c.Param("name"), Input: input, IdempotencyKey: c.GetHeader("Idempotency-Key"), Authorization: authorization,
	})
	if err != nil {
		toolFailure(c, err)
		return
	}
	if result.ChangeSet != nil {
		c.JSON(http.StatusAccepted, gin.H{"data": result})
		return
	}
	toolOK(c, result)
}

func (handler *Handler) showChangeSet(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	executor, ok := handler.executor.(toolruntime.ChangeSetExecutor)
	if !ok {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	changeSet, err := executor.ShowChangeSet(c.Request.Context(), c.Param("id"), authorization)
	if err != nil {
		toolFailure(c, err)
		return
	}
	toolOK(c, changeSet)
}

func (handler *Handler) confirmChangeSet(c *gin.Context) {
	authorization, ok := handler.authorization(c)
	if !ok {
		return
	}
	if c.ContentType() == "multipart/form-data" {
		handler.confirmFile(c, authorization)
		return
	}
	executor, ok := handler.executor.(toolruntime.ChangeSetExecutor)
	if !ok {
		toolFailure(c, toolruntime.ErrToolUnavailable)
		return
	}
	changeSet, err := executor.ConfirmChangeSet(c.Request.Context(), c.Param("id"), authorization)
	if err != nil {
		toolFailure(c, err)
		return
	}
	toolOK(c, changeSet)
}

func (handler *Handler) authorization(c *gin.Context) (toolruntime.ExecutionAuthorization, bool) {
	if handler.resolver == nil {
		toolFailure(c, toolruntime.ErrAuthorizationDenied)
		return toolruntime.ExecutionAuthorization{}, false
	}
	authorization, err := handler.resolver.ResolveAuthorization(c)
	if err != nil {
		if errors.Is(err, toolruntime.ErrToolUATBlocked) {
			toolFailure(c, toolruntime.ErrToolUATBlocked)
		} else if errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) {
			toolFailure(c, err)
		} else {
			toolFailure(c, toolruntime.ErrAuthorizationDenied)
		}
		return toolruntime.ExecutionAuthorization{}, false
	}
	if toolruntime.ValidateExternalAuthorization(
		authorization, toolruntime.DeliveryCLI, toolruntime.AudienceCLI,
	) != nil {
		toolFailure(c, toolruntime.ErrAuthorizationDenied)
		return toolruntime.ExecutionAuthorization{}, false
	}
	return authorization, true
}

func invocationInput(c *gin.Context) (json.RawMessage, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxInvocationBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	var input json.RawMessage
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, toolruntime.ErrToolInputInvalid
		}
		return nil, err
	}
	return input, nil
}

func toolOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func toolFailure(c *gin.Context, err error) {
	if failure := toolruntime.PublicBusinessError(err); failure != nil {
		c.AbortWithStatusJSON(failure.Status, gin.H{"error": failure})
		return
	}
	switch {
	case errors.Is(err, toolruntime.ErrToolUnknown):
		toolError(c, http.StatusNotFound, ErrorToolNotFound, "tool not found")
	case errors.Is(err, toolruntime.ErrToolDraft):
		toolError(c, http.StatusConflict, ErrorToolDraftUnavailable, "draft tool is not available")
	case errors.Is(err, toolruntime.ErrToolUATBlocked):
		toolError(c, http.StatusForbidden, ErrorToolUATBlocked, "tool authorization is outside its effective window")
	case errors.Is(err, toolruntime.ErrAuthorizationDenied), errors.Is(err, toolruntime.ErrToolScopeDenied):
		toolError(c, http.StatusForbidden, ErrorToolAuthorizationDenied, "tool authorization denied")
	case errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive):
		workspaceID, _ := toolruntime.ExternalWorkspaceID(err)
		toolErrorWithWorkspace(c, http.StatusForbidden, ErrorToolWorkspaceMembership,
			toolruntime.ExternalWorkspaceMembershipMessage, workspaceID)
	case errors.Is(err, toolruntime.ErrChangeSetRequired):
		toolError(c, http.StatusConflict, ErrorToolChangeSetRequired, "a confirmed change set is required")
	case errors.Is(err, toolruntime.ErrChangeSetNotFound):
		toolError(c, http.StatusNotFound, ErrorChangeSetNotFound, "change set not found")
	case errors.Is(err, toolruntime.ErrChangeSetInProgress):
		toolError(c, http.StatusConflict, ErrorChangeSetInProgress, "change set execution is already in progress")
	case errors.Is(err, toolruntime.ErrChangeSetConflict):
		toolError(c, http.StatusConflict, ErrorChangeSetConflict, "change set conflicts with its current state")
	case errors.Is(err, toolruntime.ErrChangeSetInputTooLarge):
		toolError(c, http.StatusRequestEntityTooLarge, ErrorToolInputTooLarge, "tool input exceeds the change set size limit")
	case errors.Is(err, toolruntime.ErrChangeSetSensitive):
		toolError(c, http.StatusUnprocessableEntity, ErrorChangeSetSensitive, "sensitive fields are not accepted by the generic change set store")
	case errors.Is(err, toolruntime.ErrChangeSetInvalid):
		toolError(c, http.StatusUnprocessableEntity, ErrorChangeSetInvalid, "a valid idempotency key is required for mutation tools")
	case errors.Is(err, toolruntime.ErrToolInputInvalid):
		toolError(c, http.StatusUnprocessableEntity, ErrorToolInputInvalid, "tool input is invalid")
	case errors.Is(err, toolruntime.ErrToolUnavailable):
		toolError(c, http.StatusServiceUnavailable, ErrorToolUnavailable, "tool runtime is unavailable")
	default:
		toolError(c, http.StatusInternalServerError, ErrorToolExecutionFailed, "tool execution failed")
	}
}

func toolError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func toolErrorWithWorkspace(c *gin.Context, status int, code, message string, workspaceID uint) {
	errorPayload := gin.H{"code": code, "message": message}
	if workspaceID != 0 {
		errorPayload["workspace_id"] = workspaceID
	}
	c.AbortWithStatusJSON(status, gin.H{"error": errorPayload})
}
