// Package designreviewgorm contains persistence records for design-review data.
package designreviewgorm

import "time"

type Review struct {
	ID                              string  `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID                     uint    `gorm:"not null;index:idx_design_reviews_workspace_created,priority:1;index:idx_design_reviews_requirement,priority:1;index:idx_design_reviews_scope,priority:1"`
	RequirementID                   *uint   `gorm:"index:idx_design_reviews_requirement,priority:2"`
	RequirementTitleSnapshot        string  `gorm:"size:255;not null"`
	ProjectSetIDSnapshot            uint    `gorm:"not null;index:idx_design_reviews_scope,priority:2"`
	ProjectSetNameSnapshot          string  `gorm:"size:255;not null"`
	PlanUnitIDSnapshot              uint    `gorm:"not null;index:idx_design_reviews_scope,priority:3"`
	PlanUnitTitleSnapshot           string  `gorm:"size:255;not null"`
	ChainRootID                     *string `gorm:"type:varchar(36);index"`
	SourceReviewID                  *string `gorm:"type:varchar(36);index"`
	RoundNumber                     int     `gorm:"not null;default:1"`
	CreatedByUserID                 uint    `gorm:"not null;index"`
	CreatedByName                   string  `gorm:"size:255;not null;default:''"`
	ImplementationKind              string  `gorm:"size:16;not null;index"`
	ImplementationTarget            []byte  `gorm:"type:jsonb;not null"`
	ImplementationURL               *string `gorm:"type:text"`
	ImplementationActualURL         *string `gorm:"type:text"`
	FigmaURL                        string  `gorm:"type:text;not null"`
	FigmaFileKey                    string  `gorm:"size:160;not null"`
	FigmaNodeID                     string  `gorm:"size:120;not null"`
	FigmaNodeName                   string  `gorm:"size:300;not null;default:''"`
	ViewportWidth                   int     `gorm:"not null"`
	ViewportHeight                  int     `gorm:"not null"`
	DevicePixelRatio                float64 `gorm:"not null;default:1"`
	PageZoom                        float64 `gorm:"not null;default:1"`
	DesignWidth                     int     `gorm:"not null;default:0"`
	DesignHeight                    int     `gorm:"not null;default:0"`
	DimensionsMatch                 bool    `gorm:"not null;default:false"`
	DesignStructureAvailable        bool    `gorm:"not null;default:false"`
	ImplementationStructureComplete bool    `gorm:"not null;default:false"`
	CaptureWarnings                 []byte  `gorm:"type:jsonb;not null"`
	AnalysisScope                   []byte  `gorm:"type:jsonb;not null"`
	MobileTopbar                    []byte  `gorm:"type:jsonb;not null"`
	DOMNormalization                []byte  `gorm:"column:dom_normalization;type:jsonb;not null"`
	ModelCoverageComplete           bool    `gorm:"not null;default:false"`
	HeatmapDifferenceRatio          float64 `gorm:"not null;default:1"`
	Status                          string  `gorm:"size:20;not null;default:queued;index"`
	Conclusion                      *string `gorm:"size:20;index"`
	AnalysisSummary                 *string `gorm:"type:text"`
	Model                           *string `gorm:"size:120"`
	ReasoningEffort                 *string `gorm:"size:40"`
	PromptVersion                   *string `gorm:"size:100"`
	ResponseID                      *string `gorm:"size:160"`
	AnalysisDurationMS              *int
	AnalysisAttempts                int     `gorm:"not null;default:0"`
	CancelRequested                 bool    `gorm:"not null;default:false"`
	LastErrorStage                  *string `gorm:"size:80"`
	LastError                       *string `gorm:"type:text"`
	CompletedAt                     *time.Time
	AbandonedAt                     *time.Time
	DeletingAt                      *time.Time
	CreatedAt                       time.Time `gorm:"index:idx_design_reviews_workspace_created,priority:2"`
	UpdatedAt                       time.Time
}

func (Review) TableName() string { return "design_reviews" }

type Asset struct {
	ID          string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID uint   `gorm:"not null;index:idx_design_review_assets_review,priority:1;uniqueIndex:ux_design_review_asset_kind,priority:1"`
	ReviewID    string `gorm:"type:varchar(36);not null;index:idx_design_review_assets_review,priority:2;uniqueIndex:ux_design_review_asset_kind,priority:2"`
	Kind        string `gorm:"size:64;not null;uniqueIndex:ux_design_review_asset_kind,priority:3"`
	ObjectKey   string `gorm:"type:text;not null;uniqueIndex"`
	ContentType string `gorm:"size:120;not null"`
	ByteSize    int64  `gorm:"not null"`
	SHA256      string `gorm:"size:64;not null"`
	Width       *int
	Height      *int
	CreatedAt   time.Time
}

func (Asset) TableName() string { return "design_review_assets" }
