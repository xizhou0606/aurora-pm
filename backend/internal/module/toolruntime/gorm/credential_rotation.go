package toolruntimegorm

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func (repository *CredentialRepository) RotateRefreshToken(
	ctx context.Context,
	currentDigest string,
	rotatedAt time.Time,
	access toolruntime.AccessCredential,
	next toolruntime.RefreshCredential,
) error {
	accessRow, accessOK := accessCredentialRecord(access)
	nextRow, refreshOK := refreshCredentialRecord(next)
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(currentDigest) ||
		rotatedAt.IsZero() || !accessOK || !refreshOK || access.FamilyID != next.FamilyID || access.Binding != next.Binding {
		return toolruntime.ErrRefreshTokenInvalid
	}
	replayed := false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current ExternalRefreshCredential
		if err := tx.First(&current, "token_digest = ?", currentDigest).Error; err != nil {
			return toolruntime.ErrRefreshTokenInvalid
		}
		var family ExternalCredentialFamily
		if err := tx.First(&family, "id = ?", current.FamilyID).Error; err != nil {
			return toolruntime.ErrRefreshTokenInvalid
		}
		if current.ConsumedAt != nil {
			replayed = true
			return revokeCredentialFamily(tx, current.FamilyID, rotatedAt.UTC())
		}
		if current.RevokedAt != nil || family.RevokedAt != nil || current.FamilyID != access.FamilyID ||
			!rotatedAt.UTC().Before(current.ExpiresAt) || !rotatedAt.UTC().Before(family.AbsoluteExpiresAt) ||
			access.ExpiresAt.After(family.AbsoluteExpiresAt) || next.ExpiresAt.After(family.AbsoluteExpiresAt) ||
			current.Binding != accessRow.Binding || current.Binding != nextRow.Binding {
			return toolruntime.ErrRefreshTokenInvalid
		}
		result := tx.Model(&ExternalRefreshCredential{}).
			Where("token_digest = ? AND consumed_at IS NULL AND revoked_at IS NULL", currentDigest).
			Update("consumed_at", rotatedAt.UTC())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			replayed = true
			return revokeCredentialFamily(tx, current.FamilyID, rotatedAt.UTC())
		}
		if err := tx.Create(&accessRow).Error; err != nil {
			return err
		}
		return tx.Create(&nextRow).Error
	})
	if err != nil {
		return toolruntime.ErrRefreshTokenInvalid
	}
	if replayed {
		return toolruntime.ErrRefreshTokenReplay
	}
	return nil
}

func (repository *CredentialRepository) RevokeRefreshFamily(
	ctx context.Context,
	refreshDigest string,
	revokedAt time.Time,
) error {
	if repository == nil || repository.db == nil || !credentialDigestPattern.MatchString(refreshDigest) || revokedAt.IsZero() {
		return toolruntime.ErrRefreshTokenInvalid
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var refresh ExternalRefreshCredential
		if err := tx.First(&refresh, "token_digest = ?", refreshDigest).Error; err != nil {
			return toolruntime.ErrRefreshTokenInvalid
		}
		return revokeCredentialFamily(tx, refresh.FamilyID, revokedAt.UTC())
	})
	if err != nil {
		return toolruntime.ErrRefreshTokenInvalid
	}
	return nil
}

func revokeCredentialFamily(tx *gorm.DB, familyID string, revokedAt time.Time) error {
	if err := tx.Model(&ExternalCredentialFamily{}).
		Where("id = ? AND revoked_at IS NULL", familyID).
		Updates(map[string]any{"revoked_at": revokedAt, "updated_at": revokedAt}).Error; err != nil {
		return err
	}
	return tx.Model(&ExternalRefreshCredential{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", revokedAt).Error
}
