package designreview

import (
	"strings"
)

type RecheckHistoryItem struct {
	HistoricalDifferenceID  string  `json:"historicalDifferenceId"`
	LineageRootDifferenceID string  `json:"lineageRootDifferenceId"`
	LatestDifferenceID      string  `json:"latestDifferenceId"`
	Title                   string  `json:"title"`
	Description             string  `json:"description"`
	Recommendation          string  `json:"recommendation"`
	Category                string  `json:"category"`
	Severity                string  `json:"severity"`
	DesignRegion            *Region `json:"designRegion"`
	ImplementationRegion    *Region `json:"implementationRegion"`
	FirstSeenReviewID       string  `json:"firstSeenReviewId"`
	LatestReviewID          string  `json:"latestReviewId"`
	PersistenceRounds       int     `json:"persistenceRounds"`
	LineageSummary          string  `json:"lineageSummary"`
}

type RecheckLineage struct {
	LineageID               string  `json:"lineageId"`
	LatestDifferenceID      string  `json:"latestDifferenceId"`
	LineageRootDifferenceID string  `json:"lineageRootDifferenceId"`
	VerificationStatus      string  `json:"verificationStatus"`
	ResolutionStatus        string  `json:"resolutionStatus"`
	Title                   string  `json:"title"`
	Description             string  `json:"description"`
	Category                string  `json:"category"`
	Severity                string  `json:"severity"`
	DesignRegion            *Region `json:"designRegion"`
	ImplementationRegion    *Region `json:"implementationRegion"`
	DesignAnchor            any     `json:"designAnchor"`
	ImplementationAnchor    any     `json:"implementationAnchor"`
}

type RecheckSnapshot struct {
	Version           string               `json:"version"`
	SourceReviewID    string               `json:"sourceReviewId"`
	ChainRootReviewID string               `json:"chainRootReviewId"`
	Items             []RecheckHistoryItem `json:"items"`
	Lineages          []RecheckLineage     `json:"lineages"`
}

type RecheckCandidate struct {
	CandidateID          string          `json:"candidateId"`
	DiscoverySource      string          `json:"discoverySource"`
	Category             string          `json:"category"`
	Severity             string          `json:"severity"`
	Title                string          `json:"title"`
	Description          string          `json:"description"`
	Recommendation       string          `json:"recommendation"`
	Confidence           float64         `json:"confidence"`
	DesignRegion         *Region         `json:"designRegion"`
	ImplementationRegion *Region         `json:"implementationRegion"`
	EvidenceNotes        string          `json:"evidenceNotes"`
	DesignAnchor         *IdentityAnchor `json:"-"`
	ImplementationAnchor *IdentityAnchor `json:"-"`
}

func (candidate RecheckCandidate) Input() DifferenceInput {
	return DifferenceInput{Category: candidate.Category, Severity: candidate.Severity, Title: candidate.Title,
		Description: candidate.Description, Recommendation: candidate.Recommendation, Confidence: candidate.Confidence,
		DesignRegion: candidate.DesignRegion, ImplementationRegion: candidate.ImplementationRegion, EvidenceNotes: candidate.EvidenceNotes}
}

type HistoricalVerificationResult struct {
	HistoricalDifferenceID       string  `json:"historicalDifferenceId"`
	Status                       string  `json:"status"`
	CurrentDifferenceCandidateID *string `json:"currentDifferenceCandidateId"`
	Confidence                   float64 `json:"confidence"`
	Reason                       string  `json:"reason"`
	EvidenceSummary              string  `json:"evidenceSummary"`
	CurrentDesignRegion          *Region `json:"currentDesignRegion"`
	CurrentImplementationRegion  *Region `json:"currentImplementationRegion"`
}

