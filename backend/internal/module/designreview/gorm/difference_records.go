package designreviewgorm

import "time"

type DifferenceLineage struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID        uint   `gorm:"not null;index:idx_design_review_lineages_chain,priority:1"`
	ChainRootReviewID  string `gorm:"type:varchar(36);not null;index:idx_design_review_lineages_chain,priority:2"`
	VerificationStatus string `gorm:"size:20;not null;default:pending"`
	VerifiedByUserID   *uint
	VerifiedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (DifferenceLineage) TableName() string { return "design_review_difference_lineages" }

type Difference struct {
	ID                           string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID                  uint   `gorm:"not null;index:idx_design_review_differences_review,priority:1;uniqueIndex:ux_design_review_difference_ordinal,priority:1"`
	ReviewID                     string `gorm:"type:varchar(36);not null;index:idx_design_review_differences_review,priority:2;uniqueIndex:ux_design_review_difference_ordinal,priority:2"`
	LineageID                    *string
	LineageRootDifferenceID      *string
	PreviousDifferenceID         *string
	DeepJobID                    *string
	Ordinal                      int    `gorm:"not null;uniqueIndex:ux_design_review_difference_ordinal,priority:3"`
	Source                       string `gorm:"size:24;not null;default:ai"`
	AnalysisKind                 string `gorm:"size:16;not null;default:full"`
	CreatedByUserID              *uint
	Category                     string `gorm:"size:24;not null;index"`
	Severity                     string `gorm:"size:16;not null;index"`
	Title                        string `gorm:"size:160;not null"`
	Description                  string `gorm:"type:text;not null"`
	Recommendation               string `gorm:"type:text;not null"`
	Confidence                   float64
	DesignRegion                 []byte `gorm:"type:jsonb"`
	ImplementationRegion         []byte `gorm:"type:jsonb"`
	EvidenceBasis                string `gorm:"size:40;not null"`
	EvidenceNotes                string `gorm:"type:text;not null"`
	VerificationStatus           string `gorm:"size:20;not null;default:pending;index"`
	VerifiedByUserID             *uint
	VerifiedAt                   *time.Time
	ResolutionStatus             string `gorm:"size:16;not null;default:unresolved;index"`
	ResolvedByUserID             *uint
	ResolvedAt                   *time.Time
	RecheckStatus                *string `gorm:"size:20"`
	DesignIdentityAnchor         []byte  `gorm:"type:jsonb"`
	ImplementationIdentityAnchor []byte  `gorm:"type:jsonb"`
	AIOriginal                   []byte  `gorm:"type:jsonb"`
	HumanEdited                  bool    `gorm:"not null;default:false"`
	EditedByUserID               *uint
	EditedAt                     *time.Time
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func (Difference) TableName() string { return "design_review_differences" }

type LineageEvent struct {
	ID             string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID    uint   `gorm:"not null;index:idx_design_review_lineage_events,priority:1"`
	LineageID      string `gorm:"type:varchar(36);not null;index:idx_design_review_lineage_events,priority:2"`
	ReviewID       string `gorm:"type:varchar(36);not null"`
	DifferenceID   *string
	ActorUserID    uint
	PreviousStatus string    `gorm:"size:20;not null"`
	NextStatus     string    `gorm:"size:20;not null"`
	CreatedAt      time.Time `gorm:"index:idx_design_review_lineage_events,priority:3"`
}

func (LineageEvent) TableName() string { return "design_review_lineage_events" }

type RecheckMatch struct {
	ID                           string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID                  uint   `gorm:"not null;uniqueIndex:ux_design_review_match_candidate,priority:1"`
	ReviewID                     string `gorm:"type:varchar(36);not null;uniqueIndex:ux_design_review_match_candidate,priority:2"`
	CandidateID                  string `gorm:"size:80;not null;uniqueIndex:ux_design_review_match_candidate,priority:3"`
	SuggestedLineageID           string `gorm:"type:varchar(36);not null"`
	Candidate                    []byte `gorm:"type:jsonb;not null"`
	DesignIdentityAnchor         []byte `gorm:"type:jsonb"`
	ImplementationIdentityAnchor []byte `gorm:"type:jsonb"`
	Confidence                   float64
	Reason                       string `gorm:"type:text;not null"`
	Status                       string `gorm:"size:16;not null;default:pending"`
	ResolvedByUserID             *uint
	ResolvedAt                   *time.Time
	DifferenceID                 *string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func (RecheckMatch) TableName() string { return "design_review_recheck_matches" }

type RecheckVerification struct {
	ID                          string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID                 uint   `gorm:"not null;uniqueIndex:ux_design_review_history_verification,priority:1"`
	ReviewID                    string `gorm:"type:varchar(36);not null;uniqueIndex:ux_design_review_history_verification,priority:2"`
	HistoricalDifferenceID      string `gorm:"type:varchar(36);not null;uniqueIndex:ux_design_review_history_verification,priority:3"`
	LineageRootDifferenceID     string `gorm:"type:varchar(36);not null"`
	LineageID                   *string
	CurrentDifferenceID         *string
	AIStatus                    string `gorm:"size:20;not null"`
	ManualStatusOverride        *string
	Confidence                  float64
	Reason                      string `gorm:"type:text;not null"`
	EvidenceSummary             string `gorm:"type:text;not null"`
	CurrentDesignRegion         []byte `gorm:"type:jsonb"`
	CurrentImplementationRegion []byte `gorm:"type:jsonb"`
	OverriddenByUserID          *uint
	OverriddenAt                *time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

func (RecheckVerification) TableName() string { return "design_review_recheck_verifications" }

type ResolutionEvent struct {
	ID                    string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID           uint   `gorm:"not null;index:idx_design_review_resolution_events,priority:1"`
	ReviewID              string `gorm:"type:varchar(36);not null;index:idx_design_review_resolution_events,priority:2"`
	DifferenceID          *string
	RecheckVerificationID *string
	ActorUserID           uint
	EventType             string `gorm:"size:60;not null"`
	PreviousValue         *string
	NextValue             *string
	CreatedAt             time.Time `gorm:"index:idx_design_review_resolution_events,priority:3"`
}

func (ResolutionEvent) TableName() string { return "design_review_resolution_events" }
