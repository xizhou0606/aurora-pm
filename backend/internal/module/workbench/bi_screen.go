package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

var ErrBIScreenQueryInvalid = errors.New("BI screen query is invalid")

type BIScreenQuery struct {
	WorkspaceID      uint
	WorkspaceName    string
	ProjectSetIDs    []uint
	PlanUnitIDs      []uint
	RequirementIDs   []uint
	Period           string
	TeamDimension    string
	TeamDimensionKey string
	Location         *time.Location
	Now              time.Time
	// AccessibleProjectSetIDs restricts the aggregation to the caller's
	// accessible project sets. The HTTP adapter resolves it from the project
	// access service; nil means no restriction was resolved (the admin fast
	// path). An empty non-nil slice means the caller can access no project
	// and the screen aggregates nothing.
	AccessibleProjectSetIDs []uint
}

type BIScreenResult struct {
	Period                      string           `json:"period"`
	WorkspaceID                 uint             `json:"workspace_id"`
	WorkspaceName               string           `json:"workspace_name"`
	TeamCount                   int              `json:"team_count"`
	Scope                       map[string]any   `json:"scope"`
	OverviewMetrics             map[string]any   `json:"overview_metrics"`
	UrgencyDistribution         []map[string]any `json:"urgency_distribution"`
	DueDistribution             []map[string]any `json:"due_distribution"`
	PriorityDistribution        []map[string]any `json:"priority_distribution"`
	TypeDistribution            []map[string]any `json:"type_distribution"`
	StatusProgress              []map[string]any `json:"status_progress"`
	CompletionTrend             []map[string]any `json:"completion_trend"`
	VersionProgress             []map[string]any `json:"version_progress"`
	RequirementProgress         []map[string]any `json:"requirement_progress"`
	VersionRankings             []map[string]any `json:"version_rankings"`
	OwnerBoard                  []map[string]any `json:"owner_board"`
	MilestoneOptions            []any            `json:"milestone_options"`
	QualitySeverityDistribution []map[string]any `json:"quality_severity_distribution"`
	QualityStatusDistribution   []map[string]any `json:"quality_status_distribution"`
	QualityTrend                []map[string]any `json:"quality_trend"`
	VersionQualityRisks         []map[string]any `json:"version_quality_risks"`
	QualityOwnerLoad            []map[string]any `json:"quality_owner_load"`
	TeamData                    any              `json:"team_data"`
	GeneratedAt                 time.Time        `json:"generated_at"`
	Compatibility               map[string]any   `json:"-"`
}

func (result BIScreenResult) MarshalJSON() ([]byte, error) {
	type plain BIScreenResult
	payload, err := json.Marshal(plain(result))
	if err != nil {
		return nil, err
	}
	var merged map[string]any
	if err := json.Unmarshal(payload, &merged); err != nil {
		return nil, err
	}
	for key, value := range result.Compatibility {
		if _, exists := merged[key]; exists {
			return nil, fmt.Errorf("BI compatibility field %q collides with canonical result", key)
		}
		merged[key] = value
	}
	return json.Marshal(merged)
}

type BIScreenBackend interface {
	BuildBIScreen(context.Context, BIScreenQuery) (BIScreenResult, error)
}

type BITaskQueries interface {
	ListBIFacts(context.Context, uint) ([]task.Snapshot, []task.ActivitySnapshot, error)
}

type BIQualityQueries interface {
	ListBIFacts(context.Context, uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error)
}

type BIProjectQueries interface {
	LoadBIContext(context.Context, uint) (project.BIContext, error)
}

type BIRequirementQueries interface {
	ListBIFacts(context.Context, uint) ([]requirement.Snapshot, error)
}

type BIIdentityQueries interface {
	GetBIWorkspace(context.Context, uint) (identity.BIWorkspace, error)
	FindActiveBIUsersByFactIDs(context.Context, []uint) (map[uint]identity.BIUser, error)
}

type BIConfigurationQueries interface {
	GetBITeamMetricSettings(context.Context, uint) (BITeamMetricSettingsSnapshot, error)
	GetBITeamScoreProfile(context.Context, uint) (BITeamScoreProfileSnapshot, error)
}

type BIWorkflowQueries interface {
	ListBIReviewDecisions(context.Context, uint, uint, string, []uint) ([]workflow.BIReviewDecision, error)
	ListBIMetricWorkflowOptions(context.Context, uint) ([]workflow.BIMetricWorkflowOption, error)
}

type BIScreenReadDependencies struct {
	Tasks         BITaskQueries
	Quality       BIQualityQueries
	Projects      BIProjectQueries
	Requirements  BIRequirementQueries
	Identities    BIIdentityQueries
	Configuration BIConfigurationQueries
	Workflows     BIWorkflowQueries
}

func (dependencies BIScreenReadDependencies) Validate() error {
	if dependencies.Tasks == nil || dependencies.Quality == nil || dependencies.Projects == nil ||
		dependencies.Requirements == nil || dependencies.Identities == nil || dependencies.Configuration == nil ||
		dependencies.Workflows == nil {
		return ErrBIScreenQueryInvalid
	}
	return nil
}

type BIScreenService struct{ backend BIScreenBackend }

func NewBIScreenService(backend BIScreenBackend) (*BIScreenService, error) {
	if backend == nil {
		return nil, ErrBIScreenQueryInvalid
	}
	return &BIScreenService{backend: backend}, nil
}

func (service *BIScreenService) Get(ctx context.Context, query BIScreenQuery) (BIScreenResult, error) {
	query.Period = strings.ToLower(strings.TrimSpace(query.Period))
	if query.Period == "" {
		query.Period = "week"
	}
	if query.WorkspaceID == 0 || (query.Period != "week" && query.Period != "month") ||
		!validBIIDs(query.ProjectSetIDs) || !validBIIDs(query.PlanUnitIDs) || !validBIIDs(query.RequirementIDs) {
		return BIScreenResult{}, ErrBIScreenQueryInvalid
	}
	if query.Location == nil {
		query.Location = time.UTC
	}
	if query.Now.IsZero() {
		query.Now = time.Now().UTC()
	} else {
		query.Now = query.Now.UTC()
	}
	return service.backend.BuildBIScreen(ctx, query)
}

func validBIIDs(values []uint) bool {
	seen := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
