package designreviewgorm

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) loadRecheckState(ctx context.Context, workspaceID uint, detail *designreview.DesignReviewDetail) error {
	type verificationRow struct {
		RecheckVerification
		HistoricalTitle string
	}
	var verifications []verificationRow
	if err := repository.db.WithContext(ctx).Table("design_review_recheck_verifications rv").
		Select("rv.*, d.title AS historical_title").
		Joins("JOIN design_review_differences d ON d.workspace_id = rv.workspace_id AND d.id = rv.historical_difference_id").
		Where("rv.workspace_id = ? AND rv.review_id = ?", workspaceID, detail.ID).Order("rv.created_at, rv.id").Scan(&verifications).Error; err != nil {
		return err
	}
	detail.RecheckVerifications = make([]designreview.RecheckVerification, 0, len(verifications))
	suppressed := make(map[string]struct{})
	for _, row := range verifications {
		effective := row.AIStatus
		if row.ManualStatusOverride != nil {
			effective = *row.ManualStatusOverride
		}
		item := designreview.RecheckVerification{ID: row.ID, HistoricalDifferenceID: row.HistoricalDifferenceID,
			HistoricalTitle: row.HistoricalTitle, LineageID: row.LineageID, CurrentDifferenceID: row.CurrentDifferenceID,
			AIStatus: row.AIStatus, ManualStatusOverride: row.ManualStatusOverride, EffectiveStatus: effective,
			Confidence: row.Confidence, Reason: row.Reason, EvidenceSummary: row.EvidenceSummary, CanOverride: true}
		_ = json.Unmarshal(row.CurrentDesignRegion, &item.CurrentDesignRegion)
		_ = json.Unmarshal(row.CurrentImplementationRegion, &item.CurrentImplementationRegion)
		detail.RecheckVerifications = append(detail.RecheckVerifications, item)
		if row.CurrentDifferenceID != nil && effective != "still_present" {
			suppressed[*row.CurrentDifferenceID] = struct{}{}
		}
	}
	if len(suppressed) > 0 {
		visible := detail.Differences[:0]
		for _, difference := range detail.Differences {
			if _, hidden := suppressed[difference.ID]; !hidden {
				visible = append(visible, difference)
			}
		}
		detail.Differences = visible
	}
	var matches []RecheckMatch
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ? AND status = 'pending'", workspaceID, detail.ID).Order("created_at, id").Find(&matches).Error; err != nil {
		return err
	}
	detail.PendingRecheckMatches = make([]designreview.PendingRecheckMatch, 0, len(matches))
	for _, row := range matches {
		var candidate designreview.RecheckCandidate
		if json.Unmarshal(row.Candidate, &candidate) != nil {
			return designreview.ErrInvalid
		}
		var latest Difference
		if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND lineage_id = ?", workspaceID, row.SuggestedLineageID).Order("created_at DESC, id DESC").First(&latest).Error; err != nil {
			return mapNotFound(err)
		}
		detail.PendingRecheckMatches = append(detail.PendingRecheckMatches, designreview.PendingRecheckMatch{
			ID: row.ID, CandidateID: row.CandidateID, SuggestedLineageID: row.SuggestedLineageID,
			SuggestedLineageTitle: latest.Title, Candidate: candidate.Input(), DiscoverySource: candidate.DiscoverySource,
			Confidence: row.Confidence, Reason: row.Reason, CanResolve: true,
		})
	}
	return nil
}

