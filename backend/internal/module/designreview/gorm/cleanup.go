package designreviewgorm

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type DeletionBatch struct {
	WorkspaceID uint
	RootID      string
	ObjectKeys  []string
}

type CaptureObjectCleanup struct {
	DraftID   string
	ObjectKey string
}

func (repository *Repository) ClaimObjectCleanup(ctx context.Context, owner string, leaseDuration time.Duration) (*ObjectCleanupJob, error) {
	var claimed *ObjectCleanupJob
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		query := tx.Where("available_at <= ? AND (lease_until IS NULL OR lease_until < ?)", now, now).Order("available_at, id").Limit(1)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		var row ObjectCleanupJob
		result := query.Find(&row)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		leaseUntil := now.Add(leaseDuration)
		updated := tx.Model(&ObjectCleanupJob{}).
			Where("id = ? AND (lease_until IS NULL OR lease_until < ?)", row.ID, now).
			Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "lease_owner": owner, "lease_until": leaseUntil, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return nil
		}
		row.Attempts++
		row.LeaseOwner = &owner
		row.LeaseUntil = &leaseUntil
		claimed = &row
		return nil
	})
	return claimed, err
}

func (repository *Repository) CompleteObjectCleanup(ctx context.Context, workspaceID uint, objectKey string) error {
	return repository.db.WithContext(ctx).Where("workspace_id = ? AND object_key = ?", workspaceID, objectKey).Delete(&ObjectCleanupJob{}).Error
}

func (repository *Repository) FailObjectCleanup(ctx context.Context, id, owner, message string, retryAt time.Time) error {
	return repository.db.WithContext(ctx).Model(&ObjectCleanupJob{}).
		Where("id = ? AND lease_owner = ?", id, owner).
		Updates(map[string]any{"available_at": retryAt, "lease_owner": nil, "lease_until": nil, "last_error": message, "updated_at": repository.now()}).Error
}

func (repository *Repository) NextDeletion(ctx context.Context) (*DeletionBatch, error) {
	var root Review
	query := repository.db.WithContext(ctx).Where("status = 'deleting' AND id = chain_root_id").Order("deleting_at, id").Limit(1).Find(&root)
	if query.Error != nil || query.RowsAffected == 0 {
		return nil, query.Error
	}
	var keys []string
	err := repository.db.WithContext(ctx).Table("design_review_assets a").Select("a.object_key").Joins("JOIN design_reviews r ON r.id = a.review_id AND r.workspace_id = a.workspace_id").Where("r.workspace_id = ? AND r.chain_root_id = ?", root.WorkspaceID, root.ID).Pluck("a.object_key", &keys).Error
	if err != nil {
		return nil, err
	}
	return &DeletionBatch{WorkspaceID: root.WorkspaceID, RootID: root.ID, ObjectKeys: keys}, nil
}

func (repository *Repository) PurgeDeletion(ctx context.Context, batch DeletionBatch) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var root Review
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND status = 'deleting' AND id = chain_root_id", batch.WorkspaceID, batch.RootID).First(&root)
		if query.Error != nil {
			return mapNotFound(query.Error)
		}
		var reviewIDs []string
		if err := tx.Model(&Review{}).Where("workspace_id = ? AND chain_root_id = ?", batch.WorkspaceID, batch.RootID).Pluck("id", &reviewIDs).Error; err != nil {
			return err
		}
		if len(reviewIDs) == 0 {
			return designreview.ErrNotFound
		}
		deletions := []struct {
			model any
			where string
			args  []any
		}{
			{&ResolutionEvent{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&RecheckVerification{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&RecheckMatch{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&LineageEvent{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&Difference{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&DifferenceLineage{}, "workspace_id = ? AND chain_root_review_id = ?", []any{batch.WorkspaceID, batch.RootID}},
			{&AnalysisJob{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&DeepJob{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
			{&Asset{}, "workspace_id = ? AND review_id IN ?", []any{batch.WorkspaceID, reviewIDs}},
		}
		for _, deletion := range deletions {
			if err := tx.Where(deletion.where, deletion.args...).Delete(deletion.model).Error; err != nil {
				return err
			}
		}
		return tx.Where("workspace_id = ? AND id IN ?", batch.WorkspaceID, reviewIDs).Delete(&Review{}).Error
	})
}

func (repository *Repository) ExpireCaptureDrafts(ctx context.Context, before time.Time) ([]CaptureObjectCleanup, error) {
	objects := make([]CaptureObjectCleanup, 0)
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": "expired", "updated_at": before}
		if err := tx.Model(&MobileCaptureDraft{}).Where("expires_at <= ? AND status IN ?", before, []string{"queued", "claimed", "succeeded", "consumed"}).Updates(updates).Error; err != nil {
			return err
		}
		var rows []MobileCaptureDraft
		if err := tx.Where("status = 'expired' AND screenshot_object_key IS NOT NULL").Order("expires_at, id").Limit(20).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			objects = append(objects, CaptureObjectCleanup{DraftID: row.ID, ObjectKey: *row.ScreenshotObjectKey})
		}
		return tx.Model(&MobileCaptureDraft{}).Where("status = 'expired' AND screenshot_object_key IS NULL").Updates(map[string]any{
			"target": nil, "elements": nil, "warnings": []byte("[]"), "updated_at": before,
		}).Error
	})
	return objects, err
}

func (repository *Repository) CompleteCaptureObjectCleanup(ctx context.Context, draftID string, before time.Time) error {
	updated := repository.db.WithContext(ctx).Model(&MobileCaptureDraft{}).
		Where("id = ? AND status = 'expired' AND screenshot_object_key IS NOT NULL", draftID).
		Updates(map[string]any{
			"screenshot_object_key": nil, "screenshot_media_type": nil, "screenshot_sha256": nil,
			"screenshot_byte_size": nil, "target": nil, "elements": nil, "warnings": []byte("[]"), "updated_at": before,
		})
	return updated.Error
}

func (repository *Repository) PurgeExpiredCaptureState(ctx context.Context, before time.Time) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ? AND status IN ? AND screenshot_object_key IS NULL", before.Add(-24*time.Hour), []string{"failed", "cancelled", "consumed", "expired"}).Delete(&MobileCaptureDraft{}).Error; err != nil {
			return err
		}
		return tx.Where("expires_at <= ? OR (used_at IS NOT NULL AND used_at <= ?)", before, before.Add(-24*time.Hour)).Delete(&MobilePairingCode{}).Error
	})
}
