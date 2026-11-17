package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimehttp "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/http"
)

type parityAuthorizer struct {
	grant toolruntime.AuthorizedGrant
}

type parityChangeSetState struct {
	changeSet toolruntime.ChangeSet
	input     json.RawMessage
	claimID   string
}

type parityChangeSetRepository struct {
	states map[string]*parityChangeSetState
}

func (repository *parityChangeSetRepository) CreateOrFind(_ context.Context, draft toolruntime.ChangeSetDraft) (toolruntime.ChangeSetCreateResult, error) {
	if repository.states == nil {
		repository.states = map[string]*parityChangeSetState{}
	}
	repository.states[draft.ID] = &parityChangeSetState{changeSet: draft.ChangeSet, input: append(json.RawMessage(nil), draft.Input...)}
	return toolruntime.ChangeSetCreateResult{ChangeSet: draft.ChangeSet, Created: true}, nil
}

func (repository *parityChangeSetRepository) FindBound(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization, _ time.Time) (toolruntime.ChangeSet, error) {
	state := repository.states[id]
	if state == nil || state.changeSet.Authorization != authorization {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetNotFound
	}
	return state.changeSet, nil
}

func (repository *parityChangeSetRepository) ConfirmAndClaim(_ context.Context, command toolruntime.ChangeSetClaimCommand) (toolruntime.ChangeSetClaim, error) {
	state := repository.states[command.ID]
	if state == nil || state.changeSet.Authorization != command.Authorization || state.changeSet.Status != toolruntime.ChangeSetPending {
		return toolruntime.ChangeSetClaim{}, toolruntime.ErrChangeSetConflict
	}
	state.changeSet.Status, state.changeSet.ConfirmedAt, state.claimID = toolruntime.ChangeSetConfirmed, &command.Now, command.ClaimID
	return toolruntime.ChangeSetClaim{ChangeSet: state.changeSet, ClaimID: command.ClaimID, Input: append(json.RawMessage(nil), state.input...)}, nil
}

func (repository *parityChangeSetRepository) Succeed(_ context.Context, command toolruntime.ChangeSetFinishCommand) (toolruntime.ChangeSet, error) {
	state := repository.states[command.ID]
	if state == nil || state.changeSet.Authorization != command.Authorization || state.claimID != command.ClaimID {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetConflict
	}
	state.changeSet.Status, state.changeSet.Result, state.changeSet.CompletedAt = toolruntime.ChangeSetSucceeded, append(json.RawMessage(nil), command.Result...), &command.Now
	state.input = nil
	return state.changeSet, nil
}

func (repository *parityChangeSetRepository) Fail(_ context.Context, command toolruntime.ChangeSetFinishCommand) (toolruntime.ChangeSet, error) {
	state := repository.states[command.ID]
	if state == nil {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetNotFound
	}
	state.changeSet.Status, state.changeSet.ErrorCode, state.changeSet.CompletedAt = toolruntime.ChangeSetFailed, command.ErrorCode, &command.Now
	state.input = nil
	return state.changeSet, nil
}

func (repository *parityChangeSetRepository) RejectClaim(_ context.Context, command toolruntime.ChangeSetFinishCommand) (toolruntime.ChangeSet, error) {
	state := repository.states[command.ID]
	if state == nil || state.changeSet.Authorization != command.Authorization || state.claimID != command.ClaimID {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetConflict
	}
	state.changeSet.Status, state.changeSet.ErrorCode, state.changeSet.CompletedAt = toolruntime.ChangeSetRejected, command.ErrorCode, &command.Now
	state.input = nil
	return state.changeSet, nil
}

func (repository *parityChangeSetRepository) ReleaseClaim(_ context.Context, command toolruntime.ChangeSetReleaseCommand) (toolruntime.ChangeSet, error) {
	state := repository.states[command.ID]
	if state == nil || state.changeSet.Status != toolruntime.ChangeSetConfirmed || state.claimID != command.ClaimID ||
		state.changeSet.Authorization != command.Authorization {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetConflict
	}
	state.claimID = ""
	state.changeSet.UpdatedAt = command.Now
	return state.changeSet, nil
}

