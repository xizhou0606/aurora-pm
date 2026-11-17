package identitygorm

import (
	"context"
	"errors"
	"strings"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LoginRepository struct {
	db *gorm.DB
	// newUserHook fires after a fresh user row is provisioned by Feishu login
	// (composition-root optional; e.g. triggers the Feishu org sync so the
	// new colleague enters the department tree immediately).
	newUserHook func()
}

func NewLoginRepository(db *gorm.DB) *LoginRepository { return &LoginRepository{db: db} }

// SetNewUserHook registers the post-provisioning callback.
func (r *LoginRepository) SetNewUserHook(hook func()) { r.newUserHook = hook }

func (r *LoginRepository) CreateLoginTransaction(ctx context.Context, value *identity.LoginTransaction) error {
	row := loginTransactionRecord(*value)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	*value = loginTransactionDomain(row)
	return nil
}

func (r *LoginRepository) ConsumeLoginTransaction(ctx context.Context, flow, stateHash string, now time.Time) (identity.LoginTransaction, error) {
	var value LoginTransaction
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		consumedAt := now.UTC()
		result := tx.Model(&LoginTransaction{}).
			Where("flow = ? AND state_hash = ? AND consumed_at IS NULL AND expires_at > ?", flow, stateHash, now.UTC()).
			Updates(map[string]any{"consumed_at": consumedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("state_hash = ?", stateHash).First(&value).Error; err != nil {
			return err
		}
		if value.PKCEVerifier != "" {
			return tx.Model(&LoginTransaction{}).Where("id = ?", value.ID).Update("pkce_verifier", "").Error
		}
		return nil
	})
	if err != nil {
		return identity.LoginTransaction{}, err
	}
	return loginTransactionDomain(value), nil
}

func (r *LoginRepository) CreateLoginCode(ctx context.Context, value *identity.LoginCode) error {
	row := loginCodeRecord(*value)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	*value = loginCodeDomain(row)
	return nil
}

func (r *LoginRepository) ConsumeLoginCode(ctx context.Context, codeHash string, now time.Time) (identity.LoginCode, error) {
	consumedAt := now.UTC()
	result := r.db.WithContext(ctx).Model(&LoginCode{}).
		Where("code_hash = ? AND consumed_at IS NULL AND expires_at > ?", codeHash, now.UTC()).
		Updates(map[string]any{"consumed_at": consumedAt})
	if result.Error != nil {
		return identity.LoginCode{}, result.Error
	}
	if result.RowsAffected != 1 {
		return identity.LoginCode{}, gorm.ErrRecordNotFound
	}
	var value LoginCode
	if err := r.db.WithContext(ctx).Where("code_hash = ?", codeHash).First(&value).Error; err != nil {
		return identity.LoginCode{}, err
	}
	return loginCodeDomain(value), nil
}

func (r *LoginRepository) RegisterFeishuTenants(ctx context.Context, tenantKeys []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, tenantKey := range tenantKeys {
			value := FeishuTenant{TenantKey: tenantKey, Enabled: true}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tenant_key"}},
				DoNothing: true,
			}).Create(&value).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *LoginRepository) ProvisionFeishuUser(ctx context.Context, provider identity.FeishuIdentity) (identity.User, error) {
	var result User
	provisionedNow := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := resolveOrBootstrapFeishuTenant(tx, provider.TenantKey)
		if err != nil {
			return err
		}
		if !tenant.Enabled {
			return identity.ErrLoginTenantDisabled
		}
		if err := attachUnassignedUsersToSoleTenant(tx, tenant); err != nil {
			return err
		}
		if provider.OpenID != "" {
			value, err := findUserByFeishuOpenID(tx, provider.OpenID)
			if err == nil {
				if value.FeishuTenantID == nil || *value.FeishuTenantID != tenant.ID {
					return identity.ErrLoginFeishuIdentityConflict
				}
				if !value.Active {
					return identity.ErrLoginUserDisabled
				}
				if err := EnsureUserInDefaultGroup(tx, value.ID); err != nil {
					return err
				}
				if err := persistFeishuDeliveryIdentity(tx, &value, provider); err != nil {
					return err
				}
				result = value
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		value, err := findUserByFeishuName(tx, tenant.ID, provider.DisplayName)
		if err == nil {
			if !value.Active {
				return identity.ErrLoginUserDisabled
			}
			if err := EnsureUserInDefaultGroup(tx, value.ID); err != nil {
				return err
			}
			if err := persistFeishuDeliveryIdentity(tx, &value, provider); err != nil {
				return err
			}
			result = value
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		tenantID := tenant.ID
		value = User{
			ExternalIssuer:  "feishu_tenant",
			ExternalSubject: identity.FeishuExternalSubject(provider.TenantKey, provider.DisplayName),
			Username:        identity.FeishuUsername(provider.TenantKey, provider.DisplayName),
			DisplayName:     provider.DisplayName,
			AvatarURL:       provider.AvatarURL,
			FeishuOpenID:    optionalString(provider.OpenID),
			FeishuTenantID:  &tenantID,
			Role:            "user",
			Active:          true,
			Status:          string(identity.AccountPendingApproval),
			SessionVersion:  1,
		}
		createResult := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "feishu_tenant_id"},
				{Name: "display_name"},
			},
			DoNothing: true,
		}).Create(&value)
		if createResult.Error != nil {
			return createResult.Error
		}
		createdNow := createResult.RowsAffected == 1
		provisionedNow = provisionedNow || createdNow
		if createResult.RowsAffected == 0 {
			// Another login may have provisioned this exact company/name
			// between the lookup and insert. Re-read the canonical row so both
			// requests converge on the same PM user.
			value, err = findUserByFeishuName(tx, tenant.ID, provider.DisplayName)
			if err != nil {
				return err
			}
			if !value.Active {
				return identity.ErrLoginUserDisabled
			}
		}
		if value.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := EnsureUserInDefaultGroup(tx, value.ID); err != nil {
			return err
		}
		if createdNow {
			if err := EnsureUserInDefaultWorkspace(tx, value.ID); err != nil {
				return err
			}
		}
		if err := persistFeishuDeliveryIdentity(tx, &value, provider); err != nil {
			return err
		}
		result = value
		return nil
	})
	if err == nil && provisionedNow && r.newUserHook != nil {
		r.newUserHook()
	}
	return userDomain(result), err
}

