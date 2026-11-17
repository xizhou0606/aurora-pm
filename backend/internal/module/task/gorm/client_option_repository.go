package taskgorm

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

type TaskClientOption struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID     uint      `gorm:"not null;index;uniqueIndex:ux_task_client_option_scope" json:"workspace_id"`
	Name            string    `gorm:"size:64;not null" json:"name"`
	NormalizedName  string    `gorm:"size:64;not null;uniqueIndex:ux_task_client_option_scope" json:"-"`
	CreatedByUserID uint      `gorm:"not null;index" json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ClientOptionRepository struct{ db *gorm.DB }

func NewClientOptionRepository(db *gorm.DB) *ClientOptionRepository {
	return &ClientOptionRepository{db: db}
}

func (repository *ClientOptionRepository) EnsureDefaults(ctx context.Context, workspaceID, actorUserID uint, names []string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, raw := range names {
			name, normalized, err := task.NormalizeClientOptionName(raw)
			if err != nil {
				return err
			}
			row := TaskClientOption{WorkspaceID: workspaceID, Name: name, NormalizedName: normalized, CreatedByUserID: actorUserID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *ClientOptionRepository) List(ctx context.Context, workspaceID uint) ([]task.ClientOptionRecord, error) {
	var rows []TaskClientOption
	if err := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]task.ClientOptionRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, task.ClientOptionRecord{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName})
	}
	return result, nil
}

func (repository *ClientOptionRepository) UsageCounts(ctx context.Context, workspaceID uint) (map[uint]int64, error) {
	type usageRow struct {
		TaskClientID uint
		UsageCount   int64
	}
	var rows []usageRow
	if err := repository.db.WithContext(ctx).Model(&WorkTask{}).
		Select("task_client_id, COUNT(*) AS usage_count").
		Where("workspace_id = ? AND task_client_id IS NOT NULL", workspaceID).
		Group("task_client_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]int64, len(rows))
	for _, row := range rows {
		result[row.TaskClientID] = row.UsageCount
	}
	return result, nil
}

func (repository *ClientOptionRepository) Create(ctx context.Context, workspaceID, actorUserID uint, name, normalized string) (task.ClientOptionRecord, error) {
	row := TaskClientOption{WorkspaceID: workspaceID, Name: name, NormalizedName: normalized, CreatedByUserID: actorUserID}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return task.ClientOptionRecord{}, normalizeClientOptionError(err)
	}
	return task.ClientOptionRecord{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName}, nil
}

func (repository *ClientOptionRepository) Update(ctx context.Context, workspaceID, id uint, name, normalized string) (task.ClientOptionRecord, error) {
	result := repository.db.WithContext(ctx).Model(&TaskClientOption{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(map[string]any{"name": name, "normalized_name": normalized})
	if result.Error != nil {
		return task.ClientOptionRecord{}, normalizeClientOptionError(result.Error)
	}
	if result.RowsAffected != 1 {
		return task.ClientOptionRecord{}, task.ErrClientOptionNotFound
	}
	return task.ClientOptionRecord{ID: id, Name: name, NormalizedName: normalized}, nil
}

func (repository *ClientOptionRepository) Delete(ctx context.Context, workspaceID, id uint) error {
	result := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&TaskClientOption{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return task.ErrClientOptionNotFound
	}
	return nil
}

func normalizeClientOptionError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return task.ErrClientOptionNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") || strings.Contains(message, "duplicate") {
		return task.ErrClientOptionDuplicate
	}
	return err
}