func (repository *parityChangeSetRepository) RejectPending(_ context.Context, id string, authorization toolruntime.ExecutionAuthorization, code string, now time.Time) (toolruntime.ChangeSet, error) {
	state := repository.states[id]
	if state == nil || state.changeSet.Authorization != authorization || state.changeSet.Status != toolruntime.ChangeSetPending {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetConflict
	}
	state.changeSet.Status, state.changeSet.ErrorCode, state.changeSet.CompletedAt = toolruntime.ChangeSetRejected, code, &now
	state.input = nil
	return state.changeSet, nil
}

type parityMutationHandler struct{}

func (parityMutationHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (parityMutationHandler) Apply(context.Context, toolruntime.MutationCall) (json.RawMessage, error) {
	return json.RawMessage(`{"id":42}`), nil
}

func (authorizer parityAuthorizer) AuthorizeGrant(_ context.Context, authorization toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	audience, err := toolruntime.AudienceForChannel(authorization.Channel)
	if err != nil || authorization.Audience != audience {
		return toolruntime.AuthorizedGrant{}, toolruntime.ErrAuthorizationDenied
	}
	return authorizer.grant, nil
}

type parityResolver struct {
	authorization toolruntime.ExecutionAuthorization
}

func (resolver parityResolver) ResolveAuthorization(*gin.Context) (toolruntime.ExecutionAuthorization, error) {
	return resolver.authorization, nil
}

func TestCLIAndMCPNormalizeListDescribeAndInvokeFromOneApplication(t *testing.T) {
	definition := mcpDefinition("project.list", toolruntime.LifecycleActive)
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := toolruntime.NewAuthorizedGrant([]string{definition.Name}, []string{definition.Authorization.GrantScope})
	if err != nil {
		t.Fatal(err)
	}
	output := json.RawMessage(`{"items":[{"id":1}]}`)
	application, err := toolruntime.NewApplication(catalog, parityAuthorizer{grant: grant}, map[string]toolruntime.ToolHandler{
		definition.Name: toolruntime.ToolHandlerFunc(func(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
			return output, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	cliAuthorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "codex",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	mcpAuth := cliAuthorization
	mcpAuth.Channel, mcpAuth.Audience = toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	mcpHandler := NewHandler(application, parityResolver{authorization: mcpAuth})

	gin.SetMode(gin.TestMode)
	cliEngine := gin.New()
	toolruntimehttp.NewHandler(application, parityResolver{authorization: cliAuthorization}).RegisterRoutes(cliEngine.Group("/api/v1/tool-runtime"))

	cliList := cliRequest(t, cliEngine, http.MethodGet, "/api/v1/tool-runtime/catalog", "")
	var cliListPayload struct {
		Data struct {
			Tools []toolruntime.Definition `json:"tools"`
		} `json:"data"`
	}
	decodeParityResponse(t, cliList, &cliListPayload)
	mcpTools, err := mcpHandler.ListTools(context.Background(), mcpAuth)
	if err != nil || len(cliListPayload.Data.Tools) != 1 || len(mcpTools) != 1+len(toolruntime.ChangeSetControlDefinitions()) {
		t.Fatalf("CLI=%+v MCP=%+v err=%v", cliListPayload, mcpTools, err)
	}
	var projected Tool
	for _, candidate := range mcpTools {
		if candidate.Name == definition.Name {
			projected = candidate
			break
		}
	}
	assertNormalizedDefinition(t, cliListPayload.Data.Tools[0], projected)

	cliDescribe := cliRequest(t, cliEngine, http.MethodGet, "/api/v1/tool-runtime/tools/project.list", "")
	var cliDescribePayload struct {
		Data struct {
			Tool toolruntime.Definition `json:"tool"`
		} `json:"data"`
	}
	decodeParityResponse(t, cliDescribe, &cliDescribePayload)
	mcpDescription, err := mcpHandler.DescribeTool(context.Background(), definition.Name, mcpAuth)
	if err != nil {
		t.Fatal(err)
	}
	assertNormalizedDefinition(t, cliDescribePayload.Data.Tool, mcpDescription)

	cliInvoke := cliRequest(t, cliEngine, http.MethodPost, "/api/v1/tool-runtime/tools/project.list/invoke", `{}`)
	var cliInvokePayload struct {
		Data toolruntime.InvocationResult `json:"data"`
	}
	decodeParityResponse(t, cliInvoke, &cliInvokePayload)
	mcpInvocation, err := mcpHandler.CallTool(context.Background(), definition.Name, json.RawMessage(`{}`), mcpAuth)
	if err != nil {
		t.Fatal(err)
	}
	if cliInvokePayload.Data.Name != mcpInvocation.Meta["pm/name"] ||
		cliInvokePayload.Data.Version != mcpInvocation.Meta["pm/version"] ||
		cliInvokePayload.Data.AuditEvent != mcpInvocation.Meta["pm/auditEvent"] ||
		!jsonEqual(cliInvokePayload.Data.Output, mcpInvocation.StructuredContent) {
		t.Fatalf("CLI=%+v MCP=%+v", cliInvokePayload.Data, mcpInvocation)
	}
}

func TestCLIAndMCPMutationChangeSetParityFromOneApplication(t *testing.T) {
	definition := mcpDefinition("project.update", toolruntime.LifecycleActive)
	definition.InputSchema = json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"idempotency_key":{"type":"string","minLength":8,"maxLength":128,"pattern":"^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$"}},"required":["title"],"additionalProperties":false}`)
	definition.OutputSchema = json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}`)
	definition.Authorization.GrantScope = "project:write"
	definition.Risk, definition.ChangeSet, definition.AuditEvent = toolruntime.RiskMutation, toolruntime.ChangeSetRequired, "tool.project.update"
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := toolruntime.NewAuthorizedGrant([]string{definition.Name}, []string{definition.Authorization.GrantScope})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(&parityChangeSetRepository{}, toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(
		catalog, parityAuthorizer{grant: grant}, map[string]toolruntime.ToolHandler{definition.Name: parityMutationHandler{}},
		toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}

	cliAuthorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	mcpAuthorization := cliAuthorization
	mcpAuthorization.ClientID, mcpAuthorization.Channel, mcpAuthorization.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP

	gin.SetMode(gin.TestMode)
	cliEngine := gin.New()
	toolruntimehttp.NewHandler(application, parityResolver{authorization: cliAuthorization}).RegisterRoutes(cliEngine.Group("/api/v1/tool-runtime"))
	mcpEngine := gin.New()
	NewHandler(application, parityResolver{authorization: mcpAuthorization}).RegisterRoutes(mcpEngine.Group("/api/v1"))

	cliPendingResponse := httptest.NewRecorder()
	cliPendingRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tool-runtime/tools/project.update/invoke", strings.NewReader(`{"title":"Updated"}`))
	cliPendingRequest.Header.Set("Content-Type", "application/json")
	cliPendingRequest.Header.Set("Idempotency-Key", "cli-request-key-0001")
	cliEngine.ServeHTTP(cliPendingResponse, cliPendingRequest)
	if cliPendingResponse.Code != http.StatusAccepted {
		t.Fatalf("CLI pending status=%d body=%s", cliPendingResponse.Code, cliPendingResponse.Body.String())
	}
	var cliPending struct {
		Data toolruntime.InvocationResult `json:"data"`
	}
	decodeParityResponse(t, cliPendingResponse, &cliPending)

	mcpPendingResponse := mcpRequest(mcpEngine, http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"project.update","arguments":{"title":"Updated","idempotency_key":"mcp-request-key-0001"}}}`)
	var mcpPending struct {
		Result CallToolResult `json:"result"`
	}
	decodeParityResponse(t, mcpPendingResponse, &mcpPending)
	var mcpPendingContent struct {
		ChangeSet toolruntime.ChangeSet `json:"change_set"`
	}
	if err := json.Unmarshal(mcpPending.Result.StructuredContent, &mcpPendingContent); err != nil {
		t.Fatal(err)
	}
	if cliPending.Data.ChangeSet == nil || cliPending.Data.ChangeSet.Status != toolruntime.ChangeSetPending ||
		mcpPendingContent.ChangeSet.Status != toolruntime.ChangeSetPending || cliPending.Data.ChangeSet.ToolName != mcpPendingContent.ChangeSet.ToolName {
		t.Fatalf("CLI=%+v MCP=%+v", cliPending.Data.ChangeSet, mcpPendingContent.ChangeSet)
	}
	assertParityChangeSetHasNoBindingFacts(t, cliPendingResponse.Body.String())
	assertParityChangeSetHasNoBindingFacts(t, mcpPendingResponse.Body.String())

	cliConfirmed := cliRequest(t, cliEngine, http.MethodPost, "/api/v1/tool-runtime/changesets/"+cliPending.Data.ChangeSet.ID+"/confirm", "")
	var cliConfirmedPayload struct {
		Data toolruntime.ChangeSet `json:"data"`
	}
	decodeParityResponse(t, cliConfirmed, &cliConfirmedPayload)
	mcpConfirmedResponse := mcpRequest(mcpEngine, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"pm/changesets/confirm","params":{"id":"`+mcpPendingContent.ChangeSet.ID+`"}}`)
	var mcpConfirmed struct {
		Result toolruntime.ChangeSet `json:"result"`
	}
	decodeParityResponse(t, mcpConfirmedResponse, &mcpConfirmed)
	if cliConfirmedPayload.Data.Status != toolruntime.ChangeSetSucceeded || mcpConfirmed.Result.Status != toolruntime.ChangeSetSucceeded ||
		!jsonEqual(cliConfirmedPayload.Data.Result, mcpConfirmed.Result.Result) {
		t.Fatalf("CLI=%+v MCP=%+v", cliConfirmedPayload.Data, mcpConfirmed.Result)
	}
	assertParityChangeSetHasNoBindingFacts(t, cliConfirmed.Body.String())
	assertParityChangeSetHasNoBindingFacts(t, mcpConfirmedResponse.Body.String())
}

func assertParityChangeSetHasNoBindingFacts(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"authorization", "principal_id", "workspace_id", "grant_id", "client_id", "channel", "audience"} {
		if strings.Contains(body, `"`+forbidden+`"`) {
			t.Fatalf("binding fact %q leaked in %s", forbidden, body)
		}
	}
}

func cliRequest(t *testing.T, engine *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	return response
}

func decodeParityResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func assertNormalizedDefinition(t *testing.T, cli toolruntime.Definition, mcp Tool) {
	t.Helper()
	if cli.Name != mcp.Name || cli.Version != mcp.Meta["pm/version"] || cli.Summary != mcp.Description ||
		!jsonEqual(cli.InputSchema, mcp.InputSchema) || !jsonEqual(cli.OutputSchema, mcp.OutputSchema) ||
		cli.Risk != mcp.Meta["pm/risk"] || cli.ChangeSet != mcp.Meta["pm/changeSet"] ||
		cli.UAT != mcp.Meta["pm/uat"] || cli.AuditEvent != mcp.Meta["pm/auditEvent"] {
		t.Fatalf("CLI=%+v MCP=%+v", cli, mcp)
	}
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil &&
		jsonValue(leftValue) == jsonValue(rightValue)
}

func jsonValue(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
