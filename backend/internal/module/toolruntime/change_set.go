package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

var (
	ErrChangeSetInvalid       = errors.New("change set is invalid")
	ErrChangeSetNotFound      = errors.New("change set not found")
	ErrChangeSetConflict      = errors.New("change set conflicts with an existing request")
	ErrChangeSetInProgress    = errors.New("change set execution is already in progress")
	ErrChangeSetInputTooLarge = errors.New("change set input exceeds the size limit")
	ErrChangeSetSensitive     = errors.New("change set input contains sensitive fields")
	ErrMutationRetryable      = errors.New("mutation apply can be retried")
)

type ChangeSetStatus string

const (
	ChangeSetPending   ChangeSetStatus = "pending"
	ChangeSetConfirmed ChangeSetStatus = "confirmed"
	ChangeSetExpired   ChangeSetStatus = "expired"
	ChangeSetRejected  ChangeSetStatus = "rejected"
	ChangeSetFailed    ChangeSetStatus = "failed"
	ChangeSetSucceeded ChangeSetStatus = "succeeded"
)

const (
	ChangeSetShowControlName     = "changeset.show"
	ChangeSetConfirmControlName  = "changeset.confirm"
	changeSetControlVersion      = "1.0.0"
	changeSetControlInputSchema  = `{"type":"object","properties":{"id":{"type":"string","minLength":8,"maxLength":128}},"required":["id"],"additionalProperties":false}`
	changeSetControlOutputSchema = `{"type":"object","properties":{"id":{"type":"string"},"status":{"type":"string"},"tool_name":{"type":"string"},"tool_version":{"type":"string"},"risk":{"type":"string"},"audit_event":{"type":"string"},"input_digest":{"type":"string"},"input_size":{"type":"integer"},"result":{"type":"object"},"mutation_result":{"type":"object","properties":{"confirmed":{"type":"boolean"},"changed_fields":{"type":"array","items":{"type":"string"}},"already_applied":{"type":"boolean"},"entity":{"type":"object"}},"required":["confirmed","changed_fields","already_applied"],"additionalProperties":false},"error_code":{"type":"string"},"error":{"type":"object","properties":{"code":{"type":"string"},"type":{"type":"string"},"message":{"type":"string"},"hint":{"type":"string"},"retryable":{"type":"boolean"},"status":{"type":"integer"}},"required":["code","type","message","hint","retryable","status"],"additionalProperties":false},"expires_at":{"type":"string"},"confirmed_at":{"type":"string"},"completed_at":{"type":"string"},"created_at":{"type":"string"},"updated_at":{"type":"string"}},"required":["id","status","tool_name","tool_version","risk","audit_event","input_digest","input_size","expires_at","created_at","updated_at"],"additionalProperties":false}`
)

