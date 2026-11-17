package identitygorm

import (
	"context"
	"errors"
	"strings"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *AccessRepository) ListManagedWorkspaces(ctx context.Context, principalID uint, includeDisabled bool) ([]identity.ManagedWorkspace, error) {
	administrator, err := isAccessAdministrator(ctx, repository.db, principalID)
	if err != nil {
		return nil, err
	}
	query := repository.db.WithContext(ctx).Table("workspaces AS w")
	if !includeDisabled {
		query = query.Where("w.enabled = ?", true)
	}
	if !administrator {
		query = query.Joins(
			"JOIN workspace_members AS visible_wm ON visible_wm.workspace_id = w.id AND visible_wm.user_id = ? AND visible_wm.enabled = ?",
			principalID,
			true,
		)
	}
	var items []identity.ManagedWorkspace
	err = query.Select(`w.id, w.name, w.slug, w.description, w.enabled, w.is_default,
        w.created_at, w.updated_at,
        (SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = w.id) AS member_count`).
		Order("w.is_default DESC, w.name ASC, w.id ASC").Scan(&items).Error
	return items, err
}

func (repository *AccessRepository) FindManagedWorkspace(ctx context.Context, workspaceID uint) (identity.ManagedWorkspace, error) {
	var item identity.ManagedWorkspace
	err := repository.db.WithContext(ctx).Table("workspaces AS w").
		Select(`w.id, w.name, w.slug, w.description, w.enabled, w.is_default,
        w.created_at, w.updated_at,
        (SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = w.id) AS member_count`).
		Where("w.id = ?", workspaceID).Scan(&item).Error
	if err != nil {
		return identity.ManagedWorkspace{}, err
	}
	if item.ID == 0 {
		return identity.ManagedWorkspace{}, identity.ErrWorkspaceAdminNotFound
	}
	return item, nil
}

func (repository *AccessRepository) FindVisibleManagedWorkspace(ctx context.Context, principalID, workspaceID uint) (identity.ManagedWorkspace, error) {
	administrator, err := isAccessAdministrator(ctx, repository.db, principalID)
	if err != nil {
		return identity.ManagedWorkspace{}, err
	}
	query := repository.db.WithContext(ctx).Table("workspaces AS w")
	if !administrator {
		query = query.Joins(
			"JOIN workspace_members AS visible_wm ON visible_wm.workspace_id = w.id AND visible_wm.user_id = ? AND visible_wm.enabled = ?",
			principalID,
			true,
		)
	}
	var item identity.ManagedWorkspace
	err = query.Select(`w.id, w.name, w.slug, w.description, w.enabled, w.is_default,
        w.created_at, w.updated_at,
        (SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = w.id) AS member_count`).
		Where("w.id = ?", workspaceID).Scan(&item).Error
	if err != nil {
		return identity.ManagedWorkspace{}, err
	}
	if item.ID == 0 {
		return identity.ManagedWorkspace{}, identity.ErrWorkspaceAdminNotFound
	}
	return item, nil
}

func (repository *AccessRepository) CreateManagedWorkspace(ctx context.Context, input identity.WorkspaceInput) (identity.ManagedWorkspace, error) {
	var row Workspace
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.IsDefault {
			if err := tx.Model(&Workspace{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		row = Workspace{Name: input.Name, Slug: input.Slug, Description: input.Description, Enabled: input.Enabled, IsDefault: input.IsDefault}
		if err := tx.Create(&row).Error; err != nil {
			if isDuplicateError(err) {
				return identity.ErrWorkspaceSlugConflict
			}
			return err
		}
		return nil
	})
	if err != nil {
		return identity.ManagedWorkspace{}, err
	}
	return repository.FindManagedWorkspace(ctx, row.ID)
}

func (repository *AccessRepository) UpdateManagedWorkspace(ctx context.Context, workspaceID uint, input identity.WorkspaceUpdate) (identity.ManagedWorkspace, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, workspaceID).Error; err != nil {
			return mapWorkspaceAdminError(err)
		}
		if row.IsDefault && input.Enabled != nil && !*input.Enabled {
			return identity.ErrWorkspaceDefaultProtected
		}
		if row.IsDefault && input.IsDefault != nil && !*input.IsDefault {
			return identity.ErrWorkspaceDefaultProtected
		}
		updates := map[string]any{}
		if input.Name != nil {
			updates["name"] = *input.Name
		}
		if input.Description != nil {
			updates["description"] = *input.Description
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if input.IsDefault != nil {
			updates["is_default"] = *input.IsDefault
			if *input.IsDefault {
				updates["enabled"] = true
				if err := tx.Model(&Workspace{}).Where("id <> ? AND is_default = ?", workspaceID, true).Update("is_default", false).Error; err != nil {
					return err
				}
			}
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&row).Updates(updates).Error
	})
	if err != nil {
		return identity.ManagedWorkspace{}, err
	}
	return repository.FindManagedWorkspace(ctx, workspaceID)
}

func (repository *AccessRepository) DeleteManagedWorkspace(ctx context.Context, workspaceID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Workspace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, workspaceID).Error; err != nil {
			return mapWorkspaceAdminError(err)
		}
		if row.IsDefault {
			return identity.ErrWorkspaceDefaultProtected
		}
		for _, check := range []struct {
			model any
		}{
			{model: &WorkspaceMember{}},
			{model: &Team{}},
		} {
			var count int64
			if err := tx.Model(check.model).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return identity.ErrWorkspaceInUse
			}
		}
		return tx.Delete(&row).Error
	})
}

