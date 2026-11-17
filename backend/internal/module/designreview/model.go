// Package designreview owns design-review lifecycle rules and public DTOs.
package designreview

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalid          = errors.New("design review input is invalid")
	ErrNotFound         = errors.New("design review not found")
	ErrForbidden        = errors.New("design review is not accessible")
	ErrConflict         = errors.New("design review changed concurrently")
	ErrUnavailable      = errors.New("design review capability is unavailable")
	ErrRequirement      = errors.New("an accessible scheduled requirement is required")
	ErrCreatorOnly      = errors.New("this operation is restricted to the creator")
	ErrEvidenceRequired = errors.New("design and implementation evidence are required")
)

var validCategories = setOf("layout", "size", "typography", "color", "component", "icon_image", "content", "effect")
var validSeverities = setOf("critical", "major", "minor")

// ActorIdentity keeps PM authorization identity separate from the platform
// fact ID stored on project-scoped business records.
type ActorIdentity struct {
	PrincipalUserID uint
	FactUserID      uint
}

func (actor ActorIdentity) Valid() bool {
	return actor.PrincipalUserID > 0 && actor.FactUserID > 0
}

func setOf(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

type Region struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (r Region) Valid() bool {
	return r.X >= 0 && r.Y >= 0 && r.Width > 0 && r.Height > 0 &&
		r.X+r.Width <= 1.000001 && r.Y+r.Height <= 1.000001
}

type AnalysisScope struct {
	Enabled              bool    `json:"enabled"`
	Version              string  `json:"version,omitempty"`
	SelectionSide        string  `json:"selection_side,omitempty"`
	DesignRegion         *Region `json:"design_region,omitempty"`
	ImplementationRegion *Region `json:"implementation_region,omitempty"`
}

func (s AnalysisScope) Valid(designWidth, designHeight, implementationWidth, implementationHeight int) bool {
	if !s.Enabled {
		return true
	}
	if s.Version != "1.0" || (s.SelectionSide != "design" && s.SelectionSide != "implementation") ||
		s.DesignRegion == nil || s.ImplementationRegion == nil || !s.DesignRegion.Valid() || !s.ImplementationRegion.Valid() {
		return false
	}
	if designWidth <= 0 || designHeight <= 0 || implementationWidth <= 0 || implementationHeight <= 0 {
		return false
	}
	return s.DesignRegion.Width*float64(designWidth) >= 16 && s.DesignRegion.Height*float64(designHeight) >= 16 &&
		s.ImplementationRegion.Width*float64(implementationWidth) >= 16 && s.ImplementationRegion.Height*float64(implementationHeight) >= 16
}

type DesignReviewTarget struct {
	Kind         string          `json:"kind"`
	OriginalURL  string          `json:"original_url,omitempty"`
	ActualURL    string          `json:"actual_url,omitempty"`
	Platform     string          `json:"platform,omitempty"`
	AppID        string          `json:"app_id,omitempty"`
	AppName      string          `json:"app_name,omitempty"`
	AppVersion   string          `json:"app_version,omitempty"`
	Orientation  string          `json:"orientation,omitempty"`
	Device       json.RawMessage `json:"device,omitempty"`
	Screen       json.RawMessage `json:"screen,omitempty"`
	Collector    string          `json:"collector_version,omitempty"`
	Mobilewright string          `json:"mobilewright_version,omitempty"`
}

func (target DesignReviewTarget) Valid() bool {
	switch target.Kind {
	case "web":
		return validHTTPURL(target.OriginalURL) && validHTTPURL(target.ActualURL)
	case "mobile":
		return (target.Platform == "ios" || target.Platform == "android") && strings.TrimSpace(target.AppID) != "" &&
			(target.Orientation == "portrait" || target.Orientation == "landscape") && strings.TrimSpace(target.Collector) != ""
	default:
		return false
	}
}

type EvidenceAsset struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	ContentType string    `json:"content_type"`
	ByteSize    int64     `json:"byte_size"`
	SHA256      string    `json:"sha256"`
	Width       *int      `json:"width,omitempty"`
	Height      *int      `json:"height,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type DesignDifference struct {
	ID                   string          `json:"id"`
	ReviewID             string          `json:"review_id"`
	Ordinal              int             `json:"ordinal"`
	Source               string          `json:"source"`
	AnalysisKind         string          `json:"analysis_kind"`
	CreatedByUserID      *uint           `json:"created_by_user_id,omitempty"`
	Category             string          `json:"category"`
	Severity             string          `json:"severity"`
	Title                string          `json:"title"`
	Description          string          `json:"description"`
	Recommendation       string          `json:"recommendation"`
	Confidence           float64         `json:"confidence"`
	DesignRegion         *Region         `json:"design_region,omitempty"`
	ImplementationRegion *Region         `json:"implementation_region,omitempty"`
	EvidenceBasis        string          `json:"evidence_basis"`
	EvidenceNotes        string          `json:"evidence_notes"`
	VerificationStatus   string          `json:"verification_status"`
	ResolutionStatus     string          `json:"resolution_status"`
	RecheckStatus        *string         `json:"recheck_status,omitempty"`
	HumanEdited          bool            `json:"human_edited"`
	AIOriginal           json.RawMessage `json:"ai_original,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	CanEdit              bool            `json:"can_edit"`
	CanDelete            bool            `json:"can_delete"`
	CanVerify            bool            `json:"can_verify"`
	CanResolve           bool            `json:"can_resolve"`
	CanReopen            bool            `json:"can_reopen"`
}

