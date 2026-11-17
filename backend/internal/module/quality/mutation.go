package quality

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrBugMutationInvalid  = errors.New("quality bug mutation is invalid")
	ErrBugMutationNotFound = errors.New("quality bug mutation target not found")
	ErrBugMutationConflict = errors.New("quality bug mutation conflicts with current state")
)

type BugMutationOperation string

const (
	BugCreate      BugMutationOperation = "bug.create"
	BugUpdate      BugMutationOperation = "bug.update"
	BugDelete      BugMutationOperation = "bug.delete"
	BugBatchCreate BugMutationOperation = "bug.batch_create"
	BugBatchAssign BugMutationOperation = "bug.batch_assign"
	BugBatchDelete BugMutationOperation = "bug.batch_delete"
)

type BugMutationValues struct {
	AssignmentNote        string
	AssignmentMaterials   []Material
	RegressionNote        string
	ActivityNote          string
	RegressionMaterials   []Material
	ReproductionMaterials []Material
	ResolutionMaterials   []Material
	RequirementID         uint
	Title                 string
	Status                string
	Priority              string
	BugType               string
	DefectDomain          string
	SeverityKey           string
	DueAt                 *time.Time
	FoundEnvironment      string
	OperatingSystem       string
	Browser               string
	FoundVersion          string
	ReproductionSteps     string
	ExpectedResult        string
	ActualResult          string
	ResolutionReason      string
	ResolutionNote        string
	AssigneeUserID        *uint
	ReviewerUserID        *uint
	SourceExecutionID     *uint
	SourceTestID          *uint
	Fields                map[string]bool
}

type BatchBugMutationItem struct {
	ClientID string
	Values   BugMutationValues
}

type BugMutation struct {
	StatusTransition bool
	Operation        BugMutationOperation
	WorkspaceID      uint
	PrincipalID      uint
	ApplyKey         string
	ChangeSetID      string
	InputDigest      string
	ID               uint
	IDs              []uint
	Values           BugMutationValues
	Items            []BatchBugMutationItem
}

type BatchBugMutationResult struct {
	ClientID string      `json:"client_id"`
	Bug      BugSnapshot `json:"bug"`
}

type BugMutationResult struct {
	Bug          *BugSnapshot             `json:"bug,omitempty"`
	DeletedID    uint                     `json:"deleted_id,omitempty"`
	Deleted      bool                     `json:"deleted,omitempty"`
	Created      []BatchBugMutationResult `json:"created,omitempty"`
	Updated      []BugSnapshot            `json:"updated,omitempty"`
	DeletedIDs   []uint                   `json:"deleted_ids,omitempty"`
	CreatedCount int                      `json:"created_count,omitempty"`
	UpdatedCount int                      `json:"updated_count,omitempty"`
	DeletedCount int                      `json:"deleted_count,omitempty"`
	Atomic       bool                     `json:"atomic,omitempty"`
}

type BugMutationEffect struct {
	Operation   BugMutationOperation
	WorkspaceID uint
	PrincipalID uint
	Before      *BugSnapshot
	After       BugSnapshot
}

type BugMutationEffectSink interface {
	RecordBugMutation(context.Context, BugMutationEffect) error
}

type BugMutationStore interface {
	ApplyBugMutation(context.Context, BugMutation) (BugMutationResult, error)
}

type BugMutationService struct{ store BugMutationStore }

func NewBugMutationService(store BugMutationStore) (*BugMutationService, error) {
	if store == nil {
		return nil, ErrBugMutationInvalid
	}
	return &BugMutationService{store: store}, nil
}

