package designreviewgorm

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"gorm.io/gorm"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

type materializedCandidate struct {
	candidate            designreview.RecheckCandidate
	differenceID         string
	lineageID            string
	rootID               string
	previousID           *string
	status               string
	designAnchor         *designreview.IdentityAnchor
	implementationAnchor *designreview.IdentityAnchor
}

func (repository *Repository) CompleteRecheckJob(ctx context.Context, job designreview.ClaimedAnalysisJob, snapshot designreview.RecheckSnapshot, analysis designreview.RecheckAnalysis, metadata designreview.AnalysisRunMetadata) error {
	metadataUpdates, err := completionMetadata(metadata)
	if err != nil {
		return err
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review Review
		if err := tx.Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).First(&review).Error; err != nil {
			return mapNotFound(err)
		}
		if review.SourceReviewID == nil || review.Status != "analyzing" {
			return designreview.ErrConflict
		}
		var existing int64
		if err := tx.Model(&Difference{}).Where("workspace_id = ? AND review_id = ?", job.WorkspaceID, job.ReviewID).Count(&existing).Error; err != nil || existing > 0 {
			if err != nil {
				return err
			}
			return designreview.ErrConflict
		}
		lineages := make(map[string]designreview.RecheckLineage, len(snapshot.Lineages))
		lineageByLatest := make(map[string]string, len(snapshot.Lineages))
		for _, lineage := range snapshot.Lineages {
			lineages[lineage.LineageID] = lineage
			lineageByLatest[lineage.LatestDifferenceID] = lineage.LineageID
		}
		history := make(map[string]designreview.RecheckHistoryItem, len(snapshot.Items))
		for _, item := range snapshot.Items {
			history[item.HistoricalDifferenceID] = item
		}
		matches := make(map[string]designreview.LineageMatchSuggestion, len(analysis.LineageMatches))
		for _, match := range analysis.LineageMatches {
			matches[match.CandidateID] = match
		}
		materialized := make(map[string]materializedCandidate)
		pending := 0
		ordinal := 0
		for _, candidate := range analysis.CurrentDifferences {
			match := matches[candidate.CandidateID]
			if match.LineageID != nil {
				lineage := lineages[*match.LineageID]
				if strongLineageMatch(candidate, match, lineage) {
					if lineage.VerificationStatus == "false_positive" {
						continue
					}
					ordinal++
					verificationStatus := lineage.VerificationStatus
					if verificationStatus != "confirmed" {
						verificationStatus = "pending"
					}
					item := materializedCandidate{candidate: candidate, differenceID: newID(), lineageID: lineage.LineageID, rootID: lineage.LineageRootDifferenceID, previousID: &lineage.LatestDifferenceID, status: "still_present", designAnchor: candidate.DesignAnchor, implementationAnchor: candidate.ImplementationAnchor}
					if err := tx.Create(candidateDifference(job, item, ordinal, verificationStatus)).Error; err != nil {
						return err
					}
					materialized[candidate.CandidateID] = item
					continue
				}
				pending++
				if err := tx.Create(&RecheckMatch{ID: newID(), WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID,
					CandidateID: candidate.CandidateID, SuggestedLineageID: *match.LineageID, Candidate: jsonBytes(candidate),
					DesignIdentityAnchor: jsonBytes(candidate.DesignAnchor), ImplementationIdentityAnchor: jsonBytes(candidate.ImplementationAnchor),
					Confidence: match.Confidence, Reason: strings.TrimSpace(match.Reason), Status: "pending", CreatedAt: repository.now(), UpdatedAt: repository.now()}).Error; err != nil {
					return err
				}
				continue
			}
			ordinal++
			differenceID, lineageID := newID(), newID()
			item := materializedCandidate{candidate: candidate, differenceID: differenceID, lineageID: lineageID, rootID: differenceID, status: "new", designAnchor: candidate.DesignAnchor, implementationAnchor: candidate.ImplementationAnchor}
			if err := tx.Create(&DifferenceLineage{ID: lineageID, WorkspaceID: job.WorkspaceID, ChainRootReviewID: snapshot.ChainRootReviewID, VerificationStatus: "pending", CreatedAt: repository.now(), UpdatedAt: repository.now()}).Error; err != nil {
				return err
			}
			if err := tx.Create(candidateDifference(job, item, ordinal, "pending")).Error; err != nil {
				return err
			}
			materialized[candidate.CandidateID] = item
		}
		uncertain := 0
		for _, verification := range analysis.HistoricalVerifications {
			historical := history[verification.HistoricalDifferenceID]
			lineageID := lineageByLatest[historical.LatestDifferenceID]
			var currentID *string
			if verification.CurrentDifferenceCandidateID != nil {
				if current, ok := materialized[*verification.CurrentDifferenceCandidateID]; ok {
					currentID = &current.differenceID
				}
			}
			if verification.Status == "uncertain" || verification.Status == "still_present" && currentID == nil {
				uncertain++
			}
			row := RecheckVerification{ID: newID(), WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID,
				HistoricalDifferenceID: verification.HistoricalDifferenceID, LineageRootDifferenceID: historical.LineageRootDifferenceID,
				CurrentDifferenceID: currentID, AIStatus: verification.Status, Confidence: verification.Confidence,
				Reason: strings.TrimSpace(verification.Reason), EvidenceSummary: strings.TrimSpace(verification.EvidenceSummary),
				CurrentDesignRegion: jsonBytes(verification.CurrentDesignRegion), CurrentImplementationRegion: jsonBytes(verification.CurrentImplementationRegion),
				CreatedAt: repository.now(), UpdatedAt: repository.now()}
			if lineageID != "" {
				row.LineageID = &lineageID
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&AnalysisJob{}).Where("id = ? AND workspace_id = ? AND status = 'running'", job.ID, job.WorkspaceID).Updates(map[string]any{"status": "succeeded", "lease_owner": nil, "lease_until": nil, "updated_at": repository.now()})
		if updated.Error != nil || updated.RowsAffected != 1 {
			return designreview.ErrConflict
		}
		reviewUpdates := map[string]any{"status": "completed", "analysis_summary": analysis.Summary, "model_coverage_complete": analysis.CoverageComplete, "completed_at": repository.now(), "updated_at": repository.now()}
		copyCompletionMetadata(reviewUpdates, metadataUpdates, "analysis_duration_ms")
		if err := tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).Updates(reviewUpdates).Error; err != nil {
			return err
		}
		if err := refreshConclusion(tx, job.WorkspaceID, job.ReviewID); err != nil {
			return err
		}
		if (pending > 0 || uncertain > 0) && reviewConclusion(tx, job.WorkspaceID, job.ReviewID) == "passed" {
			if err := tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", job.WorkspaceID, job.ReviewID).Update("conclusion", "needs_review").Error; err != nil {
				return err
			}
		}
		return appendAudit(tx, job.WorkspaceID, review.CreatedByUserID, "design_review", job.ReviewID, "recheck_analysis_completed", review.ProjectSetIDSnapshot)
	})
}

