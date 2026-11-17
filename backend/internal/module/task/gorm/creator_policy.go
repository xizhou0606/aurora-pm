package taskgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

type CreatorPolicyRepository struct{ db *gorm.DB }

func NewCreatorPolicyRepository(db *gorm.DB) *CreatorPolicyRepository {
	return &CreatorPolicyRepository{db: db}
}

func (repository *CreatorPolicyRepository) AllCreatedBy(
	ctx context.Context,
	workspaceID, userID uint,
	taskIDs []uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 || len(taskIDs) == 0 {
		return false, task.ErrCreatorPolicyInvalid
	}
	creators, err := repository.CreatorIDs(ctx, workspaceID, taskIDs)
	if err != nil {
		return false, err
	}
	for _, taskID := range taskIDs {
		if creators[taskID] == 0 || creators[taskID] != userID {
			return false, nil
		}
	}
	return true, nil
}

func (repository *CreatorPolicyRepository) CreatorIDs(
	ctx context.Context,
	workspaceID uint,
	taskIDs []uint,
) (map[uint]uint, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || len(taskIDs) == 0 {
		return nil, task.ErrCreatorPolicyInvalid
	}
	type creatorRow struct {
		WorkTaskID uint `gorm:"column:work_task_id"`
		UserID     uint `gorm:"column:user_id"`
	}
	var rows []creatorRow
	err := repository.db.WithContext(ctx).Raw(`
		SELECT ranked.work_task_id, ranked.user_id
		FROM (
			SELECT work_task_id, user_id,
				ROW_NUMBER() OVER (PARTITION BY work_task_id ORDER BY created_at ASC, id ASC) AS creator_rank
			FROM work_task_activities
			WHERE workspace_id = ? AND work_task_id IN ?
				AND LOWER(TRIM(action)) IN ('created', 'cloned')
		) ranked
		WHERE ranked.creator_rank = 1`, workspaceID, taskIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	creators := make(map[uint]uint, len(rows))
	for _, row := range rows {
		creators[row.WorkTaskID] = row.UserID
	}
	return creators, nil
}

var _ task.CreatorPolicyStore = (*CreatorPolicyRepository)(nil)
