package designreview

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

type Repository interface {
	ListRequirementOptions(context.Context, uint) ([]RequirementOption, error)
	ListReviews(context.Context, uint, ReviewListFilter) ([]DesignReviewSummary, int64, error)
	GetReview(context.Context, uint, string) (DesignReviewDetail, error)
	GetReviewForUpdate(context.Context, uint, string) (DesignReviewDetail, error)
	CreateReview(context.Context, CreateReviewRecord) (DesignReviewDetail, error)
	CreateRecheck(context.Context, RecheckRecord) (DesignReviewDetail, error)
	ListRounds(context.Context, uint, string) ([]ReviewRound, error)
	SaveAsset(context.Context, AssetRecord) (*string, error)
	SaveAssets(context.Context, []AssetRecord) ([]string, error)
	CompleteObjectCleanup(context.Context, uint, string) error
	GetAsset(context.Context, uint, string, string) (AssetRecord, error)
	ListAssetKinds(context.Context, uint, string) ([]string, error)
	MarkDeleting(context.Context, uint, string, uint) error
	CreateDifference(context.Context, DifferenceRecord) (DesignDifference, error)
	UpdateDifference(context.Context, uint, string, string, uint, DifferenceInput) (DesignDifference, error)
	DeleteDifference(context.Context, uint, string, string, uint) error
	TransitionDifference(context.Context, uint, string, string, uint, string, string) (DesignDifference, error)
	BuildRecheckSnapshot(context.Context, uint, string) (RecheckSnapshot, error)
	CompleteRecheckJob(context.Context, ClaimedAnalysisJob, RecheckSnapshot, RecheckAnalysis, AnalysisRunMetadata) error
	ResolveRecheckMatch(context.Context, uint, string, string, uint, string) error
	OverrideRecheckVerification(context.Context, uint, string, string, uint, *string) error
	QueueAnalysis(context.Context, uint, string, uint) error
	AbandonAnalysis(context.Context, uint, string, uint) error
	CreateDeepJob(context.Context, DeepJobRecord) (DeepReviewJob, error)
	ListDeepJobs(context.Context, uint, string) ([]DeepReviewJob, error)
	TransitionDeepJob(context.Context, uint, string, string, uint, string) error
	ListMobileCollectors(context.Context, uint, uint) ([]MobileCollector, error)
	GetMobileCollector(context.Context, uint, uint, string) (MobileCollector, error)
	CreatePairingCode(context.Context, PairingCodeRecord) error
	PairMobileCollector(context.Context, string, MobileCollectorRecord) (MobileCollector, error)
	AuthenticateCollector(context.Context, string) (MobileCollector, error)
	UpdateMobileCollectorHeartbeat(context.Context, string, MobileCollectorHeartbeatRecord) error
	RevokeMobileCollector(context.Context, uint, string, uint) error
	CreateCaptureDraft(context.Context, CaptureDraftRecord) (MobileCaptureDraft, error)
	ListCaptureDrafts(context.Context, uint, uint) ([]MobileCaptureDraft, error)
	ClaimCaptureDraft(context.Context, string) (*MobileCaptureDraft, error)
	GetCaptureDraftForCollector(context.Context, string, string) (MobileCaptureDraft, error)
	GetCaptureAsset(context.Context, uint, string, uint) (MobileCaptureAssetRecord, error)
	CompleteCaptureDraft(context.Context, CompleteCaptureRecord) error
	FailCaptureDraft(context.Context, string, string, uint, string) error
	ConsumeCaptureDraft(context.Context, uint, string, uint) (MobileCaptureDraft, error)
	DeleteCaptureDraft(context.Context, uint, string, uint) (*string, error)
}

type ClaimedAnalysisJob struct {
	ID          string
	WorkspaceID uint
	ReviewID    string
}

type AnalysisRunMetadata struct {
	Model           string
	ReasoningEffort string
	PromptVersion   string
	ResponseID      string
	DurationMS      int
}

