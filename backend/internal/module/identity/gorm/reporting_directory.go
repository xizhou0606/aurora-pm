package identitygorm

import (
	"context"
	"sort"

	"gorm.io/gorm"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

const reportingDirectoryPageSize = 500

// ReportingDirectoryRepository projects the reporting directory read. User
// facts (identity, display name, avatar) come from the SAME repository the
// baseline /core/directory/users interface uses; only the role-group binding
// and role-name hydration are layered on top for the reporting grouping.
type ReportingDirectoryRepository struct {
	directory *BaselineDirectoryRepository
	db        *gorm.DB
}

func NewReportingDirectoryRepository(db *gorm.DB) *ReportingDirectoryRepository {
	return &ReportingDirectoryRepository{
		directory: NewBaselineDirectoryRepository(db),
		db:        db,
	}
}

func (repository *ReportingDirectoryRepository) ListWorkspaceDirectoryMembers(
	ctx context.Context,
	workspaceID uint,
) ([]identity.DirectoryMember, error) {
	// Canonical user facts with avatars, keyed by internal PM user ID.
	users := make(map[uint]identity.DirectoryUser, 0)
	offset := 0
	for {
		page, err := repository.directory.SearchUsers(ctx, workspaceID, identity.DirectoryUserFilter{
			Limit:  reportingDirectoryPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		for _, user := range page.Items {
			users[user.ID] = user
		}
		if len(page.Items) < reportingDirectoryPageSize || int64(len(users)) >= page.Total {
			break
		}
		offset += reportingDirectoryPageSize
	}
	if len(users) == 0 {
		return []identity.DirectoryMember{}, nil
	}

	// Role-group bindings and effective role names, keyed by internal PM user ID.
	bindings, err := repository.groupBindings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	effective, err := repository.effectiveRolesByUser(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	departments, err := repository.userDepartments(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	members := make([]identity.DirectoryMember, 0, len(users))
	for factID, user := range users {
		binding := bindings[factID]
		names := effective[factID].roleNames
		if names == nil {
			names = []string{}
		}
		members = append(members, identity.DirectoryMember{
			ID:             binding.internalID,
			FactID:         factID,
			Username:       user.Username,
			DisplayName:    user.DisplayName,
			AvatarURL:      user.AvatarURL,
			GroupID:        binding.groupID,
			GroupCode:      binding.groupCode,
			GroupName:      binding.groupName,
			RoleNames:      names,
			JobTitle:       effective[factID].jobTitle,
			DepartmentID:   departments[factID].departmentID,
			DepartmentName: departments[factID].departmentName,
		})
	}
	sort.SliceStable(members, func(left, right int) bool {
		if members[left].GroupName != members[right].GroupName {
			return members[left].GroupName < members[right].GroupName
		}
		if members[left].DisplayName != members[right].DisplayName {
			return members[left].DisplayName < members[right].DisplayName
		}
		return members[left].FactID < members[right].FactID
	})
	return members, nil
}

func (repository *ReportingDirectoryRepository) ListEnabledGroups(
	ctx context.Context,
) ([]identity.UserGroup, error) {
	// Pin the identity-owned table: GORM would otherwise pluralize the
	// projection struct into a nonexistent "user_groups" table.
	var groups []identity.UserGroup
	err := repository.db.WithContext(ctx).Table("access_user_groups").
		Where("enabled = ?", true).
		Order("name ASC, id ASC").
		Select("id, code, name, description, enabled, system_managed, created_at, updated_at").
		Find(&groups).Error
	return groups, err
}

// ListFeishuDepartments reads the synced Feishu org tree in display order.
func (repository *ReportingDirectoryRepository) ListFeishuDepartments(
	ctx context.Context,
) ([]identity.DirectoryDepartment, error) {
	var rows []FeishuDepartment
	err := repository.db.WithContext(ctx).Table("feishu_departments").
		Order("dept_order ASC, name ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	departments := make([]identity.DirectoryDepartment, 0, len(rows))
	for _, row := range rows {
		departments = append(departments, identity.DirectoryDepartment{
			ID:          row.ID,
			OpenID:      row.OpenDepartmentID,
			ParentID:    row.ParentDepartmentID,
			Name:        row.Name,
			Order:       row.DeptOrder,
			MemberCount: row.MemberCount,
		})
	}
	return departments, nil
}

type reportingUserDepartment struct {
	departmentID   string
	departmentName string
}

// userDepartments resolves the synced Feishu department (id + display name)
// per workspace member, keyed by internal PM user ID.
func (repository *ReportingDirectoryRepository) userDepartments(
	ctx context.Context,
	workspaceID uint,
) (map[uint]reportingUserDepartment, error) {
	var rows []struct {
		FactID     uint
		DeptID     string
		Department string
	}
	err := repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("LEFT JOIN feishu_departments AS fd ON fd.open_department_id = u.feishu_department_id").
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Select("u.id AS fact_id, u.feishu_department_id AS dept_id, COALESCE(fd.name, '') AS department").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]reportingUserDepartment, len(rows))
	for _, row := range rows {
		if row.DeptID == "" {
			continue
		}
		result[row.FactID] = reportingUserDepartment{departmentID: row.DeptID, departmentName: row.Department}
	}
	return result, nil
}

func (repository *ReportingDirectoryRepository) ListEnabledWorkspaceIDs(
	ctx context.Context,
) ([]uint, error) {
	var ids []uint
	err := repository.db.WithContext(ctx).Table("workspaces AS w").
		Where("w.enabled = ?", true).
		Order("w.id ASC").
		Pluck("w.id", &ids).Error
	return ids, err
}

type reportingGroupBinding struct {
	internalID uint
	groupID    uint
	groupCode  string
	groupName  string
}

func (repository *ReportingDirectoryRepository) groupBindings(
	ctx context.Context,
	workspaceID uint,
) (map[uint]reportingGroupBinding, error) {
	var rows []struct {
		FactID     uint
		InternalID uint
		GroupID    uint
		GroupCode  string
		GroupName  string
	}
	err := repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("LEFT JOIN access_user_group_bindings AS augb ON augb.user_id = u.id").
		Joins("LEFT JOIN access_user_groups AS aug ON aug.id = augb.group_id AND aug.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Select("u.id AS fact_id, u.id AS internal_id, COALESCE(aug.id, 0) AS group_id, COALESCE(aug.code, '') AS group_code, COALESCE(aug.name, '') AS group_name").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	bindings := make(map[uint]reportingGroupBinding, len(rows))
	for _, row := range rows {
		bindings[row.FactID] = reportingGroupBinding{
			internalID: row.InternalID,
			groupID:    row.GroupID,
			groupCode:  row.GroupCode,
			groupName:  row.GroupName,
		}
	}
	return bindings, nil
}

// reportingEffectiveRoles carries the displayed role names of one member
// together with the synced Feishu job title that sourced a fallback.
type reportingEffectiveRoles struct {
	roleNames []string
	jobTitle  string
}

// effectiveRolesByUser resolves the displayed role names per active workspace
// member with a priority rule: granted permission roles (access_user_roles /
// access_roles) always win; a member without any granted role falls back to
// the synced Feishu job title (users.feishu_job_title) so the directory does
// not degrade to an all-empty 暂无角色 state; a member with neither keeps an
// empty list. The job title is returned even when permission roles exist so
// callers can label the fallback source separately from granted roles.
func (repository *ReportingDirectoryRepository) effectiveRolesByUser(
	ctx context.Context,
	workspaceID uint,
) (map[uint]reportingEffectiveRoles, error) {
	var roleRows []struct {
		FactID   uint
		RoleName string
	}
	err := repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("JOIN access_user_roles AS aur ON aur.user_id = u.id").
		Joins("JOIN access_roles AS ar ON ar.id = aur.role_id AND ar.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Order("ar.name ASC").
		Select("u.id AS fact_id, ar.name AS role_name").
		Find(&roleRows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]reportingEffectiveRoles)
	for _, row := range roleRows {
		entry := result[row.FactID]
		entry.roleNames = append(entry.roleNames, row.RoleName)
		result[row.FactID] = entry
	}

	var jobRows []struct {
		FactID   uint
		JobTitle string
	}
	err = repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
		Order("u.id ASC").
		Select("u.id AS fact_id, COALESCE(u.feishu_job_title, '') AS job_title").
		Find(&jobRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range jobRows {
		entry := result[row.FactID]
		entry.jobTitle = row.JobTitle
		if len(entry.roleNames) == 0 && row.JobTitle != "" {
			entry.roleNames = []string{row.JobTitle}
		}
		result[row.FactID] = entry
	}
	return result, nil
}
