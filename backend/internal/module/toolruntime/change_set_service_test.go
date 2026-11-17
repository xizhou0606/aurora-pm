package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type changeSetMemoryRepository struct {
	changeSet       ChangeSet
	input           json.RawMessage
	keyDigest       string
	claimID         string
	failSucceedOnce bool
}

func (repository *changeSetMemoryRepository) CreateOrFind(_ context.Context, draft ChangeSetDraft) (ChangeSetCreateResult, error) {
	if repository.changeSet.ID != "" {
		if repository.keyDigest != draft.IdempotencyDigest || repository.changeSet.InputDigest != draft.InputDigest {
			return ChangeSetCreateResult{}, ErrChangeSetConflict
		}
		return ChangeSetCreateResult{ChangeSet: repository.changeSet}, nil
	}
	repository.changeSet, repository.input, repository.keyDigest = draft.ChangeSet, append(json.RawMessage(nil), draft.Input...), draft.IdempotencyDigest
	return ChangeSetCreateResult{ChangeSet: repository.changeSet, Created: true}, nil
}

func (repository *changeSetMemoryRepository) FindBound(_ context.Context, id string, authorization ExecutionAuthorization, now time.Time) (ChangeSet, error) {
	if id != repository.changeSet.ID || authorization != repository.changeSet.Authorization {
		return ChangeSet{}, ErrChangeSetNotFound
	}
	if repository.changeSet.Status == ChangeSetPending && !now.Before(repository.changeSet.ExpiresAt) {
		repository.changeSet.Status, repository.changeSet.ErrorCode = ChangeSetExpired, "CHANGE_SET_EXPIRED"
		repository.changeSet.CompletedAt, repository.input = &now, nil
	}
	return repository.changeSet, nil
}

func (repository *changeSetMemoryRepository) ConfirmAndClaim(_ context.Context, command ChangeSetClaimCommand) (ChangeSetClaim, error) {
	if command.ID != repository.changeSet.ID || command.Authorization != repository.changeSet.Authorization {
		return ChangeSetClaim{}, ErrChangeSetNotFound
	}
	if repository.changeSet.Status == ChangeSetPending {
		repository.changeSet.Status, repository.changeSet.ConfirmedAt = ChangeSetConfirmed, &command.Now
	} else if repository.changeSet.Status != ChangeSetConfirmed {
		return ChangeSetClaim{}, ErrChangeSetConflict
	}
	repository.claimID = command.ClaimID
	return ChangeSetClaim{ChangeSet: repository.changeSet, ClaimID: command.ClaimID, Input: append(json.RawMessage(nil), repository.input...)}, nil
}

func (repository *changeSetMemoryRepository) Succeed(_ context.Context, command ChangeSetFinishCommand) (ChangeSet, error) {
	if repository.failSucceedOnce {
		repository.failSucceedOnce = false
		return ChangeSet{}, errors.New("private database failure")
	}
	repository.changeSet.Status, repository.changeSet.Result, repository.input = ChangeSetSucceeded, command.Result, nil
	repository.changeSet.CompletedAt = &command.Now
	return repository.changeSet, nil
}

func (repository *changeSetMemoryRepository) Fail(_ context.Context, command ChangeSetFinishCommand) (ChangeSet, error) {
	repository.changeSet.Status, repository.changeSet.ErrorCode, repository.input = ChangeSetFailed, command.ErrorCode, nil
	repository.changeSet.CompletedAt = &command.Now
	return repository.changeSet, nil
}

func (repository *changeSetMemoryRepository) RejectClaim(_ context.Context, command ChangeSetFinishCommand) (ChangeSet, error) {
	if repository.changeSet.Status != ChangeSetConfirmed || repository.claimID != command.ClaimID {
		return ChangeSet{}, ErrChangeSetConflict
	}
	repository.changeSet.Status, repository.changeSet.ErrorCode, repository.input = ChangeSetRejected, command.ErrorCode, nil
	repository.changeSet.CompletedAt = &command.Now
	return repository.changeSet, nil
}

func (repository *changeSetMemoryRepository) ReleaseClaim(_ context.Context, command ChangeSetReleaseCommand) (ChangeSet, error) {
	if repository.changeSet.Status != ChangeSetConfirmed || repository.claimID != command.ClaimID ||
		repository.changeSet.Authorization != command.Authorization {
		return ChangeSet{}, ErrChangeSetConflict
	}
	repository.claimID = ""
	repository.changeSet.UpdatedAt = command.Now
	return repository.changeSet, nil
}

func (repository *changeSetMemoryRepository) RejectPending(_ context.Context, id string, authorization ExecutionAuthorization, code string, now time.Time) (ChangeSet, error) {
	if id != repository.changeSet.ID || authorization != repository.changeSet.Authorization || repository.changeSet.Status != ChangeSetPending {
		return ChangeSet{}, ErrChangeSetConflict
	}
	repository.changeSet.Status, repository.changeSet.ErrorCode, repository.input = ChangeSetRejected, code, nil
	repository.changeSet.CompletedAt = &now
	return repository.changeSet, nil
}

