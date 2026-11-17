package toolruntime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"
)

type ChangeSetService struct {
	repository ChangeSetRepository
	policy     ChangeSetPolicyConfig
	now        func() time.Time
	random     io.Reader
}

func NewChangeSetService(repository ChangeSetRepository, policy ChangeSetPolicyConfig) (*ChangeSetService, error) {
	if repository == nil || policy.PendingTTL <= 0 || policy.PendingTTL > time.Hour ||
		policy.ClaimLease <= 0 || policy.ClaimLease > 5*time.Minute {
		return nil, ErrChangeSetInvalid
	}
	return &ChangeSetService{repository: repository, policy: policy, now: time.Now, random: rand.Reader}, nil
}

func (service *ChangeSetService) Create(
	ctx context.Context,
	definition Definition,
	authorization ExecutionAuthorization,
	input json.RawMessage,
	idempotencyKey string,
) (ChangeSetCreateResult, error) {
	if service == nil || service.repository == nil || definition.ChangeSet != ChangeSetRequired ||
		(definition.Risk != RiskMutation && definition.Risk != RiskHighImpact) ||
		validateExecutionAuthorization(authorization) != nil || !idempotencyKeyPattern.MatchString(idempotencyKey) {
		return ChangeSetCreateResult{}, ErrChangeSetInvalid
	}
	canonical, digest, err := canonicalChangeSetInput(input)
	if err != nil {
		return ChangeSetCreateResult{}, err
	}
	id, err := newOpaqueSecret(service.random)
	if err != nil {
		return ChangeSetCreateResult{}, ErrChangeSetInvalid
	}
	now := service.now().UTC()
	keyDigest := sha256.Sum256([]byte(idempotencyKey))
	draft := ChangeSetDraft{
		ChangeSet: ChangeSet{
			ID: id, Status: ChangeSetPending, ToolName: definition.Name, ToolVersion: definition.Version,
			Risk: definition.Risk, AuditEvent: definition.AuditEvent, Authorization: authorization,
			InputDigest: digest, InputSize: len(canonical), ExpiresAt: now.Add(service.policy.PendingTTL),
			CreatedAt: now, UpdatedAt: now,
		},
		IdempotencyDigest: hex.EncodeToString(keyDigest[:]), Input: canonical,
	}
	return service.repository.CreateOrFind(ctx, draft)
}

func (service *ChangeSetService) Show(
	ctx context.Context,
	id string,
	authorization ExecutionAuthorization,
) (ChangeSet, error) {
	if service == nil || service.repository == nil || !opaqueSecretPattern.MatchString(id) ||
		validateExecutionAuthorization(authorization) != nil {
		return ChangeSet{}, ErrChangeSetNotFound
	}
	changeSet, err := service.repository.FindBound(ctx, id, authorization, service.now().UTC())
	if err == nil && changeSet.Status == ChangeSetSucceeded {
		changeSet.MutationResult = standardMutationResult(changeSet.ToolName, changeSet.Result, nil, false)
	}
	return changeSet, err
}

func (service *ChangeSetService) Claim(
	ctx context.Context,
	id string,
	authorization ExecutionAuthorization,
) (ChangeSetClaim, error) {
	claimID, err := newOpaqueSecret(service.random)
	if err != nil {
		return ChangeSetClaim{}, ErrChangeSetInvalid
	}
	return service.repository.ConfirmAndClaim(ctx, ChangeSetClaimCommand{
		ID: id, Authorization: authorization, ClaimID: claimID,
		Now: service.now().UTC(), ClaimLease: service.policy.ClaimLease,
	})
}

func (service *ChangeSetService) Succeed(
	ctx context.Context,
	claim ChangeSetClaim,
	result json.RawMessage,
) (ChangeSet, error) {
	if len(result) == 0 || len(result) > maxChangeSetResultBytes {
		return service.Fail(ctx, claim, "TOOL_OUTPUT_INVALID")
	}
	return service.succeed(ctx, claim, result, nil)
}

func (service *ChangeSetService) SucceedWithInput(
	ctx context.Context,
	claim ChangeSetClaim,
	result json.RawMessage,
	input json.RawMessage,
) (ChangeSet, error) {
	return service.succeed(ctx, claim, result, input)
}

func (service *ChangeSetService) succeed(ctx context.Context, claim ChangeSetClaim, result, input json.RawMessage) (ChangeSet, error) {
	if len(result) == 0 || len(result) > maxChangeSetResultBytes {
		return service.Fail(ctx, claim, "TOOL_OUTPUT_INVALID")
	}
	changeSet, err := service.repository.Succeed(ctx, ChangeSetFinishCommand{
		ID: claim.ChangeSet.ID, Authorization: claim.ChangeSet.Authorization,
		ClaimID: claim.ClaimID, Result: append(json.RawMessage(nil), result...), Now: service.now().UTC(),
	})
	if err == nil {
		changeSet.MutationResult = standardMutationResult(changeSet.ToolName, result, input, false)
	}
	return changeSet, err
}

func standardMutationResult(toolName string, result, input json.RawMessage, alreadyApplied bool) *StandardMutationResult {
	summary := &StandardMutationResult{Confirmed: true, ChangedFields: changedMutationFields(input), AlreadyApplied: alreadyApplied}
	var output map[string]any
	if json.Unmarshal(result, &output) == nil {
		if value, ok := output["already_applied"].(bool); ok {
			summary.AlreadyApplied = value || alreadyApplied
		}
		summary.Entity = mutationEntity(toolName, output)
	}
	return summary
}

