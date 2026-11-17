package aigorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowActionRepository struct{ db *gorm.DB }

func NewWorkflowActionRepository(db *gorm.DB) *WorkflowActionRepository {
	return &WorkflowActionRepository{db: db}
}

func (repository *WorkflowActionRepository) PrepareWorkflowAction(
	ctx context.Context,
	command ai.WorkflowActionCommand,
) (ai.WorkflowActionResult, error) {
	if repository == nil || repository.db == nil {
		return ai.WorkflowActionResult{}, ai.ErrWorkflowActionConflict
	}
	var row AIRun
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND capability = ?", command.RunID, command.WorkspaceID, command.Capability,
	).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ai.WorkflowActionResult{}, ai.ErrWorkflowActionConflict
		}
		return ai.WorkflowActionResult{}, err
	}
	switch command.Action {
	case "complete", "override_ai":
		if row.Status != "completed" && row.Status != "applied" ||
			command.Action == "override_ai" && row.Capability != "requirement.completeness" {
			return ai.WorkflowActionResult{}, ai.ErrWorkflowActionConflict
		}
		path := workflowAIResultPath(row.Capability, row.OutputJSON)
		if command.Action == "override_ai" {
			path = "success"
		}
		return ai.WorkflowActionResult{RunID: row.ID, Path: path}, nil
	case "retry_ai", "recheck_ai":
		if command.Action == "retry_ai" && row.Status != "failed" && row.Status != "cancelled" ||
			command.Action == "recheck_ai" && row.Status != "completed" && row.Status != "applied" {
			return ai.WorkflowActionResult{}, ai.ErrWorkflowActionConflict
		}
		created := AIRun{
			WorkspaceID: row.WorkspaceID, Capability: row.Capability, SubjectType: row.SubjectType,
			SubjectID: row.SubjectID, SubjectVersion: row.SubjectVersion, Status: "queued", Model: row.Model,
			InputJSON: row.InputJSON, RequestedByUser: command.PrincipalID,
		}
		if err := repository.db.WithContext(ctx).Create(&created).Error; err != nil {
			return ai.WorkflowActionResult{}, err
		}
		return ai.WorkflowActionResult{RunID: created.ID}, nil
	default:
		return ai.WorkflowActionResult{}, ai.ErrWorkflowActionConflict
	}
}

func workflowAIResultPath(capability, raw string) string {
	if capability != "requirement.completeness" {
		return "success"
	}
	var output struct {
		CompletenessReport *struct {
			CanProceed bool `json:"canProceed"`
		} `json:"completenessReport"`
	}
	if json.Unmarshal([]byte(raw), &output) != nil || output.CompletenessReport == nil || !output.CompletenessReport.CanProceed {
		return "failure"
	}
	return "success"
}

var _ ai.WorkflowActionStore = (*WorkflowActionRepository)(nil)
