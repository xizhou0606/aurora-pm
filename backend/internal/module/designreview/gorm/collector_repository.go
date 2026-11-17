package designreviewgorm

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) ListMobileCollectors(ctx context.Context, workspaceID, userID uint) ([]designreview.MobileCollector, error) {
	var rows []MobileCollector
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]designreview.MobileCollector, 0, len(rows))
	for _, row := range rows {
		result = append(result, collectorFromRecord(row))
	}
	return result, nil
}

func (repository *Repository) GetMobileCollector(ctx context.Context, workspaceID, userID uint, collectorID string) (designreview.MobileCollector, error) {
	var row MobileCollector
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ? AND id = ? AND revoked_at IS NULL", workspaceID, userID, collectorID).First(&row).Error; err != nil {
		return designreview.MobileCollector{}, mapNotFound(err)
	}
	return collectorFromRecord(row), nil
}

func (repository *Repository) CreatePairingCode(ctx context.Context, input designreview.PairingCodeRecord) error {
	now := repository.now()
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND user_id = ? AND used_at IS NULL", input.WorkspaceID, input.UserID).Delete(&MobilePairingCode{}).Error; err != nil {
			return err
		}
		row := MobilePairingCode{ID: input.ID, WorkspaceID: input.WorkspaceID, UserID: input.UserID, CodeHash: input.CodeHash, ExpiresAt: input.ExpiresAt, CreatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return appendAudit(tx, input.WorkspaceID, input.UserID, "design_review_mobile_collector", input.ID, "pairing_code_created", 0)
	})
}

func (repository *Repository) PairMobileCollector(ctx context.Context, codeHash string, input designreview.MobileCollectorRecord) (designreview.MobileCollector, error) {
	var result designreview.MobileCollector
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var code MobilePairingCode
		if err := tx.Where("code_hash = ? AND used_at IS NULL AND expires_at > ?", codeHash, repository.now()).First(&code).Error; err != nil {
			return designreview.ErrForbidden
		}
		now := repository.now()
		row := MobileCollector{ID: input.ID, WorkspaceID: code.WorkspaceID, UserID: code.UserID, Name: input.Name, TokenHash: input.TokenHash, Version: input.Version, MobilewrightVersion: input.MobilewrightVersion, Hostname: input.Hostname, Architecture: input.Architecture, Inventory: nonemptyJSON(input.Inventory, `{}`), CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if tx.Model(&code).Where("used_at IS NULL").Update("used_at", now).RowsAffected != 1 {
			return designreview.ErrConflict
		}
		result = collectorFromRecord(row)
		return appendAudit(tx, code.WorkspaceID, code.UserID, "design_review_mobile_collector", row.ID, "paired", 0)
	})
	return result, err
}

func (repository *Repository) AuthenticateCollector(ctx context.Context, tokenHash string) (designreview.MobileCollector, error) {
	var row MobileCollector
	now := repository.now()
	if err := repository.db.WithContext(ctx).Where("token_hash = ? AND revoked_at IS NULL", tokenHash).First(&row).Error; err != nil {
		return designreview.MobileCollector{}, designreview.ErrForbidden
	}
	_ = repository.db.WithContext(ctx).Model(&row).Update("last_seen_at", now).Error
	row.LastSeenAt = &now
	return collectorFromRecord(row), nil
}

func (repository *Repository) UpdateMobileCollectorHeartbeat(ctx context.Context, collectorID string, input designreview.MobileCollectorHeartbeatRecord) error {
	now := repository.now()
	updated := repository.db.WithContext(ctx).Model(&MobileCollector{}).Where("id = ? AND revoked_at IS NULL", collectorID).Updates(map[string]any{
		"version": input.Version, "mobilewright_version": input.MobilewrightVersion, "hostname": input.Hostname,
		"architecture": input.Architecture, "inventory": input.Inventory, "doctor_ready": input.DoctorReady,
		"doctor_summary": input.DoctorSummary, "last_seen_at": now, "updated_at": now,
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return designreview.ErrNotFound
	}
	return nil
}

func (repository *Repository) RevokeMobileCollector(ctx context.Context, workspaceID uint, collectorID string, userID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		updated := tx.Model(&MobileCollector{}).Where("workspace_id = ? AND id = ? AND user_id = ? AND revoked_at IS NULL", workspaceID, collectorID, userID).Updates(map[string]any{"revoked_at": now, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return designreview.ErrNotFound
		}
		return appendAudit(tx, workspaceID, userID, "design_review_mobile_collector", collectorID, "revoked", 0)
	})
}

