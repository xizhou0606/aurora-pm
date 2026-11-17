package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	ErrExecutorInvalid     = errors.New("tool executor is invalid")
	ErrToolUnknown         = errors.New("tool is unknown")
	ErrToolDraft           = errors.New("draft tool is unavailable")
	ErrToolUATBlocked      = errors.New("tool is blocked by UAT policy")
	ErrAuthorizationDenied = errors.New("tool authorization is denied")
	ErrToolScopeDenied     = errors.New("tool scope is not granted")
	ErrChangeSetRequired   = errors.New("confirmed change set is required")
	ErrToolInputInvalid    = errors.New("tool input is invalid")
	ErrToolOutputInvalid   = errors.New("tool output is invalid")
	ErrToolUnavailable     = errors.New("tool implementation is unavailable")
	ErrToolExecution       = errors.New("tool execution failed")
)

// ExecutionAuthorization contains server-established facts only. HTTP request
// bodies must never populate these fields directly.
type ExecutionAuthorization struct {
	PrincipalID uint            `json:"principal_id"`
	WorkspaceID uint            `json:"workspace_id"`
	GrantID     uint            `json:"grant_id"`
	ClientID    string          `json:"client_id"`
	Channel     DeliveryChannel `json:"channel"`
	Audience    TokenAudience   `json:"audience"`
}

type Invocation struct {
	Name           string
	Input          json.RawMessage
	IdempotencyKey string
	Authorization  ExecutionAuthorization
}

type InvocationResult struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Output     json.RawMessage `json:"output,omitempty"`
	AuditEvent string          `json:"audit_event"`
	ChangeSet  *ChangeSet      `json:"change_set,omitempty"`
}

// Executor is the protocol-neutral application contract shared by the CLI and
// MCP adapters. Catalog and Describe are authorization-aware to prevent a
// client from enumerating tools outside its grant.
type Executor interface {
	Catalog(context.Context, ExecutionAuthorization) ([]Definition, error)
	Describe(context.Context, string, ExecutionAuthorization) (Definition, error)
	Invoke(context.Context, Invocation) (InvocationResult, error)
}

type ToolPreflightStatus string

type ToolPreflightDenialReason string

const (
	ToolPreflightAvailable   ToolPreflightStatus = "available"
	ToolPreflightConditional ToolPreflightStatus = "conditional"
	ToolPreflightNotGranted  ToolPreflightStatus = "not_granted"
	ToolPreflightUnknown     ToolPreflightStatus = "unknown"
	ToolPreflightDraft       ToolPreflightStatus = "draft"

	ToolPreflightDeniedByGrant  ToolPreflightDenialReason = "tool_grant"
	ToolPreflightDeniedByAction ToolPreflightDenialReason = "business_action"
)

type ToolPreflightItem struct {
	Tool         string                    `json:"tool"`
	Status       ToolPreflightStatus       `json:"status"`
	DenialReason ToolPreflightDenialReason `json:"denial_reason,omitempty"`
}

type ToolPreflightExecutor interface {
	Preflight(context.Context, []string, ExecutionAuthorization) ([]ToolPreflightItem, error)
}

type BusinessActionDecision string

const (
	BusinessActionAllowed     BusinessActionDecision = "allowed"
	BusinessActionConditional BusinessActionDecision = "conditional"
	BusinessActionDenied      BusinessActionDecision = "denied"
)

// BusinessActionRequest binds a Tool invocation to the PM principal and its
// canonical input. Input is nil during preflight, where an object-scoped
// policy can only report that final authorization is conditional.
type BusinessActionRequest struct {
	ToolName      string
	Authorization ExecutionAuthorization
	Input         json.RawMessage
}

// BusinessActionAuthorizer applies personnel Action permissions and declared
// object policies after the exact Tool Grant gate. It must fail closed.
type BusinessActionAuthorizer interface {
	PreflightBusinessActions(context.Context, []string, ExecutionAuthorization) (map[string]BusinessActionDecision, error)
	AuthorizeBusinessAction(context.Context, BusinessActionRequest) (BusinessActionDecision, error)
}

// InvocationAuthorizer validates persisted grant facts once per request and
// returns its server-owned exact tool and scope entitlements. Application
// performs both checks in memory, avoiding one repository query per tool.
type InvocationAuthorizer interface {
	AuthorizeGrant(context.Context, ExecutionAuthorization) (AuthorizedGrant, error)
}

