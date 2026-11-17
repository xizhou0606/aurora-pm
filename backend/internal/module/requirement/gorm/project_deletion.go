package requirementgorm

import (
	"context"
	"sort"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

type ProjectDeletionRepository struct{ db *gorm.DB }

func NewProjectDeletionRepository(db *gorm.DB) *ProjectDeletionRepository {
	return &ProjectDeletionRepository{db: db}
}

func (repository *ProjectDeletionRepository) DeleteProjectRequirements(ctx context.Context, workspaceID, projectSetID uint, expectedIDs []uint) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 {
		return requirement.ErrProjectDeletionInvalid
	}
	expectedIDs = canonicalProjectRequirementIDs(expectedIDs)
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var currentIDs []uint
		if err := tx.Model(&Requirement{}).Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectSetID).
			Order("id").Pluck("id", &currentIDs).Error; err != nil {
			return err
		}
		// Empty current state is the durable replay of an already committed
		// deletion step. A non-empty partial/different snapshot fails closed.
		if len(currentIDs) > 0 && !sameProjectRequirementIDs(currentIDs, expectedIDs) {
			return requirement.ErrProjectDeletionInvalid
		}
		if len(currentIDs) == 0 {
			return nil
		}
		for _, requirementID := range expectedIDs {
			if err := MarkCoverDeletedTx(tx, workspaceID, requirementID); err != nil {
				return err
			}
		}
		if len(expectedIDs) == 0 {
			return nil
		}
		if err := tx.Where("workspace_id = ? AND requirement_id IN ?", workspaceID, expectedIDs).Delete(&RequirementMaterial{}).Error; err != nil {
			return err
		}
		return tx.Where("workspace_id = ? AND project_new_id = ? AND id IN ?", workspaceID, projectSetID, expectedIDs).Delete(&Requirement{}).Error
	})
}

func canonicalProjectRequirementIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(values))
	for _, value := range values {
		if value > 0 {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				out = append(out, value)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func sameProjectRequirementIDs(left, right []uint) bool {
	left, right = canonicalProjectRequirementIDs(left), canonicalProjectRequirementIDs(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

var _ requirement.ProjectDeletionStore = (*ProjectDeletionRepository)(nil)
