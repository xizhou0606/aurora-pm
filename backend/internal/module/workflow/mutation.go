package workflow

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrMutationInvalid     = errors.New("workflow mutation is invalid")
	ErrMutationNotFound    = errors.New("workflow mutation target not found")
	ErrMutationConflict    = errors.New("workflow mutation conflicts with current state")
	ErrMutationUnavailable = errors.New("workflow mutation dependency is unavailable")
	// ErrRequirementLockConfirmationRequired rejects completing a project
	// requirement-intake node without the explicit lock confirmation.
	ErrRequirementLockConfirmationRequired = errors.New("requirement intake lock confirmation is required")
)

type MutationOperation string

const (
	ActionExecute        MutationOperation = "action.execute"
	NodePlanUpdate       MutationOperation = "node_plan.update"
	OpinionCreate        MutationOperation = "opinion.create"
	OpinionUpdate        MutationOperation = "opinion.update"
	TestReportLinkCreate MutationOperation = "test_report_link.create"
)

type OptionalTime struct {
	Set   bool
	Value *time.Time
}

type NodePlanMutation struct {
	NodeID            string
	PlannedStartAt    OptionalTime
	PlannedEndAt      OptionalTime
	ExpectedStartAt   OptionalTime
	ExpectedEndAt     OptionalTime
	AssigneeUserID    uint
	AssigneeUserIDSet bool
}

type Mutation struct {
	Operation   MutationOperation
	WorkspaceID uint
	PrincipalID uint
	ApplyKey    string
	InputDigest string

	InstanceID      uint
	NodeID          string
	OpinionID       uint
	Action          string
	TargetNodeID    string
	Comment         string
	DesignURL       string
	NextAssignees   map[string]uint
	NodePlans       []NodePlanMutation
	ConfirmSchedule bool
	// ConfirmRequirementLock carries the explicit second confirmation that
	// completing a project requirement-intake node locks the plan unit. It
	// expresses user intent only; permissions are evaluated separately.
	ConfirmRequirementLock bool
	Body                   string
	InviteUserIDs          []uint
	URL                    string
}

type MutationResult struct {
	ID             uint                `json:"id"`
	WorkspaceID    uint                `json:"workspace_id,omitempty"`
	InstanceID     uint                `json:"instance_id,omitempty"`
	NodeID         string              `json:"node_id,omitempty"`
	Status         string              `json:"status,omitempty"`
	Action         string              `json:"action,omitempty"`
	CurrentNodeIDs []string            `json:"current_node_ids,omitempty"`
	NodePlans      map[string]NodePlan `json:"node_plans,omitempty"`
	Opinion        *NodeOpinion        `json:"opinion,omitempty"`
	InvitedUserIDs []uint              `json:"invited_user_ids,omitempty"`
	TestReport     *TestReportLink     `json:"test_report,omitempty"`
}

type NodePlan struct {
	PlannedStartAt *time.Time `json:"planned_start_at,omitempty"`
	PlannedEndAt   *time.Time `json:"planned_end_at,omitempty"`
	AssigneeUserID *uint      `json:"assignee_user_id,omitempty"`
}

