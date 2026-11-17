package projectgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowScheduleRepository struct{ db *gorm.DB }

func NewWorkflowScheduleRepository(db *gorm.DB) *WorkflowScheduleRepository {
	return &WorkflowScheduleRepository{db: db}
}

func (repository *WorkflowScheduleRepository) ConfirmWorkflowSchedule(
	ctx context.Context,
	command project.WorkflowScheduleCommand,
) error {
	if repository == nil || repository.db == nil {
		return project.ErrWorkflowScheduleConflict
	}
	var projectSet ProjectSet
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ?", command.ProjectSetID, command.WorkspaceID,
	).First(&projectSet).Error; err != nil {
		return normalizeWorkflowScheduleError(err)
	}
	var unit PlanUnit
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND project_new_id = ?",
		command.PlanUnitID, command.WorkspaceID, command.ProjectSetID,
	).First(&unit).Error; err != nil {
		return normalizeWorkflowScheduleError(err)
	}
	start, end := unit.PlannedStartAt, unit.PlannedEndAt
	if command.SuggestedStartAt != nil && unit.PlannedStartAt != nil && command.SuggestedStartAt.Before(*unit.PlannedStartAt) ||
		command.SuggestedEndAt != nil && unit.PlannedEndAt != nil && command.SuggestedEndAt.After(*unit.PlannedEndAt) {
		return project.ErrWorkflowScheduleConflict
	}
	if start == nil && command.SuggestedStartAt != nil {
		start = command.SuggestedStartAt
	}
	if end == nil && command.SuggestedEndAt != nil {
		end = command.SuggestedEndAt
	}
	if start != nil && end != nil && start.After(*end) ||
		start != nil && projectSet.PlannedStartAt != nil && start.Before(*projectSet.PlannedStartAt) ||
		start != nil && projectSet.PlannedEndAt != nil && start.After(*projectSet.PlannedEndAt) ||
		end != nil && projectSet.PlannedStartAt != nil && end.Before(*projectSet.PlannedStartAt) ||
		end != nil && projectSet.PlannedEndAt != nil && end.After(*projectSet.PlannedEndAt) {
		return project.ErrWorkflowScheduleConflict
	}
	updates := map[string]any{}
	if unit.PlannedStartAt == nil && start != nil {
		updates["planned_start_at"] = start
	}
	if unit.PlannedEndAt == nil && end != nil {
		updates["planned_end_at"] = end
	}
	if len(updates) == 0 {
		return nil
	}
	updated := repository.db.WithContext(ctx).Model(&PlanUnit{}).Where(
		"id = ? AND workspace_id = ? AND project_new_id = ?",
		unit.ID, unit.WorkspaceID, unit.ProjectSetID,
	).Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return project.ErrWorkflowScheduleConflict
	}
	return nil
}

func normalizeWorkflowScheduleError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.ErrWorkflowScheduleConflict
	}
	return err
}

var _ project.WorkflowScheduleStore = (*WorkflowScheduleRepository)(nil)
