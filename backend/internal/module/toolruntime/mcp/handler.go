package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const maxRequestBodyBytes = 1 << 20

type AuthorizationResolver interface {
	ResolveAuthorization(*gin.Context) (toolruntime.ExecutionAuthorization, error)
}

type Handler struct {
	executor            toolruntime.Executor
	resolver            AuthorizationResolver
	authorizationURL    string
	resourceMetadataURL string
}

func NewHandler(executor toolruntime.Executor, resolver AuthorizationResolver, authorizationURLs ...string) *Handler {
	authorizationURL := ""
	if len(authorizationURLs) > 0 {
		authorizationURL = strings.TrimSpace(authorizationURLs[0])
	}
	resourceMetadataURL := ""
	if len(authorizationURLs) > 1 {
		resourceMetadataURL = strings.TrimSpace(authorizationURLs[1])
	}
	return &Handler{executor: executor, resolver: resolver, authorizationURL: authorizationURL, resourceMetadataURL: resourceMetadataURL}
}

// RegisterRoutes mounts the single MCP Streamable HTTP endpoint. This
// stateless implementation does not offer a server-initiated SSE stream.
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/mcp", handler.get)
	group.POST("/mcp", handler.post)
}

func (handler *Handler) ListTools(ctx context.Context, authorization toolruntime.ExecutionAuthorization) ([]Tool, error) {
	definitions, err := handler.executor.Catalog(ctx, authorization)
	if err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(definitions)+len(toolruntime.ChangeSetControlDefinitions()))
	for _, definition := range definitions {
		if definition.Lifecycle != toolruntime.LifecycleDraft {
			tools = append(tools, projectTool(definition))
		}
	}
	if _, ok := handler.executor.(toolruntime.ChangeSetExecutor); ok {
		for _, definition := range toolruntime.ChangeSetControlDefinitions() {
			tools = append(tools, projectChangeSetControl(definition))
		}
	}
	return tools, nil
}

func (handler *Handler) DescribeTool(ctx context.Context, name string, authorization toolruntime.ExecutionAuthorization) (Tool, error) {
	if definition, ok := toolruntime.FindChangeSetControlDefinition(name); ok {
		if _, supported := handler.executor.(toolruntime.ChangeSetExecutor); !supported {
			return Tool{}, toolruntime.ErrToolUnavailable
		}
		return projectChangeSetControl(definition), nil
	}
	definition, err := handler.executor.Describe(ctx, name, authorization)
	if err != nil {
		return Tool{}, err
	}
	if definition.Lifecycle == toolruntime.LifecycleDraft {
		return Tool{}, toolruntime.ErrToolUnknown
	}
	return projectTool(definition), nil
}

func (handler *Handler) CallTool(
	ctx context.Context,
	name string,
	input json.RawMessage,
	authorization toolruntime.ExecutionAuthorization,
) (CallToolResult, error) {
	return handler.callTool(ctx, name, input, "", authorization)
}

func (handler *Handler) get(c *gin.Context) {
	if !validOrigin(c) {
		c.Status(http.StatusForbidden)
		return
	}
	if _, err := handler.authorization(c); err != nil {
		handler.writeAuthorizationFailure(c, err)
		return
	}
	c.Header("Allow", http.MethodPost)
	c.Status(http.StatusMethodNotAllowed)
}

func (handler *Handler) post(c *gin.Context) {
	if !validOrigin(c) {
		c.Status(http.StatusForbidden)
		return
	}
	if !acceptsStreamableHTTP(c.GetHeader("Accept")) {
		c.Status(http.StatusNotAcceptable)
		return
	}
	if !isJSONContentType(c.GetHeader("Content-Type")) {
		c.Status(http.StatusUnsupportedMediaType)
		return
	}
	authorization, err := handler.authorization(c)
	if err != nil {
		handler.writeAuthorizationFailure(c, err)
		return
	}
	message, err := decodeRequest(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, nil, -32700, "parse error")
		return
	}
	if len(message.ID) == 0 {
		if !validProtocolHeader(message.Method, c.GetHeader("MCP-Protocol-Version")) {
			writeError(c, http.StatusBadRequest, nil, -32600, "unsupported protocol version")
			return
		}
		handler.notification(c, message)
		return
	}
	if !validRequest(message) {
		writeError(c, http.StatusBadRequest, message.ID, -32600, "invalid request")
		return
	}
	if !validProtocolHeader(message.Method, c.GetHeader("MCP-Protocol-Version")) {
		writeError(c, http.StatusBadRequest, message.ID, -32600, "unsupported protocol version")
		return
	}
	handler.request(c, message, authorization)
}

func (handler *Handler) writeAuthorizationRequired(c *gin.Context) {
	challenge := "Bearer"
	if handler != nil && handler.resourceMetadataURL != "" {
		challenge += ` resource_metadata="` + handler.resourceMetadataURL + `", scope="` + toolruntime.OAuthScopeTools + `"`
	}
	c.Header("WWW-Authenticate", challenge)
	if handler == nil || handler.authorizationURL == "" {
		c.Status(http.StatusUnauthorized)
		return
	}
	c.Header("Link", "<"+handler.authorizationURL+">; rel=\"authorization\"")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":              "MCP_AUTHORIZATION_REQUIRED",
			"message":           "PM authorization is required",
			"authorization_url": handler.authorizationURL,
		},
	})
}

