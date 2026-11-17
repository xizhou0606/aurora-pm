package workflow

import (
	"context"
	"errors"
	"time"
)

var ErrBIReadInvalid = errors.New("workflow BI read query is invalid")

type BIReviewDecision struct {
	ID            uint
	RequirementID uint
	Status        string
	DecidedAt     time.Time
}

type BIMetricApprovalNode struct {
	ID   string
	Name string
}

type BIMetricWorkflowOption struct {
	TemplateID       uint
	TemplateName     string
	PublishedVersion uint
	ApprovalNodes    []BIMetricApprovalNode
}

type BIReadStore interface {
	ListBIReviewDecisions(context.Context, uint, uint, string, []uint) ([]BIReviewDecision, error)
	ListBIMetricWorkflowOptions(context.Context, uint) ([]BIMetricWorkflowOption, error)
}

func (service *BIReadService) ListBIMetricWorkflowOptions(ctx context.Context, workspaceID uint) ([]BIMetricWorkflowOption, error) {
	if workspaceID == 0 {
		return nil, ErrBIReadInvalid
	}
	return service.store.ListBIMetricWorkflowOptions(ctx, workspaceID)
}

type BIReadService struct{ store BIReadStore }

func NewBIReadService(store BIReadStore) (*BIReadService, error) {
	if store == nil {
		return nil, ErrBIReadInvalid
	}
	return &BIReadService{store: store}, nil
}

func (service *BIReadService) ListBIReviewDecisions(ctx context.Context, workspaceID, templateID uint, nodeID string, requirementIDs []uint) ([]BIReviewDecision, error) {
	if workspaceID == 0 || templateID == 0 || nodeID == "" {
		return nil, ErrBIReadInvalid
	}
	if len(requirementIDs) == 0 {
		return []BIReviewDecision{}, nil
	}
	return service.store.ListBIReviewDecisions(ctx, workspaceID, templateID, nodeID, requirementIDs)
}
