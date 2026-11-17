package identitygorm

import (
	"context"
	"errors"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

// AccessRepository persists identity-owned global RBAC records. It is the
// only package that joins user-role and role-interface grant tables.
type AccessRepository struct {
	db                              *gorm.DB
	passwordResetNotificationWriter PasswordResetNotificationWriter
}

func NewAccessRepository(db *gorm.DB) *AccessRepository { return &AccessRepository{db: db} }

type PasswordResetNotificationWriter func(
	tx *gorm.DB,
	resetID uint,
	input identity.PasswordResetRequest,
	target User,
	actor User,
) error

func (repository *AccessRepository) SetPasswordResetNotificationWriter(writer PasswordResetNotificationWriter) {
	repository.passwordResetNotificationWriter = writer
}

func (repository *AccessRepository) ListActiveUsers(ctx context.Context) ([]identity.UserSummary, error) {
	var rows []User
	if err := repository.db.WithContext(ctx).
		Select("id", "username", "display_name", "email", "active").
		Where("active = ? AND status = ?", true, string(identity.AccountActive)).
		Order("display_name ASC, username ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]identity.UserSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, identity.UserSummary{
			ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, Email: row.Email, Active: row.Active,
		})
	}
	return result, nil
}

func (repository *AccessRepository) ListRoles(ctx context.Context, includeDisabled bool) ([]identity.Role, error) {
	query := repository.db.WithContext(ctx).Model(&AccessRole{}).Order("name ASC, id ASC")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	var rows []AccessRole
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rolesFromRecords(rows), nil
}

func (repository *AccessRepository) FindRole(ctx context.Context, id uint) (identity.Role, error) {
	var row AccessRole
	if err := repository.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return identity.Role{}, mapRoleError(err)
	}
	return roleFromRecord(row), nil
}

func (repository *AccessRepository) CreateRole(ctx context.Context, input identity.RoleInput) (identity.Role, error) {
	if err := ensureRoleNameAvailable(repository.db.WithContext(ctx), input.Name, 0); err != nil {
		return identity.Role{}, err
	}
	row := AccessRole{Code: input.Code, Name: input.Name, Description: input.Description, Enabled: input.Enabled}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return identity.Role{}, mapAccessWriteError(err, identity.ErrAccessCodeConflict)
	}
	return roleFromRecord(row), nil
}

func (repository *AccessRepository) UpdateRole(ctx context.Context, id uint, input identity.RoleUpdate) (identity.Role, error) {
	returnRole := identity.Role{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AccessRole
		if err := tx.First(&row, id).Error; err != nil {
			return mapRoleError(err)
		}
		if row.SystemManaged && (input.Name != nil || input.Description != nil || input.Enabled != nil) {
			return identity.ErrAccessSystemManaged
		}
		updates := map[string]any{}
		if input.Name != nil {
			if err := ensureRoleNameAvailable(tx, *input.Name, id); err != nil {
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
		}
		if err := tx.First(&row, id).Error; err != nil {
			return err
		}
		returnRole = roleFromRecord(row)
		return nil
	})
	return returnRole, err
}

func ensureRoleNameAvailable(tx *gorm.DB, name string, excludedRoleID uint) error {
	query := tx.Model(&AccessRole{}).Where("name = ?", name)
	if excludedRoleID > 0 {
		query = query.Where("id <> ?", excludedRoleID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return identity.ErrAccessRoleNameConflict
	}
	return nil
}

func (repository *AccessRepository) DeleteRole(ctx context.Context, id uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AccessRole
		if err := tx.First(&row, id).Error; err != nil {
			return mapRoleError(err)
		}
		if row.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		for _, model := range []any{&AccessUserRole{}, &AccessRoleServiceInterface{}, &AccessRoleMenu{}} {
			var count int64
			if err := tx.Model(model).Where("role_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return identity.ErrAccessResourceInUse
			}
		}
		return tx.Delete(&row).Error
	})
}

func (repository *AccessRepository) ListServiceInterfaces(ctx context.Context, includeDisabled bool) ([]identity.ServiceInterface, error) {
	query := repository.db.WithContext(ctx).Model(&AccessServiceInterface{}).Order("http_method ASC, route_template ASC, id ASC")
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	var rows []AccessServiceInterface
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]identity.ServiceInterface, 0, len(rows))
	for _, row := range rows {
		result = append(result, serviceInterfaceFromRecord(row))
	}
	return result, nil
}

