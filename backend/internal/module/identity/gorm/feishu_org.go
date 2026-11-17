package identitygorm

import (
	"context"
	"time"

	"gorm.io/gorm"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

// FeishuOrgStoreRepository persists the synced Feishu org projection: the
// department snapshot table and the per-user department/open_id mapping.
type FeishuOrgStoreRepository struct {
	db *gorm.DB
}

func NewFeishuOrgStoreRepository(db *gorm.DB) *FeishuOrgStoreRepository {
	return &FeishuOrgStoreRepository{db: db}
}

// ReplaceDepartments swaps the whole department snapshot atomically. The
// table mirrors the Feishu tree and stays small (tens of rows), so a full
// swap keeps the parent-order invariant without diffing.
func (repository *FeishuOrgStoreRepository) ReplaceDepartments(
	ctx context.Context,
	departments []identity.FeishuDepartment,
) error {
	if repository == nil || repository.db == nil {
		return identity.ErrReportingDirectoryQueryInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM feishu_departments").Error; err != nil {
			return err
		}
		if len(departments) == 0 {
			return nil
		}
		now := time.Now()
		rows := make([]FeishuDepartment, 0, len(departments))
		for _, department := range departments {
			rows = append(rows, FeishuDepartment{
				OpenDepartmentID:   department.OpenID,
				ParentDepartmentID: department.ParentID,
				Name:               department.Name,
				DeptOrder:          department.Order,
				MemberCount:        department.MemberCount,
				SyncedAt:           now,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (repository *FeishuOrgStoreRepository) DepartmentSyncedAt(
	ctx context.Context,
) (time.Time, bool, error) {
	var latest *time.Time
	err := repository.db.WithContext(ctx).Table("feishu_departments").
		Select("MAX(synced_at)").
		Scan(&latest).Error
	if err != nil || latest == nil {
		return time.Time{}, false, err
	}
	return *latest, true, nil
}

func (repository *FeishuOrgStoreRepository) activeUserScope() string {
	return "active = ? AND status = ?"
}

// ResolveUsersByOpenID maps Feishu open ids onto active PM user ids.
func (repository *FeishuOrgStoreRepository) ResolveUsersByOpenID(
	ctx context.Context,
	openIDs []string,
) (map[string]uint, error) {
	result := make(map[string]uint, len(openIDs))
	if len(openIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID        uint
		FeishuOID string
	}
	err := repository.db.WithContext(ctx).Table("users").
		Where("feishu_open_id IN ? AND "+repository.activeUserScope(), openIDs, true, string(identity.AccountActive)).
		Select("id, feishu_open_id AS feishu_oid").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.FeishuOID] = row.ID
	}
	return result, nil
}

// ResolveUniqueUsersByName maps display names to the single active user with
// that name; ambiguous names resolve to nothing so same-named colleagues are
// never mis-mapped. open_id matching cannot work across apps (open_id is
// app-scoped and the stored ids come from the login app), so the name is the
// primary fallback; SetUserDepartments never overwrites an existing open_id.
func (repository *FeishuOrgStoreRepository) ResolveUniqueUsersByName(
	ctx context.Context,
	names []string,
) (map[string]uint, error) {
	result := make(map[string]uint, len(names))
	if len(names) == 0 {
		return result, nil
	}
	var rows []struct {
		DisplayName string
		UserCount   int
		UserID      uint
	}
	err := repository.db.WithContext(ctx).Table("users").
		Where("display_name IN ? AND "+repository.activeUserScope(), names, true, string(identity.AccountActive)).
		Select("display_name, COUNT(*) AS user_count, MIN(id) AS user_id").
		Group("display_name").
		Having("COUNT(*) = 1").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserCount == 1 {
			result[row.DisplayName] = row.UserID
		}
	}
	return result, nil
}

// SetUserDepartments writes user→department mappings and adopts the Feishu
// open_id for users that had none. The job title follows the department:
// every sync overwrites it with the contact's latest Feishu job title.
func (repository *FeishuOrgStoreRepository) SetUserDepartments(
	ctx context.Context,
	assignments []identity.FeishuUserDepartmentAssignment,
) error {
	if len(assignments) == 0 {
		return nil
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, assignment := range assignments {
			err := tx.Exec(
				"UPDATE users SET feishu_open_id = COALESCE(feishu_open_id, ?), feishu_department_id = ?, feishu_job_title = ?, updated_at = ? WHERE id = ?",
				assignment.OpenID, assignment.DepartmentID, assignment.JobTitle, time.Now(), assignment.UserID,
			).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearUserDepartments empties the department and job title of previously
// mapped users that no longer appear in the contact data.
func (repository *FeishuOrgStoreRepository) ClearUserDepartments(
	ctx context.Context,
	keepUserIDs []uint,
) error {
	if len(keepUserIDs) == 0 {
		// NOT IN () is invalid SQL; an empty keep-set means clear everyone.
		return repository.db.WithContext(ctx).Exec(
			"UPDATE users SET feishu_department_id = '', feishu_job_title = '', updated_at = ? WHERE feishu_department_id <> ''",
			time.Now(),
		).Error
	}
	return repository.db.WithContext(ctx).Exec(
		"UPDATE users SET feishu_department_id = '', feishu_job_title = '', updated_at = ? WHERE feishu_department_id <> '' AND id NOT IN ?",
		time.Now(), keepUserIDs,
	).Error
}

// FeishuOrgStatus reports the operator-facing org projection state: the last
// successful sync, department count, and how many active users are mapped.
func (repository *FeishuOrgStoreRepository) FeishuOrgStatus(
	ctx context.Context,
) (identity.FeishuOrgSyncStatus, error) {
	status := identity.FeishuOrgSyncStatus{}
	var latest *time.Time
	if err := repository.db.WithContext(ctx).Table("feishu_departments").
		Select("MAX(synced_at)").Scan(&latest).Error; err != nil {
		return status, err
	}
	if latest != nil {
		status.HasSynced = true
		status.SyncedAt = *latest
	}
	if err := repository.db.WithContext(ctx).Table("feishu_departments").
		Select("COUNT(*)").Scan(&status.Departments).Error; err != nil {
		return status, err
	}
	var activeTotal, mapped int64
	if err := repository.db.WithContext(ctx).Table("users").
		Where("active = ? AND status = ?", true, string(identity.AccountActive)).
		Select("COUNT(*)").Scan(&activeTotal).Error; err != nil {
		return status, err
	}
	if err := repository.db.WithContext(ctx).Table("users").
		Where("feishu_department_id <> '' AND active = ? AND status = ?", true, string(identity.AccountActive)).
		Select("COUNT(*)").Scan(&mapped).Error; err != nil {
		return status, err
	}
	status.MappedUsers = int(mapped)
	status.UnmappedUsers = int(activeTotal) - int(mapped)
	return status, nil
}
