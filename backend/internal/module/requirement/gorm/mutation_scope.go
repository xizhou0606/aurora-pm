package requirementgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MutationScopeRepository struct{ db *gorm.DB }

func NewMutationScopeRepository(db *gorm.DB) *MutationScopeRepository {
	return &MutationScopeRepository{db: db}
}

func (repository *MutationScopeRepository) LockMutationScope(
	ctx context.Context, workspaceID, requirementID uint,
) (requirement.MutationScope, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || requirementID == 0 {
		return requirement.MutationScope{}, requirement.ErrMutationScopeInvalid
	}
	var row Requirement
	err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND id = ?", workspaceID, requirementID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirement.MutationScope{}, requirement.ErrMutationScopeNotFound
	}
	if err != nil {
		return requirement.MutationScope{}, err
	}
	if row.ScheduledPlanUnitID == nil || *row.ScheduledPlanUnitID == 0 {
		return requirement.MutationScope{}, requirement.ErrMutationScopeInvalid
	}
	return requirement.MutationScope{
		WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		PlanUnitID: *row.ScheduledPlanUnitID, RequirementID: row.ID, Version: row.Version,
	}, nil
}

var _ requirement.MutationScopeStore = (*MutationScopeRepository)(nil)
