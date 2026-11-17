package requirementgorm

import "time"

// RequirementCover is both the durable generation job and the persisted
// pointer to a requirement card thumbnail. Rows are only created by writes
// after this feature is deployed, so historical requirements are not backfilled.
type RequirementCover struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID   uint       `gorm:"not null;uniqueIndex:ux_requirement_cover_scope,priority:1;index:idx_requirement_cover_queue,priority:2" json:"workspace_id"`
	RequirementID uint       `gorm:"not null;uniqueIndex:ux_requirement_cover_scope,priority:2;index" json:"requirement_id"`
	SourceHash    string     `gorm:"size:64;not null" json:"-"`
	Generation    uint       `gorm:"not null;default:1" json:"generation"`
	Status        string     `gorm:"size:20;not null;index:idx_requirement_cover_queue,priority:1" json:"status"`
	ObjectKey     string     `gorm:"size:1024" json:"-"`
	ContentType   string     `gorm:"size:64" json:"content_type,omitempty"`
	Size          int64      `gorm:"not null;default:0" json:"size,omitempty"`
	Width         int        `gorm:"not null;default:0" json:"width,omitempty"`
	Height        int        `gorm:"not null;default:0" json:"height,omitempty"`
	Attempts      int        `gorm:"not null;default:0" json:"-"`
	NextAttemptAt time.Time  `gorm:"not null;index:idx_requirement_cover_queue,priority:3" json:"-"`
	ClaimedAt     *time.Time `gorm:"index" json:"-"`
	ClaimedBy     string     `gorm:"size:160" json:"-"`
	LastError     string     `gorm:"size:256" json:"-"`
	GeneratedAt   *time.Time `json:"generated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
