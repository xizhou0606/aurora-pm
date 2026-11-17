package designreviewgorm

import "time"

type MobileCollector struct {
	ID                  string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID         uint   `gorm:"not null;index:idx_design_review_collectors_user,priority:1"`
	UserID              uint   `gorm:"not null;index:idx_design_review_collectors_user,priority:2"`
	Name                string `gorm:"size:200;not null"`
	TokenHash           string `gorm:"size:64;not null;uniqueIndex"`
	Version             string `gorm:"size:80;not null"`
	MobilewrightVersion string `gorm:"size:80;not null"`
	Hostname            string `gorm:"size:200;not null"`
	Architecture        string `gorm:"size:16;not null"`
	Inventory           []byte `gorm:"type:jsonb;not null"`
	DoctorReady         bool   `gorm:"not null;default:false"`
	DoctorSummary       string `gorm:"type:text;not null"`
	LastSeenAt          *time.Time
	RevokedAt           *time.Time
	CreatedAt           time.Time `gorm:"index:idx_design_review_collectors_user,priority:3"`
	UpdatedAt           time.Time
}

func (MobileCollector) TableName() string { return "design_review_mobile_collectors" }

type MobilePairingCode struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID uint      `gorm:"not null;index"`
	UserID      uint      `gorm:"not null;index"`
	CodeHash    string    `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt   time.Time `gorm:"not null;index"`
	UsedAt      *time.Time
	CreatedAt   time.Time
}

func (MobilePairingCode) TableName() string { return "design_review_mobile_pairing_codes" }

type MobileCaptureDraft struct {
	ID                  string `gorm:"type:varchar(36);primaryKey"`
	WorkspaceID         uint   `gorm:"not null;index:idx_design_review_mobile_drafts_user,priority:1"`
	UserID              uint   `gorm:"not null;index:idx_design_review_mobile_drafts_user,priority:2"`
	CollectorID         string `gorm:"type:varchar(36);not null;index:idx_design_review_mobile_drafts_claim,priority:1"`
	Status              string `gorm:"size:16;not null;default:queued;index:idx_design_review_mobile_drafts_claim,priority:2"`
	Platform            string `gorm:"size:16;not null"`
	DeviceID            string `gorm:"size:300;not null"`
	AppID               *string
	Target              []byte `gorm:"type:jsonb"`
	Elements            []byte `gorm:"type:jsonb"`
	StructureTruncated  *bool
	Warnings            []byte  `gorm:"type:jsonb;not null"`
	ScreenshotObjectKey *string `gorm:"type:text"`
	ScreenshotMediaType *string
	ScreenshotSHA256    *string
	ScreenshotByteSize  *int64
	ClaimedAt           *time.Time
	CapturedAt          *time.Time
	ConsumedAt          *time.Time
	ExpiresAt           time.Time `gorm:"not null;index"`
	LastError           *string   `gorm:"type:text"`
	CreatedAt           time.Time `gorm:"index:idx_design_review_mobile_drafts_user,priority:3;index:idx_design_review_mobile_drafts_claim,priority:3"`
	UpdatedAt           time.Time
}

func (MobileCaptureDraft) TableName() string { return "design_review_mobile_capture_drafts" }
