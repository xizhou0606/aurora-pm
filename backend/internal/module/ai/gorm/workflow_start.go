package aigorm

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
)

type WorkflowStartRepository struct {
	db      *gorm.DB
	enabled bool
	model   string
}

func NewWorkflowStartRepository(db *gorm.DB, enabled bool, model string) *WorkflowStartRepository {
	return &WorkflowStartRepository{db: db, enabled: enabled, model: strings.TrimSpace(model)}
}

func (repository *WorkflowStartRepository) StartWorkflowNode(
	ctx context.Context,
	command ai.WorkflowStartCommand,
) (ai.WorkflowStartResult, error) {
	if repository == nil || repository.db == nil || !repository.enabled || repository.model == "" {
		return ai.WorkflowStartResult{}, ai.ErrWorkflowStartUnavailable
	}
	input, err := json.Marshal(map[string]any{
		"capability": command.Capability, "workspace_id": command.WorkspaceID,
		"subject": json.RawMessage(command.SubjectSnapshot),
	})
	if err != nil {
		return ai.WorkflowStartResult{}, err
	}
	run := AIRun{
		WorkspaceID: command.WorkspaceID, Capability: command.Capability,
		SubjectType: command.SubjectType, SubjectID: command.SubjectID, SubjectVersion: command.SubjectVersion,
		Status: "queued", Model: repository.model, InputJSON: string(input), RequestedByUser: command.PrincipalID,
	}
	if err := repository.db.WithContext(ctx).Create(&run).Error; err != nil {
		return ai.WorkflowStartResult{}, err
	}
	return ai.WorkflowStartResult{RunID: run.ID}, nil
}

var _ ai.WorkflowStartStore = (*WorkflowStartRepository)(nil)
