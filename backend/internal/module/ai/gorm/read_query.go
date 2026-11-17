package aigorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
)

type RunReadRepository struct{ db *gorm.DB }

func NewRunReadRepository(db *gorm.DB) *RunReadRepository { return &RunReadRepository{db: db} }

func (repository *RunReadRepository) ListRunStatuses(ctx context.Context, workspaceID uint, ids []uint) ([]ai.RunStatus, error) {
	var rows []AIRun
	if err := repository.db.WithContext(ctx).Select("id", "status").
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ai.RunStatus, 0, len(rows))
	for _, row := range rows {
		items = append(items, ai.RunStatus{ID: row.ID, Status: row.Status})
	}
	return items, nil
}

var _ ai.RunReadStore = (*RunReadRepository)(nil)