type LineageMatchSuggestion struct {
	CandidateID string  `json:"candidateId"`
	LineageID   *string `json:"lineageId"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
}

type RecheckAnalysis struct {
	CurrentDifferences      []RecheckCandidate             `json:"currentDifferences"`
	HistoricalVerifications []HistoricalVerificationResult `json:"historicalVerifications"`
	LineageMatches          []LineageMatchSuggestion       `json:"lineageMatches"`
	CoverageComplete        bool                           `json:"coverageComplete"`
	CoverageNotes           []string                       `json:"coverageNotes"`
	Summary                 string                         `json:"summary"`
}

func (analysis *RecheckAnalysis) Validate(snapshot RecheckSnapshot) error {
	if strings.TrimSpace(analysis.Summary) == "" || len(analysis.CurrentDifferences) > 100 || len(analysis.HistoricalVerifications) > 200 || len(analysis.LineageMatches) > 100 {
		return ErrInvalid
	}
	history := make(map[string]RecheckHistoryItem, len(snapshot.Items))
	lineageByLatest := make(map[string]string, len(snapshot.Lineages))
	lineages := make(map[string]RecheckLineage, len(snapshot.Lineages))
	for _, item := range snapshot.Items {
		history[item.HistoricalDifferenceID] = item
	}
	for _, lineage := range snapshot.Lineages {
		lineages[lineage.LineageID] = lineage
		lineageByLatest[lineage.LatestDifferenceID] = lineage.LineageID
	}
	candidates := make(map[string]RecheckCandidate, len(analysis.CurrentDifferences))
	for _, candidate := range analysis.CurrentDifferences {
		if !validCurrentID(candidate.CandidateID) || (candidate.DiscoverySource != "full_page" && candidate.DiscoverySource != "history_recovered") || !candidate.Input().Valid() {
			return ErrInvalid
		}
		if _, duplicate := candidates[candidate.CandidateID]; duplicate {
			return ErrInvalid
		}
		candidates[candidate.CandidateID] = candidate
	}
	matches := make(map[string]LineageMatchSuggestion, len(analysis.LineageMatches))
	usedLineages := make(map[string]struct{})
	for _, match := range analysis.LineageMatches {
		if _, ok := candidates[match.CandidateID]; !ok || match.Confidence < 0 || match.Confidence > 1 || strings.TrimSpace(match.Reason) == "" {
			return ErrInvalid
		}
		if _, duplicate := matches[match.CandidateID]; duplicate {
			return ErrInvalid
		}
		if match.LineageID != nil {
			if _, ok := lineages[*match.LineageID]; !ok {
				return ErrInvalid
			}
			if _, duplicate := usedLineages[*match.LineageID]; duplicate {
				return ErrInvalid
			}
			usedLineages[*match.LineageID] = struct{}{}
		}
		matches[match.CandidateID] = match
	}
	if len(matches) != len(candidates) {
		return ErrInvalid
	}
	seenHistory := make(map[string]struct{}, len(analysis.HistoricalVerifications))
	usedCandidates := make(map[string]struct{})
	for index := range analysis.HistoricalVerifications {
		verification := &analysis.HistoricalVerifications[index]
		item, ok := history[verification.HistoricalDifferenceID]
		if !ok || strings.TrimSpace(verification.Reason) == "" || strings.TrimSpace(verification.EvidenceSummary) == "" || verification.Confidence < 0 || verification.Confidence > 1 {
			return ErrInvalid
		}
		if _, duplicate := seenHistory[verification.HistoricalDifferenceID]; duplicate {
			return ErrInvalid
		}
		seenHistory[verification.HistoricalDifferenceID] = struct{}{}
		if (verification.Status == "still_present") != (verification.CurrentDifferenceCandidateID != nil) || !validHistoricalStatus(verification.Status) {
			return ErrInvalid
		}
		if verification.Status == "still_present" && verification.Confidence < 0.8 || (verification.Status == "resolved" || verification.Status == "not_applicable") && verification.Confidence < 0.9 {
			verification.Status = "uncertain"
			verification.CurrentDifferenceCandidateID = nil
		}
		if verification.CurrentDifferenceCandidateID != nil {
			candidateID := *verification.CurrentDifferenceCandidateID
			if _, ok := candidates[candidateID]; !ok {
				return ErrInvalid
			}
			if _, duplicate := usedCandidates[candidateID]; duplicate {
				return ErrInvalid
			}
			usedCandidates[candidateID] = struct{}{}
			expectedLineage := lineageByLatest[item.LatestDifferenceID]
			match := matches[candidateID]
			if expectedLineage == "" || match.LineageID == nil || *match.LineageID != expectedLineage {
				return ErrInvalid
			}
		}
	}
	if len(seenHistory) != len(history) {
		return ErrInvalid
	}
	for id, candidate := range candidates {
		if candidate.DiscoverySource == "history_recovered" {
			if _, ok := usedCandidates[id]; !ok {
				return ErrInvalid
			}
		}
	}
	return nil
}

func validHistoricalStatus(value string) bool {
	return value == "still_present" || value == "resolved" || value == "uncertain" || value == "not_applicable"
}

func validCurrentID(value string) bool {
	if !strings.HasPrefix(value, "current-") {
		return false
	}
	for _, character := range strings.TrimPrefix(value, "current-") {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != "current-" && !strings.HasPrefix(value, "current-0")
}

type RecheckVerification struct {
	ID                          string  `json:"id"`
	HistoricalDifferenceID      string  `json:"historical_difference_id"`
	HistoricalTitle             string  `json:"historical_title"`
	LineageID                   *string `json:"lineage_id,omitempty"`
	CurrentDifferenceID         *string `json:"current_difference_id,omitempty"`
	AIStatus                    string  `json:"ai_status"`
	ManualStatusOverride        *string `json:"manual_status_override,omitempty"`
	EffectiveStatus             string  `json:"effective_status"`
	Confidence                  float64 `json:"confidence"`
	Reason                      string  `json:"reason"`
	EvidenceSummary             string  `json:"evidence_summary"`
	CurrentDesignRegion         *Region `json:"current_design_region,omitempty"`
	CurrentImplementationRegion *Region `json:"current_implementation_region,omitempty"`
	CanOverride                 bool    `json:"can_override"`
}

type PendingRecheckMatch struct {
	ID                    string          `json:"id"`
	CandidateID           string          `json:"candidate_id"`
	SuggestedLineageID    string          `json:"suggested_lineage_id"`
	SuggestedLineageTitle string          `json:"suggested_lineage_title"`
	Candidate             DifferenceInput `json:"candidate"`
	DiscoverySource       string          `json:"discovery_source"`
	Confidence            float64         `json:"confidence"`
	Reason                string          `json:"reason"`
	CanResolve            bool            `json:"can_resolve"`
}
