package identitygorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func EnsureDefaultUserGroup(tx *gorm.DB) (AccessUserGroup, error) {
	var group AccessUserGroup
	err := tx.Where("code = ?", identity.DefaultUserGroupCode).First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		group = AccessUserGroup{
			Code: identity.DefaultUserGroupCode, Name: "Default users", Enabled: true, SystemManaged: true,
		}
		if err := tx.Create(&group).Error; err != nil {
			return AccessUserGroup{}, err
		}
		return group, nil
	}
	if err != nil {
		return AccessUserGroup{}, err
	}
	if err := tx.Model(&group).Updates(map[string]any{"enabled": true, "system_managed": true}).Error; err != nil {
		return AccessUserGroup{}, err
	}
	return group, nil
}

func EnsureUserInDefaultGroup(tx *gorm.DB, userID uint) error {
	group, err := EnsureDefaultUserGroup(tx)
	if err != nil {
		return err
	}
	binding := AccessUserGroupBinding{UserID: userID, GroupID: group.ID}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(&binding).Error
}

func BackfillDefaultUserGroup(tx *gorm.DB) error {
	group, err := EnsureDefaultUserGroup(tx)
	if err != nil {
		return err
	}
	var userIDs []uint
	if err := tx.Model(&User{}).
		Where("NOT EXISTS (?)", tx.Model(&AccessUserGroupBinding{}).
			Select("1").Where("access_user_group_bindings.user_id = users.id")).
		Order("id ASC").Pluck("id", &userIDs).Error; err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}
	bindings := make([]AccessUserGroupBinding, 0, len(userIDs))
	for _, userID := range userIDs {
		bindings = append(bindings, AccessUserGroupBinding{UserID: userID, GroupID: group.ID})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&bindings).Error
}

func (repository *AccessRepository) ListPersonnelUsers(ctx context.Context, filter identity.PersonnelUserFilter) (identity.PersonnelUserPage, error) {
	query := repository.db.WithContext(ctx).Model(&User{})
	if filter.Status != "" {
		query = query.Where("users.status = ?", string(filter.Status))
	}
	if filter.Query != "" {
		pattern := "%" + strings.ToLower(filter.Query) + "%"
		query = query.Where(
			"LOWER(users.username) LIKE ? OR LOWER(users.display_name) LIKE ? OR LOWER(users.email) LIKE ?",
			pattern, pattern, pattern,
		)
	}
	if filter.GroupID > 0 {
		query = query.Joins("JOIN access_user_group_bindings augb ON augb.user_id = users.id AND augb.group_id = ?", filter.GroupID)
	}
	if filter.RoleID > 0 {
		query = query.Joins("JOIN access_user_roles aur ON aur.user_id = users.id AND aur.role_id = ?", filter.RoleID)
	}
	var total int64
	if err := query.Distinct("users.id").Count(&total).Error; err != nil {
		return identity.PersonnelUserPage{}, err
	}
	var rows []User
	if err := query.Distinct("users.*").
		Order("display_name ASC, username ASC, users.id ASC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).
		Find(&rows).Error; err != nil {
		return identity.PersonnelUserPage{}, err
	}
	result := make([]identity.PersonnelUser, 0, len(rows))
	for _, row := range rows {
		user, err := repository.personnelUser(ctx, repository.db, row)
		if err != nil {
			return identity.PersonnelUserPage{}, err
		}
		result = append(result, user)
	}
	return identity.PersonnelUserPage{
		Items: result, Page: filter.Page, PageSize: filter.PageSize, Total: total,
		HasMore: int64(filter.Page*filter.PageSize) < total,
	}, nil
}

func (repository *AccessRepository) FindPersonnelUser(ctx context.Context, userID uint) (identity.PersonnelUser, error) {
	var row User
	if err := repository.db.WithContext(ctx).First(&row, userID).Error; err != nil {
		return identity.PersonnelUser{}, mapPersonnelUserError(err)
	}
	return repository.personnelUser(ctx, repository.db, row)
}

func (repository *AccessRepository) ListUserGroups(ctx context.Context, includeDisabled bool) ([]identity.UserGroup, error) {
	query := repository.db.WithContext(ctx).Model(&AccessUserGroup{}).Order("system_managed DESC, code ASC, id ASC")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	var rows []AccessUserGroup
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]identity.UserGroup, 0, len(rows))
	for _, row := range rows {
		result = append(result, userGroupDomain(row))
	}
	return result, nil
}

