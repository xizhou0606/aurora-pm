package task

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrBatchMutationInvalid     = errors.New("task batch mutation is invalid")
	ErrBatchMutationNotFound    = errors.New("task batch mutation target not found")
	ErrBatchMutationConflict    = errors.New("task batch mutation conflicts with current state")
	ErrBatchMutationAmbiguous   = errors.New("task reference is ambiguous")
	ErrBatchMutationUnavailable = errors.New("task batch mutation dependency is unavailable")
)

type BatchMutationOperation string

const (
	BatchCreate         BatchMutationOperation = "batch_create"
	ChildrenBatchCreate BatchMutationOperation = "children.batch_create"
	BatchDelete         BatchMutationOperation = "batch_delete"
	BatchUpdate         BatchMutationOperation = "batch_update"
)

type Optional[T any] struct {
	Set   bool
	Value *T
}

type BatchTaskInput struct {
	ClientID         string
	RequirementID    uint
	TaskClientID     *uint
	Title            string
	Description      *string
	Status           *string
	Priority         *string
	AssigneeID       *uint
	ReporterUserID   *uint
	EstimatedMinutes *int
	StartsAt         *time.Time
	DueAt            *time.Time
	Children         []BatchTaskInput
}

type BatchTaskPatch struct {
	TaskClientID     Optional[uint]
	Title            Optional[string]
	Description      Optional[string]
	Status           Optional[string]
	Priority         Optional[string]
	AssigneeID       Optional[uint]
	ReporterUserID   Optional[uint]
	EstimatedMinutes Optional[int]
	StartsAt         Optional[time.Time]
	DueAt            Optional[time.Time]
}

type BatchTaskSelector struct {
	IDs           []uint
	TaskRefs      []string
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
}

type BatchMutation struct {
	Operation   BatchMutationOperation
	WorkspaceID uint
	PrincipalID uint
	ApplyKey    string
	ChangeSetID string
	InputDigest string
	ParentID    uint
	Cascade     bool
	Tasks       []BatchTaskInput
	Selector    BatchTaskSelector
	Updates     BatchTaskPatch
}

type BatchCreatedTask struct {
	ClientID string             `json:"client_id"`
	ID       uint               `json:"id"`
	TaskKey  string             `json:"task_key"`
	Children []BatchCreatedTask `json:"children,omitempty"`
}

type BatchMutationResult struct {
	Created                    []BatchCreatedTask `json:"created,omitempty"`
	CreatedCount               int                `json:"created_count,omitempty"`
	ParentID                   uint               `json:"parent_id,omitempty"`
	UpdatedIDs                 []uint             `json:"updated_ids,omitempty"`
	UpdatedCount               int                `json:"updated_count,omitempty"`
	DeletedIDs                 []uint             `json:"deleted_ids,omitempty"`
	DeletedCount               int                `json:"deleted_count,omitempty"`
	AffectedIDs                []uint             `json:"affected_ids,omitempty"`
	AutoCompletedAncestorIDs   []uint             `json:"auto_completed_ancestor_ids,omitempty"`
	DuePropagatedDescendantIDs []uint             `json:"due_propagated_descendant_ids,omitempty"`
	Atomic                     bool               `json:"atomic"`
}

type BatchRequirementScope struct {
	WorkspaceID   uint
	TeamID        uint
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
	ProjectCode   string
}

type BatchScopeResolver interface {
	ResolveTaskRequirementScope(context.Context, uint, uint) (BatchRequirementScope, error)
}

type BatchUserVerifier interface {
	VerifyTaskBatchUsers(context.Context, uint, []uint) error
}

type BatchEffect struct {
	PreviousStatus string
	WorkspaceID    uint
	PrincipalID    uint
	ProjectSetID   uint
	TaskID         uint
	Action         string
	ApplyKey       string
	Payload        map[string]any
}

type BatchEffectWriter interface {
	RecordTaskBatchEffect(context.Context, BatchEffect) error
}

type AssignmentNotification struct {
	WorkspaceID    uint
	PrincipalID    uint
	TaskID         uint
	AssigneeUserID uint
	TaskKey        string
	Title          string
}