func (repository *Repository) ResolveRecheckMatch(ctx context.Context, workspaceID uint, reviewID, matchID string, actorUserID uint, resolution string) error {
	if resolution != "inherit" && resolution != "new" {
		return designreview.ErrInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var match RecheckMatch
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, matchID).First(&match).Error; err != nil {
			return mapNotFound(err)
		}
		if match.Status != "pending" {
			return designreview.ErrConflict
		}
		var review Review
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, match.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if err := ensureReviewHistoryMutable(tx, review); err != nil {
			return err
		}
		var candidate designreview.RecheckCandidate
		if json.Unmarshal(match.Candidate, &candidate) != nil || !candidate.Input().Valid() {
			return designreview.ErrInvalid
		}
		var designAnchor, implementationAnchor *designreview.IdentityAnchor
		_ = json.Unmarshal(match.DesignIdentityAnchor, &designAnchor)
		_ = json.Unmarshal(match.ImplementationIdentityAnchor, &implementationAnchor)
		lineageID := match.SuggestedLineageID
		var lineage DifferenceLineage
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, lineageID).First(&lineage).Error; err != nil {
			return mapNotFound(err)
		}
		var previous *Difference
		if resolution == "new" {
			lineageID = newID()
			lineage = DifferenceLineage{ID: lineageID, WorkspaceID: workspaceID, ChainRootReviewID: *review.ChainRootID, VerificationStatus: "pending", CreatedAt: repository.now(), UpdatedAt: repository.now()}
			if err := tx.Create(&lineage).Error; err != nil {
				return err
			}
		} else {
			if lineage.ChainRootReviewID != *review.ChainRootID {
				return designreview.ErrConflict
			}
			var row Difference
			if err := tx.Where("workspace_id = ? AND lineage_id = ? AND review_id <> ?", workspaceID, lineageID, review.ID).Order("created_at DESC, id DESC").First(&row).Error; err != nil {
				return mapNotFound(err)
			}
			previous = &row
		}
		var differenceID *string
		if !(resolution == "inherit" && lineage.VerificationStatus == "false_positive") {
			var ordinal int
			if err := tx.Model(&Difference{}).Where("workspace_id = ? AND review_id = ?", workspaceID, review.ID).Select("COALESCE(MAX(ordinal), 0)").Scan(&ordinal).Error; err != nil {
				return err
			}
			id := newID()
			rootID := id
			var previousID *string
			status := "new"
			verificationStatus := "pending"
			if previous != nil {
				rootID, previousID, status = lineageRoot(*previous), &previous.ID, "still_present"
				if lineage.VerificationStatus == "confirmed" {
					verificationStatus = "confirmed"
				}
			}
			item := materializedCandidate{candidate: candidate, differenceID: id, lineageID: lineageID, rootID: rootID, previousID: previousID, status: status, designAnchor: designAnchor, implementationAnchor: implementationAnchor}
			if err := tx.Create(candidateDifference(designreview.ClaimedAnalysisJob{WorkspaceID: workspaceID, ReviewID: review.ID}, item, ordinal+1, verificationStatus)).Error; err != nil {
				return mapConflict(err)
			}
			differenceID = &id
			if resolution == "inherit" {
				if err := tx.Model(&RecheckVerification{}).Where("workspace_id = ? AND review_id = ? AND lineage_id = ? AND current_difference_id IS NULL AND ai_status = 'still_present'", workspaceID, review.ID, lineageID).Updates(map[string]any{"current_difference_id": id, "updated_at": repository.now()}).Error; err != nil {
					return err
				}
			}
		}
		status := "new"
		if resolution == "inherit" {
			status = "inherited"
		}
		if err := tx.Model(&match).Updates(map[string]any{"status": status, "resolved_by_user_id": actorUserID, "resolved_at": repository.now(), "difference_id": differenceID, "updated_at": repository.now()}).Error; err != nil {
			return err
		}
		if err := recomputeRecheckConclusion(tx, workspaceID, review.ID); err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review_recheck_match", match.ID, "resolved_"+resolution, review.ProjectSetIDSnapshot)
	})
}

func (repository *Repository) OverrideRecheckVerification(ctx context.Context, workspaceID uint, reviewID, verificationID string, actorUserID uint, status *string) error {
	if status != nil && !validRecheckStatus(*status) {
		return designreview.ErrInvalid
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var verification RecheckVerification
		if err := tx.Where("workspace_id = ? AND review_id = ? AND id = ?", workspaceID, reviewID, verificationID).First(&verification).Error; err != nil {
			return mapNotFound(err)
		}
		var review Review
		if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, verification.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if err := ensureReviewHistoryMutable(tx, review); err != nil {
			return err
		}
		previous := verification.AIStatus
		if verification.ManualStatusOverride != nil {
			previous = *verification.ManualStatusOverride
		}
		effective := verification.AIStatus
		if status != nil {
			effective = *status
		}
		if effective == "still_present" && verification.CurrentDifferenceID == nil {
			if err := repository.reopenHistoricalDifference(tx, &verification, review, actorUserID); err != nil {
				return err
			}
		}
		updates := map[string]any{"manual_status_override": status, "overridden_by_user_id": nil, "overridden_at": nil, "updated_at": repository.now()}
		if status != nil {
			updates["overridden_by_user_id"], updates["overridden_at"] = actorUserID, repository.now()
		}
		if err := tx.Model(&verification).Updates(updates).Error; err != nil {
			return err
		}
		event := ResolutionEvent{ID: newID(), WorkspaceID: workspaceID, ReviewID: review.ID, DifferenceID: verification.CurrentDifferenceID,
			RecheckVerificationID: &verification.ID, ActorUserID: actorUserID, EventType: "recheck_status_overridden", PreviousValue: &previous, NextValue: &effective, CreatedAt: repository.now()}
		if status == nil {
			event.EventType = "recheck_status_restored"
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if err := recomputeRecheckConclusion(tx, workspaceID, review.ID); err != nil {
			return err
		}
		return appendAudit(tx, workspaceID, actorUserID, "design_review_recheck_verification", verification.ID, event.EventType, review.ProjectSetIDSnapshot)
	})
}