type AuthorizedGrant struct {
	tools  map[string]struct{}
	scopes map[string]struct{}
}

func NewAuthorizedGrant(tools, scopes []string) (AuthorizedGrant, error) {
	canonicalToolNames := canonicalTools(tools)
	canonicalGrantScopes := canonicalScopes(scopes)
	if len(canonicalToolNames) == 0 || len(canonicalGrantScopes) == 0 {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	grant := AuthorizedGrant{
		tools: make(map[string]struct{}, len(canonicalToolNames)), scopes: make(map[string]struct{}, len(canonicalGrantScopes)),
	}
	for _, tool := range canonicalToolNames {
		if !toolNamePattern.MatchString(tool) {
			return AuthorizedGrant{}, ErrAuthorizationDenied
		}
		grant.tools[tool] = struct{}{}
	}
	for _, scope := range canonicalGrantScopes {
		if !grantScopePattern.MatchString(scope) {
			return AuthorizedGrant{}, ErrAuthorizationDenied
		}
		grant.scopes[scope] = struct{}{}
	}
	return grant, nil
}

func (grant AuthorizedGrant) Allows(tool, scope string) bool {
	_, toolAllowed := grant.tools[tool]
	_, scopeAllowed := grant.scopes[scope]
	return toolAllowed && scopeAllowed
}

type ToolCall struct {
	Definition    Definition
	Authorization ExecutionAuthorization
	Input         json.RawMessage
}

type ToolHandler interface {
	Invoke(context.Context, ToolCall) (json.RawMessage, error)
}

type ToolHandlerFunc func(context.Context, ToolCall) (json.RawMessage, error)

func (function ToolHandlerFunc) Invoke(ctx context.Context, call ToolCall) (json.RawMessage, error) {
	return function(ctx, call)
}

type compiledTool struct {
	definition Definition
	input      *jsonschema.Schema
	output     *jsonschema.Schema
	handler    ToolHandler
}

type Application struct {
	catalog    *Catalog
	authorizer InvocationAuthorizer
	tools      map[string]compiledTool
	changeSets *ChangeSetService
	actions    BusinessActionAuthorizer
}

type ApplicationOption func(*Application) error

func WithChangeSetService(service *ChangeSetService) ApplicationOption {
	return func(application *Application) error {
		if service == nil {
			return ErrExecutorInvalid
		}
		application.changeSets = service
		return nil
	}
}

func WithBusinessActionAuthorizer(authorizer BusinessActionAuthorizer) ApplicationOption {
	return func(application *Application) error {
		if authorizer == nil {
			return ErrExecutorInvalid
		}
		application.actions = authorizer
		return nil
	}
}

func NewApplication(catalog *Catalog, authorizer InvocationAuthorizer, handlers map[string]ToolHandler, options ...ApplicationOption) (*Application, error) {
	if catalog == nil || authorizer == nil {
		return nil, ErrExecutorInvalid
	}
	application := &Application{catalog: catalog, authorizer: authorizer, tools: make(map[string]compiledTool)}
	for _, definition := range catalog.Definitions() {
		input, err := compileExecutionSchema(definition.Name+"-input", definition.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("%w: %s input schema", ErrExecutorInvalid, definition.Name)
		}
		output, err := compileExecutionSchema(definition.Name+"-output", definition.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("%w: %s output schema", ErrExecutorInvalid, definition.Name)
		}
		handler := handlers[definition.Name]
		if definition.Lifecycle != LifecycleDraft && handler == nil {
			return nil, fmt.Errorf("%w: %s", ErrToolUnavailable, definition.Name)
		}
		application.tools[definition.Name] = compiledTool{definition: definition, input: input, output: output, handler: handler}
	}
	for name := range handlers {
		if _, exists := application.tools[name]; !exists {
			return nil, fmt.Errorf("%w: handler registered for unknown tool %s", ErrExecutorInvalid, name)
		}
	}
	for _, option := range options {
		if option == nil || option(application) != nil {
			return nil, ErrExecutorInvalid
		}
	}
	return application, nil
}

func (application *Application) Catalog(ctx context.Context, authorization ExecutionAuthorization) ([]Definition, error) {
	grant, err := application.authorizeGrant(ctx, authorization)
	if err != nil {
		return nil, err
	}
	candidates := make([]Definition, 0)
	for _, definition := range application.catalog.Definitions() {
		if definition.Lifecycle == LifecycleDraft {
			continue
		}
		if !grant.Allows(definition.Name, definition.Authorization.GrantScope) {
			continue
		}
		candidates = append(candidates, definition)
	}
	names := make([]string, 0, len(candidates))
	for _, definition := range candidates {
		names = append(names, definition.Name)
	}
	decisions, err := application.preflightBusinessActions(ctx, names, authorization)
	if err != nil {
		return nil, err
	}
	definitions := make([]Definition, 0, len(candidates))
	for _, definition := range candidates {
		decision := decisions[definition.Name]
		if decision == BusinessActionAllowed || decision == BusinessActionConditional {
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

func (application *Application) Preflight(ctx context.Context, names []string, authorization ExecutionAuthorization) ([]ToolPreflightItem, error) {
	if application == nil || len(names) == 0 || len(names) > 100 {
		return nil, ErrToolInputInvalid
	}
	grant, err := application.authorizeGrant(ctx, authorization)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(names))
	canonicalNames := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !toolNamePattern.MatchString(name) {
			return nil, ErrToolInputInvalid
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		canonicalNames = append(canonicalNames, name)
	}
	decisions, err := application.preflightBusinessActions(ctx, canonicalNames, authorization)
	if err != nil {
		return nil, err
	}
	seen = make(map[string]struct{}, len(canonicalNames))
	items := make([]ToolPreflightItem, 0, len(names))
	for _, name := range canonicalNames {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		compiled, found := application.tools[name]
		status := ToolPreflightUnknown
		var denialReason ToolPreflightDenialReason
		if found && compiled.definition.Lifecycle == LifecycleDraft {
			status = ToolPreflightDraft
		} else if found && grant.Allows(name, compiled.definition.Authorization.GrantScope) {
			decision := decisions[name]
			switch decision {
			case BusinessActionAllowed:
				status = ToolPreflightAvailable
			case BusinessActionConditional:
				status = ToolPreflightConditional
			default:
				status = ToolPreflightNotGranted
				denialReason = ToolPreflightDeniedByAction
			}
		} else if found {
			status = ToolPreflightNotGranted
			denialReason = ToolPreflightDeniedByGrant
		}
		items = append(items, ToolPreflightItem{Tool: name, Status: status, DenialReason: denialReason})
	}
	return items, nil
}

func (application *Application) Describe(ctx context.Context, name string, authorization ExecutionAuthorization) (Definition, error) {
	grant, err := application.authorizeGrant(ctx, authorization)
	if err != nil {
		return Definition{}, err
	}
	definition, found := application.catalog.Find(name)
	if !found || definition.Lifecycle == LifecycleDraft {
		return Definition{}, ErrToolUnknown
	}
	if !grant.Allows(definition.Name, definition.Authorization.GrantScope) {
		return Definition{}, ErrToolUnknown
	}
	decisions, err := application.preflightBusinessActions(ctx, []string{definition.Name}, authorization)
	decision := decisions[definition.Name]
	if err != nil || (decision != BusinessActionAllowed && decision != BusinessActionConditional) {
		return Definition{}, ErrToolUnknown
	}
	return definition, nil
}

func (application *Application) Invoke(ctx context.Context, invocation Invocation) (InvocationResult, error) {
	grant, err := application.authorizeGrant(ctx, invocation.Authorization)
	if err != nil {
		return InvocationResult{}, err
	}
	tool, found := application.tools[invocation.Name]
	if !found {
		return InvocationResult{}, ErrToolUnknown
	}
	if tool.definition.Lifecycle == LifecycleDraft {
		return InvocationResult{}, ErrToolDraft
	}
	if !grant.Allows(tool.definition.Name, tool.definition.Authorization.GrantScope) {
		return InvocationResult{}, ErrToolScopeDenied
	}
	if tool.definition.UAT == UATBlocked {
		return InvocationResult{}, ErrToolUATBlocked
	}
	input, err := decodeObject(invocation.Input)
	if err != nil || tool.input.Validate(input) != nil {
		return InvocationResult{}, ErrToolInputInvalid
	}
	decision, err := application.authorizeBusinessAction(
		ctx, tool.definition.Name, invocation.Authorization, invocation.Input,
	)
	if failure := PublicBusinessError(err); failure != nil {
		return InvocationResult{}, failure
	}
	if err != nil || decision != BusinessActionAllowed {
		return InvocationResult{}, ErrAuthorizationDenied
	}
	if tool.handler == nil {
		return InvocationResult{}, ErrToolUnavailable
	}
	if tool.definition.ChangeSet == ChangeSetRequired {
		if application.changeSets == nil {
			return InvocationResult{}, ErrToolUnavailable
		}
		if _, ok := tool.handler.(IdempotentMutationHandler); !ok {
			return InvocationResult{}, ErrToolUnavailable
		}
		changeSetInput, idempotencyKey, err := prepareChangeSetInput(input, invocation.Input, invocation.IdempotencyKey)
		if err != nil {
			return InvocationResult{}, err
		}
		created, err := application.changeSets.Create(
			ctx, tool.definition, invocation.Authorization, changeSetInput, idempotencyKey,
		)
		if err != nil {
			return InvocationResult{}, err
		}
		changeSet := created.ChangeSet
		return InvocationResult{
			Name: tool.definition.Name, Version: tool.definition.Version,
			AuditEvent: tool.definition.AuditEvent, ChangeSet: &changeSet,
		}, nil
	}
	output, err := tool.handler.Invoke(ctx, ToolCall{
		Definition: tool.definition, Authorization: invocation.Authorization, Input: append(json.RawMessage(nil), invocation.Input...),
	})
	if err != nil {
		if failure := PublicBusinessError(err); failure != nil {
			return InvocationResult{}, failure
		}
		if errors.Is(err, ErrToolInputInvalid) {
			return InvocationResult{}, ErrToolInputInvalid
		}
		return InvocationResult{}, fmt.Errorf("%w: %v", ErrToolExecution, err)
	}
	outputValue, err := decodeObject(output)
	if err != nil || tool.output.Validate(outputValue) != nil {
		return InvocationResult{}, ErrToolOutputInvalid
	}
	return InvocationResult{
		Name: tool.definition.Name, Version: tool.definition.Version,
		Output: append(json.RawMessage(nil), output...), AuditEvent: tool.definition.AuditEvent,
	}, nil
}

// prepareChangeSetInput accepts the retry key from either protocol metadata
// (HTTP/CLI headers and MCP _meta) or the canonical Tool argument exposed in
// the Catalog. The raw key is removed from the business input before it can be
// persisted; ChangeSetService stores only its digest.
func prepareChangeSetInput(input map[string]any, rawInput json.RawMessage, transportKey string) (json.RawMessage, string, error) {
	argument, exists := input["idempotency_key"]
	if !exists {
		return append(json.RawMessage(nil), rawInput...), transportKey, nil
	}
	argumentKey, ok := argument.(string)
	if !ok || argumentKey == "" {
		return nil, "", ErrChangeSetInvalid
	}
	if transportKey != "" && transportKey != argumentKey {
		return nil, "", ErrChangeSetConflict
	}
	delete(input, "idempotency_key")
	changeSetInput, err := json.Marshal(input)
	if err != nil {
		return nil, "", ErrChangeSetInvalid
	}
	return changeSetInput, argumentKey, nil
}

func (application *Application) ShowChangeSet(
	ctx context.Context,
	id string,
	authorization ExecutionAuthorization,
) (ChangeSet, error) {
	if application == nil || application.changeSets == nil || validateExecutionAuthorization(authorization) != nil {
		return ChangeSet{}, ErrChangeSetNotFound
	}
	return application.changeSets.Show(ctx, id, authorization)
}

func (application *Application) ConfirmChangeSet(
	ctx context.Context,
	id string,
	authorization ExecutionAuthorization,
) (ChangeSet, error) {
	return application.confirmChangeSet(ctx, id, authorization, nil)
}

func (application *Application) confirmChangeSet(ctx context.Context, id string, authorization ExecutionAuthorization, file *BinaryFile) (ChangeSet, error) {
	if application == nil || application.changeSets == nil {
		return ChangeSet{}, ErrToolUnavailable
	}
	changeSet, err := application.changeSets.Show(ctx, id, authorization)
	if err != nil || terminalChangeSetStatus(changeSet.Status) {
		if err == nil && changeSet.Status == ChangeSetSucceeded && changeSet.MutationResult != nil {
			changeSet.MutationResult.AlreadyApplied = true
		}
		return changeSet, err
	}
	if tool, ok := application.tools[changeSet.ToolName]; ok {
		_, needsFile := tool.handler.(BinaryMutationHandler)
		if needsFile && file == nil {
			return ChangeSet{}, LookupBusinessError("TOOL_FILE_REQUIRED")
		}
		if !needsFile && file != nil {
			return ChangeSet{}, LookupBusinessError("TOOL_FILE_INVALID")
		}
	}
	if changeSet.Status == ChangeSetPending {
		_, _, code := application.validateChangeSetExecution(ctx, changeSet, authorization)
		if code != "" {
			return application.rejectPendingChangeSet(ctx, changeSet, code)
		}
	}
	claim, err := application.changeSets.Claim(ctx, id, authorization)
	if err != nil {
		return ChangeSet{}, err
	}
	// Re-read the canonical workspace configuration and grant after the
	// pending->confirmed CAS. A concurrent disable therefore rejects this
	// exact claim and scrubs its input before any domain Apply can run.
	tool, handler, code := application.validateChangeSetExecution(ctx, claim.ChangeSet, authorization)
	if code != "" {
		return application.changeSets.RejectClaim(ctx, claim, code)
	}
	decision, actionErr := application.authorizeBusinessAction(
		ctx, tool.definition.Name, authorization, claim.Input,
	)
	if failure := PublicBusinessError(actionErr); failure != nil {
		return application.changeSets.Fail(ctx, claim, failure.Code)
	}
	if actionErr != nil || decision != BusinessActionAllowed {
		return application.changeSets.RejectClaim(ctx, claim, "TOOL_AUTHORIZATION_DENIED")
	}
	call := MutationCall{
		Definition: tool.definition, Authorization: authorization, Input: append(json.RawMessage(nil), claim.Input...),
		ChangeSetID: claim.ChangeSet.ID, ApplyKey: claim.ChangeSet.ID,
	}
	var output json.RawMessage
	var applyErr error
	if file != nil {
		output, applyErr = handler.(BinaryMutationHandler).ApplyFile(ctx, call, *file)
	} else {
		output, applyErr = handler.Apply(ctx, call)
	}
	if applyErr != nil {
		if errors.Is(applyErr, ErrMutationRetryable) {
			released, releaseErr := application.changeSets.ReleaseClaim(ctx, claim)
			if releaseErr != nil {
				return ChangeSet{}, releaseErr
			}
			return released, ErrChangeSetInProgress
		}
		if failure := PublicBusinessError(applyErr); failure != nil {
			return application.changeSets.Fail(ctx, claim, failure.Code)
		}
		return application.changeSets.Fail(ctx, claim, "TOOL_EXECUTION_FAILED")
	}
	outputValue, outputErr := decodeObject(output)
	if outputErr != nil || tool.output.Validate(outputValue) != nil {
		return application.changeSets.Fail(ctx, claim, "TOOL_OUTPUT_INVALID")
	}
	return application.changeSets.SucceedWithInput(ctx, claim, output, claim.Input)
}

func (application *Application) validateChangeSetExecution(
	ctx context.Context,
	changeSet ChangeSet,
	authorization ExecutionAuthorization,
) (compiledTool, IdempotentMutationHandler, string) {
	tool, found := application.tools[changeSet.ToolName]
	if !found || tool.definition.Lifecycle == LifecycleDraft || tool.definition.Version != changeSet.ToolVersion ||
		tool.definition.ChangeSet != ChangeSetRequired {
		return compiledTool{}, nil, "TOOL_VERSION_UNAVAILABLE"
	}
	grant, err := application.authorizeGrant(ctx, authorization)
	if err != nil {
		if errors.Is(err, ErrToolUATBlocked) {
			return compiledTool{}, nil, "TOOL_UAT_BLOCKED"
		}
		return compiledTool{}, nil, "TOOL_AUTHORIZATION_DENIED"
	}
	if !grant.Allows(tool.definition.Name, tool.definition.Authorization.GrantScope) {
		return compiledTool{}, nil, "TOOL_AUTHORIZATION_DENIED"
	}
	if tool.definition.UAT == UATBlocked {
		return compiledTool{}, nil, "TOOL_UAT_BLOCKED"
	}
	handler, ok := tool.handler.(IdempotentMutationHandler)
	if !ok {
		return compiledTool{}, nil, "TOOL_UNAVAILABLE"
	}
	return tool, handler, ""
}

func (application *Application) rejectPendingChangeSet(
	ctx context.Context,
	changeSet ChangeSet,
	code string,
) (ChangeSet, error) {
	if changeSet.Status != ChangeSetPending {
		return ChangeSet{}, ErrChangeSetConflict
	}
	return application.changeSets.RejectPending(ctx, changeSet.ID, changeSet.Authorization, code)
}

func (application *Application) authorizeGrant(ctx context.Context, authorization ExecutionAuthorization) (AuthorizedGrant, error) {
	if authorization.PrincipalID == 0 || authorization.WorkspaceID == 0 || authorization.GrantID == 0 ||
		strings.TrimSpace(authorization.ClientID) == "" || strings.TrimSpace(string(authorization.Audience)) == "" ||
		(authorization.Channel != DeliveryMCP && authorization.Channel != DeliveryCLI) {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	grant, err := application.authorizer.AuthorizeGrant(ctx, authorization)
	if err != nil {
		return AuthorizedGrant{}, normalizeAuthorizationError(err)
	}
	if len(grant.tools) == 0 || len(grant.scopes) == 0 {
		return AuthorizedGrant{}, ErrAuthorizationDenied
	}
	return grant, nil
}

func (application *Application) authorizeBusinessAction(
	ctx context.Context,
	toolName string,
	authorization ExecutionAuthorization,
	input json.RawMessage,
) (BusinessActionDecision, error) {
	// The option is additive for embedders and older unit fixtures. Production
	// assembly always supplies the fail-closed canonical Action authorizer.
	if application.actions == nil {
		return BusinessActionAllowed, nil
	}
	decision, err := application.actions.AuthorizeBusinessAction(ctx, BusinessActionRequest{
		ToolName: toolName, Authorization: authorization, Input: append(json.RawMessage(nil), input...),
	})
	if err != nil {
		if failure := PublicBusinessError(err); failure != nil {
			return BusinessActionDenied, failure
		}
		return BusinessActionDenied, ErrAuthorizationDenied
	}
	switch decision {
	case BusinessActionAllowed, BusinessActionConditional, BusinessActionDenied:
		return decision, nil
	default:
		return BusinessActionDenied, ErrAuthorizationDenied
	}
}

func (application *Application) preflightBusinessActions(
	ctx context.Context,
	toolNames []string,
	authorization ExecutionAuthorization,
) (map[string]BusinessActionDecision, error) {
	result := make(map[string]BusinessActionDecision, len(toolNames))
	if application.actions == nil {
		for _, name := range toolNames {
			result[name] = BusinessActionAllowed
		}
		return result, nil
	}
	decisions, err := application.actions.PreflightBusinessActions(ctx, toolNames, authorization)
	if err != nil {
		return nil, ErrAuthorizationDenied
	}
	for _, name := range toolNames {
		decision := decisions[name]
		switch decision {
		case BusinessActionAllowed, BusinessActionConditional, BusinessActionDenied:
			result[name] = decision
		default:
			result[name] = BusinessActionDenied
		}
	}
	return result, nil
}

func normalizeAuthorizationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrToolUATBlocked) {
		return ErrToolUATBlocked
	}
	if errors.Is(err, ErrToolScopeDenied) {
		return ErrToolScopeDenied
	}
	return ErrAuthorizationDenied
}

func compileExecutionSchema(name string, raw json.RawMessage) (*jsonschema.Schema, error) {
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	url := "https://pm.invalid/tool-runtime/" + name + ".json"
	if err := compiler.AddResource(url, document); err != nil {
		return nil, err
	}
	return compiler.Compile(url)
}

func decodeObject(raw json.RawMessage) (map[string]any, error) {
	var value map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || value == nil {
		return nil, ErrToolInputInvalid
	}
	return value, nil
}
