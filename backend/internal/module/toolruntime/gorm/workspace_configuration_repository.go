package toolruntimegorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkspaceConfigurationRepository struct {
	db *gorm.DB
}

func NewWorkspaceConfigurationRepository(db *gorm.DB) *WorkspaceConfigurationRepository {
	return &WorkspaceConfigurationRepository{db: db}
}

func (repository *WorkspaceConfigurationRepository) FindByWorkspace(
	ctx context.Context,
	workspaceID uint,
) (toolruntime.WorkspaceConfiguration, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return toolruntime.WorkspaceConfiguration{}, toolruntime.ErrWorkspaceConfigurationNotFound
	}
	var row ExternalToolingWorkspaceConfiguration
	if err := repository.db.WithContext(ctx).First(&row, "workspace_id = ?", workspaceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return toolruntime.WorkspaceConfiguration{}, toolruntime.ErrWorkspaceConfigurationNotFound
		}
		return toolruntime.WorkspaceConfiguration{}, err
	}
	return workspaceConfigurationDomain(row)
}

func (repository *WorkspaceConfigurationRepository) Save(
	ctx context.Context,
	configuration toolruntime.WorkspaceConfiguration,
) (toolruntime.WorkspaceConfiguration, error) {
	if repository == nil || repository.db == nil || configuration.Validate() != nil {
		return toolruntime.WorkspaceConfiguration{}, toolruntime.ErrWorkspaceConfigurationInvalid
	}
	row, err := workspaceConfigurationRecord(configuration)
	if err != nil {
		return toolruntime.WorkspaceConfiguration{}, err
	}
	err = repository.db.WithContext(ctx).Clauses(workspaceConfigurationOnConflict(row)).Create(&row).Error
	if err != nil {
		return toolruntime.WorkspaceConfiguration{}, err
	}
	return repository.FindByWorkspace(ctx, configuration.WorkspaceID)
}

func workspaceConfigurationOnConflict(row ExternalToolingWorkspaceConfiguration) clause.OnConflict {
	return clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"mode": row.Mode, "allowed_tools": row.AllowedTools, "allowed_scopes": row.AllowedScopes,
			"effective_from": row.EffectiveFrom, "expires_at": row.ExpiresAt,
			"cli_grant_ttl_hours": row.CLIGrantTTLHours,
			"updated_by":          row.UpdatedBy, "updated_at": row.UpdatedAt,
			// PostgreSQL exposes both the target row and EXCLUDED row inside
			// ON CONFLICT. Qualify the target version so the increment is not
			// ambiguous on the production database dialect.
			"version": gorm.Expr("pm_external_tooling_workspace_configurations.version + 1"),
		}),
	}
}

func workspaceConfigurationRecord(
	configuration toolruntime.WorkspaceConfiguration,
) (ExternalToolingWorkspaceConfiguration, error) {
	tools, err := json.Marshal(configuration.AllowedTools)
	if err != nil {
		return ExternalToolingWorkspaceConfiguration{}, fmt.Errorf("%w: tools cannot be encoded", toolruntime.ErrWorkspaceConfigurationInvalid)
	}
	scopes, err := json.Marshal(configuration.AllowedScopes)
	if err != nil {
		return ExternalToolingWorkspaceConfiguration{}, fmt.Errorf("%w: scopes cannot be encoded", toolruntime.ErrWorkspaceConfigurationInvalid)
	}
	var effectiveFrom, expiresAt *time.Time
	if configuration.Mode == toolruntime.WorkspaceConfigurationWindow {
		effective, expires := configuration.EffectiveFrom.UTC(), configuration.ExpiresAt.UTC()
		effectiveFrom, expiresAt = &effective, &expires
	}
	return ExternalToolingWorkspaceConfiguration{
		WorkspaceID: configuration.WorkspaceID, Mode: string(configuration.Mode),
		AllowedTools: string(tools), AllowedScopes: string(scopes),
		EffectiveFrom: effectiveFrom, ExpiresAt: expiresAt,
		CLIGrantTTLHours: configuration.EffectiveCLIGrantTTLHours(),
		UpdatedBy:        configuration.UpdatedBy, Version: configuration.Version,
		CreatedAt: configuration.CreatedAt.UTC(), UpdatedAt: configuration.UpdatedAt.UTC(),
	}, nil
}

func workspaceConfigurationDomain(
	row ExternalToolingWorkspaceConfiguration,
) (toolruntime.WorkspaceConfiguration, error) {
	var tools, scopes []string
	if json.Unmarshal([]byte(row.AllowedTools), &tools) != nil || json.Unmarshal([]byte(row.AllowedScopes), &scopes) != nil {
		return toolruntime.WorkspaceConfiguration{}, toolruntime.ErrWorkspaceConfigurationInvalid
	}
	configuration := toolruntime.WorkspaceConfiguration{
		WorkspaceID: row.WorkspaceID, Mode: toolruntime.WorkspaceConfigurationMode(row.Mode),
		AllowedTools: tools, AllowedScopes: scopes, UpdatedBy: row.UpdatedBy,
		CLIGrantTTLHours: row.CLIGrantTTLHours,
		CreatedAt:        row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), Version: row.Version,
	}
	if row.EffectiveFrom != nil {
		configuration.EffectiveFrom = row.EffectiveFrom.UTC()
	}
	if row.ExpiresAt != nil {
		configuration.ExpiresAt = row.ExpiresAt.UTC()
	}
	if err := configuration.Validate(); err != nil {
		return toolruntime.WorkspaceConfiguration{}, err
	}
	return configuration, nil
}

var _ toolruntime.WorkspaceConfigurationRepository = (*WorkspaceConfigurationRepository)(nil)
