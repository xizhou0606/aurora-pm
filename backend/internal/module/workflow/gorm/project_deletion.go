package workflowgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type ProjectDeletionRepository struct{ db *gorm.DB }

func NewProjectDeletionRepository(db *gorm.DB) *ProjectDeletionRepository {
	return &ProjectDeletionRepository{db: db}
}

func (repository *ProjectDeletionRepository) DeleteProjectSubjects(
	ctx context.Context,
	workspaceID uint,
	subjects workflow.ProjectSubjects,
) error {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return workflow.ErrProjectDeletionInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&WorkflowInstance{}).Where("workspace_id = ?", workspaceID)
		switch {
		case len(subjects.PlanUnitIDs) > 0 && len(subjects.RequirementIDs) > 0:
			query = query.Where("(subject_type = ? AND subject_id IN ?) OR (subject_type = ? AND subject_id IN ?)",
				"plan_unit", subjects.PlanUnitIDs, "requirement", subjects.RequirementIDs)
		case len(subjects.PlanUnitIDs) > 0:
			query = query.Where("subject_type = ? AND subject_id IN ?", "plan_unit", subjects.PlanUnitIDs)
		case len(subjects.RequirementIDs) > 0:
			query = query.Where("subject_type = ? AND subject_id IN ?", "requirement", subjects.RequirementIDs)
		default:
			return nil
		}
		var instanceIDs []uint
		if err := query.Order("id").Pluck("id", &instanceIDs).Error; err != nil {
			return err
		}
		return DeleteInstanceRowsTx(tx, workspaceID, instanceIDs)
	})
}

var _ workflow.ProjectDeletionStore = (*ProjectDeletionRepository)(nil)

func (repository *ProjectDeletionRepository) PlanUnitHasExecutionHistory(ctx context.Context, workspaceID, planUnitID uint) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return false, workflow.ErrProjectDeletionInvalid
	}
	var ids []uint
	if err := repository.db.WithContext(ctx).Model(&WorkflowInstance{}).Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "plan_unit", planUnitID).Pluck("id", &ids).Error; err != nil {
		return false, err
	}
	return InstancesHaveExecutionHistoryTx(repository.db.WithContext(ctx), workspaceID, ids)
}
func (repository *ProjectDeletionRepository) DeletePlanUnitScaffolding(ctx context.Context, workspaceID, planUnitID uint) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return workflow.ErrProjectDeletionInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids, err := SubjectInstanceIDsTx(tx, workspaceID, "plan_unit", planUnitID)
		if err != nil {
			return err
		}
		if err := GateInstancesTx(tx, workspaceID, ids); err != nil {
			return err
		}
		history, err := InstancesHaveExecutionHistoryTx(tx, workspaceID, ids)
		if err != nil {
			return err
		}
		if history {
			return workflow.ErrProjectDeletionInvalid
		}
		return DeleteInstanceRowsAfterGateTx(tx, workspaceID, ids)
	})
}
func (repository *ProjectDeletionRepository) SubjectHasExecutionHistory(ctx context.Context, workspaceID uint, subjectType string, subjectID uint) (bool, error) {
	ids, err := SubjectInstanceIDsTx(repository.db.WithContext(ctx), workspaceID, subjectType, subjectID)
	if err != nil {
		return false, err
	}
	return InstancesHaveExecutionHistoryTx(repository.db.WithContext(ctx), workspaceID, ids)
}
func (repository *ProjectDeletionRepository) DeleteSubjectScaffolding(ctx context.Context, workspaceID uint, subjectType string, subjectID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids, err := SubjectInstanceIDsTx(tx, workspaceID, subjectType, subjectID)
		if err != nil {
			return err
		}
		if err := GateInstancesTx(tx, workspaceID, ids); err != nil {
			return err
		}
		history, err := InstancesHaveExecutionHistoryTx(tx, workspaceID, ids)
		if err != nil {
			return err
		}
		if history {
			return workflow.ErrProjectDeletionInvalid
		}
		return DeleteInstanceRowsAfterGateTx(tx, workspaceID, ids)
	})
}