func (service *BugMutationService) ApplyBugMutation(ctx context.Context, command BugMutation) (BugMutationResult, error) {
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 {
		return BugMutationResult{}, ErrBugMutationInvalid
	}
	switch command.Operation {
	case BugCreate:
		command.Values = normalizeBugValues(command.Values, false)
		if err := validateCreateBug(command.Values); err != nil {
			return BugMutationResult{}, err
		}
	case BugUpdate:
		if command.ID == 0 || len(command.Values.Fields) == 0 {
			return BugMutationResult{}, ErrBugMutationInvalid
		}
		command.Values = normalizeBugValues(command.Values, true)
		if err := validateUpdateBug(command.Values); err != nil {
			return BugMutationResult{}, err
		}
	case BugDelete:
		if command.ID == 0 {
			return BugMutationResult{}, ErrBugMutationInvalid
		}
	case BugBatchCreate:
		if len(command.Items) == 0 || len(command.Items) > 100 {
			return BugMutationResult{}, ErrBugMutationInvalid
		}
		seen := make(map[string]struct{}, len(command.Items))
		for index := range command.Items {
			command.Items[index].ClientID = strings.TrimSpace(command.Items[index].ClientID)
			command.Items[index].Values = normalizeBugValues(command.Items[index].Values, false)
			if command.Items[index].ClientID == "" || len(command.Items[index].ClientID) > 100 {
				return BugMutationResult{}, ErrBugMutationInvalid
			}
			if _, duplicate := seen[command.Items[index].ClientID]; duplicate {
				return BugMutationResult{}, ErrBugMutationInvalid
			}
			seen[command.Items[index].ClientID] = struct{}{}
			if err := validateCreateBug(command.Items[index].Values); err != nil {
				return BugMutationResult{}, err
			}
		}
	case BugBatchDelete:
		if len(command.IDs) == 0 || len(command.IDs) > 100 {
			return BugMutationResult{}, ErrBugMutationInvalid
		}
		seen := make(map[uint]struct{}, len(command.IDs))
		for _, id := range command.IDs {
			if id == 0 {
				return BugMutationResult{}, ErrBugMutationInvalid
			}
			if _, duplicate := seen[id]; duplicate {
				return BugMutationResult{}, ErrBugMutationInvalid
			}
			seen[id] = struct{}{}
		}
	case BugBatchAssign:
		if err := validateBatchBugIDs(command.IDs); err != nil ||
			len(command.Values.Fields) != 1 || !command.Values.Fields["assignee_user_id"] ||
			command.Values.AssigneeUserID == nil || *command.Values.AssigneeUserID == 0 {
			return BugMutationResult{}, ErrBugMutationInvalid
		}
		sort.Slice(command.IDs, func(i, j int) bool { return command.IDs[i] < command.IDs[j] })
		command.Values = normalizeBugValues(command.Values, true)
	default:
		return BugMutationResult{}, ErrBugMutationInvalid
	}
	result, err := service.store.ApplyBugMutation(ctx, command)
	if err != nil {
		return BugMutationResult{}, err
	}
	if result.Bug != nil {
		normalized := normalizeBugSnapshot(*result.Bug)
		result.Bug = &normalized
	}
	for index := range result.Created {
		result.Created[index].Bug = normalizeBugSnapshot(result.Created[index].Bug)
	}
	for index := range result.Updated {
		result.Updated[index] = normalizeBugSnapshot(result.Updated[index])
	}
	return result, nil
}

func validateBatchBugIDs(ids []uint) error {
	if len(ids) == 0 || len(ids) > 100 {
		return ErrBugMutationInvalid
	}
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return ErrBugMutationInvalid
		}
		if _, duplicate := seen[id]; duplicate {
			return ErrBugMutationInvalid
		}
		seen[id] = struct{}{}
	}
	return nil
}

