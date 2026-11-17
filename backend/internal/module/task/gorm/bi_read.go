package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) ListBIFacts(ctx context.Context, workspaceID uint) ([]task.Snapshot, []task.ActivitySnapshot, error) {
	var rows []WorkTask
	query := repository.db.WithContext(ctx).Where("work_tasks.workspace_id = ? AND work_tasks.mutation_state = ?", workspaceID, workTaskStateActive).Where(`EXISTS (
		SELECT 1 FROM project_sets ps
		JOIN teams tm ON tm.id = ps.team_id AND tm.workspace_id = ps.workspace_id
		JOIN plan_units pu ON pu.id = work_tasks.project_id AND pu.workspace_id = ps.workspace_id AND pu.project_new_id = ps.id AND pu.team_id = ps.team_id
		JOIN requirements req ON req.id = work_tasks.requirement_id AND req.workspace_id = ps.workspace_id AND req.project_new_id = ps.id AND req.version_task_new_id = pu.id AND req.team_id = ps.team_id
		WHERE ps.id = work_tasks.project_set_id AND ps.workspace_id = ? AND work_tasks.workspace_id = ps.workspace_id AND work_tasks.team_id = ps.team_id
	)`, workspaceID)
	if err := query.Order("work_tasks.id").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	items := make([]task.Snapshot, 0, len(rows))
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		items = append(items, taskSnapshot(row))
		ids = append(ids, row.ID)
	}
	activities := []task.ActivitySnapshot{}
	if len(ids) > 0 {
		var activityRows []WorkTaskActivity
		if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND work_task_id IN ?", workspaceID, ids).
			Order("created_at ASC, id ASC").Find(&activityRows).Error; err != nil {
			return nil, nil, err
		}
		for _, row := range activityRows {
			activities = append(activities, task.ActivitySnapshot{ID: row.ID, WorkspaceID: row.WorkspaceID,
				WorkTaskID: row.WorkTaskID, UserID: row.UserID, Action: row.Action, Detail: row.Detail,
				FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Summary: row.Summary, CreatedAt: row.CreatedAt})
		}
	}
	return items, activities, nil
}

var _ task.BIReadStore = (*BIReadRepository)(nil)
