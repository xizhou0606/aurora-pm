package designreviewgorm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) CreateDifference(ctx context.Context, input designreview.DifferenceRecord) (designreview.DesignDifference, error) {
	var created Difference
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review Review
		if err := tx.Where("workspace_id = ? AND id = ? AND deleting_at IS NULL", input.WorkspaceID, input.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if err := ensureReviewMutable(tx, review); err != nil {
			return err
		}
		var ordinal int
		if err := tx.Model(&Difference{}).Where("workspace_id = ? AND review_id = ?", input.WorkspaceID, input.ReviewID).Select("COALESCE(MAX(ordinal), 0)").Scan(&ordinal).Error; err != nil {
			return err
		}
		actor := input.ActorUserID
		if review.ChainRootID == nil {
			return designreview.ErrConflict
		}
		lineageID := newID()
		created = Difference{
			ID: input.ID, WorkspaceID: input.WorkspaceID, ReviewID: input.ReviewID, LineageID: &lineageID, LineageRootDifferenceID: &input.ID, Ordinal: ordinal + 1,
			Source: "manual", AnalysisKind: "full", CreatedByUserID: &actor, Category: input.Input.Category,
			Severity: input.Input.Severity, Title: strings.TrimSpace(input.Input.Title), Description: strings.TrimSpace(input.Input.Description),
			Recommendation: strings.TrimSpace(input.Input.Recommendation), Confidence: 1, DesignRegion: jsonBytes(input.Input.DesignRegion),
			ImplementationRegion: jsonBytes(input.Input.ImplementationRegion), EvidenceBasis: "human", EvidenceNotes: strings.TrimSpace(input.Input.EvidenceNotes),
			VerificationStatus: "confirmed", ResolutionStatus: "unresolved", HumanEdited: true,
		}
		if err := tx.Create(&DifferenceLineage{ID: lineageID, WorkspaceID: input.WorkspaceID, ChainRootReviewID: *review.ChainRootID, VerificationStatus: "confirmed", VerifiedByUserID: &actor, VerifiedAt: ptrTime(repository.now()), CreatedAt: repository.now(), UpdatedAt: repository.now()}).Error; err != nil {
			return err
		}
		if err := tx.Create(&created).Error; err != nil {
			return mapConflict(err)
		}
		if err := refreshConclusion(tx, input.WorkspaceID, input.ReviewID); err != nil {
			return err
		}
		return appendAudit(tx, input.WorkspaceID, input.ActorUserID, "design_review_difference", input.ID, "created", review.ProjectSetIDSnapshot)
	})
	return differenceFromRecord(created), err
}

func (repository *Repository) UpdateDifference(ctx context.Context, workspaceID uint, reviewID, differenceID string, actorUserID uint, input designreview.DifferenceInput) (designreview.DesignDifference, error) {
	var row Difference
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		review, err := differenceWithReview(tx, workspaceID, reviewID, differenceID, &row)
		if err != nil {
			return err
		}
		if err := ensureReviewMutable(tx, review); err != nil {
			return err
		}
		if isManualDifference(row) && !canManageManualDifference(review, row, actorUserID) {
			return designreview.ErrCreatorOnly
		}
		now := repository.now()
		updates := map[string]any{"category": input.Category, "severity": input.Severity, "title": strings.TrimSpace(input.Title), "description": strings.TrimSpace(input.Description), "recommendation": strings.TrimSpace(input.Recommendation), "design_region": nullableJSON(input.DesignRegion), "implementation_region": nullableJSON(input.ImplementationRegion), "evidence_notes": strings.TrimSpace(input.EvidenceNotes), "verification_status": "confirmed", "verified_by_user_id": actorUserID, "verified_at": now, "human_edited": true, "edited_by_user_id": actorUserID, "edited_at": now, "updated_at": now}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := updateLineageDecision(tx, row, actorUserID, "confirmed", now); err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, row.ID).First(&row).Error; err != nil {
			return err
		}
		if err := refreshConclusion(tx, workspaceID, row.ReviewID); err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review_difference", differenceID, "updated", review.ProjectSetIDSnapshot)
	})
	return differenceFromRecord(row), err
}

func (repository *Repository) DeleteDifference(ctx context.Context, workspaceID uint, reviewID, differenceID string, actorUserID uint) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Difference
		review, err := differenceWithReview(tx, workspaceID, reviewID, differenceID, &row)
		if err != nil {
			return err
		}
		if err := ensureReviewMutable(tx, review); err != nil {
			return err
		}
		if !isManualDifference(row) {
			return designreview.ErrConflict
		}
		if !canManageManualDifference(review, row, actorUserID) {
			return designreview.ErrCreatorOnly
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		if err := refreshConclusion(tx, workspaceID, row.ReviewID); err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review_difference", differenceID, "deleted", review.ProjectSetIDSnapshot)
	})
}

