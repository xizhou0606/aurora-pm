package identitygorm

import (
	"context"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

// FindPasswordLoginUser preserves exact username lookup and additionally
// accepts an unambiguous normalized profile email. Colliding identities fail
// closed rather than selecting whichever row happens to be returned first.
// The caller still owns password and account-state validation.
func (r *LoginRepository) FindPasswordLoginUser(ctx context.Context, identifier string) (User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return User{}, gorm.ErrRecordNotFound
	}
	query := r.db.WithContext(ctx).Model(&User{}).Where("username = ?", identifier)
	if email := identity.NormalizeLoginEmail(identifier); email != "" {
		query = query.Or("LOWER(TRIM(email)) = ?", email)
	}
	var users []User
	if err := query.Limit(2).Find(&users).Error; err != nil {
		return User{}, err
	}
	if len(users) != 1 {
		return User{}, gorm.ErrRecordNotFound
	}
	return users[0], nil
}
