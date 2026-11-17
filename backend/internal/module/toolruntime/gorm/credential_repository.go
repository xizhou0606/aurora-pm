package toolruntimegorm

import (
	"context"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

var (
	credentialDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
	familyIDPattern         = regexp.MustCompile(`^[A-Za-z0-9_-]{43,64}$`)
	authorizationIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)
	pkceChallengePattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

type CredentialRepository struct {
	db *gorm.DB
}

func NewCredentialRepository(db *gorm.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

func (repository *CredentialRepository) CreateAuthorizationCode(
	ctx context.Context,
	code toolruntime.AuthorizationCodeCredential,
) error {
	row, ok := authorizationCodeRecord(code)
	if repository == nil || repository.db == nil || !ok {
		return toolruntime.ErrAuthorizationCodeInvalid
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return toolruntime.ErrAuthorizationCodeInvalid
	}
	return nil
}

func (repository *CredentialRepository) FindAuthorizationCode(
	ctx context.Context,
	digest string,
) (toolruntime.AuthorizationCodeCredential, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) {
		return toolruntime.AuthorizationCodeCredential{}, toolruntime.ErrAuthorizationCodeInvalid
	}
	var row ExternalAuthorizationCode
	if err := repository.db.WithContext(ctx).First(&row, "code_digest = ?", digest).Error; err != nil {
		return toolruntime.AuthorizationCodeCredential{}, toolruntime.ErrAuthorizationCodeInvalid
	}
	value, ok := authorizationCodeDomain(row)
	if !ok {
		return toolruntime.AuthorizationCodeCredential{}, toolruntime.ErrAuthorizationCodeInvalid
	}
	return value, nil
}

func (repository *CredentialRepository) ExchangeAuthorizationCode(
	ctx context.Context,
	digest string,
	consumedAt time.Time,
	family toolruntime.CredentialFamily,
	access toolruntime.AccessCredential,
	refresh toolruntime.RefreshCredential,
) error {
	familyRow, familyOK := credentialFamilyRecord(family)
	accessRow, accessOK := accessCredentialRecord(access)
	refreshRow, refreshOK := refreshCredentialRecord(refresh)
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) || consumedAt.IsZero() ||
		!familyOK || !accessOK || !refreshOK || access.FamilyID != family.ID || refresh.FamilyID != family.ID ||
		access.Binding != family.Binding || refresh.Binding != family.Binding {
		return toolruntime.ErrAuthorizationCodeInvalid
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current ExternalAuthorizationCode
		if err := tx.First(&current, "code_digest = ?", digest).Error; err != nil ||
			current.ConsumedAt != nil || !consumedAt.UTC().Before(current.ExpiresAt) || current.Binding != familyRow.Binding {
			return toolruntime.ErrAuthorizationCodeInvalid
		}
		result := tx.Model(&ExternalAuthorizationCode{}).
			Where("code_digest = ? AND consumed_at IS NULL AND expires_at > ?", digest, consumedAt.UTC()).
			Update("consumed_at", consumedAt.UTC())
		if result.Error != nil || result.RowsAffected != 1 {
			return toolruntime.ErrAuthorizationCodeInvalid
		}
		if err := tx.Create(&familyRow).Error; err != nil {
			return err
		}
		if err := tx.Create(&accessRow).Error; err != nil {
			return err
		}
		return tx.Create(&refreshRow).Error
	})
	if err != nil {
		return toolruntime.ErrAuthorizationCodeInvalid
	}
	return nil
}

func (repository *CredentialRepository) FindCredentialFamily(
	ctx context.Context,
	id string,
) (toolruntime.CredentialFamily, error) {
	if repository == nil || repository.db == nil || !validFamilyID(id) {
		return toolruntime.CredentialFamily{}, toolruntime.ErrExternalTokenInvalid
	}
	var row ExternalCredentialFamily
	if err := repository.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return toolruntime.CredentialFamily{}, toolruntime.ErrExternalTokenInvalid
	}
	value, ok := credentialFamilyDomain(row)
	if !ok {
		return toolruntime.CredentialFamily{}, toolruntime.ErrExternalTokenInvalid
	}
	return value, nil
}

func (repository *CredentialRepository) FindAccessCredential(
	ctx context.Context,
	digest string,
) (toolruntime.AccessCredential, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) {
		return toolruntime.AccessCredential{}, toolruntime.ErrExternalTokenInvalid
	}
	var row ExternalAccessCredential
	if err := repository.db.WithContext(ctx).First(&row, "token_digest = ?", digest).Error; err != nil {
		return toolruntime.AccessCredential{}, toolruntime.ErrExternalTokenInvalid
	}
	value, ok := accessCredentialDomain(row)
	if !ok {
		return toolruntime.AccessCredential{}, toolruntime.ErrExternalTokenInvalid
	}
	return value, nil
}

func (repository *CredentialRepository) FindRefreshCredential(
	ctx context.Context,
	digest string,
) (toolruntime.RefreshCredential, error) {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(digest) {
		return toolruntime.RefreshCredential{}, toolruntime.ErrRefreshTokenInvalid
	}
	var row ExternalRefreshCredential
	if err := repository.db.WithContext(ctx).First(&row, "token_digest = ?", digest).Error; err != nil {
		return toolruntime.RefreshCredential{}, toolruntime.ErrRefreshTokenInvalid
	}
	value, ok := refreshCredentialDomain(row)
	if !ok {
		return toolruntime.RefreshCredential{}, toolruntime.ErrRefreshTokenInvalid
	}
	return value, nil
}

func validFamilyID(id string) bool {
	return familyIDPattern.MatchString(id) && !strings.ContainsAny(id, " \t\r\n")
}

var _ toolruntime.CredentialRepository = (*CredentialRepository)(nil)