type ReviewCapabilities struct {
	CanView                bool     `json:"can_view"`
	CanCreate              bool     `json:"can_create"`
	CanManageDifferences   bool     `json:"can_manage_differences"`
	CanManageAnalysis      bool     `json:"can_manage_analysis"`
	CanDelete              bool     `json:"can_delete"`
	CanManageCollectors    bool     `json:"can_manage_collectors"`
	CreationAvailable      bool     `json:"creation_available"`
	FigmaPreviewAvailable  bool     `json:"figma_preview_available"`
	AnalysisAvailable      bool     `json:"analysis_available"`
	ObjectStorageAvailable bool     `json:"object_storage_available"`
	UnavailableReasons     []string `json:"unavailable_reasons"`
	AllowedActions         []string `json:"allowed_actions"`
}

type ReviewRound struct {
	ID          string     `json:"id"`
	RoundNumber int        `json:"round_number"`
	Status      string     `json:"status"`
	Conclusion  *string    `json:"conclusion,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type DesignReviewSummary struct {
	ID                        string             `json:"id"`
	ChainRootID               string             `json:"chain_root_id"`
	SourceReviewID            *string            `json:"source_review_id,omitempty"`
	RoundNumber               int                `json:"round_number"`
	RequirementID             *uint              `json:"requirement_id,omitempty"`
	RequirementTitle          string             `json:"requirement_title"`
	ProjectSetID              uint               `json:"project_set_id"`
	ProjectSetName            string             `json:"project_set_name"`
	PlanUnitID                uint               `json:"plan_unit_id"`
	PlanUnitTitle             string             `json:"plan_unit_title"`
	ImplementationKind        string             `json:"implementation_kind"`
	ImplementationTarget      DesignReviewTarget `json:"implementation_target"`
	FigmaURL                  string             `json:"figma_url"`
	FigmaNodeName             string             `json:"figma_node_name"`
	Status                    string             `json:"status"`
	Conclusion                *string            `json:"conclusion,omitempty"`
	DifferenceCount           int64              `json:"difference_count"`
	UnresolvedDifferenceCount int64              `json:"unresolved_difference_count"`
	CreatedByUserID           uint               `json:"created_by_user_id"`
	CreatedByName             string             `json:"created_by_name"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
	CanDelete                 bool               `json:"can_delete"`
}

type DesignReviewDetail struct {
	DesignReviewSummary
	ViewportWidth                   int                   `json:"viewport_width"`
	ViewportHeight                  int                   `json:"viewport_height"`
	DevicePixelRatio                float64               `json:"device_pixel_ratio"`
	PageZoom                        float64               `json:"page_zoom"`
	DesignWidth                     int                   `json:"design_width"`
	DesignHeight                    int                   `json:"design_height"`
	DimensionsMatch                 bool                  `json:"dimensions_match"`
	DesignStructureAvailable        bool                  `json:"design_structure_available"`
	ImplementationStructureComplete bool                  `json:"implementation_structure_complete"`
	CaptureWarnings                 json.RawMessage       `json:"capture_warnings"`
	AnalysisScope                   AnalysisScope         `json:"analysis_scope"`
	MobileTopbar                    json.RawMessage       `json:"mobile_topbar"`
	DOMNormalization                json.RawMessage       `json:"dom_normalization"`
	AnalysisSummary                 *string               `json:"analysis_summary,omitempty"`
	Model                           *string               `json:"model,omitempty"`
	ReasoningEffort                 *string               `json:"reasoning_effort,omitempty"`
	PromptVersion                   *string               `json:"prompt_version,omitempty"`
	ResponseID                      *string               `json:"response_id,omitempty"`
	AnalysisDurationMS              *int                  `json:"analysis_duration_ms,omitempty"`
	AnalysisAttempts                int                   `json:"analysis_attempts"`
	LastErrorStage                  *string               `json:"last_error_stage,omitempty"`
	LastError                       *string               `json:"last_error,omitempty"`
	Assets                          []EvidenceAsset       `json:"assets"`
	Differences                     []DesignDifference    `json:"differences"`
	Rounds                          []ReviewRound         `json:"rounds"`
	DeepJobs                        []DeepReviewJob       `json:"deep_review_jobs"`
	RecheckVerifications            []RecheckVerification `json:"recheck_verifications"`
	PendingRecheckMatches           []PendingRecheckMatch `json:"pending_recheck_matches"`
	Capabilities                    ReviewCapabilities    `json:"capabilities"`
}