func normalizeBugValues(values BugMutationValues, update bool) BugMutationValues {
	values.Title = strings.TrimSpace(values.Title)
	values.Status = strings.ToLower(strings.TrimSpace(values.Status))
	values.Priority = strings.ToLower(strings.TrimSpace(values.Priority))
	values.BugType = strings.TrimSpace(values.BugType)
	values.DefectDomain = strings.ToLower(strings.TrimSpace(values.DefectDomain))
	values.SeverityKey = strings.ToLower(strings.TrimSpace(values.SeverityKey))
	values.FoundEnvironment = strings.TrimSpace(values.FoundEnvironment)
	values.OperatingSystem = strings.TrimSpace(values.OperatingSystem)
	values.Browser = strings.TrimSpace(values.Browser)
	values.FoundVersion = strings.TrimSpace(values.FoundVersion)
	values.ReproductionSteps = strings.TrimSpace(values.ReproductionSteps)
	values.ExpectedResult = strings.TrimSpace(values.ExpectedResult)
	values.ActualResult = strings.TrimSpace(values.ActualResult)
	values.ResolutionReason = strings.ToLower(strings.TrimSpace(values.ResolutionReason))
	values.ResolutionNote = strings.TrimSpace(values.ResolutionNote)
	if !update {
		if values.Status == "" {
			values.Status = "triage"
		}
		if values.Priority == "" {
			values.Priority = "medium"
		}
		if values.SeverityKey == "" {
			values.SeverityKey = "p2"
		}
		if values.DefectDomain == "" {
			values.DefectDomain = BugDefectDomainFunctional
		}
		if values.FoundEnvironment == "" {
			values.FoundEnvironment = DefaultEnvironmentName
		}
	}
	return values
}

func validateCreateBug(values BugMutationValues) error {
	if values.RequirementID == 0 || values.Title == "" || utf8.RuneCountInString(values.Title) > 160 {
		return ErrBugMutationInvalid
	}
	return validateBugFields(values, false)
}

func validateUpdateBug(values BugMutationValues) error {
	if values.Fields["title"] && (values.Title == "" || utf8.RuneCountInString(values.Title) > 160) {
		return ErrBugMutationInvalid
	}
	if values.Fields["requirement_id"] && values.RequirementID == 0 {
		return ErrBugMutationInvalid
	}
	return validateBugFields(values, true)
}

func validateBugFields(values BugMutationValues, update bool) error {
	check := func(field string) bool { return !update || values.Fields[field] }
	if (values.Fields["regression_note"] || values.Fields["regression_materials"]) &&
		(!values.Fields["status"] || (values.Status != "reopened" && values.Status != "in_progress")) {
		return ErrBugMutationInvalid
	}
	if values.Fields["activity_note"] && !values.Fields["status"] {
		return ErrBugMutationInvalid
	}
	if ValidateMaterials(values.AssignmentMaterials) != nil || ValidateMaterials(values.RegressionMaterials) != nil || ValidateMaterials(values.ReproductionMaterials) != nil || ValidateMaterials(values.ResolutionMaterials) != nil {
		return ErrBugMutationInvalid
	}
	if check("status") && !bugMutationValueAllowed(values.Status, "triage", "todo", "in_progress", "reopened", "review", "done", "canceled", "duplicate") {
		return ErrBugMutationInvalid
	}
	if check("priority") && !bugMutationValueAllowed(values.Priority, "low", "medium", "high", "urgent") {
		return ErrBugMutationInvalid
	}
	if check("severity_key") && !bugMutationValueAllowed(values.SeverityKey, "p0", "p1", "p2", "p3") {
		return ErrBugMutationInvalid
	}
	if check("defect_domain") && !ValidBugDefectDomain(values.DefectDomain) {
		return ErrBugMutationInvalid
	}
	if check("resolution_reason") && !ValidBugResolutionReason(values.ResolutionReason) {
		return ErrBugMutationInvalid
	}
	if utf8.RuneCountInString(values.AssignmentNote) > 1000 || utf8.RuneCountInString(values.RegressionNote) > 1000 || utf8.RuneCountInString(values.ActivityNote) > 1000 || utf8.RuneCountInString(values.ResolutionNote) > 1000 {
		return ErrBugMutationInvalid
	}
	if !update && (values.Status == "review" || values.Status == "done") && strings.TrimSpace(values.ResolutionReason) == "" {
		return ErrBugMutationInvalid
	}
	return nil
}

func bugMutationValueAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
