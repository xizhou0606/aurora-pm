package identitygorm

import (
	"context"
	"errors"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *AccessRepository) CreatePasswordReset(ctx context.Context, input identity.PasswordResetRequest) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requirePersonnelUser(tx, input.ActorUserID); err != nil {
			return err
		}
		var target User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, input.TargetUserID).Error; err != nil {
			return mapPersonnelUserError(err)
		}
		consumedAt := input.CreatedAt.UTC()
		if err := tx.Model(&PasswordResetTransaction{}).
			Where("user_id = ? AND consumed_at IS NULL", input.TargetUserID).
			Update("consumed_at", consumedAt).Error; err != nil {
			return err
		}
		reset := PasswordResetTransaction{
			TokenHash: input.TokenHash, UserID: input.TargetUserID,
			RequestedByUserID: input.ActorUserID, ExpiresAt: input.ExpiresAt.UTC(), CreatedAt: input.CreatedAt.UTC(),
		}
		if err := tx.Create(&reset).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", target.ID).Updates(map[string]any{
			"password_reset_required": true,
			"session_version":         gorm.Expr("session_version + 1"),
		}).Error; err != nil {
			return err
		}
		if err := appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "user.password_reset_requested", ActorUserID: input.ActorUserID,
			SubjectUserID: uintPointer(input.TargetUserID),
			BeforeJSON:    auditJSON(map[string]bool{"password_reset_required": target.PasswordResetRequired}),
			AfterJSON:     auditJSON(map[string]bool{"password_reset_required": true, "notification_queued": input.Notify}),
		}); err != nil {
			return err
		}
		if !input.Notify {
			return nil
		}
		var actor User
		if err := tx.First(&actor, input.ActorUserID).Error; err != nil {
			return err
		}
		if repository.passwordResetNotificationWriter == nil {
			return identity.ErrPasswordResetNotificationUnavailable
		}
		return repository.passwordResetNotificationWriter(tx, reset.ID, input, target, actor)
	})
}

func (repository *AccessRepository) CompletePasswordReset(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reset PasswordResetTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND consumed_at IS NULL AND expires_at > ?", tokenHash, now.UTC()).
			First(&reset).Error; err != nil {
			return err
		}
		consumedAt := now.UTC()
		result := tx.Model(&PasswordResetTransaction{}).
			Where("id = ? AND consumed_at IS NULL", reset.ID).
			Update("consumed_at", consumedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		result = tx.Model(&User{}).Where("id = ?", reset.UserID).Updates(map[string]any{
			"password_hash":           passwordHash,
			"password_reset_required": false,
			"session_version":         gorm.Expr("session_version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&PasswordResetTransaction{}).
			Where("user_id = ? AND consumed_at IS NULL", reset.UserID).
			Update("consumed_at", consumedAt).Error; err != nil {
			return err
		}
		return appendPersonnelAudit(tx, PersonnelAuditEvent{
			EventType: "user.password_reset_completed", ActorUserID: reset.UserID,
			SubjectUserID: uintPointer(reset.UserID),
			BeforeJSON:    auditJSON(map[string]bool{"password_reset_required": true}),
			AfterJSON:     auditJSON(map[string]bool{"password_reset_required": false}),
		})
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.ErrPasswordResetInvalid
	}
	return err
}
