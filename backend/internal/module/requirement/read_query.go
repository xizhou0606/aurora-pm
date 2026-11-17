package requirement

import (
	"context"
	"errors"
	"time"
)

var ErrReadQueryInvalid = errors.New("requirement read query is invalid")

type Snapshot struct {
	ID                   uint      `json:"id"`
	WorkspaceID          uint      `json:"workspace_id"`
	TeamID               uint      `json:"team_id"`
	ProjectSetID         uint      `json:"project_new_id"`
	PlanUnitID           *uint     `json:"version_task_new_id,omitempty"`
	Version              uint      `json:"version"`
	WorkflowInstanceID   *uint     `json:"workflow_instance_id,omitempty"`
	Title                string    `json:"title"`
	Description          string    `json:"description,omitempty"`
	ProductDocURL        string    `json:"product_doc_url,omitempty"`
	ProductOwnerUserIDs  []uint    `json:"product_owner_user_ids"`
	DesignURL            string    `json:"design_url,omitempty"`
	DesignOwnerUserIDs   []uint    `json:"design_owner_user_ids"`
	FrontendOwnerUserIDs []uint    `json:"frontend_owner_user_ids"`
	BackendOwnerUserIDs  []uint    `json:"backend_owner_user_ids"`
	TesterUserIDs        []uint    `json:"tester_user_ids"`
	DeveloperUserIDs     []uint    `json:"developer_user_ids"`
	AcceptanceCriteria   string    `json:"acceptance_criteria,omitempty"`
	Priority             string    `json:"priority"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ReadStore interface {
	GetRequirement(context.Context, uint, uint) (Snapshot, error)
}

type ReadService struct{ store ReadStore }

func NewReadService(store ReadStore) (*ReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ReadService{store: store}, nil
}

func (service *ReadService) GetRequirement(ctx context.Context, workspaceID, id uint) (Snapshot, error) {
	if workspaceID == 0 || id == 0 {
		return Snapshot{}, ErrReadQueryInvalid
	}
	return service.store.GetRequirement(ctx, workspaceID, id)
}