type idempotentMutationTestHandler struct {
	results     map[string]json.RawMessage
	applyCalls  int
	sideEffects int
	retryOnce   bool
}

func (handler *idempotentMutationTestHandler) Invoke(context.Context, ToolCall) (json.RawMessage, error) {
	return nil, ErrChangeSetRequired
}

func (handler *idempotentMutationTestHandler) Apply(_ context.Context, call MutationCall) (json.RawMessage, error) {
	handler.applyCalls++
	if handler.retryOnce {
		handler.retryOnce = false
		return nil, RetryableApplyError(errors.New("private transient database failure"))
	}
	if output, exists := handler.results[call.ApplyKey]; exists {
		return output, nil
	}
	handler.sideEffects++
	output := json.RawMessage(`{"id":42}`)
	handler.results[call.ApplyKey] = output
	return output, nil
}

func TestApplicationRetryableApplyReleasesExactClaimAndCanConfirmAgain(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}, retryOnce: true}
	application, err := NewApplication(
		catalog, &changeSetTestAuthorizer{}, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service),
	)
	if err != nil {
		t.Fatal(err)
	}
	invoked, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"title":"Updated"}`), IdempotencyKey: "request-key-retry-0001",
		Authorization: executionAuthorization(),
	})
	if err != nil || invoked.ChangeSet == nil {
		t.Fatalf("invoked=%+v err=%v", invoked, err)
	}
	released, err := application.ConfirmChangeSet(context.Background(), invoked.ChangeSet.ID, executionAuthorization())
	if !errors.Is(err, ErrChangeSetInProgress) || released.Status != ChangeSetConfirmed || repository.claimID != "" || len(repository.input) == 0 {
		t.Fatalf("released=%+v err=%v claim=%q input=%q", released, err, repository.claimID, repository.input)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), invoked.ChangeSet.ID, executionAuthorization())
	if err != nil || succeeded.Status != ChangeSetSucceeded || handler.applyCalls != 2 || handler.sideEffects != 1 {
		t.Fatalf("succeeded=%+v err=%v apply=%d sideEffects=%d", succeeded, err, handler.applyCalls, handler.sideEffects)
	}
}

type changeSetTestAuthorizer struct {
	deny   bool
	denyAt int
	calls  int
}

func (authorizer *changeSetTestAuthorizer) AuthorizeGrant(context.Context, ExecutionAuthorization) (AuthorizedGrant, error) {
	authorizer.calls++
	if authorizer.deny || (authorizer.denyAt > 0 && authorizer.calls >= authorizer.denyAt) {
		return AuthorizedGrant{}, ErrToolUATBlocked
	}
	return NewAuthorizedGrant([]string{"project.update"}, []string{"project:write"})
}

func TestApplicationConfigurationChangeAfterClaimRejectsWithoutApply(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}}
	authorizer := &changeSetTestAuthorizer{denyAt: 3}
	application, _ := NewApplication(catalog, authorizer, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service))
	result, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"title":"Updated"}`), IdempotencyKey: "request-key-0001", Authorization: executionAuthorization(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := application.ConfirmChangeSet(context.Background(), result.ChangeSet.ID, executionAuthorization())
	if err != nil || rejected.Status != ChangeSetRejected || rejected.ErrorCode != "TOOL_UAT_BLOCKED" || handler.applyCalls != 0 || len(repository.input) != 0 {
		t.Fatalf("rejected=%+v err=%v apply=%d input=%q authCalls=%d", rejected, err, handler.applyCalls, repository.input, authorizer.calls)
	}
}

