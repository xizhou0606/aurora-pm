package workflowgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type ProjectImpactRepository struct{ db *gorm.DB }

func NewProjectImpactRepository(db *gorm.DB) *ProjectImpactRepository {
	return &ProjectImpactRepository{db: db}
}

func (repository *ProjectImpactRepository) CountProjectSubjectReferences(
	ctx context.Context,
	workspaceID uint,
	subjects workflow.ProjectSubjects,
) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, workflow.ErrProjectImpactInvalid
	}
	query := repository.db.WithContext(ctx).Model(&WorkflowInstance{}).Where("workspace_id = ?", workspaceID)
	switch {
	case len(subjects.PlanUnitIDs) > 0 && len(subjects.RequirementIDs) > 0:
		query = query.Where(
			"(subject_type = ? AND subject_id IN ?) OR (subject_type = ? AND subject_id IN ?)",
			"plan_unit", subjects.PlanUnitIDs, "requirement", subjects.RequirementIDs,
		)
	case len(subjects.PlanUnitIDs) > 0:
		query = query.Where("subject_type = ? AND subject_id IN ?", "plan_unit", subjects.PlanUnitIDs)
	case len(subjects.RequirementIDs) > 0:
		query = query.Where("subject_type = ? AND subject_id IN ?", "requirement", subjects.RequirementIDs)
	default:
		return 0, nil
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

var _ workflow.ProjectImpactStore = (*ProjectImpactRepository)(nil)
