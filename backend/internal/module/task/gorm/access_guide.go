package taskgorm

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

type AccessGuideRepository struct{ db *gorm.DB }

func NewAccessGuideRepository(db *gorm.DB) *AccessGuideRepository {
	return &AccessGuideRepository{db: db}
}
func (r *AccessGuideRepository) LocateProject(ctx context.Context, id uint) (uint, uint, error) {
	var row WorkTask
	err := r.db.WithContext(ctx).Select("workspace_id", "project_id").First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, nil
	}
	return row.WorkspaceID, row.ProjectID, err
}
