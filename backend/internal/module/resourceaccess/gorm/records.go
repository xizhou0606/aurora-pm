package resourceaccessgorm

import "time"

type PolicyRecord struct {
	ID             uint   `gorm:"primaryKey"`
	WorkspaceID    uint   `gorm:"not null;uniqueIndex:ux_resource_access_policy,priority:1;index"`
	ResourceID     string `gorm:"size:64;not null;uniqueIndex:ux_resource_access_policy,priority:2;index"`
	Mode           string `gorm:"size:20;not null"`
	WorkspaceLevel *string
	Revision       uint64    `gorm:"not null;default:1"`
	CreatedBy      uint      `gorm:"not null"`
	UpdatedBy      uint      `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (PolicyRecord) TableName() string { return "document_resource_access_policies" }

type RuleRecord struct {
	ID                 uint   `gorm:"primaryKey"`
	WorkspaceID        uint   `gorm:"not null;uniqueIndex:ux_resource_access_rule,priority:1;index"`
	ResourceID         string `gorm:"size:64;not null;uniqueIndex:ux_resource_access_rule,priority:2;index"`
	SubjectKind        string `gorm:"size:32;not null;uniqueIndex:ux_resource_access_rule,priority:3;index"`
	SubjectID          string `gorm:"size:128;not null;uniqueIndex:ux_resource_access_rule,priority:4"`
	Level              string `gorm:"size:16;not null"`
	IncludeDescendants bool   `gorm:"not null;default:false"`
	CreatedAt          time.Time
}

func (RuleRecord) TableName() string { return "document_resource_access_rules" }

type ShareLinkRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	WorkspaceID uint   `gorm:"not null;index:idx_resource_share_links_resource,priority:1"`
	ResourceID  string `gorm:"size:64;not null;index:idx_resource_share_links_resource,priority:2"`
	SecretHash  string `gorm:"size:64;not null;uniqueIndex"`
	Level       string `gorm:"size:16;not null"`
	Version     uint64 `gorm:"not null;default:1"`
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	CreatedBy   uint      `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (ShareLinkRecord) TableName() string { return "document_resource_share_links" }

type LinkAccessContextRecord struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;index:idx_resource_link_context_scope,priority:1"`
	ResourceID  string `gorm:"size:64;not null;index:idx_resource_link_context_scope,priority:2"`
	LinkID      string `gorm:"size:40;not null;index"`
	LinkVersion uint64 `gorm:"not null"`
	UserID      uint   `gorm:"not null;index:idx_resource_link_context_scope,priority:3"`
	TokenHash   string `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

func (LinkAccessContextRecord) TableName() string { return "document_resource_link_access_contexts" }

type MutationReceiptRecord struct {
	ID             uint   `gorm:"primaryKey"`
	WorkspaceID    uint   `gorm:"not null;uniqueIndex:ux_resource_access_receipt,priority:1"`
	ResourceID     string `gorm:"size:64;not null;uniqueIndex:ux_resource_access_receipt,priority:2"`
	Operation      string `gorm:"size:40;not null;uniqueIndex:ux_resource_access_receipt,priority:3"`
	IdempotencyKey string `gorm:"size:160;not null;uniqueIndex:ux_resource_access_receipt,priority:4"`
	InputDigest    string `gorm:"size:64;not null"`
	ResultJSON     string `gorm:"type:text;not null"`
	CreatedAt      time.Time
}

func (MutationReceiptRecord) TableName() string { return "document_resource_access_receipts" }

type AuditRecord struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;index"`
	ResourceID  string `gorm:"size:64;not null;index"`
	EventType   string `gorm:"size:64;not null;index"`
	ActorUserID uint   `gorm:"not null"`
	BeforeJSON  string `gorm:"type:text;not null"`
	AfterJSON   string `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

func (AuditRecord) TableName() string { return "document_resource_access_audits" }

type OutboxRecord struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;index"`
	ResourceID  string `gorm:"size:64;not null;index"`
	EventType   string `gorm:"size:64;not null;index"`
	PayloadJSON string `gorm:"type:text;not null"`
	CreatedAt   time.Time
	PublishedAt *time.Time
}

func (OutboxRecord) TableName() string { return "document_resource_access_outbox" }

type LinkExchangeLimitRecord struct {
	WorkspaceID     uint      `gorm:"primaryKey"`
	UserID          uint      `gorm:"primaryKey"`
	WindowStartedAt time.Time `gorm:"primaryKey"`
	Attempts        uint      `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

func (LinkExchangeLimitRecord) TableName() string { return "document_share_link_exchange_limits" }

func MigrationRecords() []any {
	return []any{
		&AccessRequestRecord{},
		&PolicyRecord{}, &RuleRecord{}, &ShareLinkRecord{}, &LinkAccessContextRecord{},
		&MutationReceiptRecord{}, &AuditRecord{}, &OutboxRecord{}, &LinkExchangeLimitRecord{},
	}
}
