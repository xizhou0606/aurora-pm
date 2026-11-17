package codecontribgorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
	"gorm.io/gorm"
)

// AdminRepository implements the configuration stores. Every method scopes by
// workspace; cross-workspace access is impossible by construction.
type AdminRepository struct{ db *gorm.DB }

func NewAdminRepository(db *gorm.DB) *AdminRepository { return &AdminRepository{db: db} }

var errNotFound = errors.New("codecontrib record not found")

func (repository *AdminRepository) ListConnections(ctx context.Context, workspaceID uint) ([]codecontrib.Connection, error) {
	var rows []Connection
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	connections := make([]codecontrib.Connection, 0, len(rows))
	for _, row := range rows {
		connections = append(connections, connectionModel(row))
	}
	return connections, nil
}

func (repository *AdminRepository) GetConnection(ctx context.Context, workspaceID, id uint) (codecontrib.Connection, error) {
	var row Connection
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.Connection{}, errNotFound
	}
	if err != nil {
		return codecontrib.Connection{}, err
	}
	return connectionModel(row), nil
}

func (repository *AdminRepository) CreateConnection(ctx context.Context, connection codecontrib.Connection) (codecontrib.Connection, error) {
	row := connectionRow(connection)
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return codecontrib.Connection{}, err
	}
	return connectionModel(row), nil
}

func (repository *AdminRepository) UpdateConnection(ctx context.Context, connection codecontrib.Connection) (codecontrib.Connection, error) {
	row := connectionRow(connection)
	result := repository.db.WithContext(ctx).Model(&Connection{}).
		Where("workspace_id = ? AND id = ?", connection.WorkspaceID, connection.ID).
		Updates(connectionUpdateMap(row))
	if result.Error != nil {
		return codecontrib.Connection{}, result.Error
	}
	if result.RowsAffected == 0 {
		return codecontrib.Connection{}, errNotFound
	}
	return repository.GetConnection(ctx, connection.WorkspaceID, connection.ID)
}

func (repository *AdminRepository) DeleteConnection(ctx context.Context, workspaceID, id uint) error {
	result := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&Connection{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (repository *AdminRepository) ListRepositories(ctx context.Context, workspaceID uint) ([]codecontrib.RepositoryConfig, error) {
	var rows []Repository
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	repositories := make([]codecontrib.RepositoryConfig, 0, len(rows))
	for _, row := range rows {
		item, err := repositoryModel(row)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, item)
	}
	return repositories, nil
}

func (repository *AdminRepository) ListEnabledRepositories(ctx context.Context) ([]codecontrib.RepositoryConfig, error) {
	var rows []Repository
	if err := repository.db.WithContext(ctx).
		Where("enabled = ?", true).Order("workspace_id, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	repositories := make([]codecontrib.RepositoryConfig, 0, len(rows))
	for _, row := range rows {
		item, err := repositoryModel(row)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, item)
	}
	return repositories, nil
}

func (repository *AdminRepository) GetRepository(ctx context.Context, workspaceID, id uint) (codecontrib.RepositoryConfig, error) {
	var row Repository
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.RepositoryConfig{}, errNotFound
	}
	if err != nil {
		return codecontrib.RepositoryConfig{}, err
	}
	return repositoryModel(row)
}

func (repository *AdminRepository) CreateRepository(ctx context.Context, config codecontrib.RepositoryConfig) (codecontrib.RepositoryConfig, error) {
	row, err := repositoryRow(config)
	if err != nil {
		return codecontrib.RepositoryConfig{}, err
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return codecontrib.RepositoryConfig{}, err
	}
	return repositoryModel(row)
}

func (repository *AdminRepository) UpdateRepository(ctx context.Context, config codecontrib.RepositoryConfig) (codecontrib.RepositoryConfig, error) {
	row, err := repositoryRow(config)
	if err != nil {
		return codecontrib.RepositoryConfig{}, err
	}
	result := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", config.WorkspaceID, config.ID).
		Updates(map[string]any{
			"connection_id": row.ConnectionID, "external_id": row.ExternalID, "path": row.Path,
			"default_branch": row.DefaultBranch, "filter_rules_json": row.FilterRulesJSON,
			"project_set_id": row.ProjectSetID, "enabled": row.Enabled,
			"updated_by": row.UpdatedBy, "version": row.Version, "updated_at": row.UpdatedAt,
		})
	if result.Error != nil {
		return codecontrib.RepositoryConfig{}, result.Error
	}
	if result.RowsAffected == 0 {
		return codecontrib.RepositoryConfig{}, errNotFound
	}
	return repository.GetRepository(ctx, config.WorkspaceID, config.ID)
}

func (repository *AdminRepository) DeleteRepository(ctx context.Context, workspaceID, id uint) error {
	result := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&Repository{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errNotFound
	}
	return nil
}

func (repository *AdminRepository) GetSettings(ctx context.Context, workspaceID uint) (codecontrib.CollectionConfig, bool, error) {
	var row CollectionSetting
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.CollectionConfig{}, false, nil
	}
	if err != nil {
		return codecontrib.CollectionConfig{}, false, err
	}
	config, err := settingsModel(row)
	return config, true, err
}

