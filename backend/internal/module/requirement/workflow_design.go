package requirement

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var ErrWorkflowDesignConflict = errors.New("requirement workflow design conflicts with current state")

type WorkflowDesignStore interface {
	AttachWorkflowDesign(context.Context, uint, uint, string) error
}

type WorkflowDesignService struct{ store WorkflowDesignStore }

func NewWorkflowDesignService(store WorkflowDesignStore) (*WorkflowDesignService, error) {
	if store == nil {
		return nil, ErrWorkflowDesignConflict
	}
	return &WorkflowDesignService{store: store}, nil
}

func (service *WorkflowDesignService) AttachWorkflowDesign(
	ctx context.Context,
	workspaceID, requirementID uint,
	rawURL string,
) error {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.ParseRequestURI(rawURL)
	if service == nil || service.store == nil || workspaceID == 0 || requirementID == 0 || err != nil ||
		parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrWorkflowDesignConflict
	}
	return service.store.AttachWorkflowDesign(ctx, workspaceID, requirementID, rawURL)
}