func candidateDifference(job designreview.ClaimedAnalysisJob, item materializedCandidate, ordinal int, verificationStatus string) *Difference {
	recheckStatus := item.status
	input := item.candidate.Input()
	return &Difference{ID: item.differenceID, WorkspaceID: job.WorkspaceID, ReviewID: job.ReviewID,
		LineageID: &item.lineageID, LineageRootDifferenceID: &item.rootID, PreviousDifferenceID: item.previousID,
		Ordinal: ordinal, Source: mapDiscoverySource(item.candidate.DiscoverySource), AnalysisKind: "full",
		Category: input.Category, Severity: input.Severity, Title: input.Title, Description: input.Description,
		Recommendation: input.Recommendation, Confidence: input.Confidence, DesignRegion: jsonBytes(input.DesignRegion),
		ImplementationRegion: jsonBytes(input.ImplementationRegion), EvidenceBasis: "visual", EvidenceNotes: input.EvidenceNotes,
		VerificationStatus: verificationStatus, ResolutionStatus: "unresolved", RecheckStatus: &recheckStatus,
		DesignIdentityAnchor: jsonBytes(item.designAnchor), ImplementationIdentityAnchor: jsonBytes(item.implementationAnchor), AIOriginal: jsonBytes(item.candidate)}
}

func strongLineageMatch(candidate designreview.RecheckCandidate, match designreview.LineageMatchSuggestion, lineage designreview.RecheckLineage) bool {
	if match.Confidence < 0.9 || candidate.Category != lineage.Category {
		return false
	}
	return anchorMatches(candidate.DesignAnchor, lineage.DesignAnchor) || anchorMatches(candidate.ImplementationAnchor, lineage.ImplementationAnchor)
}

func anchorMatches(candidate *designreview.IdentityAnchor, historical any) bool {
	if candidate == nil || historical == nil {
		return false
	}
	raw, err := json.Marshal(historical)
	if err != nil {
		return false
	}
	var anchor designreview.IdentityAnchor
	return json.Unmarshal(raw, &anchor) == nil && candidate.Kind == anchor.Kind && candidate.StableID == anchor.StableID
}

func regionIOU(left, right *designreview.Region) float64 {
	if left == nil || right == nil {
		return 0
	}
	width := math.Max(0, math.Min(left.X+left.Width, right.X+right.Width)-math.Max(left.X, right.X))
	height := math.Max(0, math.Min(left.Y+left.Height, right.Y+right.Height)-math.Max(left.Y, right.Y))
	intersection := width * height
	if intersection <= 0 {
		return 0
	}
	return intersection / (left.Width*left.Height + right.Width*right.Height - intersection)
}

func mapDiscoverySource(value string) string {
	if value == "history_recovered" {
		return "history_recovered"
	}
	return "ai"
}

func reviewConclusion(tx *gorm.DB, workspaceID uint, reviewID string) string {
	var row Review
	if tx.Select("conclusion").Where("workspace_id = ? AND id = ?", workspaceID, reviewID).First(&row).Error != nil || row.Conclusion == nil {
		return ""
	}
	return *row.Conclusion
}