// resolveOrBootstrapFeishuTenant supports the one-time migration from the
// legacy single-company users table. Once a company has been recorded, all
// later companies must already be registered by configuration or operations.
func resolveOrBootstrapFeishuTenant(tx *gorm.DB, tenantKey string) (FeishuTenant, error) {
	var tenant FeishuTenant
	err := tx.Where("tenant_key = ?", tenantKey).First(&tenant).Error
	if err == nil {
		return tenant, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return FeishuTenant{}, err
	}

	var tenantCount int64
	if err := tx.Model(&FeishuTenant{}).Count(&tenantCount).Error; err != nil {
		return FeishuTenant{}, err
	}
	if tenantCount != 0 {
		return FeishuTenant{}, identity.ErrLoginTenantNotRegistered
	}
	tenant = FeishuTenant{TenantKey: tenantKey, Enabled: true}
	if err := tx.Create(&tenant).Error; err != nil {
		return FeishuTenant{}, err
	}
	return tenant, nil
}

// attachUnassignedUsersToSoleTenant performs the one-time legacy company
// association only when PM has exactly one allowed company and no existing
// user-company links. It never changes project, task, or workspace data.
func attachUnassignedUsersToSoleTenant(tx *gorm.DB, tenant FeishuTenant) error {
	var tenantCount, assignedUserCount int64
	if err := tx.Model(&FeishuTenant{}).Count(&tenantCount).Error; err != nil {
		return err
	}
	if tenantCount != 1 {
		return nil
	}
	if err := tx.Model(&User{}).Where("feishu_tenant_id IS NOT NULL").Count(&assignedUserCount).Error; err != nil {
		return err
	}
	if assignedUserCount != 0 {
		return nil
	}
	return tx.Model(&User{}).Where("feishu_tenant_id IS NULL").Update("feishu_tenant_id", tenant.ID).Error
}

