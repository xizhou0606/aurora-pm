package identitygorm

import (
	"context"

	"gorm.io/gorm"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func (repository *AccessRepository) CreateRoleAudited(ctx context.Context, actorUserID uint, input identity.RoleInput) (identity.Role, error) {
	var row AccessRole
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		if err := ensureRoleNameAvailable(tx, input.Name, 0); err != nil {
			return err
		}
		row = AccessRole{Code: input.Code, Name: input.Name, Description: input.Description, Enabled: input.Enabled}
		if err := tx.Create(&row).Error; err != nil {
			return mapAccessWriteError(err, identity.ErrAccessCodeConflict)
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "role.created", ActorUserID: actorUserID, SubjectRoleID: uintPointer(row.ID),
			AfterJSON: auditJSON(roleFromRecord(row)),
		})
	})
	return roleFromRecord(row), err
}

func (repository *AccessRepository) UpdateRoleAudited(ctx context.Context, actorUserID, roleID uint, input identity.RoleUpdate) (identity.Role, error) {
	var row AccessRole
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		if err := tx.First(&row, roleID).Error; err != nil {
			return mapRoleError(err)
		}
		if row.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		before := roleFromRecord(row)
		updates := map[string]any{}
		if input.Name != nil {
			if err := ensureRoleNameAvailable(tx, *input.Name, roleID); err != nil {
				return err
			}
			updates["name"] = *input.Name
		}
		if input.Description != nil {
			updates["description"] = *input.Description
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if len(updates) > 0 {
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return mapAccessWriteError(err, identity.ErrAccessRoleNameConflict)
			}
			if err := tx.First(&row, roleID).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "role.updated", ActorUserID: actorUserID, SubjectRoleID: uintPointer(roleID),
			BeforeJSON: auditJSON(before), AfterJSON: auditJSON(roleFromRecord(row)),
		})
	})
	return roleFromRecord(row), err
}

func (repository *AccessRepository) DeleteRoleAudited(ctx context.Context, actorUserID, roleID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var row AccessRole
		if err := tx.First(&row, roleID).Error; err != nil {
			return mapRoleError(err)
		}
		if row.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		for _, model := range []any{&AccessUserRole{}, &AccessRoleServiceInterface{}, &AccessRoleMenu{}} {
			var count int64
			if err := tx.Model(model).Where("role_id = ?", roleID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return identity.ErrAccessResourceInUse
			}
		}
		if err := appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "role.deleted", ActorUserID: actorUserID, SubjectRoleID: uintPointer(roleID),
			BeforeJSON: auditJSON(roleFromRecord(row)),
		}); err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

func (repository *AccessRepository) ReplaceRoleActionCodes(ctx context.Context, actorUserID, roleID uint, actionCodes []string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var role AccessRole
		if err := tx.First(&role, roleID).Error; err != nil {
			return mapRoleError(err)
		}
		if role.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		interfaceIDs, err := enabledInterfaceIDsForActions(ctx, tx, actionCodes)
		if err != nil {
			return err
		}
		before, err := actionCodesForGrant(ctx, tx, "access_role_service_interfaces", "role_id", roleID)
		if err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&AccessRoleServiceInterface{}).Error; err != nil {
			return err
		}
		if len(interfaceIDs) > 0 {
			rows := make([]AccessRoleServiceInterface, 0, len(interfaceIDs))
			for _, interfaceID := range interfaceIDs {
				rows = append(rows, AccessRoleServiceInterface{RoleID: roleID, ServiceInterfaceID: interfaceID})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "role.action_grants_replaced", ActorUserID: actorUserID, SubjectRoleID: uintPointer(roleID),
			BeforeJSON: auditJSON(map[string][]string{"action_codes": before}),
			AfterJSON:  auditJSON(map[string][]string{"action_codes": actionCodes}),
		})
	})
}