func (repository *AdminRepository) SaveSettings(ctx context.Context, workspaceID uint, config codecontrib.CollectionConfig) (codecontrib.CollectionConfig, error) {
	row, err := settingsRow(workspaceID, config)
	if err != nil {
		return codecontrib.CollectionConfig{}, err
	}
	// GORM Save 以零主键走 INSERT；这里补回存量行主键，让第二次及以后的
	// 保存走 UPDATE，而不是撞工作区唯一索引（历史实现只在首存时可用）。
	var existing CollectionSetting
	err = repository.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).First(&existing).Error
	if err == nil {
		row.ID = existing.ID
		row.CreatedAt = existing.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return codecontrib.CollectionConfig{}, err
	}
	if err := repository.db.WithContext(ctx).Save(&row).Error; err != nil {
		return codecontrib.CollectionConfig{}, err
	}
	return settingsModel(row)
}

var _ codecontrib.ConnectionStore = (*AdminRepository)(nil)
var _ codecontrib.RepositoryStore = (*AdminRepository)(nil)
var _ codecontrib.SettingsStore = (*AdminRepository)(nil)

// connectionUpdateMap treats an empty token as "keep the stored one" so a
// metadata-only admin update can never wipe a credential.
func connectionUpdateMap(row Connection) map[string]any {
	updates := map[string]any{
		"name": row.Name, "provider": row.Provider, "base_url": row.BaseURL,
		"enabled": row.Enabled, "updated_by": row.UpdatedBy,
		"version": row.Version, "updated_at": row.UpdatedAt,
	}
	if row.Token != "" {
		updates["token"] = row.Token
	}
	return updates
}

func connectionModel(row Connection) codecontrib.Connection {
	return codecontrib.Connection{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Name: row.Name,
		Provider: codecontrib.Provider(row.Provider), BaseURL: row.BaseURL,
		Token: row.Token, Enabled: row.Enabled, CreatedBy: row.CreatedBy,
		UpdatedBy: row.UpdatedBy, Version: row.Version,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func connectionRow(model codecontrib.Connection) Connection {
	return Connection{
		ID: model.ID, WorkspaceID: model.WorkspaceID, Name: model.Name,
		Provider: string(model.Provider), BaseURL: model.BaseURL, Token: model.Token,
		Enabled: model.Enabled, CreatedBy: model.CreatedBy, UpdatedBy: model.UpdatedBy,
		Version: model.Version, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func repositoryModel(row Repository) (codecontrib.RepositoryConfig, error) {
	rules := codecontrib.FilterRules{}
	if err := json.Unmarshal([]byte(row.FilterRulesJSON), &rules); err != nil {
		return codecontrib.RepositoryConfig{}, err
	}
	return codecontrib.RepositoryConfig{
		ID: row.ID, WorkspaceID: row.WorkspaceID, ConnectionID: row.ConnectionID,
		ExternalID: row.ExternalID, Path: row.Path, DefaultBranch: row.DefaultBranch,
		FilterRules: rules, ProjectSetID: row.ProjectSetID, Enabled: row.Enabled,
		CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, Version: row.Version,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func repositoryRow(model codecontrib.RepositoryConfig) (Repository, error) {
	rules, err := json.Marshal(model.FilterRules)
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		ID: model.ID, WorkspaceID: model.WorkspaceID, ConnectionID: model.ConnectionID,
		ExternalID: model.ExternalID, Path: model.Path, DefaultBranch: model.DefaultBranch,
		FilterRulesJSON: string(rules), ProjectSetID: model.ProjectSetID, Enabled: model.Enabled,
		CreatedBy: model.CreatedBy, UpdatedBy: model.UpdatedBy, Version: model.Version,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}, nil
}

func settingsModel(row CollectionSetting) (codecontrib.CollectionConfig, error) {
	patterns := []string{}
	if err := json.Unmarshal([]byte(row.BotPatternsJSON), &patterns); err != nil {
		return codecontrib.CollectionConfig{}, err
	}
	return codecontrib.CollectionConfig{
		Timezone: row.Timezone, CoauthorPolicy: row.CoauthorPolicy,
		BotPatterns: patterns, PageCap: row.PageCap,
		DailySyncHour: row.DailySyncHour, DailySyncMinute: row.DailySyncMinute,
		WeeklySyncWeekday: row.WeeklySyncWeekday, WeeklySyncHour: row.WeeklySyncHour,
		WeeklySyncMinute: row.WeeklySyncMinute,
		MonthlySyncDay:   row.MonthlySyncDay, MonthlySyncHour: row.MonthlySyncHour,
		MonthlySyncMinute: row.MonthlySyncMinute,
		Version:           row.Version, UpdatedBy: row.UpdatedBy,
	}, nil
}

func settingsRow(workspaceID uint, model codecontrib.CollectionConfig) (CollectionSetting, error) {
	patterns, err := json.Marshal(model.BotPatterns)
	if err != nil {
		return CollectionSetting{}, err
	}
	return CollectionSetting{
		WorkspaceID: workspaceID, Timezone: model.Timezone,
		CoauthorPolicy: model.CoauthorPolicy, BotPatternsJSON: string(patterns),
		PageCap: model.PageCap,
		DailySyncHour: model.DailySyncHour, DailySyncMinute: model.DailySyncMinute,
		WeeklySyncWeekday: model.WeeklySyncWeekday, WeeklySyncHour: model.WeeklySyncHour,
		WeeklySyncMinute: model.WeeklySyncMinute,
		MonthlySyncDay:   model.MonthlySyncDay, MonthlySyncHour: model.MonthlySyncHour,
		MonthlySyncMinute: model.MonthlySyncMinute,
		Version:           model.Version, UpdatedBy: model.UpdatedBy,
		UpdatedAt: time.Now(),
	}, nil
}