func findUserByFeishuName(tx *gorm.DB, tenantID uint, displayName string) (User, error) {
	var displayMatches []User
	if err := tx.Where("feishu_tenant_id = ? AND display_name = ?", tenantID, displayName).
		Order("id ASC").Limit(2).Find(&displayMatches).Error; err != nil {
		return User{}, err
	}
	if len(displayMatches) == 1 {
		return displayMatches[0], nil
	}
	if len(displayMatches) > 1 {
		return User{}, identity.ErrLoginFeishuIdentityAmbiguous
	}
	return User{}, gorm.ErrRecordNotFound
}

func findUserByFeishuOpenID(tx *gorm.DB, openID string) (User, error) {
	var value User
	err := tx.Where("feishu_open_id = ?", openID).First(&value).Error
	return value, err
}

func (r *LoginRepository) FindActiveUserByExternalIdentity(ctx context.Context, issuer, subject string) (identity.User, error) {
	var value User
	err := r.db.WithContext(ctx).Where("external_issuer = ? AND external_subject = ? AND active = ?", issuer, subject, true).First(&value).Error
	return userDomain(value), err
}

func (r *LoginRepository) ResolveOrBindGoogleUser(ctx context.Context, provider identity.GoogleIdentity) (identity.User, error) {
	issuer := strings.TrimSpace(provider.Issuer)
	subject := strings.TrimSpace(provider.Subject)
	email := strings.ToLower(strings.TrimSpace(provider.Email))
	if issuer == "" || subject == "" || email == "" || !provider.EmailVerified {
		return identity.User{}, identity.ErrLoginGoogleIdentityInvalid
	}

	binding, err := findGoogleBinding(r.db.WithContext(ctx), issuer, subject)
	if err != nil {
		return identity.User{}, identity.ErrLoginUserNotFound
	}
	var resolved User
	if err := loadGoogleBindingUser(r.db.WithContext(ctx), binding, &resolved); err != nil {
		return identity.User{}, err
	}
	return userDomain(resolved), nil
}

func (r *LoginRepository) ResolveTelegramUser(ctx context.Context, provider identity.TelegramIdentity) (identity.User, error) {
	issuer, subject := strings.TrimSpace(provider.Issuer), strings.TrimSpace(provider.Subject)
	if issuer != "https://oauth.telegram.org" || subject == "" || provider.TelegramUserID <= 0 {
		return identity.User{}, identity.ErrLoginTelegramIdentityInvalid
	}
	var user User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		binding, err := findLoginIdentity(tx, identity.FlowTelegram, issuer, subject)
		if err != nil {
			return identity.ErrLoginUserNotFound
		}
		if binding.TelegramUserID != nil && *binding.TelegramUserID != provider.TelegramUserID {
			return identity.ErrLoginTelegramIdentityConflict
		}
		if binding.TelegramUserID == nil {
			result := tx.Model(&UserLoginIdentity{}).
				Where("id = ? AND telegram_user_id IS NULL", binding.ID).
				Update("telegram_user_id", provider.TelegramUserID)
			if result.Error != nil {
				if isDuplicateError(result.Error) {
					return identity.ErrLoginTelegramIdentityConflict
				}
				return result.Error
			}
			if result.RowsAffected != 1 {
				var current UserLoginIdentity
				if err := tx.Where("id = ?", binding.ID).First(&current).Error; err != nil ||
					current.TelegramUserID == nil || *current.TelegramUserID != provider.TelegramUserID {
					return identity.ErrLoginTelegramIdentityConflict
				}
			}
		}
		if err := tx.Where("id = ?", binding.UserID).First(&user).Error; err != nil {
			return identity.ErrLoginUserNotFound
		}
		if !user.Active {
			return identity.ErrLoginUserDisabled
		}
		return nil
	})
	return userDomain(user), err
}

func findGoogleBinding(tx *gorm.DB, issuer, subject string) (UserLoginIdentity, error) {
	return findLoginIdentity(tx, identity.FlowGoogle, issuer, subject)
}

