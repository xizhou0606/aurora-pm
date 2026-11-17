package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrReadQueryInvalid          = errors.New("workflow read query is invalid")
	ErrWorkflowTemplateNotFound  = errors.New("workflow template not found")
	ErrWorkflowInstanceNotFound  = errors.New("workflow instance not found")
	ErrWorkflowDefinitionInvalid = errors.New("workflow definition is invalid")
)

type TemplateSnapshot struct {
	ID               uint            `json:"id"`
	WorkspaceID      uint            `json:"workspace_id"`
	Name             string          `json:"name"`
	SubjectType      string          `json:"subject_type"`
	Description      string          `json:"description,omitempty"`
	PublishedVersion uint            `json:"published_version"`
	CreatedByUserID  uint            `json:"created_by_user_id"`
	UpdatedByUserID  uint            `json:"updated_by_user_id"`
	Definition       json.RawMessage `json:"definition,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type TemplateFilter struct{ SubjectType string }

type InstanceFilter struct {
	SubjectType string
	SubjectID   uint
}

type InstanceSnapshot struct {
	ID                  uint            `json:"id"`
	WorkspaceID         uint            `json:"workspace_id"`
	TemplateID          uint            `json:"template_id"`
	TemplateVersionID   uint            `json:"template_version_id"`
	TemplateName        string          `json:"template_name,omitempty"`
	TemplateVersion     uint            `json:"template_version,omitempty"`
	SubjectType         string          `json:"subject_type"`
	SubjectID           uint            `json:"subject_id"`
	Status              string          `json:"status"`
	CanAct              bool            `json:"can_act"`
	AllowedActions      []string        `json:"allowed_actions"`
	CurrentNodeIDs      []string        `json:"current_node_ids"`
	NodePlans           json.RawMessage `json:"node_plans"`
	PublishedDefinition json.RawMessage `json:"published_definition,omitempty"`
	StartedByUserID     uint            `json:"started_by_user_id"`
	StartedAt           time.Time       `json:"started_at"`
	CompletedAt         *time.Time      `json:"completed_at,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at"`
	NodeRuns            []NodeRun       `json:"node_runs,omitempty"`
	Approvals           []Approval      `json:"approvals,omitempty"`
}

type NodeRun struct {
	ID           uint       `json:"id"`
	NodeID       string     `json:"node_id"`
	NodeType     string     `json:"node_type"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type Approval struct {
	ID             uint       `json:"id"`
	NodeRunID      uint       `json:"node_run_id"`
	Status         string     `json:"status"`
	ApproverUserID *uint      `json:"approver_user_id,omitempty"`
	Comment        string     `json:"comment,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ReadStore interface {
	ListTemplates(context.Context, uint, TemplateFilter) ([]TemplateSnapshot, error)
	GetTemplate(context.Context, uint, uint) (TemplateSnapshot, error)
	ListInstances(context.Context, uint, InstanceFilter) ([]InstanceSnapshot, error)
	GetInstance(context.Context, uint, uint) (InstanceSnapshot, error)
}

type ReadService struct{ store ReadStore }

func NewReadService(store ReadStore) (*ReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ReadService{store: store}, nil
}

func (service *ReadService) ListTemplates(ctx context.Context, workspaceID uint, filter TemplateFilter) ([]TemplateSnapshot, error) {
	if workspaceID == 0 || (filter.SubjectType != "" && !validSubjectType(filter.SubjectType)) {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListTemplates(ctx, workspaceID, filter)
}

func (service *ReadService) GetTemplate(ctx context.Context, workspaceID, id uint) (TemplateSnapshot, error) {
	if workspaceID == 0 || id == 0 {
		return TemplateSnapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetTemplate(ctx, workspaceID, id)
}

func (service *ReadService) ListInstances(ctx context.Context, workspaceID uint, filter InstanceFilter) ([]InstanceSnapshot, error) {
	if workspaceID == 0 || filter.SubjectID == 0 || !validSubjectType(filter.SubjectType) {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListInstances(ctx, workspaceID, filter)
}

func (service *ReadService) GetInstance(ctx context.Context, workspaceID, id uint) (InstanceSnapshot, error) {
	if workspaceID == 0 || id == 0 {
		return InstanceSnapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetInstance(ctx, workspaceID, id)
}

func validSubjectType(value string) bool { return value == "plan_unit" || value == "requirement" }
