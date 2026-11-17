package task

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrCoreMutationInvalid  = errors.New("task core mutation is invalid")
	ErrCoreMutationNotFound = errors.New("task core mutation target not found")
	ErrCoreMutationConflict = errors.New("task core mutation conflicts with current state")
)

type CoreMutationOperation string

const (
	CoreCreate CoreMutationOperation = "task.create"
	CoreUpdate CoreMutationOperation = "task.update"
	CoreClone  CoreMutationOperation = "task.clone"
)

type CoreMutationValues struct {
	RequirementID    uint
	TaskClientID     *uint
	Kind             string
	Title            string
	Description      string
	Status           string
	Priority         string
	AssigneeID       *uint
	ReporterUserID   *uint
	ParentID         *uint
	EstimatedMinutes *int
	StartsAt         *time.Time
	DueAt            *time.Time
	Fields           map[string]bool
}

type CoreMutation struct {
	Operation           CoreMutationOperation
	WorkspaceID         uint
	PrincipalID         uint
	ApplyKey            string
	ChangeSetID         string
	InputDigest         string
	ID                  uint
	ExpectedUpdatedAt   time.Time
	TitleSuffix         string
	TitleSuffixSet      bool
	TargetRequirementID uint
	Values              CoreMutationValues
}

type CoreMutationResult struct {
	Task       *Snapshot  `json:"task,omitempty"`
	ClonedFrom *Reference `json:"cloned_from,omitempty"`
}

type CoreMutationStore interface {
	ApplyCoreMutation(context.Context, CoreMutation) (CoreMutationResult, error)
}

type CoreMutationService struct{ store CoreMutationStore }

func NewCoreMutationService(store CoreMutationStore) (*CoreMutationService, error) {
	if store == nil {
		return nil, ErrCoreMutationInvalid
	}
	return &CoreMutationService{store: store}, nil
}

func (service *CoreMutationService) ApplyCoreMutation(ctx context.Context, command CoreMutation) (CoreMutationResult, error) {
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 {
		return CoreMutationResult{}, ErrCoreMutationInvalid
	}
	command.Values = normalizeCoreValues(command.Values)
	switch command.Operation {
	case CoreCreate:
		if err := validateCoreCreate(command.Values); err != nil {
			return CoreMutationResult{}, err
		}
	case CoreUpdate:
		if command.ID == 0 || command.ExpectedUpdatedAt.IsZero() || len(command.Values.Fields) == 0 {
			return CoreMutationResult{}, ErrCoreMutationInvalid
		}
		if err := validateCoreUpdate(command.Values); err != nil {
			return CoreMutationResult{}, err
		}
	case CoreClone:
		command.TitleSuffix = strings.TrimSpace(command.TitleSuffix)
		if command.ID == 0 || command.ExpectedUpdatedAt.IsZero() || utf8.RuneCountInString(command.TitleSuffix) > 160 {
			return CoreMutationResult{}, ErrCoreMutationInvalid
		}
	default:
		return CoreMutationResult{}, ErrCoreMutationInvalid
	}
	return service.store.ApplyCoreMutation(ctx, command)
}

func normalizeCoreValues(values CoreMutationValues) CoreMutationValues {
	values.Kind = NormalizePMWorkTaskKind(values.Kind)
	values.Title = strings.TrimSpace(values.Title)
	values.Description = strings.TrimSpace(values.Description)
	values.Status = strings.ToLower(strings.TrimSpace(values.Status))
	values.Priority = strings.ToLower(strings.TrimSpace(values.Priority))
	if values.Priority == "medium" {
		values.Priority = "normal"
	}
	return values
}

func validateCoreCreate(values CoreMutationValues) error {
	if values.RequirementID == 0 || values.Title == "" || utf8.RuneCountInString(values.Title) > 160 {
		return ErrCoreMutationInvalid
	}
	if values.Kind == "" {
		values.Kind = WorkTaskKindTask
	}
	return validateCoreValues(values, false)
}

func validateCoreUpdate(values CoreMutationValues) error {
	if values.Fields["title"] && (values.Title == "" || utf8.RuneCountInString(values.Title) > 160) {
		return ErrCoreMutationInvalid
	}
	if values.Fields["requirement_id"] && values.RequirementID == 0 {
		return ErrCoreMutationInvalid
	}
	return validateCoreValues(values, true)
}

func validateCoreValues(values CoreMutationValues, update bool) error {
	check := func(field string) bool { return !update || values.Fields[field] }
	if check("kind") && values.Kind != "" && !IsPMWorkTaskKind(values.Kind) {
		return ErrCoreMutationInvalid
	}
	if check("status") && values.Status != "" && !coreValueAllowed(values.Status, "draft", "in_dev", "pending_test", "completed", "cancelled") {
		return ErrCoreMutationInvalid
	}
	if check("priority") && values.Priority != "" && !coreValueAllowed(values.Priority, "low", "normal", "high", "urgent") {
		return ErrCoreMutationInvalid
	}
	if values.TaskClientID != nil && *values.TaskClientID == 0 || values.AssigneeID != nil && *values.AssigneeID == 0 || values.ReporterUserID != nil && *values.ReporterUserID == 0 ||
		values.ParentID != nil && *values.ParentID == 0 || values.EstimatedMinutes != nil && *values.EstimatedMinutes < 0 {
		return ErrCoreMutationInvalid
	}
	if values.StartsAt != nil && values.DueAt != nil && values.StartsAt.After(*values.DueAt) {
		return ErrCoreMutationInvalid
	}
	return nil
}

func coreValueAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func IsCoreMutationBusinessError(err error) bool {
	return errors.Is(err, ErrCoreMutationInvalid) || errors.Is(err, ErrCoreMutationNotFound) || errors.Is(err, ErrCoreMutationConflict)
}