func TestChangeSetRejectsSensitiveAndOversizedCanonicalInput(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, err := NewChangeSetService(repository, DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	definition := changeSetTestDefinition()
	for _, input := range []json.RawMessage{
		json.RawMessage(`{"nested":{"access_token":"must-not-store"}}`),
		json.RawMessage(`{"value":"` + strings.Repeat("x", maxChangeSetInputBytes) + `"}`),
	} {
		if _, err := service.Create(context.Background(), definition, executionAuthorization(), input, "request-key-0001"); !errors.Is(err, ErrChangeSetSensitive) && !errors.Is(err, ErrChangeSetInputTooLarge) {
			t.Fatalf("input err=%v", err)
		}
	}
}

func TestApplicationMutationCreatesIdempotentPendingChangeSetAndCrashReplayIsSafe(t *testing.T) {
	repository := &changeSetMemoryRepository{failSucceedOnce: true}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}}
	application, err := NewApplication(catalog, &changeSetTestAuthorizer{}, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service))
	if err != nil {
		t.Fatal(err)
	}
	invocation := Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"title":"Updated"}`), IdempotencyKey: "request-key-0001",
		Authorization: executionAuthorization(),
	}
	first, err := application.Invoke(context.Background(), invocation)
	if err != nil || first.ChangeSet == nil || first.ChangeSet.Status != ChangeSetPending {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := application.Invoke(context.Background(), invocation)
	if err != nil || second.ChangeSet == nil || second.ChangeSet.ID != first.ChangeSet.ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if _, err := application.ConfirmChangeSet(context.Background(), first.ChangeSet.ID, executionAuthorization()); err == nil || repository.changeSet.Status != ChangeSetConfirmed {
		t.Fatalf("first confirm status=%s err=%v", repository.changeSet.Status, err)
	}
	confirmed, err := application.ConfirmChangeSet(context.Background(), first.ChangeSet.ID, executionAuthorization())
	if err != nil || confirmed.Status != ChangeSetSucceeded || handler.applyCalls != 2 || handler.sideEffects != 1 {
		t.Fatalf("confirmed=%+v err=%v apply=%d sideEffects=%d", confirmed, err, handler.applyCalls, handler.sideEffects)
	}
	if len(repository.input) != 0 || string(confirmed.Result) != `{"id":42}` {
		t.Fatalf("raw input retained or result missing: input=%q result=%s", repository.input, confirmed.Result)
	}
	if confirmed.MutationResult == nil || !confirmed.MutationResult.Confirmed || confirmed.MutationResult.AlreadyApplied ||
		strings.Join(confirmed.MutationResult.ChangedFields, ",") != "title" || confirmed.MutationResult.Entity == nil || confirmed.MutationResult.Entity.ID != 42 {
		t.Fatalf("standard mutation result=%+v", confirmed.MutationResult)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), first.ChangeSet.ID, executionAuthorization())
	if err != nil || replayed.MutationResult == nil || !replayed.MutationResult.AlreadyApplied {
		t.Fatalf("replayed=%+v err=%v", replayed.MutationResult, err)
	}
}

func TestApplicationMutationUsesArgumentIdempotencyKeyWithoutPersistingIt(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}}
	application, err := NewApplication(
		catalog, &changeSetTestAuthorizer{}, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := application.Invoke(context.Background(), Invocation{
		Name:          definition.Name,
		Input:         json.RawMessage(`{"title":"Updated","idempotency_key":"mcp-request-key-0001"}`),
		Authorization: executionAuthorization(),
	})
	if err != nil || result.ChangeSet == nil || result.ChangeSet.Status != ChangeSetPending {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if string(repository.input) != `{"title":"Updated"}` || repository.keyDigest == "" || strings.Contains(string(repository.input), "idempotency") {
		t.Fatalf("persisted input=%s keyDigest=%q", repository.input, repository.keyDigest)
	}
}

func TestApplicationMutationRejectsConflictingTransportAndArgumentIdempotencyKeys(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}}
	application, _ := NewApplication(
		catalog, &changeSetTestAuthorizer{}, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service),
	)
	_, err := application.Invoke(context.Background(), Invocation{
		Name:           definition.Name,
		Input:          json.RawMessage(`{"title":"Updated","idempotency_key":"argument-key-0001"}`),
		IdempotencyKey: "transport-key-0001", Authorization: executionAuthorization(),
	})
	if !errors.Is(err, ErrChangeSetConflict) || repository.changeSet.ID != "" {
		t.Fatalf("err=%v changeSet=%+v", err, repository.changeSet)
	}
}

func TestApplicationConfigurationChangeRejectsPendingWithoutApply(t *testing.T) {
	repository := &changeSetMemoryRepository{}
	service, _ := NewChangeSetService(repository, DefaultChangeSetPolicy())
	definition := changeSetTestDefinition()
	catalog, _ := NewCatalog([]Definition{definition})
	handler := &idempotentMutationTestHandler{results: map[string]json.RawMessage{}}
	authorizer := &changeSetTestAuthorizer{}
	application, _ := NewApplication(catalog, authorizer, map[string]ToolHandler{definition.Name: handler}, WithChangeSetService(service))
	result, err := application.Invoke(context.Background(), Invocation{
		Name: definition.Name, Input: json.RawMessage(`{"title":"Updated"}`), IdempotencyKey: "request-key-0001", Authorization: executionAuthorization(),
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizer.deny = true
	rejected, err := application.ConfirmChangeSet(context.Background(), result.ChangeSet.ID, executionAuthorization())
	if err != nil || rejected.Status != ChangeSetRejected || rejected.ErrorCode != "TOOL_UAT_BLOCKED" || handler.applyCalls != 0 || len(repository.input) != 0 {
		t.Fatalf("rejected=%+v err=%v apply=%d input=%q", rejected, err, handler.applyCalls, repository.input)
	}
}

func changeSetTestDefinition() Definition {
	return Definition{
		Name: "project.update", Version: "1.0.0", Summary: "Update", Lifecycle: LifecycleActive,
		InputSchema:   json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"},"idempotency_key":{"type":"string","minLength":8,"maxLength":128,"pattern":"^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$"}},"required":["title"],"additionalProperties":false}`),
		OutputSchema:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}`),
		Authorization: AuthorizationContract{IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true, GrantScope: "project:write"},
		Risk:          RiskMutation, ChangeSet: ChangeSetRequired, UAT: UATWithinGrantWindow, AuditEvent: "tool.project.update",
	}
}