func (repository *AccessRepository) FindServiceInterface(ctx context.Context, id uint) (identity.ServiceInterface, error) {
	var row AccessServiceInterface
	if err := repository.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.ServiceInterface{}, identity.ErrAccessServiceInterfaceNotFound
		}
		return identity.ServiceInterface{}, err
	}
	return serviceInterfaceFromRecord(row), nil
}

func (repository *AccessRepository) CreateServiceInterface(ctx context.Context, input identity.ServiceInterfaceInput) (identity.ServiceInterface, error) {
	row := AccessServiceInterface{
		Code: input.Code, ActionCode: input.ActionCode, HTTPMethod: input.HTTPMethod, RouteTemplate: input.RouteTemplate,
		Name: input.Name, Description: input.Description, Enabled: input.Enabled,
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return identity.ServiceInterface{}, mapServiceInterfaceCreateError(err)
	}
	return serviceInterfaceFromRecord(row), nil
}

func (repository *AccessRepository) CreateServiceInterfaces(ctx context.Context, inputs []identity.ServiceInterfaceInput) ([]identity.ServiceInterface, error) {
	result := make([]identity.ServiceInterface, 0, len(inputs))
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, input := range inputs {
			row := AccessServiceInterface{
				Code: input.Code, ActionCode: input.ActionCode, HTTPMethod: input.HTTPMethod, RouteTemplate: input.RouteTemplate,
				Name: input.Name, Description: input.Description, Enabled: input.Enabled,
			}
			if err := tx.Create(&row).Error; err != nil {
				return mapServiceInterfaceCreateError(err)
			}
			result = append(result, serviceInterfaceFromRecord(row))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (repository *AccessRepository) Bootstrap(ctx context.Context, spec identity.BootstrapSpec) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := BackfillDefaultUserGroup(tx); err != nil {
			return err
		}
		if spec.AdministratorUserID > 0 {
			if err := repository.requireActiveUser(ctx, tx, spec.AdministratorUserID); err != nil {
				return err
			}
		}
		role, err := ensureAccessAdministratorRole(tx)
		if err != nil {
			return err
		}
		interfaceIDs := make([]uint, 0, len(spec.ServiceInterfaces))
		interfaces := make([]AccessServiceInterface, 0, len(spec.ServiceInterfaces))
		for _, input := range spec.ServiceInterfaces {
			row, err := ensureSystemServiceInterface(tx, input)
			if err != nil {
				return err
			}
			interfaceIDs = append(interfaceIDs, row.ID)
			interfaces = append(interfaces, row)
		}
		if err := extendExistingActionGrants(tx, interfaces); err != nil {
			return err
		}
		for _, interfaceID := range interfaceIDs {
			grant := AccessRoleServiceInterface{RoleID: role.ID, ServiceInterfaceID: interfaceID}
			if err := tx.Where("role_id = ? AND service_interface_id = ?", role.ID, interfaceID).
				FirstOrCreate(&grant).Error; err != nil {
				return err
			}
		}
		if err := bindLegacySuperAdmins(tx, role.ID); err != nil {
			return err
		}
		if spec.AdministratorUserID > 0 {
			binding := AccessUserRole{UserID: spec.AdministratorUserID, RoleID: role.ID}
			if err := tx.Where("user_id = ? AND role_id = ?", binding.UserID, binding.RoleID).
				FirstOrCreate(&binding).Error; err != nil {
				return err
			}
		}
		if err := seedDefaultGroupBaselineGrants(
			tx,
			spec.ServiceInterfaces,
			interfaceIDs,
			spec.DefaultGroupExcludedActionCodes,
		); err != nil {
			return err
		}
		return ensureDefaultGroupActionGrants(ctx, tx, spec.DefaultGroupActionCodes)
	})
}

func ensureDefaultGroupActionGrants(ctx context.Context, tx *gorm.DB, actionCodes []string) error {
	if len(actionCodes) == 0 {
		return nil
	}
	group, err := EnsureDefaultUserGroup(tx)
	if err != nil {
		return err
	}
	interfaceIDs, err := enabledInterfaceIDsForActions(ctx, tx, actionCodes)
	if err != nil {
		return err
	}
	rows := make([]AccessGroupServiceInterface, 0, len(interfaceIDs))
	for _, interfaceID := range interfaceIDs {
		rows = append(rows, AccessGroupServiceInterface{GroupID: group.ID, ServiceInterfaceID: interfaceID})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&rows, 500).Error
}

