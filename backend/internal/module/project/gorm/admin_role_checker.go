package projectgorm

import (
	"context"
	"errors"

	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

// AdminRoleChecker implements project.AdminRoleChecker and
// project.UserActiveChecker using GORM to check PM user table and RBAC roles.
type AdminRoleChecker struct {
	db *gorm.DB
}

func NewAdminRoleChecker(db *gorm.DB) *AdminRoleChecker {
	return &AdminRoleChecker{db: db}
}

// adminUserPredicate matches only the canonical internal users.id.
type adminUserPredicate struct {
	internalID uint
}

// resolveAdminUserPredicate returns an active internal identity, or an empty match.
func (c *AdminRoleChecker) resolveAdminUserPredicate(
	ctx context.Context,
	workspaceID, userID uint,
) (adminUserPredicate, error) {
	if c == nil || c.db == nil || userID == 0 || workspaceID == 0 {
		return adminUserPredicate{}, nil
	}
	var row struct {
		ID   uint
		Role string
	}
	err := c.db.WithContext(ctx).Table("users").
		Select("users.id").
		Where("users.active = ? AND users.status = ?", true, "active").
		Where("users.id = ?", userID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return adminUserPredicate{}, nil
	}
	if err != nil {
		return adminUserPredicate{}, err
	}
	return adminUserPredicate{internalID: row.ID}, nil
}

// IsAccessAdministrator reports whether the user has the maximum access level
// for a workspace. A platform super_admin (users.role = 'super_admin') is
// always granted access as long as the account is active; it does not require
// a workspace_members row. Otherwise the user must be an active workspace
// member AND hold the access-administrator role. userID is always users.id.
func (c *AdminRoleChecker) IsAccessAdministrator(ctx context.Context, workspaceID, userID uint) (bool, error) {
	if c == nil || c.db == nil || userID == 0 || workspaceID == 0 {
		return false, nil
	}
	resolved, err := c.resolveAdminUserPredicate(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if resolved.internalID == 0 {
		return false, nil
	}
	// Step 1: a super_admin is an administrator in every workspace. This branch
	// only requires the account itself to be active/status=active; it must NOT
	// require a workspace_members row, because super_admins own global access
	// and are frequently not enrolled as members of every workspace.
	var superAdminCount int64
	if err := c.db.WithContext(ctx).Table("users").
		Where("id = ? AND role = ? AND active = ? AND status = ?", resolved.internalID, "super_admin", true, "active").
		Count(&superAdminCount).Error; err != nil {
		return false, err
	}
	if superAdminCount > 0 {
		return true, nil
	}
	// Step 2: verify the user is active and a member of this workspace. The
	// workspace_members row stores users.id, so match the internal ID.
	var count int64
	if err := c.db.WithContext(ctx).Table("users").
		Joins("JOIN workspace_members ON workspace_members.user_id = users.id AND workspace_members.workspace_id = ? AND workspace_members.enabled = ?", workspaceID, true).
		Where("users.id = ? AND users.active = ? AND users.status = ?", resolved.internalID, true, "active").
		Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	// Step 3: find the access-administrator role.
	var role identitygorm.AccessRole
	if err := c.db.WithContext(ctx).
		Where("code = ?", "access-administrator").First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	// Step 4: check if user has this role. AccessUserRole rows store users.id.
	var bindingCount int64
	if err := c.db.WithContext(ctx).Model(&identitygorm.AccessUserRole{}).
		Where("user_id = ? AND role_id = ?", resolved.internalID, role.ID).
		Count(&bindingCount).Error; err != nil {
		return false, err
	}
	return bindingCount > 0, nil
}

// IsActive checks whether the PM user account is active and is a member of the
// workspace. The userID may be either the PM-internal users.id or the external
// fact ID.
func (c *AdminRoleChecker) IsActive(ctx context.Context, workspaceID, userID uint) (bool, error) {
	if c == nil || c.db == nil || userID == 0 || workspaceID == 0 {
		return false, project.ErrParticipantInvalid
	}
	resolved, err := c.resolveAdminUserPredicate(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if resolved.internalID == 0 {
		return false, nil
	}
	var count int64
	if err := c.db.WithContext(ctx).Table("users").
		Joins("JOIN workspace_members ON workspace_members.user_id = users.id AND workspace_members.workspace_id = ? AND workspace_members.enabled = ?", workspaceID, true).
		Where("users.id = ? AND users.active = ? AND users.status = ?", resolved.internalID, true, "active").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FactUserResolver validates the canonical internal PM principal. The legacy
// name remains for interface compatibility; external IDs are never accepted.
type FactUserResolver struct {
	db *gorm.DB
}

func NewFactUserResolver(db *gorm.DB) *FactUserResolver {
	return &FactUserResolver{db: db}
}

// FactUserID resolves the canonical fact user ID for the given principal. A
// principal that matches no active user resolves to 0 (fail closed), and a
// lookup error is returned as-is.
func (r *FactUserResolver) FactUserID(ctx context.Context, principalID uint) (uint, error) {
	if r == nil || r.db == nil || principalID == 0 {
		return 0, nil
	}
	var row struct {
		ID uint
	}
	err := r.db.WithContext(ctx).Table("users").
		Select("users.id").
		Where("users.active = ? AND users.status = ?", true, "active").
		Where("users.id = ?", principalID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}
