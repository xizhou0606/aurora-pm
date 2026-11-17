// Package toolruntime owns the protocol-neutral contract for PM tools.
// Transport adapters project MCP and CLI manifests from the same Catalog.
package toolruntime

import "encoding/json"

type DeliveryChannel string

const (
	DeliveryMCP DeliveryChannel = "mcp"
	DeliveryCLI DeliveryChannel = "cli"
)

type Lifecycle string

const (
	LifecycleDraft      Lifecycle = "draft"
	LifecycleActive     Lifecycle = "active"
	LifecycleDeprecated Lifecycle = "deprecated"
)

type RiskLevel string

const (
	RiskRead       RiskLevel = "read"
	RiskMutation   RiskLevel = "mutation"
	RiskHighImpact RiskLevel = "high_impact"
)

type ChangeSetPolicy string

const (
	ChangeSetForbidden ChangeSetPolicy = "forbidden"
	ChangeSetRequired  ChangeSetPolicy = "required"
)

type UATPolicy string

const (
	UATBlocked           UATPolicy = "blocked"
	UATWithinGrantWindow UATPolicy = "within_grant_window"
)

// AuthorizationContract contains only the facts the external-tool gate must
// establish. It intentionally has no PM role, menu, or permission fields.
type AuthorizationContract struct {
	IdentityRequired  bool   `json:"identity_required"`
	WorkspaceRequired bool   `json:"workspace_required"`
	GrantRequired     bool   `json:"grant_required"`
	GrantScope        string `json:"grant_scope"`
}

// DeprecationContract is a comparable value so delivery parity compares its
// content rather than pointer identity. Replacement is optional.
type DeprecationContract struct {
	SinceVersion string `json:"since_version"`
	Replacement  string `json:"replacement,omitempty"`
	Notice       string `json:"notice"`
}

// Definition is the single source of truth consumed by every delivery
// adapter. Delivery channels are deliberately not configurable per tool.
type Definition struct {
	Name          string                `json:"name"`
	Version       string                `json:"version"`
	Summary       string                `json:"summary"`
	Lifecycle     Lifecycle             `json:"lifecycle"`
	Deprecation   DeprecationContract   `json:"deprecation"`
	InputSchema   json.RawMessage       `json:"input_schema"`
	OutputSchema  json.RawMessage       `json:"output_schema"`
	Authorization AuthorizationContract `json:"authorization"`
	Risk          RiskLevel             `json:"risk"`
	ChangeSet     ChangeSetPolicy       `json:"change_set"`
	UAT           UATPolicy             `json:"uat"`
	AuditEvent    string                `json:"audit_event"`
}

// DeliveryContract is the transport-independent manifest record used by the
// parity gate. Schema payloads are represented by canonical digests so schema
// formatting cannot create false drift.
type DeliveryContract struct {
	Name             string                `json:"name"`
	Version          string                `json:"version"`
	Summary          string                `json:"summary"`
	Lifecycle        Lifecycle             `json:"lifecycle"`
	Deprecation      DeprecationContract   `json:"deprecation"`
	InputSchemaHash  string                `json:"input_schema_hash"`
	OutputSchemaHash string                `json:"output_schema_hash"`
	Authorization    AuthorizationContract `json:"authorization"`
	Risk             RiskLevel             `json:"risk"`
	ChangeSet        ChangeSetPolicy       `json:"change_set"`
	UAT              UATPolicy             `json:"uat"`
	AuditEvent       string                `json:"audit_event"`
}
