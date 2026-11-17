package identitygorm

import (
	"context"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type WorkspaceMembershipRepository struct {
	db *gorm.DB
}

func NewWorkspaceMembershipRepository(db *gorm.DB) *WorkspaceMembershipRepository {
	return &WorkspaceMembershipRepository{db: db}
}

func (repository *WorkspaceMembershipRepository) HasActiveWorkspaceMembership(
	ctx context.Context,
	principalID uint,
	workspaceID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || principalID == 0 || workspaceID == 0 {
		return false, identity.ErrWorkspaceMembershipInactive
	}
	administrator, err := isAccessAdministrator(ctx, repository.db, principalID)
	if err != nil {
		return false, err
	}
	query := repository.db.WithContext(ctx).
		Table("users AS u").
		Joins("JOIN workspaces AS w ON w.id = ?", workspaceID)
	if !administrator {
		query = query.Joins(
			"JOIN workspace_members AS wm ON wm.workspace_id = w.id AND wm.user_id = u.id AND wm.enabled = ?",
			true,
		)
	}
	var count int64
	err = query.
		Where(
			"u.id = ? AND w.enabled = ? AND u.active = ? AND u.status = ?",
			principalID, true, true, string(identity.AccountActive),
		).
		Count(&count).Error
	return count == 1, err
}

var _ identity.WorkspaceMembershipStore = (*WorkspaceMembershipRepository)(nil)