type BatchNotificationWriter interface {
	RecordTaskAssignment(context.Context, AssignmentNotification) error
}

type BatchMutationStore interface {
	ApplyTaskBatchMutation(context.Context, BatchMutation) (BatchMutationResult, error)
	ResolveTaskBatchDeletion(context.Context, BatchMutation) ([]uint, error)
}

type BatchMutationService struct {
	store     BatchMutationStore
	deletions TaskDeletionExecutor
}

func NewBatchMutationService(store BatchMutationStore, deletions TaskDeletionExecutor) (*BatchMutationService, error) {
	if store == nil {
		return nil, ErrBatchMutationInvalid
	}
	return &BatchMutationService{store: store, deletions: deletions}, nil
}

func (service *BatchMutationService) ApplyTaskBatchMutation(ctx context.Context, command BatchMutation) (BatchMutationResult, error) {
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 {
		return BatchMutationResult{}, ErrBatchMutationInvalid
	}
	if err := validateBatchMutation(command); err != nil {
		return BatchMutationResult{}, err
	}
	if command.Operation == BatchDelete {
		if service.deletions == nil {
			return BatchMutationResult{}, ErrBatchMutationUnavailable
		}
		ids, err := service.store.ResolveTaskBatchDeletion(ctx, command)
		if err != nil {
			return BatchMutationResult{}, err
		}
		deleted, err := service.deletions.DeleteTasks(ctx, TaskDeletionCommand{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
			ApplyKey: command.ApplyKey, ChangeSetID: command.ChangeSetID, InputDigest: command.InputDigest,
			RootIDs: ids, Cascade: command.Cascade,
		})
		if err != nil {
			if IsTaskDeletionBusinessError(err) {
				return BatchMutationResult{}, ErrBatchMutationConflict
			}
			return BatchMutationResult{}, err
		}
		return BatchMutationResult{
			DeletedIDs: deleted.DeletedIDs, DeletedCount: deleted.DeletedCount,
			AffectedIDs: deleted.DeletedIDs, Atomic: true,
		}, nil
	}
	return service.store.ApplyTaskBatchMutation(ctx, command)
}

func validateBatchMutation(command BatchMutation) error {
	switch command.Operation {
	case BatchCreate:
		if len(command.Tasks) == 0 || len(command.Tasks) > 50 || batchTaskCount(command.Tasks) > 200 {
			return ErrBatchMutationInvalid
		}
	case ChildrenBatchCreate:
		if command.ParentID == 0 || len(command.Tasks) == 0 || len(command.Tasks) > 200 {
			return ErrBatchMutationInvalid
		}
	case BatchDelete:
		if !validBatchSelector(command.Selector) {
			return ErrBatchMutationInvalid
		}
	case BatchUpdate:
		if !validBatchSelector(command.Selector) || command.Updates.empty() {
			return ErrBatchMutationInvalid
		}
	default:
		return ErrBatchMutationInvalid
	}
	return nil
}

func validBatchSelector(selector BatchTaskSelector) bool {
	return len(selector.IDs)+len(selector.TaskRefs) > 0 && len(selector.IDs)+len(selector.TaskRefs) <= 500
}

func batchTaskCount(tasks []BatchTaskInput) int {
	total := len(tasks)
	for _, item := range tasks {
		total += batchTaskCount(item.Children)
	}
	return total
}

func (patch BatchTaskPatch) empty() bool {
	return !patch.TaskClientID.Set && !patch.Title.Set && !patch.Description.Set && !patch.Status.Set && !patch.Priority.Set && !patch.AssigneeID.Set &&
		!patch.ReporterUserID.Set && !patch.EstimatedMinutes.Set && !patch.StartsAt.Set && !patch.DueAt.Set
}

func ValidBatchText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= maximum
}

func IsBatchMutationBusinessError(err error) bool {
	return errors.Is(err, ErrBatchMutationInvalid) || errors.Is(err, ErrBatchMutationNotFound) ||
		errors.Is(err, ErrBatchMutationConflict) || errors.Is(err, ErrBatchMutationAmbiguous) || errors.Is(err, ErrBatchMutationUnavailable)
}
