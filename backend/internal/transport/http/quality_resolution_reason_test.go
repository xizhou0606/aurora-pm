package httpapi

import (
	"net/http"
	"testing"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestQualityLegacyResolutionReasonExposesCompletionWithoutRewriting(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 6675, "review", false)
	if err := fix.app.db.Model(&bug).Update("resolution_reason", "已修复").Error; err != nil {
		t.Fatal(err)
	}
	response := fix.request(t, "reviewer", http.MethodGet, bug.ID, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["resolution_reason"] != "fixed" {
		t.Fatalf("read reason=%v", data["resolution_reason"])
	}
	complete := false
	for _, action := range actionStrings(data["allowed_actions"]) {
		complete = complete || action == "complete"
	}
	if !complete {
		t.Fatalf("legacy reason hid completion: %v", data["allowed_actions"])
	}
	var stored qualitygorm.QualityBug
	if err := fix.app.db.First(&stored, bug.ID).Error; err != nil || stored.ResolutionReason != "已修复" || stored.Status != "review" {
		t.Fatalf("read changed stored row: reason=%q status=%q err=%v", stored.ResolutionReason, stored.Status, err)
	}
	response = fix.request(t, "reviewer", http.MethodPatch, bug.ID, map[string]any{"status": "done"})
	requireStatus(t, response, http.StatusOK)
	if err := fix.app.db.First(&stored, bug.ID).Error; err != nil || stored.Status != "done" || stored.ResolutionReason != "已修复" {
		t.Fatalf("legacy completion: reason=%q status=%q err=%v", stored.ResolutionReason, stored.Status, err)
	}
}

func TestQualityHTTPRejectsNewDisplayLabelResolutionReasons(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 6676, "in_progress", false)
	for _, reason := range []string{"已修复", "已解决", "invalid_reason"} {
		response := fix.request(t, "assignee", http.MethodPatch, bug.ID, map[string]any{
			"status": "review", "resolution_reason": reason,
		})
		requireStatus(t, response, http.StatusBadRequest)
	}
	response := fix.request(t, "assignee", http.MethodPatch, bug.ID, map[string]any{
		"status": "review", "resolution_reason": "fixed",
	})
	requireStatus(t, response, http.StatusOK)
}
