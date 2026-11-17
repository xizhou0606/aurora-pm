package requirement

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrMutationInvalid  = errors.New("requirement mutation is invalid")
	ErrMutationConflict = errors.New("requirement mutation conflicts with current state")
	ErrMutationNotFound = errors.New("requirement mutation target not found")
	// ErrRequirementsLocked rejects brand-new requirements only: the target plan
	// unit confirmed requirement-intake completion. Moving or rescheduling an
	// existing requirement stays allowed and must never return this error.
	ErrRequirementsLocked = errors.New("target plan unit requirements are locked")
)

type MutationOperation string

const (
	MutationCreate    MutationOperation = "requirement.create"
	MutationUpdate    MutationOperation = "requirement.update"
	MutationDelete    MutationOperation = "requirement.delete"
	MutationBatchMove MutationOperation = "requirement.batch_move"
)

type BatchMoveItem struct {
	RequirementID   uint `json:"requirement_id"`
	ExpectedVersion uint `json:"expected_version"`
}
type Mutation struct {
	Operation                                                                                    MutationOperation
	WorkspaceID, PrincipalID                                                                     uint
	ApplyKey, ChangeSetID, InputDigest                                                           string
	ID, ExpectedVersion, PlanUnitID, WorkflowTemplateID                                          uint
	Title, Description, ProductDocURL, DesignURL, AcceptanceCriteria, Priority                   string
	TitleSet, DescriptionSet, ProductDocURLSet, DesignURLSet, AcceptanceCriteriaSet, PrioritySet bool
	ProductOwnerUserID, DesignOwnerUserID                                                        *uint
	ProductOwnerSet, DesignOwnerSet, ProductOwnerUserIDsSet, DesignOwnerUserIDsSet               bool
	FrontendOwnerUserIDsSet, BackendOwnerUserIDsSet, TesterUserIDsSet, DeveloperUserIDsSet       bool
	ProductOwnerUserIDs, DesignOwnerUserIDs, FrontendOwnerUserIDs, BackendOwnerUserIDs           []uint
	TesterUserIDs, DeveloperUserIDs                                                              []uint
	Items                                                                                        []BatchMoveItem
	ConfirmSubjectSchedule                                                                       bool
}
type MutationResult struct {
	ID                 uint   `json:"id"`
	WorkspaceID        uint   `json:"workspace_id"`
	TeamID             uint   `json:"team_id"`
	ProjectSetID       uint   `json:"project_new_id"`
	PlanUnitID         uint   `json:"version_task_new_id"`
	Version            uint   `json:"version"`
	WorkflowInstanceID *uint  `json:"workflow_instance_id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	ProductDocURL      string `json:"product_doc_url"`
	DesignURL          string `json:"design_url"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	Priority           string `json:"priority"`
	Status             string `json:"status"`
	ProductOwnerUserID *uint  `json:"product_owner_user_id"`
	DesignOwnerUserID  *uint  `json:"design_owner_user_id"`
	DeveloperUserIDs   []uint `json:"developer_user_ids"`
	Deleted            bool   `json:"deleted"`
}
type BatchMoveResult struct {
	Items                []MutationResult `json:"items"`
	MovedCount           int              `json:"moved_count"`
	AffectedTaskCount    int64            `json:"affected_task_count"`
	AffectedQualityCount int64            `json:"affected_quality_bug_count"`
}
type ApplyResult struct {
	Requirement *MutationResult  `json:"requirement,omitempty"`
	Batch       *BatchMoveResult `json:"batch,omitempty"`
}
type PlanScope struct {
	WorkspaceID, TeamID, ProjectSetID, PlanUnitID uint
	Type                                          string
	// RequirementsLockedAt is the requirement-intake lock fact read under the
	// plan unit row lock; only creation treats it as a hard block.
	RequirementsLockedAt         *time.Time
	PlannedStartAt, PlannedEndAt *time.Time
}
type PlanScopeResolver interface {
	ResolveRequirementPlan(context.Context, uint, uint) (PlanScope, error)
}
type WorkflowLifecycle interface {
	StartRequirementWorkflow(context.Context, Mutation, MutationResult) (uint, string, error)
	ValidateRequirementSchedule(context.Context, uint, uint, uint, bool) error
	RequirementHasWorkflowHistory(context.Context, uint, uint) (bool, error)
	DeleteRequirementWorkflow(context.Context, uint, uint) error
}
type DependentLifecycle interface {
	MoveRequirementDependents(context.Context, uint, uint, PlanScope) (int64, int64, error)
	ValidateRequirementDeletion(context.Context, uint, uint) error
	DeleteRequirementDependents(context.Context, uint, uint) (int64, int64, error)
}
type MutationEffect struct {
	WorkspaceID, PrincipalID, ProjectSetID, RequirementID uint
	Action, ApplyKey                                      string
	// AffectedPlanUnitIDs lists the plan units whose current requirement set
	// changed with this mutation (creation, both move ends, deletion). The
	// effects adapter turns them into delivery reconcile events.
	AffectedPlanUnitIDs []uint
}

