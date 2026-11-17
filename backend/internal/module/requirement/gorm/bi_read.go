package requirementgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) ListBIFacts(ctx context.Context, workspaceID uint) ([]requirement.Snapshot, error) {
	var rows []Requirement
	query := repository.db.WithContext(ctx).Where("requirements.workspace_id = ?", workspaceID).Where(`EXISTS (
		SELECT 1 FROM project_sets ps
		JOIN teams tm ON tm.id = ps.team_id AND tm.workspace_id = ps.workspace_id
		JOIN plan_units pu ON pu.id = requirements.version_task_new_id AND pu.workspace_id = ps.workspace_id AND pu.project_new_id = ps.id AND pu.team_id = ps.team_id
		WHERE ps.id = requirements.project_new_id AND ps.workspace_id = ? AND requirements.workspace_id = ps.workspace_id AND requirements.team_id = ps.team_id
	)`, workspaceID)
	if err := query.Order("requirements.id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]requirement.Snapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, requirementSnapshot(row))
	}
	return items, nil
}

var _ requirement.BIReadStore = (*BIReadRepository)(nil)