func findLoginIdentity(tx *gorm.DB, provider, issuer, subject string) (UserLoginIdentity, error) {
	var binding UserLoginIdentity
	err := tx.Where("provider = ? AND issuer = ? AND subject = ?", provider, issuer, subject).First(&binding).Error
	return binding, err
}

func loadGoogleBindingUser(tx *gorm.DB, binding UserLoginIdentity, target *User) error {
	if err := tx.Where("id = ?", binding.UserID).First(target).Error; err != nil {
		return identity.ErrLoginUserNotFound
	}
	if !target.Active {
		return identity.ErrLoginUserDisabled
	}
	return nil
}

func (r *LoginRepository) FindActiveUserByID(ctx context.Context, id uint) (identity.User, error) {
	var value User
	err := r.db.WithContext(ctx).Where("id = ? AND active = ?", id, true).First(&value).Error
	return userDomain(value), err
}

var _ interface {
	CreateLoginTransaction(context.Context, *identity.LoginTransaction) error
	ConsumeLoginTransaction(context.Context, string, string, time.Time) (identity.LoginTransaction, error)
	CreateLoginCode(context.Context, *identity.LoginCode) error
	ConsumeLoginCode(context.Context, string, time.Time) (identity.LoginCode, error)
	RegisterFeishuTenants(context.Context, []string) error
	ProvisionFeishuUser(context.Context, identity.FeishuIdentity) (identity.User, error)
	FindActiveUserByExternalIdentity(context.Context, string, string) (identity.User, error)
	ResolveOrBindGoogleUser(context.Context, identity.GoogleIdentity) (identity.User, error)
	ResolveTelegramUser(context.Context, identity.TelegramIdentity) (identity.User, error)
	FindActiveUserByID(context.Context, uint) (identity.User, error)
	FeishuOpenIDs(context.Context, []uint) (map[uint]string, error)
	FeishuEmails(context.Context, []uint) (map[uint]string, error)
	SaveFeishuOpenID(context.Context, uint, string) error
} = (*LoginRepository)(nil)

func loginTransactionRecord(value identity.LoginTransaction) LoginTransaction {
	return LoginTransaction{ID: value.ID, StateHash: value.StateHash, NonceHash: value.NonceHash, PKCEVerifier: value.PKCEVerifier, Flow: value.Flow, ReturnTo: value.ReturnTo, ExpiresAt: value.ExpiresAt, ConsumedAt: value.ConsumedAt, CreatedAt: value.CreatedAt}
}

func loginTransactionDomain(value LoginTransaction) identity.LoginTransaction {
	return identity.LoginTransaction{ID: value.ID, StateHash: value.StateHash, NonceHash: value.NonceHash, PKCEVerifier: value.PKCEVerifier, Flow: value.Flow, ReturnTo: value.ReturnTo, ExpiresAt: value.ExpiresAt, ConsumedAt: value.ConsumedAt, CreatedAt: value.CreatedAt}
}

func loginCodeRecord(value identity.LoginCode) LoginCode {
	return LoginCode{ID: value.ID, CodeHash: value.CodeHash, UserID: value.UserID, Flow: value.Flow, ExpiresAt: value.ExpiresAt, ConsumedAt: value.ConsumedAt, CreatedAt: value.CreatedAt}
}

func loginCodeDomain(value LoginCode) identity.LoginCode {
	return identity.LoginCode{ID: value.ID, CodeHash: value.CodeHash, UserID: value.UserID, Flow: value.Flow, ExpiresAt: value.ExpiresAt, ConsumedAt: value.ConsumedAt, CreatedAt: value.CreatedAt}
}

func userDomain(value User) identity.User {
	return identity.User{ID: value.ID, ExternalSubject: value.ExternalSubject, ExternalIssuer: value.ExternalIssuer, Username: value.Username, DisplayName: value.DisplayName, Email: value.Email, AvatarURL: value.AvatarURL, PasswordHash: value.PasswordHash, Role: value.Role, Active: value.Active, Status: identity.AccountStatus(value.Status), SessionVersion: value.SessionVersion, PasswordResetRequired: value.PasswordResetRequired, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
