package httpapi

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

// qualityAllowedActions describes valid workflow transitions only. Entry-level
// interface authorization is decided before the request reaches this handler.
func qualityAllowedActions(bug qualitygorm.QualityBug) []string {
	actions := make([]string, 0, 7)
	status := normalizeQualityBugStatus(bug.Status)
	if status == "triage" || status == "todo" {
		actions = append(actions, "start", "submit_review")
	} else if status == "in_progress" || status == "reopened" {
		actions = append(actions, "submit_review")
	}
	if status == "review" {
		reason := quality.NormalizeBugResolutionReason(bug.ResolutionReason)
		if reason != "" && quality.ValidBugResolutionReason(reason) {
			actions = append(actions, "complete")
		}
		actions = append(actions, "return")
	}
	if status != "triage" {
		actions = append(actions, "reset_triage")
	}
	return actions
}

func qualityPatchHasChanges(updates map[string]any, hasActivityNote bool) bool {
	return len(updates) > 0 || hasActivityNote
}