type bootstrapSubjectActionGrant struct {
	SubjectID  uint   `gorm:"column:subject_id"`
	ActionCode string `gorm:"column:action_code"`
}

// extendExistingActionGrants preserves action-level authorization when a
// release adds another system route to an existing action. Action grants are
// stored as concrete interface IDs, so without this expansion a role or group
// could still report the action as selected while the new route returned 403.
func extendExistingActionGrants(tx *gorm.DB, interfaces []AccessServiceInterface) error {
	interfaceIDsByAction := make(map[string][]uint)
	actionCodes := make([]string, 0)
	for _, serviceInterface := range interfaces {
		if _, exists := interfaceIDsByAction[serviceInterface.ActionCode]; !exists {
			actionCodes = append(actionCodes, serviceInterface.ActionCode)
		}
		interfaceIDsByAction[serviceInterface.ActionCode] = append(
			interfaceIDsByAction[serviceInterface.ActionCode],
			serviceInterface.ID,
		)
	}
	if len(actionCodes) == 0 {
		return nil
	}

	roleGrants, err := existingBootstrapActionGrants(
		tx,
		"access_role_service_interfaces",
		"role_id",
		actionCodes,
	)
	if err != nil {
		return err
	}
	roleRows := make([]AccessRoleServiceInterface, 0)
	for _, grant := range roleGrants {
		for _, serviceInterfaceID := range interfaceIDsByAction[grant.ActionCode] {
			roleRows = append(roleRows, AccessRoleServiceInterface{
				RoleID: grant.SubjectID, ServiceInterfaceID: serviceInterfaceID,
			})
		}
	}
	if len(roleRows) > 0 {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&roleRows, 500).Error; err != nil {
			return err
		}
	}

	groupGrants, err := existingBootstrapActionGrants(
		tx,
		"access_group_service_interfaces",
		"group_id",
		actionCodes,
	)
	if err != nil {
		return err
	}
	groupRows := make([]AccessGroupServiceInterface, 0)
	for _, grant := range groupGrants {
		for _, serviceInterfaceID := range interfaceIDsByAction[grant.ActionCode] {
			groupRows = append(groupRows, AccessGroupServiceInterface{
				GroupID: grant.SubjectID, ServiceInterfaceID: serviceInterfaceID,
			})
		}
	}
	if len(groupRows) > 0 {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&groupRows, 500).Error; err != nil {
			return err
		}
	}

	return nil
}

func existingBootstrapActionGrants(
	tx *gorm.DB,
	grantTable string,
	subjectColumn string,
	actionCodes []string,
) ([]bootstrapSubjectActionGrant, error) {
	var grants []bootstrapSubjectActionGrant
	err := tx.Table(grantTable+" grants").
		Select("DISTINCT grants."+subjectColumn+" AS subject_id, interfaces.action_code").
		Joins("JOIN access_service_interfaces interfaces ON interfaces.id = grants.service_interface_id AND interfaces.enabled = ?", true).
		Where("interfaces.action_code IN ?", actionCodes).
		Scan(&grants).Error
	return grants, err
}

func (repository *AccessRepository) UpdateServiceInterface(ctx context.Context, id uint, input identity.ServiceInterfaceUpdate) (identity.ServiceInterface, error) {
	result := identity.ServiceInterface{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AccessServiceInterface
		if err := tx.First(&row, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return identity.ErrAccessServiceInterfaceNotFound
			}
			return err
		}
		if row.SystemManaged && (input.Name != nil || input.Description != nil || input.Enabled != nil) {
			return identity.ErrAccessSystemManaged
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
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return err
			}
		}
		if err := tx.First(&row, id).Error; err != nil {
			return err
		}
		result = serviceInterfaceFromRecord(row)
		return nil
	})
	return result, err
}