func (repository *AccessRepository) FindUserGroup(ctx context.Context, groupID uint) (identity.UserGroup, error) {
	var row AccessUserGroup
	if err := repository.db.WithContext(ctx).First(&row, groupID).Error; err != nil {
		return identity.UserGroup{}, mapUserGroupError(err)
	}
	return userGroupDomain(row), nil
}

func (repository *AccessRepository) CreateUserGroup(ctx context.Context, actorUserID uint, input identity.UserGroupInput) (identity.UserGroup, error) {
	var result AccessUserGroup
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		result = AccessUserGroup{
			Code: input.Code, Name: input.Name, Description: input.Description, Enabled: input.Enabled,
		}
		if err := tx.Create(&result).Error; err != nil {
			if isDuplicateError(err) {
				return identity.ErrUserGroupCodeConflict
			}
			return err
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "group.created", ActorUserID: actorUserID, SubjectGroupID: uintPointer(result.ID),
			AfterJSON: auditJSON(userGroupDomain(result)),
		})
	})
	return userGroupDomain(result), err
}

func (repository *AccessRepository) UpdateUserGroup(ctx context.Context, actorUserID, groupID uint, input identity.UserGroupUpdate) (identity.UserGroup, error) {
	var result AccessUserGroup
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, groupID).Error; err != nil {
			return mapUserGroupError(err)
		}
		before := userGroupDomain(result)
		if result.Code == identity.DefaultUserGroupCode && input.Enabled != nil && !*input.Enabled {
			return identity.ErrDefaultUserGroupProtected
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
		if len(updates) > 0 {
			if err := tx.Model(&result).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.First(&result, groupID).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "group.updated", ActorUserID: actorUserID, SubjectGroupID: uintPointer(groupID),
			BeforeJSON: auditJSON(before), AfterJSON: auditJSON(userGroupDomain(result)),
		})
	})
	return userGroupDomain(result), err
}

func (repository *AccessRepository) DeleteUserGroup(ctx context.Context, actorUserID, groupID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var row AccessUserGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, groupID).Error; err != nil {
			return mapUserGroupError(err)
		}
		if row.Code == identity.DefaultUserGroupCode || row.SystemManaged {
			return identity.ErrDefaultUserGroupProtected
		}
		var count int64
		if err := tx.Model(&AccessUserGroupBinding{}).Where("group_id = ?", groupID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return identity.ErrUserGroupInUse
		}
		if err := tx.Model(&AccessGroupServiceInterface{}).Where("group_id = ?", groupID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return identity.ErrAccessResourceInUse
		}
		if err := tx.Model(&AccessGroupMenu{}).Where("group_id = ?", groupID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return identity.ErrAccessResourceInUse
		}
		if err := appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "group.deleted", ActorUserID: actorUserID, SubjectGroupID: uintPointer(groupID),
			BeforeJSON: auditJSON(userGroupDomain(row)),
		}); err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

func (repository *AccessRepository) ReplacePersonnelUserGroup(ctx context.Context, actorUserID, userID, groupID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		if err := requirePersonnelUser(tx, userID); err != nil {
			return err
		}
		var group AccessUserGroup
		if err := tx.Where("id = ? AND enabled = ?", groupID, true).First(&group).Error; err != nil {
			return mapUserGroupError(err)
		}
		var existing AccessUserGroupBinding
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		beforeID := existing.GroupID
		if errors.Is(err, gorm.ErrRecordNotFound) {
			existing = AccessUserGroupBinding{UserID: userID, GroupID: groupID}
			if err := tx.Create(&existing).Error; err != nil {
				return err
			}
		} else if existing.GroupID != groupID {
			if err := tx.Model(&existing).Update("group_id", groupID).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "user.group_replaced", ActorUserID: actorUserID, SubjectUserID: uintPointer(userID),
			BeforeJSON: auditJSON(map[string]uint{"group_id": beforeID}),
			AfterJSON:  auditJSON(map[string]uint{"group_id": groupID}),
		})
	})
}

func (repository *AccessRepository) ReplacePersonnelUserRoles(ctx context.Context, actorUserID, userID uint, roleIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		return repository.replaceUserRoles(tx, actorUserID, userID, roleIDs)
	})
}

