package workflow

import (
	"context"
	"errors"
	"time"
)

var ErrTodoReadInvalid = errors.New("workflow todo read query is invalid")

type TodoSnapshot struct {
	TokenID        uint       `json:"workflow_token_id"`
	InstanceID     uint       `json:"workflow_instance_id"`
	WorkspaceID    uint       `json:"workspace_id"`
	NodeID         string     `json:"workflow_node_id"`
	NodeName       string     `json:"title"`
	NodeType       string     `json:"node_type"`
	Capability     string     `json:"capability,omitempty"`
	TemplateName   string     `json:"template_name"`
	SubjectType    string     `json:"subject_type"`
	SubjectID      uint       `json:"subject_id"`
	AssigneeUserID uint       `json:"assignee_user_id"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	AIRunID        uint       `json:"ai_run_id,omitempty"`
	AIStatus       string     `json:"ai_status,omitempty"`
	ActionKind     string     `json:"action_kind"`
	AllowedActions []string   `json:"allowed_actions"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type TodoReadStore interface {
	ListPrincipalTodos(context.Context, uint, uint) ([]TodoSnapshot, error)
}

type TodoReadService struct{ store TodoReadStore }

func NewTodoReadService(store TodoReadStore) (*TodoReadService, error) {
	if store == nil {
		return nil, ErrTodoReadInvalid
	}
	return &TodoReadService{store: store}, nil
}

func (service *TodoReadService) ListPrincipalTodos(ctx context.Context, workspaceID, principalID uint) ([]TodoSnapshot, error) {
	if workspaceID == 0 || principalID == 0 {
		return nil, ErrTodoReadInvalid
	}
	items, err := service.store.ListPrincipalTodos(ctx, workspaceID, principalID)
	if items == nil && err == nil {
		items = []TodoSnapshot{}
	}
	return items, err
}
