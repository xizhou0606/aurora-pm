package workbenchgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	"gorm.io/gorm"
)

type BIConfigurationReadRepository struct{ db *gorm.DB }

func NewBIConfigurationReadRepository(db *gorm.DB) *BIConfigurationReadRepository {
	return &BIConfigurationReadRepository{db: db}
}

func (repository *BIConfigurationReadRepository) GetBITeamMetricSettings(ctx context.Context, workspaceID uint) (workbench.BITeamMetricSettingsSnapshot, error) {
	var row BITeamMetricSettings
	err := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workbench.BITeamMetricSettingsSnapshot{}, nil
	}
	if err != nil {
		return workbench.BITeamMetricSettingsSnapshot{}, err
	}
	return workbench.BITeamMetricSettingsSnapshot{Configured: true,
		RequirementWorkflowTemplateID: row.RequirementWorkflowTemplateID, ApprovalNodeID: row.ApprovalNodeID}, nil
}

func (repository *BIConfigurationReadRepository) GetBITeamScoreProfile(ctx context.Context, workspaceID uint) (workbench.BITeamScoreProfileSnapshot, error) {
	var profile BITeamScoreProfile
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND status = ?", workspaceID, "active").
		Order("version DESC").First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workbench.BITeamScoreProfileSnapshot{}, nil
	}
	if err != nil {
		return workbench.BITeamScoreProfileSnapshot{}, err
	}
	var rows []BITeamScoreProfileItem
	if err := repository.db.WithContext(ctx).Where("profile_id = ?", profile.ID).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return workbench.BITeamScoreProfileSnapshot{}, err
	}
	result := workbench.BITeamScoreProfileSnapshot{ID: profile.ID, WorkspaceID: profile.WorkspaceID,
		Version: profile.Version, Status: profile.Status, ActivatedAt: profile.ActivatedAt,
		Items: make([]workbench.BITeamScoreProfileItemSnapshot, 0, len(rows))}
	for _, row := range rows {
		result.Items = append(result.Items, workbench.BITeamScoreProfileItemSnapshot{MetricKey: row.MetricKey,
			RoleKey: row.RoleKey, Enabled: row.Enabled, WeightBPS: row.WeightBPS, MissingPolicy: row.MissingPolicy,
			MinimumSamples: row.MinimumSamples, ParametersJSON: row.ParametersJSON, SortOrder: row.SortOrder})
	}
	return result, nil
}

var _ workbench.BIConfigurationReadStore = (*BIConfigurationReadRepository)(nil)