func (repository *AccessRepository) ReplacePersonnelUserAccess(ctx context.Context, actorUserID, userID, groupID uint, roleIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		if err := requirePersonnelUser(tx, userID); err != nil {
			return err
		}
		var group AccessUserGroup
		if err := tx.Where("id = ? AND enabled = ?", groupID, true).First(&group).Error; err != nil {
			return mapUserGroupError(err)
		}
		if err := requireEnabledRows(tx, &AccessRole{}, "id", roleIDs, identity.ErrAccessRoleNotFound); err != nil {
			return err
		}
		if err := repository.protectLastAdministrator(tx, userID, roleIDs); err != nil {
			return err
		}
		var binding AccessUserGroupBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&binding).Error; err != nil {
			return err
		}
		var beforeRoles []uint
		if err := tx.Model(&AccessUserRole{}).Where("user_id = ?", userID).Order("role_id ASC").Pluck("role_id", &beforeRoles).Error; err != nil {
			return err
		}
		beforeGroupID := binding.GroupID
		if binding.GroupID != groupID {
			if err := tx.Model(&binding).Update("group_id", groupID).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", userID).Delete(&AccessUserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) > 0 {
			rows := make([]AccessUserRole, 0, len(roleIDs))
			for _, roleID := range roleIDs {
				rows = append(rows, AccessUserRole{UserID: userID, RoleID: roleID})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "user.access_replaced", ActorUserID: actorUserID, SubjectUserID: uintPointer(userID),
			BeforeJSON: auditJSON(map[string]any{"group_id": beforeGroupID, "role_ids": beforeRoles}),
			AfterJSON:  auditJSON(map[string]any{"group_id": groupID, "role_ids": roleIDs}),
		})
	})
}

func (repository *AccessRepository) TransitionPersonnelUserStatus(ctx context.Context, actorUserID, userID uint, target identity.AccountStatus, reason string) (identity.PersonnelUser, error) {
	var result identity.PersonnelUser
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var row User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, userID).Error; err != nil {
			return mapPersonnelUserError(err)
		}
		current := accountStatus(row)
		if !identity.ValidAccountTransition(current, target) {
			return identity.ErrInvalidAccountTransition
		}
		if current == identity.AccountActive && target != identity.AccountActive {
			if err := repository.protectLastAdministrator(tx, userID, nil); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		updates := map[string]any{
			"status": string(target), "status_reason": reason, "active": target != identity.AccountBanned,
		}
		switch target {
		case identity.AccountActive:
			if current == identity.AccountPendingApproval {
				updates["approved_at"] = now
				updates["approved_by_user_id"] = actorUserID
			}
			updates["banned_at"] = nil
			updates["banned_by_user_id"] = nil
		case identity.AccountBanned:
			updates["banned_at"] = now
			updates["banned_by_user_id"] = actorUserID
			updates["session_version"] = gorm.Expr("session_version + 1")
		case identity.AccountPendingApproval:
			updates["approved_at"] = nil
			updates["approved_by_user_id"] = nil
			updates["banned_at"] = nil
			updates["banned_by_user_id"] = nil
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&row, userID).Error; err != nil {
			return err
		}
		var err error
		result, err = repository.personnelUser(ctx, tx, row)
		if err != nil {
			return err
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "user.status_changed", ActorUserID: actorUserID, SubjectUserID: uintPointer(userID),
			BeforeJSON: auditJSON(map[string]string{"status": string(current)}),
			AfterJSON:  auditJSON(map[string]string{"status": string(target)}), Reason: reason,
		})
	})
	return result, err
}

func (repository *AccessRepository) ListPersonnelAuditEvents(ctx context.Context, filter identity.PersonnelAuditFilter) ([]identity.PersonnelAuditEvent, error) {
	query := repository.db.WithContext(ctx).Model(&PersonnelAuditEvent{}).Order("id DESC")
	if filter.SubjectUserID > 0 {
		query = query.Where("subject_user_id = ?", filter.SubjectUserID)
	}
	if filter.SubjectGroupID > 0 {
		query = query.Where("subject_group_id = ?", filter.SubjectGroupID)
	}
	if filter.SubjectRoleID > 0 {
		query = query.Where("subject_role_id = ?", filter.SubjectRoleID)
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	var rows []PersonnelAuditEvent
	if err := query.Limit(filter.Limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]identity.PersonnelAuditEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, personnelAuditDomain(row))
	}
	return result, nil
}