func (metadata AnalysisRunMetadata) Valid() bool {
	return strings.TrimSpace(metadata.Model) != "" && len(metadata.Model) <= 120 &&
		(metadata.ReasoningEffort == "low" || metadata.ReasoningEffort == "medium" || metadata.ReasoningEffort == "high") &&
		strings.TrimSpace(metadata.PromptVersion) != "" && len(metadata.PromptVersion) <= 100 &&
		strings.TrimSpace(metadata.ResponseID) != "" && len(metadata.ResponseID) <= 160 &&
		metadata.DurationMS >= 0 && metadata.DurationMS <= int((24*time.Hour)/time.Millisecond)
}

type ProjectAccess interface {
	CanAccessProject(context.Context, uint, uint, uint) (bool, error)
	IsAccessAdministrator(context.Context, uint, uint) (bool, error)
}

// EffectiveActionReader exposes identity's authoritative role and user-group
// Action union without coupling design review to identity persistence.
type EffectiveActionReader interface {
	EffectiveActionCodes(context.Context, uint) ([]string, error)
}

type ObjectStore interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
	GetObject(context.Context, string) (io.ReadCloser, error)
	RemoveObject(context.Context, string) error
}

type FigmaPreviewer interface {
	Preview(context.Context, string, string) (FigmaPreview, error)
	Evidence(context.Context, string, string) (FigmaPreview, []byte, string, error)
}

type DOMNormalizer interface {
	Plan(context.Context, DOMNormalizationRequest) (DOMNormalizationPlan, error)
}

type Availability struct {
	ObjectStorage   bool
	VisualModel     bool
	Figma           bool
	Model           string
	ReasoningEffort string
	ImageDetail     string
}

type CreateReviewRecord struct {
	ID                   string
	WorkspaceID          uint
	Requirement          RequirementOption
	SourceReviewID       *string
	ChainRootID          string
	RoundNumber          int
	CreatedByUserID      uint
	CreatedByName        string
	FigmaURL             string
	FigmaFileKey         string
	FigmaNodeID          string
	FigmaNodeName        string
	ImplementationTarget DesignReviewTarget
	ViewportWidth        int
	ViewportHeight       int
	DevicePixelRatio     float64
	PageZoom             float64
	AnalysisScope        AnalysisScope
	MobileTopbar         []byte
	DOMNormalization     []byte
	InitialAssets        []AssetRecord
}

type RecheckRecord struct {
	ID              string
	WorkspaceID     uint
	SourceReviewID  string
	ChainRootID     string
	RoundNumber     int
	CreatedByUserID uint
	CreatedByName   string
	Input           CreateReviewInput
	FigmaFileKey    string
	FigmaNodeID     string
	FigmaNodeName   string
	InitialAssets   []AssetRecord
}

type AssetRecord struct {
	ID          string
	WorkspaceID uint
	ReviewID    string
	Kind        string
	ObjectKey   string
	ContentType string
	ByteSize    int64
	SHA256      string
	Width       *int
	Height      *int
	CreatedAt   time.Time
	ActorUserID uint
	// StructureComplete is populated only for implementation_structure assets.
	// CaptureWarnings carries the already-validated collector/extension warnings.
	StructureComplete *bool
	CaptureWarnings   json.RawMessage
	DOMNormalization  json.RawMessage
}

type DifferenceRecord struct {
	ID          string
	WorkspaceID uint
	ReviewID    string
	ActorUserID uint
	Input       DifferenceInput
}

type DeepJobRecord struct {
	ID                   string
	WorkspaceID          uint
	ReviewID             string
	CreatedByUserID      uint
	DesignRegion         Region
	ImplementationRegion Region
	Focus                string
	RetryOfID            *string
}