func (repository *Repository) TransitionDifference(ctx context.Context, workspaceID uint, reviewID, differenceID string, actorUserID uint, operation, status string) (designreview.DesignDifference, error) {
	var row Difference
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review Review
		if err := tx.Table("design_reviews dr").Select("dr.*").Joins("JOIN design_review_differences d ON d.review_id = dr.id AND d.workspace_id = dr.workspace_id").Where("dr.workspace_id = ? AND dr.id = ? AND d.review_id = ? AND d.id = ?", workspaceID, reviewID, reviewID, differenceID).Scan(&review).Error; err != nil {
			return err
		}
		if review.ID == "" {
			return designreview.ErrNotFound
		}
		if err := ensureReviewMutable(tx, review); err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, differenceID).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		updates := map[string]any{"updated_at": repository.now()}
		action := operation
		var previousValue, nextValue *string
		switch operation {
		case "verify":
			if status != "confirmed" && status != "false_positive" {
				return designreview.ErrInvalid
			}
			updates["verification_status"] = status
			updates["verified_by_user_id"] = actorUserID
			updates["verified_at"] = repository.now()
			if err := updateLineageDecision(tx, row, actorUserID, status, repository.now()); err != nil {
				return err
			}
		case "resolve":
			previous, next := row.ResolutionStatus, "resolved"
			previousValue, nextValue = &previous, &next
			updates["resolution_status"] = "resolved"
			updates["resolved_by_user_id"] = actorUserID
			updates["resolved_at"] = repository.now()
		case "reopen":
			previous, next := row.ResolutionStatus, "unresolved"
			previousValue, nextValue = &previous, &next
			updates["resolution_status"] = "unresolved"
			updates["resolved_by_user_id"] = nil
			updates["resolved_at"] = nil
		default:
			return designreview.ErrInvalid
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, row.ID).First(&row).Error; err != nil {
			return err
		}
		if operation != "verify" {
			event := ResolutionEvent{ID: newID(), WorkspaceID: workspaceID, ReviewID: row.ReviewID, DifferenceID: &row.ID, ActorUserID: actorUserID, EventType: action, PreviousValue: previousValue, NextValue: nextValue, CreatedAt: repository.now()}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
		}
		if err := refreshConclusion(tx, workspaceID, row.ReviewID); err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review_difference", differenceID, action, review.ProjectSetIDSnapshot)
	})
	return differenceFromRecord(row), err
}

func differenceWithReview(tx *gorm.DB, workspaceID uint, reviewID, differenceID string, row *Difference) (Review, error) {
	if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, differenceID).First(row).Error; err != nil {
		return Review{}, mapNotFound(err)
	}
	var review Review
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, reviewID).First(&review).Error; err != nil {
		return Review{}, mapNotFound(err)
	}
	return review, nil
}

func isManualDifference(row Difference) bool {
	return row.Source == "manual" || row.Source == "manual_reopen"
}

func canManageManualDifference(review Review, row Difference, actorUserID uint) bool {
	return review.CreatedByUserID == actorUserID || (row.CreatedByUserID != nil && *row.CreatedByUserID == actorUserID)
}

func updateLineageDecision(tx *gorm.DB, row Difference, actorUserID uint, nextStatus string, now time.Time) error {
	if row.LineageID == nil {
		return nil
	}
	var lineage DifferenceLineage
	if err := tx.Where("workspace_id = ? AND id = ?", row.WorkspaceID, *row.LineageID).First(&lineage).Error; err != nil {
		return mapNotFound(err)
	}
	previousStatus := lineage.VerificationStatus
	if err := tx.Model(&lineage).Updates(map[string]any{"verification_status": nextStatus, "verified_by_user_id": actorUserID, "verified_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	if previousStatus == nextStatus {
		return nil
	}
	return tx.Create(&LineageEvent{ID: newID(), WorkspaceID: row.WorkspaceID, LineageID: lineage.ID, ReviewID: row.ReviewID, DifferenceID: &row.ID, ActorUserID: actorUserID, PreviousStatus: previousStatus, NextStatus: nextStatus, CreatedAt: now}).Error
}

func refreshConclusion(tx *gorm.DB, workspaceID uint, reviewID string) error {
	var effective int64
	effectiveQuery := "workspace_id = ? AND review_id = ? AND verification_status <> 'false_positive' AND resolution_status = 'unresolved' AND NOT EXISTS (SELECT 1 FROM design_review_recheck_verifications rv WHERE rv.workspace_id = design_review_differences.workspace_id AND rv.current_difference_id = design_review_differences.id AND COALESCE(rv.manual_status_override, rv.ai_status) <> 'still_present')"
	if err := tx.Model(&Difference{}).Where(effectiveQuery, workspaceID, reviewID).Count(&effective).Error; err != nil {
		return err
	}
	var blocking int64
	if err := tx.Model(&Difference{}).Where(effectiveQuery+" AND severity IN ?", workspaceID, reviewID, []string{"critical", "major"}).Count(&blocking).Error; err != nil {
		return err
	}
	var review Review
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, reviewID).First(&review).Error; err != nil {
		return mapNotFound(err)
	}
	var warnings []json.RawMessage
	warningsValid := json.Unmarshal(review.CaptureWarnings, &warnings) == nil
	var normalization struct {
		Enabled bool   `json:"enabled"`
		Status  string `json:"status"`
	}
	normalizationValid := json.Unmarshal(review.DOMNormalization, &normalization) == nil
	evidenceComplete := review.DimensionsMatch && review.DesignStructureAvailable && review.ImplementationStructureComplete &&
		warningsValid && len(warnings) == 0 && review.ModelCoverageComplete && review.HeatmapDifferenceRatio <= 0.005 &&
		normalizationValid && (!normalization.Enabled || normalization.Status == "complete")
	conclusion := "passed"
	if blocking > 0 {
		conclusion = "needs_fix"
	} else if effective > 0 || !evidenceComplete {
		conclusion = "needs_review"
	}
	return tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", workspaceID, reviewID).Updates(map[string]any{"conclusion": conclusion, "updated_at": timeNow()}).Error
}

func nullableJSON(value any) any {
	if value == nil {
		return nil
	}
	raw, _ := json.Marshal(value)
	return raw
}

var timeNow = func() any { return time.Now().UTC() }