func (repository *Repository) CreateCaptureDraft(ctx context.Context, input designreview.CaptureDraftRecord) (designreview.MobileCaptureDraft, error) {
	row := MobileCaptureDraft{ID: input.ID, WorkspaceID: input.WorkspaceID, UserID: input.UserID, CollectorID: input.CollectorID, Status: "queued", Platform: input.Platform, DeviceID: input.DeviceID, Warnings: []byte("[]"), ExpiresAt: input.ExpiresAt}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return appendAudit(tx, input.WorkspaceID, input.UserID, "design_review_mobile_capture", input.ID, "queued", 0)
	}); err != nil {
		return designreview.MobileCaptureDraft{}, err
	}
	return captureFromRecord(row), nil
}

func (repository *Repository) ListCaptureDrafts(ctx context.Context, workspaceID, userID uint) ([]designreview.MobileCaptureDraft, error) {
	var rows []MobileCaptureDraft
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ? AND expires_at > ?", workspaceID, userID, repository.now()).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]designreview.MobileCaptureDraft, 0, len(rows))
	for _, row := range rows {
		result = append(result, captureFromRecord(row))
	}
	return result, nil
}

func (repository *Repository) ClaimCaptureDraft(ctx context.Context, collectorID string) (*designreview.MobileCaptureDraft, error) {
	var result *designreview.MobileCaptureDraft
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row MobileCaptureDraft
		now := repository.now()
		if err := tx.Model(&MobileCaptureDraft{}).Where("collector_id = ? AND status = 'claimed' AND claimed_at < ?", collectorID, now.Add(-5*time.Minute)).Updates(map[string]any{"status": "queued", "claimed_at": nil, "updated_at": now}).Error; err != nil {
			return err
		}
		query := tx.Where("collector_id = ? AND status = 'queued' AND expires_at > ?", collectorID, now).Order("created_at, id").Limit(1)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		query = query.Find(&row)
		if query.Error != nil || query.RowsAffected == 0 {
			return query.Error
		}
		updated := tx.Model(&row).Where("status = 'queued'").Updates(map[string]any{"status": "claimed", "claimed_at": now, "updated_at": now})
		if updated.Error != nil || updated.RowsAffected != 1 {
			return designreview.ErrConflict
		}
		value := captureFromRecord(row)
		value.Status = "claimed"
		result = &value
		return nil
	})
	return result, err
}

func (repository *Repository) GetCaptureDraftForCollector(ctx context.Context, collectorID, draftID string) (designreview.MobileCaptureDraft, error) {
	var row MobileCaptureDraft
	if err := repository.db.WithContext(ctx).Where("collector_id = ? AND id = ? AND expires_at > ?", collectorID, draftID, repository.now()).First(&row).Error; err != nil {
		return designreview.MobileCaptureDraft{}, mapNotFound(err)
	}
	return captureFromRecord(row), nil
}

func (repository *Repository) GetCaptureAsset(ctx context.Context, workspaceID uint, draftID string, userID uint) (designreview.MobileCaptureAssetRecord, error) {
	var row MobileCaptureDraft
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ? AND id = ? AND status IN ? AND screenshot_object_key IS NOT NULL", workspaceID, userID, draftID, []string{"succeeded", "consumed"}).First(&row).Error; err != nil {
		return designreview.MobileCaptureAssetRecord{}, mapNotFound(err)
	}
	if row.ScreenshotObjectKey == nil || row.ScreenshotMediaType == nil || row.ScreenshotByteSize == nil {
		return designreview.MobileCaptureAssetRecord{}, designreview.ErrNotFound
	}
	return designreview.MobileCaptureAssetRecord{ObjectKey: *row.ScreenshotObjectKey, ContentType: *row.ScreenshotMediaType, ByteSize: *row.ScreenshotByteSize}, nil
}

