// Package aigorm contains AI-module GORM records.
package aigorm

import "time"

type AIRun struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint   `gorm:"not null;index:idx_ai_run_subject,priority:1;index" json:"workspace_id"`
	Capability      string `gorm:"size:100;not null;index" json:"capability"`
	SubjectType     string `gorm:"size:40;not null;index:idx_ai_run_subject,priority:2" json:"subject_type"`
	SubjectID       uint   `gorm:"not null;index:idx_ai_run_subject,priority:3" json:"subject_id"`
	SubjectVersion  uint   `gorm:"not null" json:"subject_version"`
	Status          string `gorm:"size:30;not null;default:queued;index" json:"status"`
	Model           string `gorm:"size:100;not null" json:"model"`
	InputJSON       string `gorm:"type:text;not null" json:"-"`
	OutputJSON      string `gorm:"type:text" json:"-"`
	ErrorMessage    string `gorm:"type:text" json:"error_message,omitempty"`
	RequestedByUser uint   `gorm:"not null;index" json:"requested_by_user_id"`
	AppliedByUserID *uint  `gorm:"index" json:"applied_by_user_id,omitempty"`
	// Report-run Feishu export mapping. Empty export_document_id means the
	// run has not been published yet; the claim update only fills it once,
	// which makes publishing idempotent per run.
	ExportDocumentID  string     `gorm:"size:128;not null;default:''" json:"-"`
	ExportDocumentURL string     `gorm:"size:512;not null;default:''" json:"-"`
	ExportedAt        *time.Time `json:"-"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	AppliedAt         *time.Time `json:"applied_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type AIRunSource struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index" json:"workspace_id"`
	RunID       uint      `gorm:"not null;index" json:"run_id"`
	SourceType  string    `gorm:"size:40;not null" json:"source_type"`
	SourceURL   string    `gorm:"size:1000" json:"source_url,omitempty"`
	Revision    string    `gorm:"size:160" json:"revision,omitempty"`
	ContentHash string    `gorm:"size:64" json:"content_hash,omitempty"`
	BlockRefs   string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type AIArtifact struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index" json:"workspace_id"`
	RunID       uint      `gorm:"not null;index" json:"run_id"`
	Kind        string    `gorm:"size:60;not null" json:"kind"`
	Title       string    `gorm:"type:text;not null" json:"title"`
	ContentJSON string    `gorm:"type:text;not null" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type AIArtifactApplication struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index" json:"workspace_id"`
	RunID          uint      `gorm:"not null;index" json:"run_id"`
	ArtifactID     uint      `gorm:"not null;index" json:"artifact_id"`
	SubjectVersion uint      `gorm:"not null" json:"subject_version"`
	ChangesJSON    string    `gorm:"type:text;not null" json:"-"`
	AppliedByUser  uint      `gorm:"not null;index" json:"applied_by_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type AIFeedback struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index" json:"workspace_id"`
	RunID       uint      `gorm:"not null;index" json:"run_id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Rating      string    `gorm:"size:20;not null" json:"rating"`
	Comment     string    `gorm:"type:text" json:"comment,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type AIUsageDaily struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;uniqueIndex:ux_ai_usage_day,priority:1" json:"workspace_id"`
	UsageDate    string    `gorm:"size:10;not null;uniqueIndex:ux_ai_usage_day,priority:2" json:"usage_date"`
	Model        string    `gorm:"size:100;not null;uniqueIndex:ux_ai_usage_day,priority:3" json:"model"`
	InputTokens  int64     `gorm:"not null;default:0" json:"input_tokens"`
	OutputTokens int64     `gorm:"not null;default:0" json:"output_tokens"`
	RunCount     int64     `gorm:"not null;default:0" json:"run_count"`
	UpdatedAt    time.Time `json:"updated_at"`
}