func (repository *AccessRepository) DeleteServiceInterface(ctx context.Context, id uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AccessServiceInterface
		if err := tx.First(&row, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return identity.ErrAccessServiceInterfaceNotFound
			}
			return err
		}
		if row.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		for _, grant := range []any{&AccessRoleServiceInterface{}, &AccessGroupServiceInterface{}} {
			var count int64
			if err := tx.Model(grant).Where("service_interface_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return identity.ErrAccessResourceInUse
			}
		}
		return tx.Delete(&row).Error
	})
}

func (repository *AccessRepository) UserRoleIDs(ctx context.Context, userID uint) ([]uint, error) {
	if err := repository.requireActiveUser(ctx, repository.db, userID); err != nil {
		return nil, err
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Model(&AccessUserRole{}).Where("user_id = ?", userID).Order("role_id ASC").Pluck("role_id", &ids).Error
	return ids, err
}

func (repository *AccessRepository) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repository.replaceUserRoles(tx, 0, userID, roleIDs)
	})
}

func (repository *AccessRepository) RoleServiceInterfaceIDs(ctx context.Context, roleID uint) ([]uint, error) {
	if _, err := repository.FindRole(ctx, roleID); err != nil {
		return nil, err
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Model(&AccessRoleServiceInterface{}).Where("role_id = ?", roleID).Order("service_interface_id ASC").Pluck("service_interface_id", &ids).Error
	return ids, err
}

func (repository *AccessRepository) ReplaceRoleServiceInterfaces(ctx context.Context, roleID uint, interfaceIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role AccessRole
		if err := tx.First(&role, roleID).Error; err != nil {
			return mapRoleError(err)
		}
		if role.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		if err := repository.requireRole(tx, roleID); err != nil {
			return err
		}
		if err := requireEnabledRows(tx, &AccessServiceInterface{}, "id", interfaceIDs, identity.ErrAccessServiceInterfaceNotFound); err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&AccessRoleServiceInterface{}).Error; err != nil {
			return err
		}
		if len(interfaceIDs) == 0 {
			return nil
		}
		rows := make([]AccessRoleServiceInterface, 0, len(interfaceIDs))
		for _, interfaceID := range interfaceIDs {
			rows = append(rows, AccessRoleServiceInterface{RoleID: roleID, ServiceInterfaceID: interfaceID})
		}
		return tx.Create(&rows).Error
	})
}

func (repository *AccessRepository) GroupServiceInterfaceIDs(ctx context.Context, groupID uint) ([]uint, error) {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&AccessUserGroup{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, identity.ErrUserGroupNotFound
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Model(&AccessGroupServiceInterface{}).
		Where("group_id = ?", groupID).Order("service_interface_id ASC").Pluck("service_interface_id", &ids).Error
	return ids, err
}

func (repository *AccessRepository) ReplaceGroupServiceInterfaces(ctx context.Context, actorUserID, groupID uint, interfaceIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var group AccessUserGroup
		if err := tx.Where("id = ?", groupID).First(&group).Error; err != nil {
			return mapUserGroupError(err)
		}
		if err := requireEnabledRows(tx, &AccessServiceInterface{}, "id", interfaceIDs, identity.ErrAccessServiceInterfaceNotFound); err != nil {
			return err
		}
		var before []uint
		if err := tx.Model(&AccessGroupServiceInterface{}).Where("group_id = ?", groupID).
			Order("service_interface_id ASC").Pluck("service_interface_id", &before).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&AccessGroupServiceInterface{}).Error; err != nil {
			return err
		}
		if len(interfaceIDs) > 0 {
			rows := make([]AccessGroupServiceInterface, 0, len(interfaceIDs))
			for _, interfaceID := range interfaceIDs {
				rows = append(rows, AccessGroupServiceInterface{GroupID: groupID, ServiceInterfaceID: interfaceID})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "group.interface_grants_replaced", ActorUserID: actorUserID, SubjectGroupID: uintPointer(groupID),
			BeforeJSON: auditJSON(map[string][]uint{"service_interface_ids": before}),
			AfterJSON:  auditJSON(map[string][]uint{"service_interface_ids": interfaceIDs}),
		})
	})
}

func (repository *AccessRepository) RoleActionCodes(ctx context.Context, roleID uint) ([]string, error) {
	if err := repository.requireRole(repository.db.WithContext(ctx), roleID); err != nil {
		return nil, err
	}
	return actionCodesForGrant(ctx, repository.db, "access_role_service_interfaces", "role_id", roleID)
}