func changedMutationFields(input json.RawMessage) []string {
	var value map[string]any
	if len(input) == 0 || json.Unmarshal(input, &value) != nil {
		return []string{}
	}
	if updates, ok := value["updates"].(map[string]any); ok {
		value = updates
	}
	ignored := map[string]bool{"id": true, "ids": true, "task_refs": true, "expected_version": true, "idempotency_key": true, "confirm_subject_schedule": true}
	fields := make([]string, 0, len(value))
	for key := range value {
		if !ignored[key] {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

func mutationEntity(toolName string, value map[string]any) *StandardMutationEntity {
	typeName := strings.Split(toolName, ".")[0]
	switch {
	case strings.HasPrefix(toolName, "quality.bug."):
		typeName = "bug"
	case strings.HasPrefix(toolName, "project.requirement."):
		typeName = "requirement"
	case strings.HasPrefix(toolName, "project.plan_unit."):
		typeName = "plan_unit"
	case strings.HasPrefix(toolName, "project.set."):
		typeName = "project_set"
	}
	types := []string{typeName}
	objects := []map[string]any{value}
	for _, key := range []string{"task", "bug", "requirement", "project", "item", "entity"} {
		if nested, ok := value[key].(map[string]any); ok {
			objects = append(objects, nested)
			types = append(types, key)
		}
	}
	for index, object := range objects {
		entity := &StandardMutationEntity{Type: types[index]}
		if id, ok := object["id"].(float64); ok {
			entity.ID = int64(id)
		}
		for _, key := range []string{"task_key", "bug_key", "key", "code"} {
			if text, ok := object[key].(string); ok {
				entity.Key = text
				break
			}
		}
		if text, ok := object["url"].(string); ok {
			entity.URL = text
		}
		if entity.ID != 0 || entity.Key != "" || entity.URL != "" {
			return entity
		}
	}
	return nil
}

func (service *ChangeSetService) Fail(
	ctx context.Context,
	claim ChangeSetClaim,
	errorCode string,
) (ChangeSet, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" || len(errorCode) > 80 {
		errorCode = "TOOL_EXECUTION_FAILED"
	}
	return service.repository.Fail(ctx, ChangeSetFinishCommand{
		ID: claim.ChangeSet.ID, Authorization: claim.ChangeSet.Authorization,
		ClaimID: claim.ClaimID, ErrorCode: errorCode, Now: service.now().UTC(),
	})
}

func (service *ChangeSetService) ReleaseClaim(
	ctx context.Context,
	claim ChangeSetClaim,
) (ChangeSet, error) {
	if service == nil || service.repository == nil || claim.ClaimID == "" {
		return ChangeSet{}, ErrChangeSetConflict
	}
	return service.repository.ReleaseClaim(ctx, ChangeSetReleaseCommand{
		ID: claim.ChangeSet.ID, Authorization: claim.ChangeSet.Authorization,
		ClaimID: claim.ClaimID, ErrorCode: "CHANGE_SET_RETRYABLE", Now: service.now().UTC(),
	})
}

func (service *ChangeSetService) RejectPending(
	ctx context.Context,
	id string,
	authorization ExecutionAuthorization,
	errorCode string,
) (ChangeSet, error) {
	return service.repository.RejectPending(ctx, id, authorization, strings.TrimSpace(errorCode), service.now().UTC())
}

func (service *ChangeSetService) RejectClaim(
	ctx context.Context,
	claim ChangeSetClaim,
	errorCode string,
) (ChangeSet, error) {
	errorCode = strings.TrimSpace(errorCode)
	if errorCode == "" || len(errorCode) > 80 {
		errorCode = "TOOL_AUTHORIZATION_DENIED"
	}
	return service.repository.RejectClaim(ctx, ChangeSetFinishCommand{
		ID: claim.ChangeSet.ID, Authorization: claim.ChangeSet.Authorization,
		ClaimID: claim.ClaimID, ErrorCode: errorCode, Now: service.now().UTC(),
	})
}

func canonicalChangeSetInput(input json.RawMessage) (json.RawMessage, string, error) {
	value, err := decodeObject(input)
	if err != nil {
		return nil, "", ErrToolInputInvalid
	}
	if containsSensitiveChangeSetField(value) {
		return nil, "", ErrChangeSetSensitive
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, "", ErrToolInputInvalid
	}
	if len(canonical) > maxChangeSetInputBytes {
		return nil, "", ErrChangeSetInputTooLarge
	}
	digest := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(digest[:]), nil
}

func containsSensitiveChangeSetField(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
			for _, forbidden := range []string{"password", "passwd", "secret", "token", "cookie", "authorization", "privatekey", "credential"} {
				if strings.Contains(normalized, forbidden) {
					return true
				}
			}
			if containsSensitiveChangeSetField(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSensitiveChangeSetField(child) {
				return true
			}
		}
	}
	return false
}

func validateExecutionAuthorization(authorization ExecutionAuthorization) error {
	if authorization.PrincipalID == 0 || authorization.WorkspaceID == 0 || authorization.GrantID == 0 ||
		strings.TrimSpace(authorization.ClientID) == "" ||
		(authorization.Channel != DeliveryMCP && authorization.Channel != DeliveryCLI) {
		return ErrAuthorizationDenied
	}
	audience, err := AudienceForChannel(authorization.Channel)
	if err != nil || authorization.Audience != audience {
		return ErrAuthorizationDenied
	}
	return nil
}
