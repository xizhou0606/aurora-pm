package identitygorm

import (
	"context"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func isAccessAdministrator(ctx context.Context, db *gorm.DB, principalID uint) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("access_user_roles AS aur").
		Joins("JOIN access_roles AS ar ON ar.id = aur.role_id").
		Where(
			"aur.user_id = ? AND ar.code = ? AND ar.enabled = ?",
			principalID,
			identity.AccessAdministratorRoleCode,
			true,
		).
		Count(&count).Error
	return count > 0, err
}