func (repository *AccessRepository) GroupActionCodes(ctx context.Context, groupID uint) ([]string, error) {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&AccessUserGroup{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, identity.ErrUserGroupNotFound
	}
	return actionCodesForGrant(ctx, repository.db, "access_group_service_interfaces", "group_id", groupID)
}

func (repository *AccessRepository) ReplaceGroupActionCodes(ctx context.Context, actorUserID, groupID uint, actionCodes []string) error {
	interfaceIDs, err := enabledInterfaceIDsForActions(ctx, repository.db, actionCodes)
	if err != nil {
		return err
	}
	return repository.ReplaceGroupServiceInterfaces(ctx, actorUserID, groupID, interfaceIDs)
}

func (repository *AccessRepository) RoleMenuIDs(ctx context.Context, roleID uint) ([]uint, error) {
	if _, err := repository.FindRole(ctx, roleID); err != nil {
		return nil, err
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Model(&AccessRoleMenu{}).Where("role_id = ?", roleID).Order("menu_id ASC").Pluck("menu_id", &ids).Error
	return ids, err
}

func (repository *AccessRepository) ReplaceRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role AccessRole
		if err := tx.First(&role, roleID).Error; err != nil {
			return mapRoleError(err)
		}
		if role.SystemManaged {
			return identity.ErrAccessSystemManaged
		}
		if err := repository.requireRole(tx, roleID); err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&AccessRoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]AccessRoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			rows = append(rows, AccessRoleMenu{RoleID: roleID, MenuID: menuID})
		}
		return tx.Create(&rows).Error
	})
}

func (repository *AccessRepository) EnsureAdministratorMenus(ctx context.Context, menuIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role AccessRole
		if err := tx.Where("code = ? AND system_managed = ?", identity.AccessAdministratorRoleCode, true).First(&role).Error; err != nil {
			return mapRoleError(err)
		}
		if err := tx.Where("role_id = ?", role.ID).Delete(&AccessRoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]AccessRoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			rows = append(rows, AccessRoleMenu{RoleID: role.ID, MenuID: menuID})
		}
		return tx.Create(&rows).Error
	})
}

func (repository *AccessRepository) GroupMenuIDs(ctx context.Context, groupID uint) ([]uint, error) {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&AccessUserGroup{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, identity.ErrUserGroupNotFound
	}
	var ids []uint
	err := repository.db.WithContext(ctx).Model(&AccessGroupMenu{}).Where("group_id = ?", groupID).Order("menu_id ASC").Pluck("menu_id", &ids).Error
	return ids, err
}

func (repository *AccessRepository) ReplaceGroupMenus(ctx context.Context, actorUserID, groupID uint, menuIDs []uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, actorUserID); err != nil {
			return err
		}
		var group AccessUserGroup
		if err := tx.First(&group, groupID).Error; err != nil {
			return mapUserGroupError(err)
		}
		var before []uint
		if err := tx.Model(&AccessGroupMenu{}).Where("group_id = ?", groupID).Order("menu_id ASC").Pluck("menu_id", &before).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&AccessGroupMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) > 0 {
			rows := make([]AccessGroupMenu, 0, len(menuIDs))
			for _, menuID := range menuIDs {
				rows = append(rows, AccessGroupMenu{GroupID: groupID, MenuID: menuID})
			}
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "group.menu_grants_replaced", ActorUserID: actorUserID, SubjectGroupID: uintPointer(groupID),
			BeforeJSON: auditJSON(map[string][]uint{"menu_ids": before}), AfterJSON: auditJSON(map[string][]uint{"menu_ids": menuIDs}),
		})
	})
}

