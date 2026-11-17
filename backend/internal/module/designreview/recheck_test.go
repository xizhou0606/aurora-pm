package designreview

import "testing"

const (
	historyID = "10000000-0000-4000-8000-000000000001"
	lineageID = "20000000-0000-4000-8000-000000000002"
	reviewID  = "30000000-0000-4000-8000-000000000003"
	rootID    = "40000000-0000-4000-8000-000000000004"
)

func TestRecheckAnalysisValidatesCompleteOneToOneLedger(t *testing.T) {
	snapshot, analysis := validRecheckFixture()
	if err := analysis.Validate(snapshot); err != nil {
		t.Fatalf("valid recheck rejected: %v", err)
	}
}

func TestRecheckAnalysisDowngradesLowConfidenceClosure(t *testing.T) {
	snapshot, analysis := validRecheckFixture()
	analysis.HistoricalVerifications[0].Status = "resolved"
	analysis.HistoricalVerifications[0].CurrentDifferenceCandidateID = nil
	analysis.HistoricalVerifications[0].Confidence = 0.89
	if err := analysis.Validate(snapshot); err != nil {
		t.Fatal(err)
	}
	if analysis.HistoricalVerifications[0].Status != "uncertain" {
		t.Fatalf("status=%s, want uncertain", analysis.HistoricalVerifications[0].Status)
	}
}

func TestRecheckAnalysisRejectsMissingHistoryAndDuplicateLineage(t *testing.T) {
	snapshot, analysis := validRecheckFixture()
	analysis.HistoricalVerifications = nil
	if err := analysis.Validate(snapshot); err == nil {
		t.Fatal("missing historical verification was accepted")
	}

	_, analysis = validRecheckFixture()
	second := analysis.CurrentDifferences[0]
	second.CandidateID = "current-2"
	analysis.CurrentDifferences = append(analysis.CurrentDifferences, second)
	match := analysis.LineageMatches[0]
	match.CandidateID = "current-2"
	analysis.LineageMatches = append(analysis.LineageMatches, match)
	if err := analysis.Validate(snapshot); err == nil {
		t.Fatal("duplicate lineage suggestion was accepted")
	}
}

func TestRecheckAnalysisRejectsHistoryMappedToAnotherLineage(t *testing.T) {
	snapshot, analysis := validRecheckFixture()
	other := "50000000-0000-4000-8000-000000000005"
	snapshot.Lineages = append(snapshot.Lineages, RecheckLineage{LineageID: other, LatestDifferenceID: "60000000-0000-4000-8000-000000000006", LineageRootDifferenceID: "70000000-0000-4000-8000-000000000007", Category: "layout"})
	analysis.LineageMatches[0].LineageID = &other
	if err := analysis.Validate(snapshot); err == nil {
		t.Fatal("history mapped to a different lineage was accepted")
	}
}

func validRecheckFixture() (RecheckSnapshot, RecheckAnalysis) {
	region := &Region{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.2}
	snapshot := RecheckSnapshot{Version: "1.0", SourceReviewID: reviewID, ChainRootReviewID: rootID,
		Items:    []RecheckHistoryItem{{HistoricalDifferenceID: historyID, LatestDifferenceID: historyID, LineageRootDifferenceID: historyID}},
		Lineages: []RecheckLineage{{LineageID: lineageID, LatestDifferenceID: historyID, LineageRootDifferenceID: historyID, Category: "layout", Title: "间距错误"}}}
	candidateID := "current-1"
	analysis := RecheckAnalysis{Summary: "复查完成", CoverageComplete: true,
		CurrentDifferences:      []RecheckCandidate{{CandidateID: candidateID, DiscoverySource: "full_page", Category: "layout", Severity: "major", Title: "间距错误", Description: "间距不一致", Recommendation: "调整间距", Confidence: 0.98, DesignRegion: region, ImplementationRegion: region, EvidenceNotes: "双侧可见"}},
		HistoricalVerifications: []HistoricalVerificationResult{{HistoricalDifferenceID: historyID, Status: "still_present", CurrentDifferenceCandidateID: &candidateID, Confidence: 0.95, Reason: "同一问题仍存在", EvidenceSummary: "位置和根因一致", CurrentDesignRegion: region, CurrentImplementationRegion: region}},
		LineageMatches:          []LineageMatchSuggestion{{CandidateID: candidateID, LineageID: stringPointer(lineageID), Confidence: 0.96, Reason: "标题和区域一致"}}}
	return snapshot, analysis
}

func stringPointer(value string) *string { return &value }
