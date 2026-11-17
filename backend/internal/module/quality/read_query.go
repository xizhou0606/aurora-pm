package quality

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrBugReadInvalid  = errors.New("quality bug read query is invalid")
	ErrBugReadNotFound = errors.New("quality bug not found")
)

type BugReadFilter struct {
	ProjectSetID      uint
	PlanUnitID        uint
	RequirementID     uint
	Query             string
	Statuses          []string
	Priorities        []string
	SeverityKeys      []string
	FoundEnvironments []string
	FoundVersion      string
	BugTypes          []string
	DefectDomains     []string
	CurrentOwnerID    uint
	AssigneeID        uint
	ReporterUserID    uint
	Limit             int
	Offset            int
}

type BugSnapshot struct {
	ReproductionMaterials []Material `json:"reproduction_materials"`
	ResolutionMaterials   []Material `json:"resolution_materials"`
	ID                    uint       `json:"id"`
	WorkspaceID           uint       `json:"workspace_id"`
	TeamID                uint       `json:"team_id"`
	ProjectSetID          uint       `json:"project_set_id"`
	PlanUnitID            uint       `json:"project_new_id"`
	RequirementID         uint       `json:"requirement_id"`
	BugKey                string     `json:"bug_key"`
	Title                 string     `json:"title"`
	Status                string     `json:"status"`
	Priority              string     `json:"priority"`
	BugType               string     `json:"bug_type,omitempty"`
	DefectDomain          string     `json:"defect_domain"`
	SeverityKey           string     `json:"severity_key,omitempty"`
	DueAt                 *time.Time `json:"due_at,omitempty"`
	FoundEnvironment      string     `json:"found_environment,omitempty"`
	OperatingSystem       string     `json:"operating_system,omitempty"`
	Browser               string     `json:"browser,omitempty"`
	FoundVersion          string     `json:"found_version,omitempty"`
	ReproductionSteps     string     `json:"reproduction_steps,omitempty"`
	ExpectedResult        string     `json:"expected_result,omitempty"`
	ActualResult          string     `json:"actual_result,omitempty"`
	ResolutionReason      string     `json:"resolution_reason,omitempty"`
	ResolutionNote        string     `json:"resolution_note,omitempty"`
	AssigneeUserID        *uint      `json:"assignee_user_id,omitempty"`
	ReporterUserID        *uint      `json:"reporter_user_id,omitempty"`
	ReviewerUserID        *uint      `json:"reviewer_user_id,omitempty"`
	SourceExecutionID     *uint      `json:"source_execution_id,omitempty"`
	SourceTestID          *uint      `json:"source_test_id,omitempty"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	ReviewedAt            *time.Time `json:"reviewed_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CanceledAt            *time.Time `json:"canceled_at,omitempty"`
	StatusChangedAt       time.Time  `json:"status_changed_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type BugActivitySnapshot struct {
	ID           uint      `json:"id"`
	WorkspaceID  uint      `json:"workspace_id"`
	QualityBugID uint      `json:"quality_bug_id"`
	UserID       uint      `json:"user_id"`
	Action       string    `json:"action"`
	FieldKey     string    `json:"field_key,omitempty"`
	OldValue     string    `json:"old_value,omitempty"`
	NewValue     string    `json:"new_value,omitempty"`
	Summary      string    `json:"summary,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type BugReadPage struct {
	Items []BugSnapshot `json:"items"`
	Total int64         `json:"total"`
}

type BugActivityPage struct {
	Items []BugActivitySnapshot `json:"items"`
	Total int64                 `json:"total"`
}

type BugReadStore interface {
	ListBugs(context.Context, uint, BugReadFilter) (BugReadPage, error)
	GetBug(context.Context, uint, uint) (BugSnapshot, error)
	ListBugActivities(context.Context, uint, uint, int, int) (BugActivityPage, error)
}

type BugReadService struct{ store BugReadStore }

func NewBugReadService(store BugReadStore) (*BugReadService, error) {
	if store == nil {
		return nil, ErrBugReadInvalid
	}
	return &BugReadService{store: store}, nil
}

func (service *BugReadService) ListBugs(ctx context.Context, workspaceID uint, filter BugReadFilter) (BugReadPage, error) {
	if workspaceID == 0 || filter.Offset < 0 {
		return BugReadPage{}, ErrBugReadInvalid
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.FoundVersion = strings.ToLower(strings.TrimSpace(filter.FoundVersion))
	filter.Statuses = normalizedBugReadValues(filter.Statuses)
	filter.Priorities = normalizedBugReadValues(filter.Priorities)
	filter.SeverityKeys = normalizedBugReadValues(filter.SeverityKeys)
	filter.FoundEnvironments = normalizedBugReadValues(filter.FoundEnvironments)
	filter.BugTypes = normalizedBugReadValues(filter.BugTypes)
	filter.DefectDomains = normalizedBugReadValues(filter.DefectDomains)
	for _, value := range filter.DefectDomains {
		if !ValidBugDefectDomain(value) {
			return BugReadPage{}, ErrBugReadInvalid
		}
	}
	filter.Limit = normalizedBugReadLimit(filter.Limit)
	page, err := service.store.ListBugs(ctx, workspaceID, filter)
	if err != nil {
		return BugReadPage{}, err
	}
	for index := range page.Items {
		page.Items[index] = normalizeBugSnapshot(page.Items[index])
	}
	return page, nil
}

func (service *BugReadService) GetBug(ctx context.Context, workspaceID, bugID uint) (BugSnapshot, error) {
	if workspaceID == 0 || bugID == 0 {
		return BugSnapshot{}, ErrBugReadInvalid
	}
	bug, err := service.store.GetBug(ctx, workspaceID, bugID)
	if err != nil {
		return BugSnapshot{}, err
	}
	return normalizeBugSnapshot(bug), nil
}

func (service *BugReadService) ListBugActivities(ctx context.Context, workspaceID, bugID uint, limit, offset int) (BugActivityPage, error) {
	if workspaceID == 0 || bugID == 0 || offset < 0 {
		return BugActivityPage{}, ErrBugReadInvalid
	}
	return service.store.ListBugActivities(ctx, workspaceID, bugID, normalizedBugReadLimit(limit), offset)
}

func normalizedBugReadLimit(value int) int {
	if value <= 0 {
		return 50
	}
	if value > 500 {
		return 500
	}
	return value
}

func normalizedBugReadValues(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.ToLower(strings.TrimSpace(part))
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func normalizeBugSnapshot(bug BugSnapshot) BugSnapshot {
	bug.DefectDomain = NormalizeBugDefectDomain(bug.DefectDomain)
	bug.ResolutionReason = NormalizeBugResolutionReason(bug.ResolutionReason)
	return bug
}