func (repository *AccessRepository) DirectMenuIDsForUser(ctx context.Context, userID uint) ([]uint, error) {
	var roleIDs []uint
	err := repository.db.WithContext(ctx).Table("access_role_menus arm").
		Select("DISTINCT arm.menu_id").
		Joins("JOIN access_user_roles aur ON aur.role_id = arm.role_id").
		Joins("JOIN access_roles ar ON ar.id = aur.role_id AND ar.enabled = ?", true).
		Joins("JOIN users u ON u.id = aur.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Where("aur.user_id = ?", userID).
		Order("arm.menu_id ASC").
		Pluck("arm.menu_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}
	var groupIDs []uint
	err = repository.db.WithContext(ctx).Table("access_group_menus agm").
		Select("DISTINCT agm.menu_id").
		Joins("JOIN access_user_group_bindings augb ON augb.group_id = agm.group_id").
		Joins("JOIN access_user_groups aug ON aug.id = augb.group_id AND aug.enabled = ?", true).
		Joins("JOIN users u ON u.id = augb.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Where("augb.user_id = ?", userID).
		Order("agm.menu_id ASC").Pluck("agm.menu_id", &groupIDs).Error
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]struct{}, len(roleIDs)+len(groupIDs))
	for _, id := range append(roleIDs, groupIDs...) {
		seen[id] = struct{}{}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (repository *AccessRepository) CountMenuGrants(ctx context.Context, menuID uint) (int64, error) {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&AccessRoleMenu{}).Where("menu_id = ?", menuID).Count(&count).Error; err != nil {
		return 0, err
	}
	var groupCount int64
	err := repository.db.WithContext(ctx).Model(&AccessGroupMenu{}).Where("menu_id = ?", menuID).Count(&groupCount).Error
	return count + groupCount, err
}

func (repository *AccessRepository) EffectiveActionCodes(ctx context.Context, userID uint) ([]string, error) {
	var codes []string
	err := repository.db.WithContext(ctx).Raw(`
		SELECT DISTINCT asi.action_code
		FROM access_service_interfaces asi
		JOIN access_role_service_interfaces arsi ON arsi.service_interface_id = asi.id
		JOIN access_user_roles aur ON aur.role_id = arsi.role_id
		JOIN access_roles ar ON ar.id = aur.role_id AND ar.enabled = ?
		JOIN users u ON u.id = aur.user_id AND u.active = ? AND u.status = ?
		WHERE aur.user_id = ? AND asi.enabled = ?
		UNION
		SELECT DISTINCT asi.action_code
		FROM access_service_interfaces asi
		JOIN access_group_service_interfaces agsi ON agsi.service_interface_id = asi.id
		JOIN access_user_group_bindings augb ON augb.group_id = agsi.group_id
		JOIN access_user_groups aug ON aug.id = augb.group_id AND aug.enabled = ?
		JOIN users u ON u.id = augb.user_id AND u.active = ? AND u.status = ?
		WHERE augb.user_id = ? AND asi.enabled = ?
		ORDER BY action_code`, true, true, string(identity.AccountActive), userID, true,
		true, true, string(identity.AccountActive), userID, true).Scan(&codes).Error
	return codes, err
}

func (repository *AccessRepository) Authorize(ctx context.Context, userID uint, method, routeTemplate string) (identity.AuthorizationResult, error) {
	var serviceInterface AccessServiceInterface
	err := repository.db.WithContext(ctx).
		Where("http_method = ? AND route_template = ? AND enabled = ?", method, routeTemplate, true).
		First(&serviceInterface).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.AuthorizationInterfaceUnregistered, nil
	}
	if err != nil {
		return identity.AuthorizationForbidden, err
	}
	var count int64
	err = repository.db.WithContext(ctx).Table("access_user_roles aur").
		Joins("JOIN access_roles ar ON ar.id = aur.role_id AND ar.enabled = ?", true).
		Joins("JOIN access_role_service_interfaces arsi ON arsi.role_id = ar.id").
		Joins("JOIN users u ON u.id = aur.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Where("aur.user_id = ? AND arsi.service_interface_id = ?", userID, serviceInterface.ID).
		Count(&count).Error
	if err != nil {
		return identity.AuthorizationForbidden, err
	}
	if count == 0 {
		err = repository.db.WithContext(ctx).Table("access_user_group_bindings augb").
			Joins("JOIN access_user_groups aug ON aug.id = augb.group_id AND aug.enabled = ?", true).
			Joins("JOIN access_group_service_interfaces agsi ON agsi.group_id = aug.id").
			Joins("JOIN users u ON u.id = augb.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
			Where("augb.user_id = ? AND agsi.service_interface_id = ?", userID, serviceInterface.ID).
			Count(&count).Error
		if err != nil {
			return identity.AuthorizationForbidden, err
		}
		if count == 0 {
			return identity.AuthorizationForbidden, nil
		}
	}
	return identity.AuthorizationAllowed, nil
}

func (repository *AccessRepository) requireActiveUser(ctx context.Context, db *gorm.DB, userID uint) error {
	var count int64
	if err := db.WithContext(ctx).Model(&User{}).Where("id = ? AND active = ?", userID, true).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return identity.ErrAccessUserNotFound
	}
	return nil
}

func (repository *AccessRepository) requireRole(db *gorm.DB, roleID uint) error {
	var count int64
	if err := db.Model(&AccessRole{}).Where("id = ?", roleID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return identity.ErrAccessRoleNotFound
	}
	return nil
}

func requireEnabledRows(db *gorm.DB, model any, column string, ids []uint, notFound error) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := db.Model(model).Where(column+" IN ? AND enabled = ?", ids, true).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return notFound
	}
	return nil
}

