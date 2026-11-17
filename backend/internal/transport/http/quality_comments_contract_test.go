package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestQualityBugCommentsContractAndActivity(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 980, "triage", false)
	reporterToken := fix.app.token(t, fix.users["reporter"])

	created := fix.app.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID),
		reporterToken, fix.workspaceID, map[string]any{"body": "请复核这个问题"})
	requireStatus(t, created, http.StatusCreated)
	row := decodeData(t, created).(map[string]any)
	commentID := uint(row["id"].(float64))
	if row["quality_bug_id"] != float64(bug.ID) || row["body"] != "请复核这个问题" ||
		row["display_name"] != fix.users["reporter"].DisplayName {
		t.Fatalf("created quality comment=%v", row)
	}

	listed := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID),
		reporterToken, fix.workspaceID, nil)
	requireStatus(t, listed, http.StatusOK)
	comments := decodeData(t, listed).([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["id"] != float64(commentID) {
		t.Fatalf("quality comment list=%s", listed.Body.String())
	}

	var activity qualitygorm.QualityBugActivity
	if err := fix.app.db.Where(
		"workspace_id = ? AND quality_bug_id = ? AND action = ?",
		fix.workspaceID, bug.ID, "comment",
	).First(&activity).Error; err != nil {
		t.Fatal(err)
	}
	if activity.Summary != "请复核这个问题" {
		t.Fatalf("comment activity summary=%q", activity.Summary)
	}

	tooLong := fix.app.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID),
		reporterToken, fix.workspaceID, map[string]any{"body": strings.Repeat("猫", qualityCommentBodyLimit+1)})
	requireStatus(t, tooLong, http.StatusBadRequest)

	deleted := fix.app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments/%d", bug.ID, commentID),
		fix.app.token(t, fix.users["tester"]), fix.workspaceID, nil)
	requireStatus(t, deleted, http.StatusNoContent)
	var commentCount, activityCount int64
	fix.app.db.Model(&qualitygorm.QualityBugComment{}).Where("id = ?", commentID).Count(&commentCount)
	fix.app.db.Model(&qualitygorm.QualityBugActivity{}).
		Where("quality_bug_id = ? AND action = ?", bug.ID, "comment").Count(&activityCount)
	if commentCount != 0 || activityCount != 0 {
		t.Fatalf("deleted comment left comment=%d activity=%d", commentCount, activityCount)
	}
}

func TestQualityBugCommentsRespectWorkspaceIsolation(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 981, "triage", false)
	response := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID),
		fix.app.token(t, fix.users["manager"]), 999, nil)
	if response.Code == http.StatusOK {
		t.Fatalf("foreign workspace unexpectedly read comments: %s", response.Body.String())
	}
}

func TestQualityBugCommentsPreserveRichTextDocument(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 982, "triage", false)
	reporterToken := fix.app.token(t, fix.users["reporter"])
	body := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"47","label":"铁奥","username":"teo"}},{"type":"text","text":" 请看"}]},{"type":"image","attrs":{"src":"https://cdn.example.test/proof.png","alt":"proof.png"}}]}`

	created := fix.app.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/comments", bug.ID),
		reporterToken, fix.workspaceID, map[string]any{"body": body})
	requireStatus(t, created, http.StatusCreated)
	row := decodeData(t, created).(map[string]any)
	if row["body"] != body {
		t.Fatalf("rich comment body was rewritten=%v", row["body"])
	}

	var activity qualitygorm.QualityBugActivity
	if err := fix.app.db.Where(
		"workspace_id = ? AND quality_bug_id = ? AND action = ?",
		fix.workspaceID, bug.ID, "comment",
	).First(&activity).Error; err != nil {
		t.Fatal(err)
	}
	if activity.Summary != "@铁奥 请看 [图片]" {
		t.Fatalf("rich comment activity summary=%q", activity.Summary)
	}
}