func (handler *Handler) writeAuthorizationFailure(c *gin.Context, err error) {
	if errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) {
		workspaceID, _ := toolruntime.ExternalWorkspaceID(err)
		errorPayload := gin.H{
			"code":    "MCP_WORKSPACE_MEMBERSHIP_REQUIRED",
			"message": toolruntime.ExternalWorkspaceMembershipMessage,
		}
		if workspaceID != 0 {
			errorPayload["workspace_id"] = workspaceID
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errorPayload})
		return
	}
	handler.writeAuthorizationRequired(c)
}

func (handler *Handler) authorization(c *gin.Context) (toolruntime.ExecutionAuthorization, error) {
	if handler == nil || handler.executor == nil || handler.resolver == nil {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	}
	authorization, err := handler.resolver.ResolveAuthorization(c)
	if errors.Is(err, toolruntime.ErrExternalWorkspaceMembershipInactive) {
		return toolruntime.ExecutionAuthorization{}, err
	}
	if err != nil || toolruntime.ValidateExternalAuthorization(
		authorization, toolruntime.DeliveryMCP, toolruntime.AudienceMCP,
	) != nil {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	}
	return authorization, nil
}

func (handler *Handler) request(c *gin.Context, message request, authorization toolruntime.ExecutionAuthorization) {
	switch message.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(message.Params, &params) != nil || strings.TrimSpace(params.ProtocolVersion) == "" {
			writeError(c, http.StatusOK, message.ID, -32602, "invalid params")
			return
		}
		writeResult(c, message.ID, gin.H{
			"protocolVersion": supportedProtocolVersion(params.ProtocolVersion),
			"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
			"serverInfo":      gin.H{"name": "pm-tool-runtime", "version": "1.0.0"},
		})
	case "ping":
		writeResult(c, message.ID, gin.H{})
	case "tools/list":
		tools, err := handler.ListTools(c.Request.Context(), authorization)
		if err != nil {
			handler.executionError(c, message.ID, err)
			return
		}
		writeResult(c, message.ID, gin.H{"tools": tools})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
			Meta      map[string]any  `json:"_meta"`
		}
		if json.Unmarshal(message.Params, &params) != nil || strings.TrimSpace(params.Name) == "" {
			writeError(c, http.StatusOK, message.ID, -32602, "invalid params")
			return
		}
		if len(params.Arguments) == 0 {
			params.Arguments = json.RawMessage(`{}`)
		}
		idempotencyKey, _ := params.Meta["pm/idempotencyKey"].(string)
		result, err := handler.callTool(c.Request.Context(), params.Name, params.Arguments, idempotencyKey, authorization)
		if err != nil {
			handler.executionError(c, message.ID, err)
			return
		}
		writeResult(c, message.ID, result)
	case "pm/changesets/show", "pm/changesets/confirm":
		var params struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(message.Params, &params) != nil || strings.TrimSpace(params.ID) == "" {
			writeError(c, http.StatusOK, message.ID, -32602, "invalid params")
			return
		}
		executor, ok := handler.executor.(toolruntime.ChangeSetExecutor)
		if !ok {
			handler.executionError(c, message.ID, toolruntime.ErrToolUnavailable)
			return
		}
		var changeSet toolruntime.ChangeSet
		var err error
		if message.Method == "pm/changesets/show" {
			changeSet, err = executor.ShowChangeSet(c.Request.Context(), params.ID, authorization)
		} else {
			changeSet, err = executor.ConfirmChangeSet(c.Request.Context(), params.ID, authorization)
		}
		if err != nil {
			handler.executionError(c, message.ID, err)
			return
		}
		writeResult(c, message.ID, changeSet)
	default:
		writeError(c, http.StatusOK, message.ID, -32601, "method not found")
	}
}

func (handler *Handler) callTool(
	ctx context.Context,
	name string,
	input json.RawMessage,
	idempotencyKey string,
	authorization toolruntime.ExecutionAuthorization,
) (CallToolResult, error) {
	if _, ok := toolruntime.FindChangeSetControlDefinition(name); ok {
		executor, supported := handler.executor.(toolruntime.ChangeSetExecutor)
		if !supported {
			return CallToolResult{}, toolruntime.ErrToolUnavailable
		}
		clean, file, err := binaryControlInput(name, input)
		if err != nil {
			return CallToolResult{}, err
		}
		id, err := decodeChangeSetControlInput(clean)
		if err != nil {
			return CallToolResult{}, err
		}
		var changeSet toolruntime.ChangeSet
		if name == toolruntime.ChangeSetShowControlName {
			changeSet, err = executor.ShowChangeSet(ctx, id, authorization)
		} else {
			if file != nil {
				binary, ok := handler.executor.(toolruntime.BinaryChangeSetExecutor)
				if !ok {
					return CallToolResult{}, toolruntime.ErrToolUnavailable
				}
				changeSet, err = binary.ConfirmChangeSetWithFile(ctx, id, authorization, *file)
			} else {
				changeSet, err = executor.ConfirmChangeSet(ctx, id, authorization)
			}
		}
		if err != nil {
			return CallToolResult{}, err
		}
		return projectChangeSetControlCall(name, changeSet), nil
	}
	result, err := handler.executor.Invoke(ctx, toolruntime.Invocation{
		Name: name, Input: input, IdempotencyKey: idempotencyKey, Authorization: authorization,
	})
	if err != nil {
		return CallToolResult{}, err
	}
	return projectCall(result), nil
}