type RequirementOption struct {
	ID             uint   `json:"id"`
	Title          string `json:"title"`
	ProjectSetID   uint   `json:"project_set_id"`
	ProjectSetName string `json:"project_set_name"`
	PlanUnitID     uint   `json:"plan_unit_id"`
	PlanUnitTitle  string `json:"plan_unit_title"`
}

type ReviewListFilter struct {
	Keyword       string
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
	Status        string
	Conclusion    string
	Page          int
	PageSize      int
}

type ReviewPage struct {
	Items    []DesignReviewSummary `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int64                 `json:"total"`
}

type CreateReviewInput struct {
	RequirementID        uint               `json:"requirement_id"`
	SourceReviewID       *string            `json:"source_review_id,omitempty"`
	FigmaURL             string             `json:"figma_url"`
	ImplementationTarget DesignReviewTarget `json:"implementation_target"`
	ViewportWidth        int                `json:"viewport_width"`
	ViewportHeight       int                `json:"viewport_height"`
	DevicePixelRatio     float64            `json:"device_pixel_ratio"`
	PageZoom             float64            `json:"page_zoom"`
	AnalysisScope        AnalysisScope      `json:"analysis_scope"`
	MobileTopbar         json.RawMessage    `json:"mobile_topbar"`
	DOMNormalization     json.RawMessage    `json:"dom_normalization"`
}

type MobileTopbarConfig struct {
	Enabled         bool   `json:"enabled"`
	Platform        string `json:"platform,omitempty"`
	Height          int    `json:"height,omitempty"`
	Time            string `json:"time,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	IconTheme       string `json:"icon_theme,omitempty"`
}

func ParseMobileTopbar(raw json.RawMessage, implementationKind string) (MobileTopbarConfig, error) {
	if len(raw) == 0 {
		return MobileTopbarConfig{}, nil
	}
	var config MobileTopbarConfig
	if json.Unmarshal(raw, &config) != nil {
		return MobileTopbarConfig{}, ErrInvalid
	}
	if !config.Enabled {
		return MobileTopbarConfig{}, nil
	}
	if implementationKind != "web" || config.Platform != "ios" || config.Height != 44 || len(config.Time) > 16 ||
		(config.IconTheme != "light" && config.IconTheme != "dark") || !validHexColor(config.BackgroundColor) {
		return MobileTopbarConfig{}, ErrInvalid
	}
	return config, nil
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(value[1:], 16, 24)
	return err == nil
}

type DifferenceInput struct {
	Category             string  `json:"category"`
	Severity             string  `json:"severity"`
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	Recommendation       string  `json:"recommendation"`
	DesignRegion         *Region `json:"design_region,omitempty"`
	ImplementationRegion *Region `json:"implementation_region,omitempty"`
	EvidenceNotes        string  `json:"evidence_notes"`
	Confidence           float64 `json:"confidence,omitempty"`
}

type AnalyzedDifference struct {
	Input                DifferenceInput
	DesignAnchor         *IdentityAnchor
	ImplementationAnchor *IdentityAnchor
}

func (input DifferenceInput) Valid() bool {
	_, categoryOK := validCategories[input.Category]
	_, severityOK := validSeverities[input.Severity]
	return categoryOK && severityOK && input.Confidence >= 0 && input.Confidence <= 1 && strings.TrimSpace(input.Title) != "" && len([]rune(input.Title)) <= 160 &&
		(input.DesignRegion == nil || input.DesignRegion.Valid()) && (input.ImplementationRegion == nil || input.ImplementationRegion.Valid())
}
