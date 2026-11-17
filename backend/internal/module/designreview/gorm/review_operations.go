package designreviewgorm

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) GetAsset(ctx context.Context, workspaceID uint, reviewID, assetID string) (designreview.AssetRecord, error) {
	var row Asset
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, assetID).First(&row).Error; err != nil {
		return designreview.AssetRecord{}, mapNotFound(err)
	}
	return designreview.AssetRecord{ID: row.ID, WorkspaceID: row.WorkspaceID, ReviewID: row.ReviewID, Kind: row.Kind, ObjectKey: row.ObjectKey, ContentType: row.ContentType, ByteSize: row.ByteSize, SHA256: row.SHA256, Width: row.Width, Height: row.Height, CreatedAt: row.CreatedAt}, nil
}

func (repository *Repository) ListAssetKinds(ctx context.Context, workspaceID uint, reviewID string) ([]string, error) {
	var kinds []string
	err := repository.db.WithContext(ctx).Model(&Asset{}).Where("workspace_id = ? AND review_id = ?", workspaceID, reviewID).Pluck("kind", &kinds).Error
	return kinds, err
}

func (repository *Repository) SetHeatmapRatio(ctx context.Context, workspaceID uint, reviewID string, ratio float64) error {
	if ratio < 0 || ratio > 1 {
		return designreview.ErrInvalid
	}
	return repository.db.WithContext(ctx).Model(&Review{}).Where("workspace_id = ? AND id = ?", workspaceID, reviewID).Updates(map[string]any{"heatmap_difference_ratio": ratio, "updated_at": repository.now()}).Error
}

func (repository *Repository) MarkDeleting(ctx context.Context, workspaceID uint, reviewID string, actorUserID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Review
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, reviewID).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.RoundNumber != 1 || row.CreatedByUserID != actorUserID {
			return designreview.ErrCreatorOnly
		}
		var activeChildren int64
		if err := tx.Model(&Review{}).Where("workspace_id = ? AND chain_root_id = ? AND id <> ? AND status IN ? AND abandoned_at IS NULL", workspaceID, reviewID, reviewID, []string{"queued", "analyzing"}).Count(&activeChildren).Error; err != nil {
			return err
		}
		if activeChildren > 0 {
			return designreview.ErrConflict
		}
		now := repository.now()
		updated := tx.Model(&Review{}).Where("workspace_id = ? AND chain_root_id = ? AND deleting_at IS NULL", workspaceID, reviewID).Updates(map[string]any{"status": "deleting", "deleting_at": now, "cancel_requested": true, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		var reviewIDs []string
		if err := tx.Model(&Review{}).Where("workspace_id = ? AND chain_root_id = ?", workspaceID, reviewID).Pluck("id", &reviewIDs).Error; err != nil {
			return err
		}
		cancelled := map[string]any{"status": "cancelled", "lease_owner": nil, "lease_until": nil, "updated_at": now}
		if err := tx.Model(&AnalysisJob{}).Where("workspace_id = ? AND review_id IN ? AND status IN ?", workspaceID, reviewIDs, []string{"pending", "running"}).Updates(cancelled).Error; err != nil {
			return err
		}
		cancelled["cancel_requested"] = true
		cancelled["completed_at"] = now
		if err := tx.Model(&DeepJob{}).Where("workspace_id = ? AND review_id IN ? AND status IN ?", workspaceID, reviewIDs, []string{"pending", "running"}).Updates(cancelled).Error; err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review", reviewID, "delete_requested", row.ProjectSetIDSnapshot)
	})
}

func (repository *Repository) QueueAnalysis(ctx context.Context, workspaceID uint, reviewID string, actorUserID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		updated := tx.Model(&Review{}).Where("workspace_id = ? AND id = ? AND status IN ?", workspaceID, reviewID, []string{"draft", "failed"}).Updates(map[string]any{"status": "queued", "last_error": nil, "last_error_stage": nil, "cancel_requested": false, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return designreview.ErrConflict
		}
		job := AnalysisJob{ID: newID(), WorkspaceID: workspaceID, ReviewID: reviewID, Status: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&job).Error; err != nil {
			return mapConflict(err)
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review", reviewID, "analysis_queued", 0)
	})
}

func (repository *Repository) AbandonAnalysis(ctx context.Context, workspaceID uint, reviewID string, actorUserID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := repository.now()
		updated := tx.Model(&Review{}).Where("workspace_id = ? AND id = ? AND created_by_user_id = ? AND status IN ?", workspaceID, reviewID, actorUserID, []string{"queued", "analyzing", "failed"}).Updates(map[string]any{"status": "abandoned", "cancel_requested": true, "abandoned_at": now, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return designreview.ErrCreatorOnly
		}
		_ = tx.Model(&AnalysisJob{}).Where("workspace_id = ? AND review_id = ? AND status IN ?", workspaceID, reviewID, []string{"pending", "running"}).Updates(map[string]any{"status": "cancelled", "updated_at": now}).Error
		return appendAudit(tx, workspaceID, actorUserID, "design_review", reviewID, "analysis_abandoned", 0)
	})
}

func appendAudit(tx *gorm.DB, workspaceID, userID uint, resource, target, action string, projectID uint) error {
	return tx.Create(&auditgorm.AuditLog{WorkspaceID: workspaceID, UserID: userID, StatusCode: 200, Resource: resource, ProjectID: projectID, Action: action, Target: target, CreatedAt: time.Now().UTC()}).Error
}

func differenceFromRecord(row Difference) designreview.DesignDifference {
	result := designreview.DesignDifference{ID: row.ID, ReviewID: row.ReviewID, Ordinal: row.Ordinal, Source: row.Source, AnalysisKind: row.AnalysisKind, CreatedByUserID: row.CreatedByUserID, Category: row.Category, Severity: row.Severity, Title: row.Title, Description: row.Description, Recommendation: row.Recommendation, Confidence: row.Confidence, EvidenceBasis: row.EvidenceBasis, EvidenceNotes: row.EvidenceNotes, VerificationStatus: row.VerificationStatus, ResolutionStatus: row.ResolutionStatus, RecheckStatus: row.RecheckStatus, HumanEdited: row.HumanEdited, AIOriginal: row.AIOriginal, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	_ = json.Unmarshal(row.DesignRegion, &result.DesignRegion)
	_ = json.Unmarshal(row.ImplementationRegion, &result.ImplementationRegion)
	return result
}

func newID() string { return uuidNewString() }
