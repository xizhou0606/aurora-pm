package projectgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PlanAssignmentRepository struct{ db *gorm.DB }

func NewPlanAssignmentRepository(db *gorm.DB) *PlanAssignmentRepository {
	return &PlanAssignmentRepository{db: db}
}

func (repository *PlanAssignmentRepository) ResolvePlanAssignment(ctx context.Context, workspaceID, planUnitID uint) (project.PlanAssignment, error) {
	// Lock order is project set first, then plan unit, matching every other
	// requirement mutation path; locking the unit before its parent can deadlock
	// against HTTP paths that take the parent lock first.
	var unit PlanUnit
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return project.PlanAssignment{}, project.ErrPlanAssignmentInvalid
		}
		return project.PlanAssignment{}, err
	}
	var parent ProjectSet
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, unit.ProjectSetID).First(&parent).Error; err != nil {
		return project.PlanAssignment{}, err
	}
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error; err != nil {
		return project.PlanAssignment{}, err
	}
	if unit.MutationState != "" || parent.MutationState != "" || unit.TeamID != parent.TeamID {
		return project.PlanAssignment{}, project.ErrPlanAssignmentInvalid
	}
	return project.PlanAssignment{WorkspaceID: unit.WorkspaceID, TeamID: unit.TeamID, ProjectSetID: unit.ProjectSetID,
		PlanUnitID: unit.ID, ProjectCode: parent.Code, Type: unit.Type, MutationState: unit.MutationState,
		RequirementsLockedAt: unit.RequirementsLockedAt,
		PlannedStartAt:       unit.PlannedStartAt, PlannedEndAt: unit.PlannedEndAt}, nil
}

var _ project.PlanAssignmentStore = (*PlanAssignmentRepository)(nil)