type DeepReviewJob struct {
	ID                       string     `json:"id"`
	ReviewID                 string     `json:"review_id"`
	RetryOfID                *string    `json:"retry_of_id,omitempty"`
	Status                   string     `json:"status"`
	CreatedByUserID          uint       `json:"created_by_user_id"`
	DesignRegion             Region     `json:"design_region"`
	ImplementationRegion     Region     `json:"implementation_region"`
	Focus                    string     `json:"focus"`
	Attempts                 int        `json:"attempts"`
	Model                    *string    `json:"model,omitempty"`
	ReasoningEffort          *string    `json:"reasoning_effort,omitempty"`
	PromptVersion            *string    `json:"prompt_version,omitempty"`
	ResponseID               *string    `json:"response_id,omitempty"`
	DurationMS               *int       `json:"duration_ms,omitempty"`
	GeneratedDifferenceCount int        `json:"generated_difference_count"`
	LastError                *string    `json:"last_error,omitempty"`
	StartedAt                *time.Time `json:"started_at,omitempty"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	CanRetry                 bool       `json:"can_retry"`
	CanCancel                bool       `json:"can_cancel"`
}

type FigmaPreview struct {
	FileKey   string          `json:"file_key"`
	NodeID    string          `json:"node_id"`
	NodeName  string          `json:"node_name"`
	ImageURL  string          `json:"image_url"`
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	ExpiresAt string          `json:"expires_at,omitempty"`
	Structure json.RawMessage `json:"-"`
}

type MobileCollector struct {
	ID                  string          `json:"id"`
	WorkspaceID         uint            `json:"-"`
	UserID              uint            `json:"-"`
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	MobilewrightVersion string          `json:"mobilewright_version"`
	Hostname            string          `json:"hostname"`
	Architecture        string          `json:"architecture"`
	Inventory           json.RawMessage `json:"inventory"`
	DoctorReady         bool            `json:"doctor_ready"`
	DoctorSummary       string          `json:"doctor_summary"`
	LastSeenAt          *time.Time      `json:"last_seen_at,omitempty"`
	RevokedAt           *time.Time      `json:"revoked_at,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

type PairingCodeRecord struct {
	ID          string
	WorkspaceID uint
	UserID      uint
	CodeHash    string
	ExpiresAt   time.Time
}

type MobileCollectorRecord struct {
	ID                  string
	TokenHash           string
	Name                string
	Version             string
	MobilewrightVersion string
	Hostname            string
	Architecture        string
	Inventory           []byte
}

type MobileCollectorHeartbeatRecord struct {
	Version             string
	MobilewrightVersion string
	Hostname            string
	Architecture        string
	Inventory           []byte
	DoctorReady         bool
	DoctorSummary       string
}

type MobileCaptureDraft struct {
	ID                  string          `json:"id"`
	CollectorID         string          `json:"collector_id"`
	Status              string          `json:"status"`
	Platform            string          `json:"platform"`
	DeviceID            string          `json:"device_id"`
	AppID               *string         `json:"app_id,omitempty"`
	Target              json.RawMessage `json:"target,omitempty"`
	Elements            json.RawMessage `json:"elements,omitempty"`
	StructureTruncated  *bool           `json:"structure_truncated,omitempty"`
	Warnings            json.RawMessage `json:"warnings"`
	ScreenshotAvailable bool            `json:"screenshot_available"`
	ExpiresAt           time.Time       `json:"expires_at"`
	CapturedAt          *time.Time      `json:"captured_at,omitempty"`
	LastError           *string         `json:"last_error,omitempty"`
}

type CaptureDraftRecord struct {
	ID          string
	WorkspaceID uint
	UserID      uint
	CollectorID string
	Platform    string
	DeviceID    string
	ExpiresAt   time.Time
}

type MobileCaptureAssetRecord struct {
	ObjectKey   string
	ContentType string
	ByteSize    int64
}

type CompleteCaptureRecord struct {
	ID                  string
	CollectorID         string
	WorkspaceID         uint
	ActorUserID         uint
	AppID               string
	Target              []byte
	Elements            []byte
	StructureTruncated  bool
	Warnings            []byte
	ScreenshotObjectKey string
	ScreenshotMediaType string
	ScreenshotSHA256    string
	ScreenshotByteSize  int64
	CapturedAt          time.Time
}
