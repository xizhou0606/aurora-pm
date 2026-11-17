package designreviewgorm

import (
	"context"
	"encoding/json"
	"fmt"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) BuildRecheckSnapshot(ctx context.Context, workspaceID uint, reviewID string) (designreview.RecheckSnapshot, error) {
	var review Review
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, reviewID).First(&review).Error; err != nil {
		return designreview.RecheckSnapshot{}, mapNotFound(err)
	}
	if review.SourceReviewID == nil || review.ChainRootID == nil {
		return designreview.RecheckSnapshot{}, designreview.ErrConflict
	}
	var source Review
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, *review.SourceReviewID).First(&source).Error; err != nil {
		return designreview.RecheckSnapshot{}, mapNotFound(err)
	}
	var rounds []Review
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND chain_root_id = ? AND round_number <= ?", workspaceID, *review.ChainRootID, source.RoundNumber).Order("round_number").Find(&rounds).Error; err != nil {
		return designreview.RecheckSnapshot{}, err
	}
	roundByReview := make(map[string]int, len(rounds))
	ids := make([]string, 0, len(rounds))
	for _, round := range rounds {
		roundByReview[round.ID] = round.RoundNumber
		ids = append(ids, round.ID)
	}
	var differences []Difference
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id IN ?", workspaceID, ids).Order("created_at, id").Find(&differences).Error; err != nil {
		return designreview.RecheckSnapshot{}, err
	}
	byID := make(map[string]Difference, len(differences))
	byLineage := make(map[string][]Difference)
	for _, difference := range differences {
		byID[difference.ID] = difference
		if difference.LineageID != nil {
			byLineage[*difference.LineageID] = append(byLineage[*difference.LineageID], difference)
		}
	}
	selected := make(map[string]Difference)
	for _, difference := range differences {
		if difference.ReviewID == source.ID && difference.VerificationStatus != "false_positive" && difference.ResolutionStatus == "unresolved" {
			selected[lineageRoot(difference)] = difference
		}
	}
	var previousVerifications []RecheckVerification
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND review_id = ?", workspaceID, source.ID).Find(&previousVerifications).Error; err != nil {
		return designreview.RecheckSnapshot{}, err
	}
	for _, verification := range previousVerifications {
		effective := verification.AIStatus
		if verification.ManualStatusOverride != nil {
			effective = *verification.ManualStatusOverride
		}
		if effective == "uncertain" {
			if historical, ok := byID[verification.HistoricalDifferenceID]; ok {
				selected[verification.LineageRootDifferenceID] = historical
			}
		}
	}
	snapshot := designreview.RecheckSnapshot{Version: "1.0", SourceReviewID: source.ID, ChainRootReviewID: *review.ChainRootID, Items: []designreview.RecheckHistoryItem{}, Lineages: []designreview.RecheckLineage{}}
	for root, latest := range selected {
		lineageRows := lineageDifferences(latest, root, differences, byLineage)
		first := latest
		seenRounds := make(map[string]struct{})
		for _, candidate := range lineageRows {
			seenRounds[candidate.ReviewID] = struct{}{}
			if roundByReview[candidate.ReviewID] < roundByReview[first.ReviewID] {
				first = candidate
			}
		}
		item := differenceFromRecord(latest)
		snapshot.Items = append(snapshot.Items, designreview.RecheckHistoryItem{
			HistoricalDifferenceID: latest.ID, LineageRootDifferenceID: root, LatestDifferenceID: latest.ID,
			Title: latest.Title, Description: latest.Description, Recommendation: latest.Recommendation,
			Category: latest.Category, Severity: latest.Severity, DesignRegion: item.DesignRegion, ImplementationRegion: item.ImplementationRegion,
			FirstSeenReviewID: first.ReviewID, LatestReviewID: latest.ReviewID, PersistenceRounds: len(seenRounds),
			LineageSummary: fmt.Sprintf("该问题已在走查链中出现 %d 轮。", len(seenRounds)),
		})
	}
	var lineages []DifferenceLineage
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND chain_root_review_id = ?", workspaceID, *review.ChainRootID).Order("created_at, id").Find(&lineages).Error; err != nil {
		return designreview.RecheckSnapshot{}, err
	}
	for _, lineage := range lineages {
		rows := byLineage[lineage.ID]
		if len(rows) == 0 {
			continue
		}
		latest := rows[0]
		for _, candidate := range rows[1:] {
			if roundByReview[candidate.ReviewID] > roundByReview[latest.ReviewID] {
				latest = candidate
			}
		}
		item := differenceFromRecord(latest)
		var designAnchor, implementationAnchor any
		_ = json.Unmarshal(latest.DesignIdentityAnchor, &designAnchor)
		_ = json.Unmarshal(latest.ImplementationIdentityAnchor, &implementationAnchor)
		snapshot.Lineages = append(snapshot.Lineages, designreview.RecheckLineage{
			LineageID: lineage.ID, LatestDifferenceID: latest.ID, LineageRootDifferenceID: lineageRoot(latest),
			VerificationStatus: lineage.VerificationStatus, ResolutionStatus: latest.ResolutionStatus,
			Title: latest.Title, Description: latest.Description, Category: latest.Category, Severity: latest.Severity,
			DesignRegion: item.DesignRegion, ImplementationRegion: item.ImplementationRegion,
			DesignAnchor: designAnchor, ImplementationAnchor: implementationAnchor,
		})
	}
	return snapshot, nil
}

func lineageRoot(row Difference) string {
	if row.LineageRootDifferenceID != nil {
		return *row.LineageRootDifferenceID
	}
	return row.ID
}

func lineageDifferences(latest Difference, root string, all []Difference, byLineage map[string][]Difference) []Difference {
	if latest.LineageID != nil {
		return byLineage[*latest.LineageID]
	}
	result := make([]Difference, 0, 1)
	for _, candidate := range all {
		if lineageRoot(candidate) == root {
			result = append(result, candidate)
		}
	}
	return result
}