func (repository *Repository) reopenHistoricalDifference(tx *gorm.DB, verification *RecheckVerification, review Review, actorUserID uint) error {
	var historical Difference
	if err := tx.Where("workspace_id = ? AND id = ?", verification.WorkspaceID, verification.HistoricalDifferenceID).First(&historical).Error; err != nil {
		return mapNotFound(err)
	}
	var ordinal int
	if err := tx.Model(&Difference{}).Where("workspace_id = ? AND review_id = ?", verification.WorkspaceID, review.ID).Select("COALESCE(MAX(ordinal), 0)").Scan(&ordinal).Error; err != nil {
		return err
	}
	id := newID()
	status := "still_present"
	row := Difference{ID: id, WorkspaceID: verification.WorkspaceID, ReviewID: review.ID, LineageID: verification.LineageID,
		LineageRootDifferenceID: &verification.LineageRootDifferenceID, PreviousDifferenceID: &historical.ID, Ordinal: ordinal + 1,
		Source: "manual_reopen", AnalysisKind: "full", CreatedByUserID: &actorUserID, Category: historical.Category, Severity: historical.Severity,
		Title: historical.Title, Description: historical.Description, Recommendation: historical.Recommendation, Confidence: 1,
		DesignRegion: verification.CurrentDesignRegion, ImplementationRegion: verification.CurrentImplementationRegion,
		EvidenceBasis: "human", EvidenceNotes: "人工确认历史差异在本轮仍存在；未复用历史坐标。", VerificationStatus: "confirmed",
		VerifiedByUserID: &actorUserID, VerifiedAt: ptrTime(repository.now()), ResolutionStatus: "unresolved", RecheckStatus: &status, HumanEdited: true}
	if err := tx.Create(&row).Error; err != nil {
		return mapConflict(err)
	}
	verification.CurrentDifferenceID = &id
	return tx.Model(verification).Update("current_difference_id", id).Error
}

func ensureReviewHistoryMutable(tx *gorm.DB, review Review) error {
	if review.Status != "completed" || review.ChainRootID == nil {
		return designreview.ErrConflict
	}
	return ensureReviewMutable(tx, review)
}

func validRecheckStatus(value string) bool {
	return value == "still_present" || value == "resolved" || value == "uncertain" || value == "not_applicable"
}

func recomputeRecheckConclusion(tx *gorm.DB, workspaceID uint, reviewID string) error {
	if err := refreshConclusion(tx, workspaceID, reviewID); err != nil {
		return err
	}
	var unresolved int64
	if err := tx.Model(&RecheckMatch{}).Where("workspace_id = ? AND review_id = ? AND status = 'pending'", workspaceID, reviewID).Count(&unresolved).Error; err != nil {
		return err
	}
	var uncertain int64
	if err := tx.Model(&RecheckVerification{}).Where("workspace_id = ? AND review_id = ? AND COALESCE(manual_status_override, ai_status) = 'uncertain'", workspaceID, reviewID).Count(&uncertain).Error; err != nil {
		return err
	}
	if unresolved+uncertain > 0 && reviewConclusion(tx, workspaceID, reviewID) == "passed" {
		return tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", workspaceID, reviewID).Update("conclusion", "needs_review").Error
	}
	return nil
}

func ptrTime(value time.Time) *time.Time { return &value }