const (
	defaultChangeSetTTL        = 10 * time.Minute
	defaultChangeSetClaimLease = 30 * time.Second
	maxChangeSetInputBytes     = 64 << 10
	maxChangeSetResultBytes    = 256 << 10
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

type ChangeSet struct {
	ID          string          `json:"id"`
	Status      ChangeSetStatus `json:"status"`
	ToolName    string          `json:"tool_name"`
	ToolVersion string          `json:"tool_version"`
	Risk        RiskLevel       `json:"risk"`
	AuditEvent  string          `json:"audit_event"`
	// Authorization is an internal exact-binding fact. Public adapters must
	// never expose principal, workspace, grant, client, channel, or audience.
	Authorization  ExecutionAuthorization  `json:"-"`
	InputDigest    string                  `json:"input_digest"`
	InputSize      int                     `json:"input_size"`
	Result         json.RawMessage         `json:"result,omitempty"`
	MutationResult *StandardMutationResult `json:"mutation_result,omitempty"`
	ErrorCode      string                  `json:"error_code,omitempty"`
	ExpiresAt      time.Time               `json:"expires_at"`
	ConfirmedAt    *time.Time              `json:"confirmed_at,omitempty"`
	CompletedAt    *time.Time              `json:"completed_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// StandardMutationResult is the transport-neutral, additive summary shared by
// CLI and MCP clients. Result remains unchanged for backward compatibility.
type StandardMutationResult struct {
	Confirmed      bool                    `json:"confirmed"`
	ChangedFields  []string                `json:"changed_fields"`
	AlreadyApplied bool                    `json:"already_applied"`
	Entity         *StandardMutationEntity `json:"entity,omitempty"`
}

type StandardMutationEntity struct {
	Type string `json:"type,omitempty"`
	ID   int64  `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	URL  string `json:"url,omitempty"`
}

type ChangeSetPolicyConfig struct {
	PendingTTL time.Duration
	ClaimLease time.Duration
}

func DefaultChangeSetPolicy() ChangeSetPolicyConfig {
	return ChangeSetPolicyConfig{PendingTTL: defaultChangeSetTTL, ClaimLease: defaultChangeSetClaimLease}
}

type ChangeSetDraft struct {
	ChangeSet
	IdempotencyDigest string
	Input             json.RawMessage
}

type ChangeSetClaim struct {
	ChangeSet ChangeSet
	ClaimID   string
	Input     json.RawMessage
}

type ChangeSetCreateResult struct {
	ChangeSet ChangeSet
	Created   bool
}

// ChangeSetControlDefinition is the protocol-neutral contract for lifecycle
// operations around a pending ChangeSet. These controls are not business Tools
// and therefore are not part of workspace Tool selection or Grant scope.
type ChangeSetControlDefinition struct {
	Name         string
	Version      string
	Summary      string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Risk         RiskLevel
	Idempotent   bool
}

func ChangeSetControlDefinitions() []ChangeSetControlDefinition {
	return []ChangeSetControlDefinition{
		{
			Name: ChangeSetShowControlName, Version: changeSetControlVersion,
			Summary:     "Show a PM ChangeSet bound to the current MCP authorization.",
			InputSchema: json.RawMessage(changeSetControlInputSchema), OutputSchema: json.RawMessage(changeSetControlOutputSchema),
			Risk: RiskRead, Idempotent: true,
		},
		{
			Name: ChangeSetConfirmControlName, Version: changeSetControlVersion,
			Summary:     "Confirm and execute a pending PM ChangeSet after explicit user approval.",
			InputSchema: binaryConfirmationSchema(), OutputSchema: json.RawMessage(changeSetControlOutputSchema),
			Risk: RiskMutation, Idempotent: true,
		},
	}
}

func FindChangeSetControlDefinition(name string) (ChangeSetControlDefinition, bool) {
	for _, definition := range ChangeSetControlDefinitions() {
		if definition.Name == name {
			definition.InputSchema = append(json.RawMessage(nil), definition.InputSchema...)
			definition.OutputSchema = append(json.RawMessage(nil), definition.OutputSchema...)
			return definition, true
		}
	}
	return ChangeSetControlDefinition{}, false
}

type ChangeSetClaimCommand struct {
	ID            string
	Authorization ExecutionAuthorization
	ClaimID       string
	Now           time.Time
	ClaimLease    time.Duration
}

type ChangeSetFinishCommand struct {
	ID            string
	Authorization ExecutionAuthorization
	ClaimID       string
	Result        json.RawMessage
	ErrorCode     string
	Now           time.Time
}

type ChangeSetReleaseCommand struct {
	ID            string
	Authorization ExecutionAuthorization
	ClaimID       string
	ErrorCode     string
	Now           time.Time
}

type ChangeSetRepository interface {
	CreateOrFind(context.Context, ChangeSetDraft) (ChangeSetCreateResult, error)
	FindBound(context.Context, string, ExecutionAuthorization, time.Time) (ChangeSet, error)
	ConfirmAndClaim(context.Context, ChangeSetClaimCommand) (ChangeSetClaim, error)
	Succeed(context.Context, ChangeSetFinishCommand) (ChangeSet, error)
	Fail(context.Context, ChangeSetFinishCommand) (ChangeSet, error)
	RejectClaim(context.Context, ChangeSetFinishCommand) (ChangeSet, error)
	ReleaseClaim(context.Context, ChangeSetReleaseCommand) (ChangeSet, error)
	RejectPending(context.Context, string, ExecutionAuthorization, string, time.Time) (ChangeSet, error)
}

type retryableApplyError struct{ cause error }

func (err retryableApplyError) Error() string { return ErrMutationRetryable.Error() }
func (err retryableApplyError) Unwrap() error { return ErrMutationRetryable }

// RetryableApplyError marks a transient owning-domain failure. The cause is
// deliberately not exposed by protocol adapters or persisted in ChangeSet
// events; callers receive the stable CHANGE_SET_IN_PROGRESS contract.
func RetryableApplyError(cause error) error {
	return retryableApplyError{cause: cause}
}

type MutationCall struct {
	Definition    Definition
	Authorization ExecutionAuthorization
	Input         json.RawMessage
	ChangeSetID   string
	ApplyKey      string
}

// IdempotentMutationHandler is the activation gate for every write Tool. Apply
// must durably deduplicate ApplyKey in the owning domain transaction and replay
// the same output after a crash without repeating side effects.
type IdempotentMutationHandler interface {
	Apply(context.Context, MutationCall) (json.RawMessage, error)
}

type ChangeSetExecutor interface {
	ShowChangeSet(context.Context, string, ExecutionAuthorization) (ChangeSet, error)
	ConfirmChangeSet(context.Context, string, ExecutionAuthorization) (ChangeSet, error)
}

func terminalChangeSetStatus(status ChangeSetStatus) bool {
	switch status {
	case ChangeSetExpired, ChangeSetRejected, ChangeSetFailed, ChangeSetSucceeded:
		return true
	default:
		return false
	}
}

func binaryConfirmationSchema() json.RawMessage {
	var schema map[string]any
	_ = json.Unmarshal([]byte(changeSetControlInputSchema), &schema)
	schema["properties"].(map[string]any)["file_base64"] = map[string]any{"type": "string", "minLength": 4, "maxLength": 699052, "description": "Optional transient file for upload confirmation, base64 encoded; maximum decoded size 512 KiB. Larger files use authenticated multipart confirmation."}
	raw, _ := json.Marshal(schema)
	return raw
}