func (repository *Repository) CompleteCaptureDraft(ctx context.Context, input designreview.CompleteCaptureRecord) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		updated := tx.Model(&MobileCaptureDraft{}).Where("workspace_id = ? AND id = ? AND collector_id = ? AND status = 'claimed' AND expires_at > ?", input.WorkspaceID, input.ID, input.CollectorID, now).Updates(map[string]any{
			"status": "succeeded", "app_id": input.AppID, "target": input.Target, "elements": input.Elements,
			"structure_truncated": input.StructureTruncated, "warnings": input.Warnings,
			"screenshot_object_key": input.ScreenshotObjectKey, "screenshot_media_type": input.ScreenshotMediaType,
			"screenshot_sha256": input.ScreenshotSHA256, "screenshot_byte_size": input.ScreenshotByteSize,
			"captured_at": input.CapturedAt, "last_error": nil, "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return designreview.ErrConflict
		}
		return appendAudit(tx, input.WorkspaceID, input.ActorUserID, "design_review_mobile_capture", input.ID, "completed", 0)
	})
}

func (repository *Repository) FailCaptureDraft(ctx context.Context, collectorID, draftID string, actorUserID uint, message string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row MobileCaptureDraft
		if err := tx.Where("collector_id = ? AND id = ? AND status = 'claimed'", collectorID, draftID).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		now := repository.now()
		if updated := tx.Model(&row).Where("status = 'claimed'").Updates(map[string]any{"status": "failed", "last_error": message, "updated_at": now}); updated.Error != nil || updated.RowsAffected != 1 {
			return designreview.ErrConflict
		}
		return appendAudit(tx, row.WorkspaceID, actorUserID, "design_review_mobile_capture", draftID, "failed", 0)
	})
}

func (repository *Repository) ConsumeCaptureDraft(ctx context.Context, workspaceID uint, draftID string, userID uint) (designreview.MobileCaptureDraft, error) {
	var result designreview.MobileCaptureDraft
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row MobileCaptureDraft
		if err := tx.Where("workspace_id = ? AND user_id = ? AND id = ? AND status IN ? AND expires_at > ?", workspaceID, userID, draftID, []string{"succeeded", "consumed"}, repository.now()).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Status == "succeeded" {
			now := repository.now()
			if updated := tx.Model(&row).Where("status = 'succeeded'").Updates(map[string]any{"status": "consumed", "consumed_at": now, "updated_at": now}); updated.Error != nil || updated.RowsAffected != 1 {
				return designreview.ErrConflict
			}
			row.Status = "consumed"
			row.ConsumedAt = &now
			if err := appendAudit(tx, workspaceID, userID, "design_review_mobile_capture", draftID, "consumed", 0); err != nil {
				return err
			}
		}
		result = captureFromRecord(row)
		return nil
	})
	return result, err
}

func (repository *Repository) DeleteCaptureDraft(ctx context.Context, workspaceID uint, draftID string, userID uint) (*string, error) {
	var row MobileCaptureDraft
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND id = ? AND user_id = ?", workspaceID, draftID, userID).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, userID, "design_review_mobile_capture", draftID, "deleted", 0)
	})
	return row.ScreenshotObjectKey, err
}

func collectorFromRecord(row MobileCollector) designreview.MobileCollector {
	return designreview.MobileCollector{ID: row.ID, WorkspaceID: row.WorkspaceID, UserID: row.UserID, Name: row.Name, Version: row.Version, MobilewrightVersion: row.MobilewrightVersion, Hostname: row.Hostname, Architecture: row.Architecture, Inventory: row.Inventory, DoctorReady: row.DoctorReady, DoctorSummary: row.DoctorSummary, LastSeenAt: row.LastSeenAt, RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt}
}

func captureFromRecord(row MobileCaptureDraft) designreview.MobileCaptureDraft {
	return designreview.MobileCaptureDraft{ID: row.ID, CollectorID: row.CollectorID, Status: row.Status, Platform: row.Platform, DeviceID: row.DeviceID, AppID: row.AppID, Target: row.Target, Elements: row.Elements, StructureTruncated: row.StructureTruncated, Warnings: row.Warnings, ScreenshotAvailable: row.ScreenshotObjectKey != nil, ExpiresAt: row.ExpiresAt, CapturedAt: row.CapturedAt, LastError: row.LastError}
}
