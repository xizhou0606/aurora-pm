package toolruntimegorm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

type GrantRepository struct {
	db *gorm.DB
}

func NewGrantRepository(db *gorm.DB) *GrantRepository {
	return &GrantRepository{db: db}
}

func (repository *GrantRepository) Create(ctx context.Context, grant *toolruntime.Grant) error {
	if grant == nil {
		return fmt.Errorf("%w: grant is required", toolruntime.ErrGrantInvalid)
	}
	if err := grant.Validate(); err != nil {
		return err
	}
	row, err := grantRecord(*grant)
	if err != nil {
		return err
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	value, err := grantDomain(row)
	if err != nil {
		return err
	}
	*grant = value
	return nil
}

func (repository *GrantRepository) FindByID(ctx context.Context, id uint) (toolruntime.Grant, error) {
	if id == 0 {
		return toolruntime.Grant{}, fmt.Errorf("%w: grant id is required", toolruntime.ErrGrantInvalid)
	}
	var row ExternalAuthorizationGrant
	if err := repository.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return toolruntime.Grant{}, err
	}
	return grantDomain(row)
}

func (repository *GrantRepository) ListForPrincipalWorkspace(ctx context.Context, principalID, workspaceID uint) ([]toolruntime.Grant, error) {
	if principalID == 0 || workspaceID == 0 {
		return nil, fmt.Errorf("%w: principal and workspace are required", toolruntime.ErrGrantInvalid)
	}
	var rows []ExternalAuthorizationGrant
	if err := repository.db.WithContext(ctx).
		Where("principal_id = ? AND workspace_id = ?", principalID, workspaceID).
		Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	grants := make([]toolruntime.Grant, 0, len(rows))
	for _, row := range rows {
		grant, err := grantDomain(row)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func (repository *GrantRepository) Revoke(ctx context.Context, id uint, at time.Time) (toolruntime.Grant, error) {
	if id == 0 || at.IsZero() {
		return toolruntime.Grant{}, fmt.Errorf("%w: grant id and revocation time are required", toolruntime.ErrGrantInvalid)
	}
	current, err := repository.FindByID(ctx, id)
	if err != nil {
		return toolruntime.Grant{}, err
	}
	if current.RevokedAt != nil {
		return current, nil
	}
	if err := current.Revoke(at); err != nil {
		return toolruntime.Grant{}, err
	}
	result := repository.db.WithContext(ctx).Model(&ExternalAuthorizationGrant{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": current.RevokedAt, "updated_at": current.UpdatedAt})
	if result.Error != nil {
		return toolruntime.Grant{}, result.Error
	}
	return repository.FindByID(ctx, id)
}

func grantRecord(grant toolruntime.Grant) (ExternalAuthorizationGrant, error) {
	tools, err := json.Marshal(grant.AllowedTools)
	if err != nil {
		return ExternalAuthorizationGrant{}, fmt.Errorf("%w: tools cannot be encoded", toolruntime.ErrGrantInvalid)
	}
	scopes, err := json.Marshal(grant.AllowedScopes)
	if err != nil {
		return ExternalAuthorizationGrant{}, fmt.Errorf("%w: scopes cannot be encoded", toolruntime.ErrGrantInvalid)
	}
	return ExternalAuthorizationGrant{
		ID: grant.ID, PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID,
		ClientID: grant.ClientID, Channel: string(grant.Channel), AllowedTools: string(tools), AllowedScopes: string(scopes),
		EffectiveFrom: grant.EffectiveFrom.UTC(), ExpiresAt: grant.ExpiresAt.UTC(),
		RevokedAt: grant.RevokedAt, CreatedAt: grant.CreatedAt.UTC(), UpdatedAt: grant.UpdatedAt.UTC(),
	}, nil
}

func grantDomain(row ExternalAuthorizationGrant) (toolruntime.Grant, error) {
	if row.ID == 0 {
		return toolruntime.Grant{}, fmt.Errorf("%w: persisted grant id is missing", toolruntime.ErrGrantInvalid)
	}
	var tools, scopes []string
	if json.Unmarshal([]byte(row.AllowedTools), &tools) != nil || json.Unmarshal([]byte(row.AllowedScopes), &scopes) != nil {
		return toolruntime.Grant{}, fmt.Errorf("%w: persisted entitlements are malformed", toolruntime.ErrGrantInvalid)
	}
	grant := toolruntime.Grant{
		ID: row.ID, PrincipalID: row.PrincipalID, WorkspaceID: row.WorkspaceID,
		ClientID: row.ClientID, Channel: toolruntime.DeliveryChannel(row.Channel), AllowedTools: tools, AllowedScopes: scopes,
		EffectiveFrom: row.EffectiveFrom, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if err := grant.Validate(); err != nil {
		return toolruntime.Grant{}, fmt.Errorf("%w: persisted grant failed validation", err)
	}
	return grant, nil
}

var _ toolruntime.GrantRepository = (*GrantRepository)(nil)
var _ toolruntime.GrantListRepository = (*GrantRepository)(nil)