func (repository *AccessRepository) ListManagedWorkspaceMembers(ctx context.Context, workspaceID uint, filter identity.WorkspaceMemberFilter) (identity.ManagedWorkspaceMemberPage, error) {
	if _, err := repository.FindManagedWorkspace(ctx, workspaceID); err != nil {
		return identity.ManagedWorkspaceMemberPage{}, err
	}
	query := repository.db.WithContext(ctx).Table("workspace_members AS wm").
		Joins("JOIN users AS u ON u.id = wm.user_id").Where("wm.workspace_id = ?", workspaceID)
	if !filter.IncludeDisabled {
		query = query.Where("wm.enabled = ?", true)
	}
	if filter.Query != "" {
		pattern := "%" + escapeWorkspaceMemberLike(strings.ToLower(filter.Query)) + "%"
		query = query.Where(
			"LOWER(u.username) LIKE ? ESCAPE '\\' OR LOWER(u.display_name) LIKE ? ESCAPE '\\' OR LOWER(u.email) LIKE ? ESCAPE '\\'",
			pattern, pattern, pattern,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return identity.ManagedWorkspaceMemberPage{}, err
	}
	var items []identity.ManagedWorkspaceMember
	err := query.Select(`wm.user_id, u.username, u.display_name, u.email, u.avatar_url,
        u.status, wm.role, wm.enabled, wm.created_at, wm.updated_at`).
		Order("wm.role DESC, u.display_name ASC, u.username ASC, wm.user_id ASC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&items).Error
	if err != nil {
		return identity.ManagedWorkspaceMemberPage{}, err
	}
	return identity.ManagedWorkspaceMemberPage{
		Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total,
		HasMore: int64(filter.Page*filter.PageSize) < total,
	}, nil
}

func escapeWorkspaceMemberLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func (repository *AccessRepository) CreateManagedWorkspaceMember(ctx context.Context, workspaceID uint, input identity.WorkspaceMemberInput) (identity.ManagedWorkspaceMember, error) {
	if _, err := repository.FindManagedWorkspace(ctx, workspaceID); err != nil {
		return identity.ManagedWorkspaceMember{}, err
	}
	var users int64
	if err := repository.db.WithContext(ctx).Model(&User{}).Where("id = ?", input.UserID).Count(&users).Error; err != nil {
		return identity.ManagedWorkspaceMember{}, err
	}
	if users != 1 {
		return identity.ManagedWorkspaceMember{}, identity.ErrWorkspaceMemberUserMissing
	}
	row := WorkspaceMember{WorkspaceID: workspaceID, UserID: input.UserID, Role: input.Role, Enabled: input.Enabled}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateError(err) {
			return identity.ManagedWorkspaceMember{}, identity.ErrWorkspaceMemberConflict
		}
		return identity.ManagedWorkspaceMember{}, err
	}
	return repository.findManagedWorkspaceMember(ctx, workspaceID, input.UserID)
}

func (repository *AccessRepository) UpdateManagedWorkspaceMember(ctx context.Context, workspaceID, userID uint, input identity.WorkspaceMemberUpdate) (identity.ManagedWorkspaceMember, error) {
	updates := map[string]any{}
	if input.Role != nil {
		updates["role"] = *input.Role
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if len(updates) > 0 {
		result := repository.db.WithContext(ctx).Model(&WorkspaceMember{}).
			Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Updates(updates)
		if result.Error != nil {
			return identity.ManagedWorkspaceMember{}, result.Error
		}
		if result.RowsAffected != 1 {
			return identity.ManagedWorkspaceMember{}, identity.ErrWorkspaceMemberNotFound
		}
	}
	return repository.findManagedWorkspaceMember(ctx, workspaceID, userID)
}

func (repository *AccessRepository) DeleteManagedWorkspaceMember(ctx context.Context, workspaceID, userID uint) error {
	result := repository.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Delete(&WorkspaceMember{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return identity.ErrWorkspaceMemberNotFound
	}
	return nil
}

func (repository *AccessRepository) findManagedWorkspaceMember(ctx context.Context, workspaceID, userID uint) (identity.ManagedWorkspaceMember, error) {
	var item identity.ManagedWorkspaceMember
	err := repository.db.WithContext(ctx).Table("workspace_members AS wm").
		Joins("JOIN users AS u ON u.id = wm.user_id").
		Where("wm.workspace_id = ? AND wm.user_id = ?", workspaceID, userID).
		Select(`wm.user_id, u.username, u.display_name, u.email, u.avatar_url,
        u.status, wm.role, wm.enabled, wm.created_at, wm.updated_at`).Scan(&item).Error
	if err != nil {
		return identity.ManagedWorkspaceMember{}, err
	}
	if item.UserID == 0 {
		return identity.ManagedWorkspaceMember{}, identity.ErrWorkspaceMemberNotFound
	}
	return item, nil
}

func mapWorkspaceAdminError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.ErrWorkspaceAdminNotFound
	}
	return err
}

var _ identity.WorkspaceAdminStore = (*AccessRepository)(nil)