func decodeChangeSetControlInput(input json.RawMessage) (string, error) {
	var params map[string]any
	if json.Unmarshal(input, &params) != nil || len(params) != 1 {
		return "", toolruntime.ErrChangeSetInvalid
	}
	id, ok := params["id"].(string)
	id = strings.TrimSpace(id)
	if !ok || len(id) < 8 || len(id) > 128 {
		return "", toolruntime.ErrChangeSetInvalid
	}
	return id, nil
}

func (handler *Handler) notification(c *gin.Context, message request) {
	if message.JSONRPC == "2.0" && message.Method == "notifications/initialized" {
		c.Status(http.StatusAccepted)
		return
	}
	writeError(c, http.StatusBadRequest, nil, -32600, "invalid request")
}

func (handler *Handler) executionError(c *gin.Context, id json.RawMessage, err error) {
	if failure := toolruntime.PublicBusinessError(err); failure != nil {
		output, _ := json.Marshal(map[string]any{"error": failure})
		c.JSON(http.StatusOK, response{JSONRPC: "2.0", ID: id, Result: CallToolResult{Content: []TextContent{{Type: "text", Text: string(output)}}, StructuredContent: output, IsError: true}})
		return
	}
	switch {
	case errors.Is(err, toolruntime.ErrAuthorizationDenied), errors.Is(err, toolruntime.ErrToolScopeDenied),
		errors.Is(err, toolruntime.ErrToolUATBlocked):
		writeError(c, http.StatusForbidden, id, -32001, "tool authorization denied")
	case errors.Is(err, toolruntime.ErrToolUnknown), errors.Is(err, toolruntime.ErrToolDraft),
		errors.Is(err, toolruntime.ErrToolInputInvalid):
		writeError(c, http.StatusOK, id, -32602, "invalid tool request")
	case errors.Is(err, toolruntime.ErrChangeSetRequired):
		writeError(c, http.StatusOK, id, -32002, "confirmed change set required")
	case errors.Is(err, toolruntime.ErrChangeSetNotFound):
		writeError(c, http.StatusOK, id, -32004, "change set not found")
	case errors.Is(err, toolruntime.ErrChangeSetInProgress):
		writeError(c, http.StatusOK, id, -32006, "change set in progress")
	case errors.Is(err, toolruntime.ErrChangeSetConflict):
		writeError(c, http.StatusOK, id, -32005, "change set conflict")
	case errors.Is(err, toolruntime.ErrChangeSetInvalid), errors.Is(err, toolruntime.ErrChangeSetSensitive),
		errors.Is(err, toolruntime.ErrChangeSetInputTooLarge):
		writeError(c, http.StatusOK, id, -32602, "invalid change set request")
	case errors.Is(err, toolruntime.ErrToolUnavailable):
		writeError(c, http.StatusServiceUnavailable, id, -32003, "tool runtime unavailable")
	default:
		writeError(c, http.StatusInternalServerError, id, -32603, "internal error")
	}
}

func decodeRequest(c *gin.Context) (request, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	var message request
	if err := decoder.Decode(&message); err != nil {
		return request{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return request{}, errors.New("multiple JSON values")
	}
	return message, nil
}

func validRequest(message request) bool {
	if message.JSONRPC != "2.0" || strings.TrimSpace(message.Method) == "" {
		return false
	}
	var id any
	return json.Unmarshal(message.ID, &id) == nil && id != nil
}

func validOrigin(c *gin.Context) bool {
	return strings.TrimSpace(c.GetHeader("Origin")) == ""
}

func acceptsStreamableHTTP(header string) bool {
	header = strings.ToLower(header)
	return strings.Contains(header, "application/json") && strings.Contains(header, "text/event-stream")
}

func isJSONContentType(header string) bool {
	mediaType, _, err := mime.ParseMediaType(header)
	return err == nil && mediaType == "application/json"
}

func validProtocolHeader(method, version string) bool {
	if method == "initialize" || version == "" {
		return true
	}
	return version == ProtocolVersion || version == legacyProtocolVersion
}

func writeResult(c *gin.Context, id json.RawMessage, result any) {
	c.JSON(http.StatusOK, response{JSONRPC: "2.0", ID: id, Result: result})
}

func writeError(c *gin.Context, status int, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage(`null`)
	}
	c.JSON(status, response{JSONRPC: "2.0", ID: id, Error: &protocolError{Code: code, Message: message}})
}
