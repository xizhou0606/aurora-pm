package requirementgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

type ReadRepository struct{ db *gorm.DB }

func NewReadRepository(db *gorm.DB) *ReadRepository { return &ReadRepository{db: db} }

func (repository *ReadRepository) GetRequirement(ctx context.Context, workspaceID, id uint) (requirement.Snapshot, error) {
	var row Requirement
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return requirement.Snapshot{}, requirement.ErrRequirementNotFound
	}
	if err != nil {
		return requirement.Snapshot{}, err
	}
	return requirementSnapshot(row), nil
}

func uintSlice(values UintList) []uint {
	if values == nil {
		return []uint{}
	}
	return append([]uint(nil), values...)
}
