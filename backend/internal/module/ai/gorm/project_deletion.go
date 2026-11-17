package aigorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
)

type ProjectDeletionRepository struct{ db *gorm.DB }

func NewProjectDeletionRepository(db *gorm.DB) *ProjectDeletionRepository {
	return &ProjectDeletionRepository{db: db}
}

func (repository *ProjectDeletionRepository) DeleteProjectSubjects(ctx context.Context, workspaceID uint, subjects ai.ProjectSubjects) error {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return ai.ErrProjectDeletionInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&AIRun{}).Where("workspace_id = ?", workspaceID)
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
		var runIDs []uint
		if err := query.Order("id").Pluck("id", &runIDs).Error; err != nil || len(runIDs) == 0 {
			return err
		}
		// AIRun is the worker CAS gate. Delete it first, then sweep children in
		// the same owning-module transaction. Replaying the step is a no-op.
		if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, runIDs).Delete(&AIRun{}).Error; err != nil {
			return err
		}
		for _, row := range []any{&AIArtifactApplication{}, &AIFeedback{}, &AIArtifact{}, &AIRunSource{}} {
			if err := tx.Where("workspace_id = ? AND run_id IN ?", workspaceID, runIDs).Delete(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var _ ai.ProjectDeletionStore = (*ProjectDeletionRepository)(nil)