type TestReportLink struct {
	ID             uint      `json:"id"`
	WorkspaceID    uint      `json:"workspace_id"`
	PlanUnitID     uint      `json:"plan_unit_id"`
	ProjectSetID   uint      `json:"project_set_id"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	UploaderUserID uint      `json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type SubjectContext struct {
	ProjectSetID uint
	PlanUnitID   uint
	TeamID       uint
	SubjectID    uint
	SubjectType  string
	SubjectTitle string
}

type SubjectResolver interface {
	ResolveWorkflowSubject(context.Context, uint, string, uint) (SubjectContext, error)
}

type UserVerifier interface {
	VerifyActiveWorkspaceUsers(context.Context, uint, []uint) error
}

type TestReportLinkWriter interface {
	CreateWorkflowTestReportLink(context.Context, uint, SubjectContext, uint, string, string, string) (TestReportLink, error)
}

type ScheduleWriter interface {
	ConfirmWorkflowSchedule(context.Context, uint, SubjectContext, *time.Time, *time.Time) error
}

type ActionEffect struct {
	WorkspaceID  uint
	PrincipalID  uint
	ProjectSetID uint
	InstanceID   uint
	NodeID       string
	Action       string
	Status       string
	ApplyKey     string
}

type ActionEffectWriter interface {
	RecordWorkflowAction(context.Context, ActionEffect) error
}

type PreReviewInvitation struct {
	WorkspaceID, PrincipalID, ProjectSetID, TeamID uint
	InstanceID, SubjectID                          uint
	SubjectType, SubjectTitle, NodeID, NodeName    string
	InviteUserIDs                                  []uint
}

type PreReviewInvitationWriter interface {
	RecordPreReviewInvitation(context.Context, PreReviewInvitation) error
}

type AIActionRequest struct {
	WorkspaceID uint
	PrincipalID uint
	RunID       uint
	Capability  string
	Action      string
}

type AIActionResult struct {
	RunID uint
	Path  string
}

type AIActionService interface {
	PrepareWorkflowAIAction(context.Context, AIActionRequest) (AIActionResult, error)
}

type RequirementDesignWriter interface {
	AttachWorkflowDesign(context.Context, uint, uint, string) error
}

type MutationStore interface {
	ApplyWorkflowMutation(context.Context, Mutation) (MutationResult, error)
}

type MutationService struct{ store MutationStore }

func NewMutationService(store MutationStore) (*MutationService, error) {
	if store == nil {
		return nil, ErrMutationInvalid
	}
	return &MutationService{store: store}, nil
}

func (service *MutationService) ApplyWorkflowMutation(ctx context.Context, command Mutation) (MutationResult, error) {
	command.NodeID = strings.TrimSpace(command.NodeID)
	command.Action = strings.TrimSpace(command.Action)
	command.TargetNodeID = strings.TrimSpace(command.TargetNodeID)
	command.Comment = strings.TrimSpace(command.Comment)
	command.DesignURL = strings.TrimSpace(command.DesignURL)
	command.Body = strings.TrimSpace(command.Body)
	command.URL = strings.TrimSpace(command.URL)
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 ||
		command.InstanceID == 0 || strings.TrimSpace(command.ApplyKey) == "" || len(command.InputDigest) != 64 {
		return MutationResult{}, ErrMutationInvalid
	}
	if err := validateMutation(command); err != nil {
		return MutationResult{}, err
	}
	return service.store.ApplyWorkflowMutation(ctx, command)
}

func validateMutation(command Mutation) error {
	switch command.Operation {
	case ActionExecute:
		if !validWorkflowAction(command.Action) {
			return ErrMutationInvalid
		}
	case NodePlanUpdate:
		if len(command.NodePlans) == 0 && !command.ConfirmSchedule {
			return ErrMutationInvalid
		}
		seen := map[string]bool{}
		for _, node := range command.NodePlans {
			id := strings.TrimSpace(node.NodeID)
			if id == "" || seen[id] || !node.PlannedStartAt.Set && !node.PlannedEndAt.Set && !node.AssigneeUserIDSet {
				return ErrMutationInvalid
			}
			if node.PlannedStartAt.Set != node.ExpectedStartAt.Set || node.PlannedEndAt.Set != node.ExpectedEndAt.Set ||
				node.AssigneeUserIDSet && node.AssigneeUserID == 0 {
				return ErrMutationInvalid
			}
			seen[id] = true
		}
	case OpinionCreate:
		if command.NodeID == "" || command.Body == "" && len(command.InviteUserIDs) == 0 || !validOpinionBody(command.Body) {
			return ErrMutationInvalid
		}
	case OpinionUpdate:
		if command.NodeID == "" || command.OpinionID == 0 || command.Body == "" || !validOpinionBody(command.Body) {
			return ErrMutationInvalid
		}
	case TestReportLinkCreate:
		if command.NodeID == "" || !validHTTPURL(command.URL) {
			return ErrMutationInvalid
		}
	default:
		return ErrMutationInvalid
	}
	return nil
}

func validOpinionBody(body string) bool {
	return body == "" || utf8.RuneCountInString(body) <= 20_000 && !strings.Contains(strings.ToLower(body), "<script")
}

func validHTTPURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validWorkflowAction(action string) bool {
	switch action {
	case "complete", "approve", "reject", "condition_yes", "condition_no", "retry_ai", "recheck_ai",
		"override_ai", "rollback", "resume", "supplement_design":
		return true
	default:
		return false
	}
}
