package identitygorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const externalRegistrationIssuer = "pm_external_registration"

func (r *LoginRepository) CreateExternalRegistration(ctx context.Context, value *identity.ExternalRegistration) error {
	row := ExternalRegistrationTransaction{
		TicketHash: value.TicketHash, Provider: value.Provider, Issuer: value.Issuer, Subject: value.Subject,
		Email: value.Email, TelegramUserID: value.TelegramUserID, DisplayName: value.DisplayName, AvatarURL: value.AvatarURL,
		ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	value.ID = row.ID
	return nil
}

func (r *LoginRepository) CompleteExternalRegistration(ctx context.Context, input identity.ExternalRegistrationComplete) (identity.User, error) {
	var created User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		consumedAt := input.Now.UTC()
		result := tx.Model(&ExternalRegistrationTransaction{}).
			Where("ticket_hash = ? AND consumed_at IS NULL AND expires_at > ?", input.TicketHash, input.Now.UTC()).
			Update("consumed_at", consumedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return identity.ErrExternalRegistrationInvalid
		}
		var registration ExternalRegistrationTransaction
		if err := tx.Where("ticket_hash = ?", input.TicketHash).First(&registration).Error; err != nil {
			return identity.ErrExternalRegistrationInvalid
		}
		var usernameMatches int64
		if err := tx.Model(&User{}).Where("LOWER(username) = ?", input.Username).Count(&usernameMatches).Error; err != nil {
			return err
		}
		if usernameMatches != 0 {
			return identity.ErrIdentityUsernameConflict
		}
		var identityMatches int64
		if err := tx.Model(&UserLoginIdentity{}).Where("issuer = ? AND subject = ?", registration.Issuer, registration.Subject).Count(&identityMatches).Error; err != nil {
			return err
		}
		if identityMatches != 0 {
			if registration.Provider == identity.FlowTelegram {
				return identity.ErrLoginTelegramIdentityConflict
			}
			return identity.ErrLoginGoogleIdentityConflict
		}
		expectedSubject := externalRegistrationSubject(registration.Provider, registration.Issuer, registration.Subject)
		var previous User
		if err := tx.Where(
			"external_issuer = ? AND external_subject = ?",
			externalRegistrationIssuer,
			expectedSubject,
		).First(&previous).Error; err == nil {
			if err := releaseExternalRegistrationSubject(tx, previous.ID, expectedSubject); err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		created = User{
			ExternalIssuer: externalRegistrationIssuer, ExternalSubject: expectedSubject,
			Username: input.Username, DisplayName: input.DisplayName, Email: registration.Email, AvatarURL: registration.AvatarURL,
			PasswordHash: input.PasswordHash, Role: "user", Active: true,
			Status: string(identity.AccountPendingApproval), SessionVersion: 1,
		}
		if err := tx.Create(&created).Error; err != nil {
			if isDuplicateError(err) {
				return identity.ErrIdentityUsernameConflict
			}
			return err
		}
		if err := EnsureUserInDefaultGroup(tx, created.ID); err != nil {
			return err
		}
		if err := EnsureUserInDefaultWorkspace(tx, created.ID); err != nil {
			return err
		}
		binding := UserLoginIdentity{
			UserID: created.ID, Provider: registration.Provider, Issuer: registration.Issuer, Subject: registration.Subject,
			EmailSnapshot: registration.Email, TelegramUserID: registration.TelegramUserID,
		}
		if err := tx.Create(&binding).Error; err != nil {
			if isDuplicateError(err) {
				if registration.Provider == identity.FlowTelegram {
					return identity.ErrLoginTelegramIdentityConflict
				}
				return identity.ErrLoginGoogleIdentityConflict
			}
			return err
		}
		return tx.Create(&LoginCode{
			CodeHash: input.LoginCodeHash, UserID: created.ID, Flow: registration.Provider,
			ExpiresAt: input.LoginCodeExpiresAt, CreatedAt: input.Now,
		}).Error
	})
	return userDomain(created), err
}

func (r *LoginRepository) Account(ctx context.Context, actorID uint) (identity.Account, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("id = ? AND active = ?", actorID, true).First(&user).Error; err != nil {
		return identity.Account{}, identity.ErrLoginUserNotFound
	}
	var bindings []UserLoginIdentity
	if err := r.db.WithContext(ctx).Where("user_id = ? AND provider IN ?", actorID, []string{identity.FlowGoogle, identity.FlowTelegram}).Order("provider ASC").Find(&bindings).Error; err != nil {
		return identity.Account{}, err
	}
	items := make([]identity.LoginIdentity, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, identity.LoginIdentity{Provider: binding.Provider, Email: binding.EmailSnapshot, TelegramUserID: binding.TelegramUserID})
	}
	return identity.Account{
		User: userDomain(user), DisplayNameEditable: user.FeishuOpenID == nil && user.FeishuTenantID == nil,
		PasswordConfigured: user.PasswordHash != "", Identities: items,
	}, nil
}

func (r *LoginRepository) UpdateProfile(ctx context.Context, actorID uint, displayName string) (identity.User, error) {
	result := r.db.WithContext(ctx).Model(&User{}).
		Where("id = ? AND active = ? AND feishu_open_id IS NULL AND feishu_tenant_id IS NULL", actorID, true).
		Update("display_name", displayName)
	if result.Error != nil {
		return identity.User{}, result.Error
	}
	if result.RowsAffected != 1 {
		return identity.User{}, identity.ErrIdentityDisplayNameReadOnly
	}
	return r.FindActiveUserByID(ctx, actorID)
}