func (repository *AccessRepository) personnelUser(ctx context.Context, db *gorm.DB, row User) (identity.PersonnelUser, error) {
	var binding AccessUserGroupBinding
	if err := db.WithContext(ctx).Where("user_id = ?", row.ID).First(&binding).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.PersonnelUser{}, err
		}
		var defaultGroup AccessUserGroup
		if err := db.WithContext(ctx).
			Where("code = ? AND enabled = ?", identity.DefaultUserGroupCode, true).
			First(&defaultGroup).Error; err != nil {
			return identity.PersonnelUser{}, err
		}
		binding.GroupID = defaultGroup.ID
	}
	var roleIDs []uint
	if err := db.WithContext(ctx).Model(&AccessUserRole{}).Where("user_id = ?", row.ID).Order("role_id ASC").Pluck("role_id", &roleIDs).Error; err != nil {
		return identity.PersonnelUser{}, err
	}
	return identity.PersonnelUser{
		ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, Email: row.Email, AvatarURL: row.AvatarURL,
		Status: accountStatus(row), StatusReason: row.StatusReason, SessionVersion: row.SessionVersion,
		GroupID: binding.GroupID, RoleIDs: roleIDs, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (repository *AccessRepository) replaceUserRoles(tx *gorm.DB, actorUserID, userID uint, roleIDs []uint) error {
	if err := requirePersonnelUser(tx, userID); err != nil {
		return err
	}
	if err := requireEnabledRows(tx, &AccessRole{}, "id", roleIDs, identity.ErrAccessRoleNotFound); err != nil {
		return err
	}
	var before []uint
	if err := tx.Model(&AccessUserRole{}).Where("user_id = ?", userID).Order("role_id ASC").Pluck("role_id", &before).Error; err != nil {
		return err
	}
	if err := repository.protectLastAdministrator(tx, userID, roleIDs); err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&AccessUserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) > 0 {
		rows := make([]AccessUserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			rows = append(rows, AccessUserRole{UserID: userID, RoleID: roleID})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	return appendPersonnelAudit(tx, PersonnelAuditEvent{
		EventType: "user.roles_replaced", ActorUserID: actorUserID, SubjectUserID: uintPointer(userID),
		BeforeJSON: auditJSON(map[string][]uint{"role_ids": before}),
		AfterJSON:  auditJSON(map[string][]uint{"role_ids": roleIDs}),
	})
}

// nextRoleIDs is nil for account-status removal of recovery access.
func (repository *AccessRepository) protectLastAdministrator(tx *gorm.DB, userID uint, nextRoleIDs []uint) error {
	var administratorRole AccessRole
	if err := tx.Where("code = ?", identity.AccessAdministratorRoleCode).First(&administratorRole).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var assigned int64
	if err := tx.Model(&AccessUserRole{}).Where("user_id = ? AND role_id = ?", userID, administratorRole.ID).Count(&assigned).Error; err != nil {
		return err
	}
	if assigned == 0 {
		return nil
	}
	if nextRoleIDs != nil {
		for _, roleID := range nextRoleIDs {
			if roleID == administratorRole.ID {
				return nil
			}
		}
	}
	var otherCount int64
	err := tx.Table("access_user_roles aur").
		Joins("JOIN users u ON u.id = aur.user_id").
		Where("aur.role_id = ? AND aur.user_id <> ? AND u.active = ? AND u.status = ?", administratorRole.ID, userID, true, string(identity.AccountActive)).
		Distinct("aur.user_id").Count(&otherCount).Error
	if err != nil {
		return err
	}
	if otherCount == 0 {
		return identity.ErrLastAdministrator
	}
	return nil
}

func requirePersonnelUser(tx *gorm.DB, userID uint) error {
	if userID == 0 {
		return identity.ErrPersonnelUserNotFound
	}
	var count int64
	if err := tx.Model(&User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return identity.ErrPersonnelUserNotFound
	}
	return nil
}

func accountStatus(row User) identity.AccountStatus {
	return identity.EffectiveAccountStatus(row.Status, row.Active)
}

func appendPersonnelAudit(tx *gorm.DB, event PersonnelAuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return tx.Create(&event).Error
}

func auditJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func userGroupDomain(row AccessUserGroup) identity.UserGroup {
	return identity.UserGroup{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description,
		Enabled: row.Enabled, SystemManaged: row.SystemManaged, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func personnelAuditDomain(row PersonnelAuditEvent) identity.PersonnelAuditEvent {
	return identity.PersonnelAuditEvent{
		ID: row.ID, EventType: row.EventType, ActorUserID: row.ActorUserID,
		SubjectUserID: row.SubjectUserID, SubjectGroupID: row.SubjectGroupID, SubjectRoleID: row.SubjectRoleID,
		BeforeJSON: row.BeforeJSON, AfterJSON: row.AfterJSON, Reason: row.Reason, CreatedAt: row.CreatedAt,
	}
}

func mapPersonnelUserError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.ErrPersonnelUserNotFound
	}
	return err
}

func mapUserGroupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.ErrUserGroupNotFound
	}
	return err
}

func uintPointer(value uint) *uint { return &value }