func roleFromRecord(row AccessRole) identity.Role {
	return identity.Role{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description, Enabled: row.Enabled,
		SystemManaged: row.SystemManaged, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func rolesFromRecords(rows []AccessRole) []identity.Role {
	result := make([]identity.Role, 0, len(rows))
	for _, row := range rows {
		result = append(result, roleFromRecord(row))
	}
	return result
}

func serviceInterfaceFromRecord(row AccessServiceInterface) identity.ServiceInterface {
	return identity.ServiceInterface{
		ID: row.ID, Code: row.Code, ActionCode: row.ActionCode, HTTPMethod: row.HTTPMethod, RouteTemplate: row.RouteTemplate,
		Name: row.Name, Description: row.Description, Enabled: row.Enabled, SystemManaged: row.SystemManaged,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func enabledInterfaceIDsForActions(ctx context.Context, db *gorm.DB, actionCodes []string) ([]uint, error) {
	if len(actionCodes) == 0 {
		return nil, nil
	}
	var found []string
	if err := db.WithContext(ctx).Model(&AccessServiceInterface{}).
		Where("action_code IN ? AND enabled = ?", actionCodes, true).
		Distinct("action_code").Pluck("action_code", &found).Error; err != nil {
		return nil, err
	}
	if len(found) != len(actionCodes) {
		return nil, identity.ErrAccessActionNotFound
	}
	var interfaceIDs []uint
	err := db.WithContext(ctx).Model(&AccessServiceInterface{}).
		Where("action_code IN ? AND enabled = ?", actionCodes, true).
		Order("id ASC").Pluck("id", &interfaceIDs).Error
	return interfaceIDs, err
}

func actionCodesForGrant(ctx context.Context, db *gorm.DB, grantTable, subjectColumn string, subjectID uint) ([]string, error) {
	var codes []string
	err := db.WithContext(ctx).Table(grantTable+" grants").
		Select("DISTINCT asi.action_code").
		Joins("JOIN access_service_interfaces asi ON asi.id = grants.service_interface_id AND asi.enabled = ?", true).
		Where("grants."+subjectColumn+" = ?", subjectID).
		Order("asi.action_code ASC").Pluck("asi.action_code", &codes).Error
	return codes, err
}

func mapRoleError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.ErrAccessRoleNotFound
	}
	return err
}

func mapAccessWriteError(err error, conflict error) error {
	if isDuplicateError(err) {
		return conflict
	}
	return err
}

func mapServiceInterfaceCreateError(err error) error {
	if !isDuplicateError(err) {
		return err
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "access_service_interfaces.code") ||
		strings.Contains(message, "access_service_interface_code") {
		return identity.ErrAccessCodeConflict
	}
	return identity.ErrAccessRouteConflict
}

func bindLegacySuperAdmins(tx *gorm.DB, roleID uint) error {
	var userIDs []uint
	if err := tx.Model(&User{}).Where("role = ?", identity.LegacySuperAdminRole).
		Order("id ASC").Pluck("id", &userIDs).Error; err != nil {
		return err
	}
	for _, userID := range userIDs {
		binding := AccessUserRole{UserID: userID, RoleID: roleID}
		if err := tx.Where("user_id = ? AND role_id = ?", userID, roleID).
			FirstOrCreate(&binding).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedDefaultGroupBaselineGrants(
	tx *gorm.DB,
	inputs []identity.ServiceInterfaceInput,
	interfaceIDs []uint,
	excludedActionCodes []string,
) error {
	if len(inputs) != len(interfaceIDs) {
		return identity.ErrAccessRouteNotRegistered
	}
	group, err := EnsureDefaultUserGroup(tx)
	if err != nil {
		return err
	}
	var count int64
	if err := tx.Model(&AccessGroupServiceInterface{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	excluded := make(map[string]struct{}, len(excludedActionCodes))
	for _, actionCode := range excludedActionCodes {
		excluded[strings.TrimSpace(actionCode)] = struct{}{}
	}
	for index, input := range inputs {
		if strings.HasPrefix(strings.TrimSpace(input.ActionCode), identity.AccessManagementActionPrefix) {
			continue
		}
		if _, skip := excluded[strings.TrimSpace(input.ActionCode)]; skip {
			continue
		}
		grant := AccessGroupServiceInterface{GroupID: group.ID, ServiceInterfaceID: interfaceIDs[index]}
		if err := tx.Where("group_id = ? AND service_interface_id = ?", group.ID, interfaceIDs[index]).
			FirstOrCreate(&grant).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureAccessAdministratorRole(tx *gorm.DB) (AccessRole, error) {
	var row AccessRole
	err := tx.Where("code = ?", identity.AccessAdministratorRoleCode).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = AccessRole{
			Code: identity.AccessAdministratorRoleCode, Name: "Access administrator",
			Enabled: true, SystemManaged: true,
		}
		if err := tx.Create(&row).Error; err != nil {
			return AccessRole{}, mapAccessWriteError(err, identity.ErrAccessCodeConflict)
		}
		return row, nil
	}
	if err != nil {
		return AccessRole{}, err
	}
	if err := tx.Model(&row).Updates(map[string]any{
		"enabled": true, "system_managed": true,
	}).Error; err != nil {
		return AccessRole{}, err
	}
	if err := tx.First(&row, row.ID).Error; err != nil {
		return AccessRole{}, err
	}
	return row, nil
}

func ensureSystemServiceInterface(tx *gorm.DB, input identity.ServiceInterfaceInput) (AccessServiceInterface, error) {
	var byCode AccessServiceInterface
	err := tx.Where("code = ?", input.Code).First(&byCode).Error
	if err == nil {
		if byCode.HTTPMethod != input.HTTPMethod || byCode.RouteTemplate != input.RouteTemplate {
			return AccessServiceInterface{}, identity.ErrAccessCodeConflict
		}
		if err := tx.Model(&byCode).Updates(map[string]any{
			"action_code": input.ActionCode, "name": input.Name, "description": input.Description,
			"enabled": true, "system_managed": true,
		}).Error; err != nil {
			return AccessServiceInterface{}, err
		}
		if err := tx.First(&byCode, byCode.ID).Error; err != nil {
			return AccessServiceInterface{}, err
		}
		return byCode, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AccessServiceInterface{}, err
	}
	var byRoute AccessServiceInterface
	err = tx.Where("http_method = ? AND route_template = ?", input.HTTPMethod, input.RouteTemplate).First(&byRoute).Error
	if err == nil {
		if !byRoute.SystemManaged {
			return AccessServiceInterface{}, identity.ErrAccessRouteConflict
		}
		if err := tx.Model(&byRoute).Updates(map[string]any{
			"code": input.Code, "action_code": input.ActionCode, "name": input.Name,
			"description": input.Description, "enabled": true, "system_managed": true,
		}).Error; err != nil {
			return AccessServiceInterface{}, mapServiceInterfaceCreateError(err)
		}
		if err := tx.First(&byRoute, byRoute.ID).Error; err != nil {
			return AccessServiceInterface{}, err
		}
		return byRoute, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AccessServiceInterface{}, err
	}
	row := AccessServiceInterface{
		Code: input.Code, ActionCode: input.ActionCode, HTTPMethod: input.HTTPMethod, RouteTemplate: input.RouteTemplate,
		Name: input.Name, Description: input.Description, Enabled: true, SystemManaged: true,
	}
	if err := tx.Create(&row).Error; err != nil {
		return AccessServiceInterface{}, mapServiceInterfaceCreateError(err)
	}
	return row, nil
}

func isDuplicateError(err error) bool {
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "unique") || strings.Contains(value, "duplicate")
}

var _ identity.AccessRepository = (*AccessRepository)(nil)
