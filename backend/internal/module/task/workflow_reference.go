package task

import (
	"context"
	"errors"
	"strings"
)

var ErrWorkflowReferenceInvalid = errors.New("task workflow reference query is invalid")

const (
	WorkTaskKindTask    = "task"
	WorkTaskKindSubtask = "subtask"
)

func NormalizePMWorkTaskKind(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func IsPMWorkTaskKind(value string) bool {
	switch NormalizePMWorkTaskKind(value) {
	case WorkTaskKindTask, WorkTaskKindSubtask:
		return true
	default:
		return false
	}
}

func PMWorkTaskKinds() []string {
	return []string{WorkTaskKindTask, WorkTaskKindSubtask}
}

type WorkflowScope struct {
	TeamID    uint
	ProjectID uint
}

type WorkflowKind struct {
	Key     string `json:"key"`
	KindKey string `json:"kind_key"`
	Label   string `json:"label"`
}

type WorkflowStatus struct {
	StatusKey string `json:"status_key"`
	Label     string `json:"label"`
	SortOrder int    `json:"sort_order"`
	Color     string `json:"color"`
}

type WorkflowTransition struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type WorkflowTransitionProfile struct {
	Enabled         bool                 `json:"enabled"`
	Enforce         bool                 `json:"enforce"`
	Standard        bool                 `json:"standard"`
	WorkflowProfile string               `json:"workflow_profile"`
	Transitions     []WorkflowTransition `json:"transitions"`
}

type WorkflowScopeStore interface {
	ValidateWorkflowScope(context.Context, uint, WorkflowScope) error
}

type WorkflowReferenceService struct{ scopes WorkflowScopeStore }

func NewWorkflowReferenceService(scopes WorkflowScopeStore) (*WorkflowReferenceService, error) {
	if scopes == nil {
		return nil, ErrWorkflowReferenceInvalid
	}
	return &WorkflowReferenceService{scopes: scopes}, nil
}

func (service *WorkflowReferenceService) ListKinds(ctx context.Context, workspaceID uint, scope WorkflowScope) ([]WorkflowKind, error) {
	if err := service.validateScope(ctx, workspaceID, scope); err != nil {
		return nil, err
	}
	return StandardWorkflowKinds(), nil
}

func (service *WorkflowReferenceService) ListStatuses(ctx context.Context, workspaceID uint, kind string, scope WorkflowScope) ([]WorkflowStatus, error) {
	if !IsPMWorkTaskKind(kind) {
		return nil, ErrWorkflowReferenceInvalid
	}
	if err := service.validateScope(ctx, workspaceID, scope); err != nil {
		return nil, err
	}
	return StandardWorkflowStatuses(), nil
}

func (service *WorkflowReferenceService) ListTransitions(ctx context.Context, workspaceID uint, kind string, scope WorkflowScope) (WorkflowTransitionProfile, error) {
	if !IsPMWorkTaskKind(kind) {
		return WorkflowTransitionProfile{}, ErrWorkflowReferenceInvalid
	}
	if err := service.validateScope(ctx, workspaceID, scope); err != nil {
		return WorkflowTransitionProfile{}, err
	}
	return StandardWorkflowTransitionProfile(), nil
}

func (service *WorkflowReferenceService) validateScope(ctx context.Context, workspaceID uint, scope WorkflowScope) error {
	if workspaceID == 0 || (scope.ProjectID > 0 && scope.TeamID == 0) {
		return ErrWorkflowReferenceInvalid
	}
	return service.scopes.ValidateWorkflowScope(ctx, workspaceID, scope)
}

func StandardWorkflowKinds() []WorkflowKind {
	return []WorkflowKind{
		{Key: WorkTaskKindTask, KindKey: WorkTaskKindTask, Label: "任务"},
		{Key: WorkTaskKindSubtask, KindKey: WorkTaskKindSubtask, Label: "子任务"},
	}
}

func StandardWorkflowStatuses() []WorkflowStatus {
	return []WorkflowStatus{
		{StatusKey: "draft", Label: "待处理", SortOrder: 10, Color: "#b45309"},
		{StatusKey: "in_dev", Label: "进行中", SortOrder: 20, Color: "#2563eb"},
		{StatusKey: "pending_test", Label: "待验证", SortOrder: 30, Color: "#b45309"},
		{StatusKey: "completed", Label: "已完成", SortOrder: 40, Color: "#15803d"},
		{StatusKey: "cancelled", Label: "已取消", SortOrder: 50, Color: "#64748b"},
	}
}

func StandardWorkflowTransitionProfile() WorkflowTransitionProfile {
	return WorkflowTransitionProfile{
		Enabled: false, Enforce: false, Standard: true, WorkflowProfile: "standard_builtin",
		Transitions: []WorkflowTransition{
			{FromStatus: "draft", ToStatus: "in_dev"}, {FromStatus: "in_dev", ToStatus: "draft"},
			{FromStatus: "in_dev", ToStatus: "pending_test"}, {FromStatus: "pending_test", ToStatus: "in_dev"},
			{FromStatus: "pending_test", ToStatus: "completed"}, {FromStatus: "completed", ToStatus: "pending_test"},
			{FromStatus: "completed", ToStatus: "in_dev"}, {FromStatus: "draft", ToStatus: "cancelled"},
			{FromStatus: "in_dev", ToStatus: "cancelled"}, {FromStatus: "pending_test", ToStatus: "cancelled"},
		},
	}
}
