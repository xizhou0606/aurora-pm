package designreviewgorm

import "time"

type AnalysisJob struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID uint      `gorm:"not null;index"`
	ReviewID    string    `gorm:"type:varchar(36);not null;index"`
	Status      string    `gorm:"size:16;not null;default:pending;index:idx_design_review_analysis_claim,priority:1"`
	Attempts    int       `gorm:"not null;default:0"`
	AvailableAt time.Time `gorm:"index:idx_design_review_analysis_claim,priority:2"`
	LeaseOwner  *string
	LeaseUntil  *time.Time `gorm:"index:idx_design_review_analysis_claim,priority:3"`
	LastError   *string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (AnalysisJob) TableName() string { return "design_review_analysis_jobs" }

type DeepJob struct {
	ID                       string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID              uint   `gorm:"not null;index:idx_design_review_deep_review,priority:1"`
	ReviewID                 string `gorm:"type:varchar(36);not null;index:idx_design_review_deep_review,priority:2"`
	CreatedByUserID          uint   `gorm:"not null;index"`
	RetryOfID                *string
	Status                   string    `gorm:"size:16;not null;default:pending;index:idx_design_review_deep_claim,priority:1"`
	DesignRegion             []byte    `gorm:"type:jsonb;not null"`
	ImplementationRegion     []byte    `gorm:"type:jsonb;not null"`
	Focus                    string    `gorm:"size:500;not null"`
	CancelRequested          bool      `gorm:"not null;default:false"`
	Attempts                 int       `gorm:"not null;default:0"`
	AvailableAt              time.Time `gorm:"index:idx_design_review_deep_claim,priority:2"`
	LeaseOwner               *string
	LeaseUntil               *time.Time `gorm:"index:idx_design_review_deep_claim,priority:3"`
	Model                    *string
	ReasoningEffort          *string
	PromptVersion            *string
	ResponseID               *string
	DurationMS               *int
	GeneratedDifferenceCount int     `gorm:"not null;default:0"`
	LastError                *string `gorm:"type:text"`
	StartedAt                *time.Time
	CompletedAt              *time.Time
	CreatedAt                time.Time `gorm:"index:idx_design_review_deep_review,priority:3"`
	UpdatedAt                time.Time
}

func (DeepJob) TableName() string { return "design_review_deep_jobs" }

type ObjectCleanupJob struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID uint      `gorm:"not null;index"`
	ObjectKey   string    `gorm:"type:text;not null;uniqueIndex"`
	Attempts    int       `gorm:"not null;default:0"`
	AvailableAt time.Time `gorm:"index:idx_design_review_object_cleanup_claim,priority:1"`
	LeaseOwner  *string
	LeaseUntil  *time.Time `gorm:"index:idx_design_review_object_cleanup_claim,priority:2"`
	LastError   *string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ObjectCleanupJob) TableName() string { return "design_review_object_cleanup_jobs" }