// DeliveryScopeBumper increments the plan unit's delivery scope revision
// inside the caller's transaction. Implemented over the project module store
// by the application composition root; optional for older wiring.
type DeliveryScopeBumper interface {
	BumpDeliveryScopeRevision(ctx context.Context, workspaceID, planUnitID uint) error
}
type EffectWriter interface {
	RecordRequirementMutation(context.Context, MutationEffect) error
}
type CoverLifecycle interface {
	EnqueueRequirementCover(context.Context, MutationResult) error
}
type MutationStore interface {
	ApplyRequirementMutation(context.Context, Mutation) (ApplyResult, error)
}
type UserValidator interface {
	ValidateRequirementUsers(context.Context, uint, []uint) (bool, error)
}
type MutationService struct {
	store MutationStore
	users UserValidator
}

func NewMutationService(store MutationStore, users UserValidator) (*MutationService, error) {
	if store == nil || users == nil {
		return nil, ErrMutationInvalid
	}
	return &MutationService{store: store, users: users}, nil
}
func (service *MutationService) ApplyRequirementMutation(ctx context.Context, command Mutation) (ApplyResult, error) {
	command.Title, command.Description, command.ProductDocURL, command.DesignURL = strings.TrimSpace(command.Title), strings.TrimSpace(command.Description), strings.TrimSpace(command.ProductDocURL), strings.TrimSpace(command.DesignURL)
	command.AcceptanceCriteria, command.Priority = strings.TrimSpace(command.AcceptanceCriteria), strings.ToLower(strings.TrimSpace(command.Priority))
	if service == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 || command.ApplyKey == "" || command.ChangeSetID == "" || len(command.InputDigest) != 64 {
		return ApplyResult{}, ErrMutationInvalid
	}
	users := []uint{}
	if command.ProductOwnerUserID != nil {
		users = append(users, *command.ProductOwnerUserID)
	}
	if command.DesignOwnerUserID != nil {
		users = append(users, *command.DesignOwnerUserID)
	}
	users = append(users, command.DeveloperUserIDs...)
	users = append(users, command.ProductOwnerUserIDs...)
	users = append(users, command.DesignOwnerUserIDs...)
	users = append(users, command.FrontendOwnerUserIDs...)
	users = append(users, command.BackendOwnerUserIDs...)
	users = append(users, command.TesterUserIDs...)
	for _, id := range users {
		if id == 0 {
			return ApplyResult{}, ErrMutationInvalid
		}
	}
	if len(users) > 0 {
		valid, err := service.users.ValidateRequirementUsers(ctx, command.WorkspaceID, users)
		if err != nil {
			return ApplyResult{}, err
		}
		if !valid {
			return ApplyResult{}, ErrMutationConflict
		}
	}
	switch command.Operation {
	case MutationCreate:
		if command.ID != 0 || command.PlanUnitID == 0 || command.WorkflowTemplateID == 0 || command.Title == "" || len(command.Title) > 220 {
			return ApplyResult{}, ErrMutationInvalid
		}
	case MutationUpdate:
		if command.ID == 0 || command.ExpectedVersion == 0 || command.WorkflowTemplateID != 0 ||
			!command.TitleSet && !command.DescriptionSet && !command.ProductDocURLSet && !command.DesignURLSet &&
				!command.AcceptanceCriteriaSet && !command.PrioritySet && !command.ProductOwnerSet && !command.DesignOwnerSet &&
				!command.ProductOwnerUserIDsSet && !command.DesignOwnerUserIDsSet && !command.FrontendOwnerUserIDsSet &&
				!command.BackendOwnerUserIDsSet && !command.TesterUserIDsSet && !command.DeveloperUserIDsSet && command.PlanUnitID == 0 {
			return ApplyResult{}, ErrMutationInvalid
		}
		if command.ConfirmSubjectSchedule && command.PlanUnitID == 0 {
			return ApplyResult{}, ErrMutationInvalid
		}
	case MutationDelete:
		if command.ID == 0 {
			return ApplyResult{}, ErrMutationInvalid
		}
	case MutationBatchMove:
		if command.PlanUnitID == 0 || len(command.Items) == 0 || len(command.Items) > 50 {
			return ApplyResult{}, ErrMutationInvalid
		}
		seen := map[uint]bool{}
		for _, item := range command.Items {
			if item.RequirementID == 0 || item.ExpectedVersion == 0 || seen[item.RequirementID] {
				return ApplyResult{}, ErrMutationInvalid
			}
			seen[item.RequirementID] = true
		}
	default:
		return ApplyResult{}, ErrMutationInvalid
	}
	if command.PrioritySet && command.Priority != "p0" && command.Priority != "p1" && command.Priority != "p2" && command.Priority != "p3" {
		return ApplyResult{}, ErrMutationInvalid
	}
	return service.store.ApplyRequirementMutation(ctx, command)
}
