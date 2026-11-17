package qualitygorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) ListBIFacts(ctx context.Context, workspaceID uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error) {
	var rows []QualityBug
	query := repository.db.WithContext(ctx).Where("quality_bugs.workspace_id = ?", workspaceID).Where(`EXISTS (
		SELECT 1 FROM project_sets ps
		JOIN teams tm ON tm.id = ps.team_id AND tm.workspace_id = ps.workspace_id
		JOIN plan_units pu ON pu.id = quality_bugs.project_id AND pu.workspace_id = ps.workspace_id AND pu.project_new_id = ps.id AND pu.team_id = ps.team_id
		JOIN requirements req ON req.id = quality_bugs.requirement_id AND req.workspace_id = ps.workspace_id AND req.project_new_id = ps.id AND req.version_task_new_id = pu.id AND req.team_id = ps.team_id
		WHERE ps.id = quality_bugs.project_set_id AND ps.workspace_id = ? AND quality_bugs.workspace_id = ps.workspace_id AND quality_bugs.team_id = ps.team_id
	)`, workspaceID)
	if err := query.Order("quality_bugs.id").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	items := make([]quality.BugSnapshot, 0, len(rows))
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		items = append(items, bugReadSnapshot(row))
		ids = append(ids, row.ID)
	}
	activities := []quality.BugActivitySnapshot{}
	if len(ids) > 0 {
		var activityRows []QualityBugActivity
		if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND quality_bug_id IN ?", workspaceID, ids).
			Order("created_at ASC, id ASC").Find(&activityRows).Error; err != nil {
			return nil, nil, err
		}
		for _, row := range activityRows {
			activities = append(activities, quality.BugActivitySnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID,
				QualityBugID: row.QualityBugID, UserID: row.UserID, Action: row.Action, FieldKey: row.FieldKey,
				OldValue: row.OldValue, NewValue: row.NewValue, Summary: row.Summary, CreatedAt: row.CreatedAt})
		}
	}
	return items, activities, nil
}

var _ quality.BIReadStore = (*BIReadRepository)(nil)