func (r *LoginRepository) UpdatePassword(ctx context.Context, actorID uint, currentPassword, passwordHash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND active = ?", actorID, true).First(&user).Error; err != nil {
			return identity.ErrLoginUserNotFound
		}
		if !user.PasswordResetRequired && user.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
			return identity.ErrIdentityCurrentPassword
		}
		if user.PasswordHash == "" {
			var externalCount int64
			if err := tx.Model(&UserLoginIdentity{}).Where("user_id = ?", actorID).Count(&externalCount).Error; err != nil {
				return err
			}
			if externalCount == 0 && user.FeishuOpenID == nil && user.FeishuTenantID == nil {
				return identity.ErrIdentityLoginMethodRequired
			}
		}
		result := tx.Model(&User{}).Where("id = ?", actorID).Updates(map[string]any{
			"password_hash":           passwordHash,
			"password_reset_required": false,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return identity.ErrLoginUserNotFound
		}
		return tx.Model(&PasswordResetTransaction{}).
			Where("user_id = ? AND consumed_at IS NULL", actorID).
			Update("consumed_at", time.Now().UTC()).Error
	})
}

func (r *LoginRepository) UnbindIdentity(ctx context.Context, actorID uint, provider string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Where("id = ? AND active = ?", actorID, true).First(&user).Error; err != nil {
			return identity.ErrLoginUserNotFound
		}
		var target UserLoginIdentity
		if err := tx.Where("user_id = ? AND provider = ?", actorID, provider).First(&target).Error; err != nil {
			return identity.ErrLoginUserNotFound
		}
		var otherCount int64
		if err := tx.Model(&UserLoginIdentity{}).Where("user_id = ? AND provider <> ?", actorID, provider).Count(&otherCount).Error; err != nil {
			return err
		}
		if user.PasswordHash == "" && user.FeishuOpenID == nil && user.FeishuTenantID == nil && otherCount == 0 {
			return identity.ErrIdentityLoginMethodRequired
		}
		expectedSubject := externalRegistrationSubject(provider, target.Issuer, target.Subject)
		if user.ExternalIssuer == externalRegistrationIssuer && user.ExternalSubject == expectedSubject {
			if err := releaseExternalRegistrationSubject(tx, user.ID, expectedSubject); err != nil {
				return err
			}
		}
		return tx.Delete(&target).Error
	})
}

func (r *LoginRepository) CreateIdentityOperation(ctx context.Context, value *identity.IdentityOperation) error {
	row := IdentityOperationTransaction{StateHash: value.StateHash, NonceHash: value.NonceHash, PKCEVerifier: value.PKCEVerifier, ActorUserID: value.ActorUserID, Provider: value.Provider, ReturnTo: value.ReturnTo, ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	value.ID = row.ID
	return nil
}

func (r *LoginRepository) ConsumeIdentityOperation(ctx context.Context, provider, stateHash string, now time.Time) (identity.IdentityOperation, error) {
	var row IdentityOperationTransaction
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&IdentityOperationTransaction{}).Where("provider = ? AND state_hash = ? AND consumed_at IS NULL AND expires_at > ?", provider, stateHash, now).Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("state_hash = ?", stateHash).First(&row).Error; err != nil {
			return err
		}
		if row.PKCEVerifier != "" {
			return tx.Model(&IdentityOperationTransaction{}).Where("id = ?", row.ID).Update("pkce_verifier", "").Error
		}
		return nil
	})
	return identity.IdentityOperation{ID: row.ID, StateHash: row.StateHash, NonceHash: row.NonceHash, PKCEVerifier: row.PKCEVerifier, ActorUserID: row.ActorUserID, Provider: row.Provider, ReturnTo: row.ReturnTo, ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt, CreatedAt: row.CreatedAt}, err
}

func (r *LoginRepository) BindIdentity(ctx context.Context, actorID uint, provider string, google identity.GoogleIdentity, telegram identity.TelegramIdentity) error {
	binding := UserLoginIdentity{UserID: actorID, Provider: provider}
	if provider == identity.FlowGoogle {
		binding.Issuer = google.Issuer
		binding.Subject = google.Subject
		binding.EmailSnapshot = google.Email
	} else {
		binding.Issuer = telegram.Issuer
		binding.Subject = telegram.Subject
		binding.TelegramUserID = &telegram.TelegramUserID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Where("id = ? AND active = ?", actorID, true).First(&user).Error; err != nil {
			return identity.ErrLoginUserNotFound
		}
		var count int64
		if err := tx.Model(&UserLoginIdentity{}).Where("issuer = ? AND subject = ?", binding.Issuer, binding.Subject).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return identity.ErrLoginGoogleIdentityConflict
		}
		if err := tx.Create(&binding).Error; err != nil {
			if isDuplicateError(err) {
				return identity.ErrLoginGoogleIdentityConflict
			}
			return err
		}
		return nil
	})
}

func externalRegistrationSubject(provider, issuer, subject string) string {
	digest := sha256.Sum256([]byte(provider + "\x00" + issuer + "\x00" + subject))
	return "registration:" + hex.EncodeToString(digest[:])
}

func releasedExternalRegistrationSubject(userID uint, subject string) string {
	digest := sha256.Sum256([]byte(strconv.FormatUint(uint64(userID), 10) + "\x00" + subject))
	return "unbound:" + hex.EncodeToString(digest[:])
}

func releaseExternalRegistrationSubject(tx *gorm.DB, userID uint, subject string) error {
	result := tx.Model(&User{}).Where(
		"id = ? AND external_issuer = ? AND external_subject = ?",
		userID,
		externalRegistrationIssuer,
		subject,
	).Update("external_subject", releasedExternalRegistrationSubject(userID, subject))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return identity.ErrLoginGoogleIdentityConflict
	}
	return nil
}

var _ identity.AccountRepository = (*LoginRepository)(nil)
