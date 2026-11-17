package workflow

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrAuxReadInvalid          = errors.New("workflow auxiliary read query is invalid")
	ErrWorkflowNodeNotFound    = errors.New("workflow node not found")
	ErrWorkflowNodeUnsupported = errors.New("workflow node does not support the requested auxiliary data")
	ErrWorkflowAuxDataInvalid  = errors.New("workflow auxiliary stored data is invalid")
)

type AuxPurpose string

const (
	AuxPurposeOpinion    AuxPurpose = "opinion"
	AuxPurposeTestCase   AuxPurpose = "test_case"
	AuxPurposeTestReport AuxPurpose = "test_report"
)

type AuxNodeContext struct {
	InstanceID  uint   `json:"instance_id"`
	WorkspaceID uint   `json:"workspace_id"`
	SubjectType string `json:"subject_type"`
	SubjectID   uint   `json:"subject_id"`
	NodeID      string `json:"node_id"`
	NodeType    string `json:"node_type"`
	NodeName    string `json:"node_name"`
}

type NodeOpinion struct {
	ID           uint      `json:"id"`
	WorkspaceID  uint      `json:"workspace_id"`
	InstanceID   uint      `json:"instance_id"`
	NodeID       string    `json:"node_id"`
	AuthorUserID uint      `json:"author_user_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Edited       bool      `json:"edited"`
}

type AuxReadStore interface {
	ResolveAuxNode(context.Context, uint, uint, string, AuxPurpose) (AuxNodeContext, error)
	ListNodeOpinions(context.Context, uint, uint, string) ([]NodeOpinion, error)
}

type AuxReadService struct{ store AuxReadStore }

func NewAuxReadService(store AuxReadStore) (*AuxReadService, error) {
	if store == nil {
		return nil, ErrAuxReadInvalid
	}
	return &AuxReadService{store: store}, nil
}

func (service *AuxReadService) ResolveNode(
	ctx context.Context,
	workspaceID, instanceID uint,
	nodeID string,
	purpose AuxPurpose,
) (AuxNodeContext, error) {
	nodeID = strings.TrimSpace(nodeID)
	if workspaceID == 0 || instanceID == 0 || nodeID == "" || !validAuxPurpose(purpose) {
		return AuxNodeContext{}, ErrAuxReadInvalid
	}
	resolved, err := service.store.ResolveAuxNode(ctx, workspaceID, instanceID, nodeID, purpose)
	if err != nil {
		return AuxNodeContext{}, err
	}
	if resolved.WorkspaceID != workspaceID || resolved.InstanceID != instanceID || resolved.SubjectID == 0 ||
		(resolved.SubjectType != "requirement" && resolved.SubjectType != "plan_unit") ||
		strings.TrimSpace(resolved.NodeID) != nodeID || strings.TrimSpace(resolved.NodeType) == "" || strings.TrimSpace(resolved.NodeName) == "" {
		return AuxNodeContext{}, ErrWorkflowAuxDataInvalid
	}
	return resolved, nil
}

func (service *AuxReadService) ListOpinions(
	ctx context.Context,
	workspaceID, instanceID uint,
	nodeID string,
) (AuxNodeContext, []NodeOpinion, error) {
	resolved, err := service.ResolveNode(ctx, workspaceID, instanceID, nodeID, AuxPurposeOpinion)
	if err != nil {
		return AuxNodeContext{}, nil, err
	}
	items, err := service.store.ListNodeOpinions(ctx, workspaceID, instanceID, resolved.NodeID)
	if err != nil {
		return AuxNodeContext{}, nil, err
	}
	if items == nil {
		items = []NodeOpinion{}
	}
	for _, item := range items {
		if item.WorkspaceID != workspaceID || item.InstanceID != instanceID || item.NodeID != resolved.NodeID ||
			item.ID == 0 || item.AuthorUserID == 0 || strings.TrimSpace(item.Body) == "" ||
			item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
			return AuxNodeContext{}, nil, ErrWorkflowAuxDataInvalid
		}
	}
	return resolved, items, nil
}

func validAuxPurpose(value AuxPurpose) bool {
	switch value {
	case AuxPurposeOpinion, AuxPurposeTestCase, AuxPurposeTestReport:
		return true
	default:
		return false
	}
}
