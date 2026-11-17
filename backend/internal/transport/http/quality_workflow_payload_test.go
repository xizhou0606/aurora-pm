package httpapi

import (
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"net/http"
	"strings"
	"testing"
)

func TestQualityBugAssignmentActivityIncludesCCAliasesAndNote(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 910, "triage", false)
	assigneeID := factUserID(fix.users["quality_owner"])
	ccID := factUserID(fix.users["reporter"])
	response := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"defect_owner_user_id": assigneeID,
		"copy_user_ids":        []uint{ccID, assigneeID, ccID},
		"remark":               "请优先处理",
		"assignment_materials": []map[string]any{
			{"url": "https://objects.example.test/assignment.pdf"},
		},
	})
	requireStatus(t, response, http.StatusOK)

	activity := latestQualityActivity(t, fix, bug.ID)
	if activity.Action != "assigned" || activity.FieldKey != "assignee_user_id" ||
		!strings.Contains(activity.Summary, "已更新负责人") ||
		!strings.Contains(activity.Summary, "已更新抄送人") ||
		strings.Contains(activity.Summary, "用户#"+uintString(ccID)) ||
		strings.Contains(activity.Summary, "用户#"+uintString(assigneeID)) ||
		!strings.Contains(activity.Summary, "附件：1个") ||
		!strings.Contains(activity.Summary, "材料：https://objects.example.test/assignment.pdf") ||
		!strings.Contains(activity.Summary, "备注：请优先处理") {
		t.Fatalf("assignment activity=%+v", activity)
	}
}

func TestQualityBugRegressionReturnRecordsMaterialsAndGenericNote(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 911, "review", false)
	response := fix.request(t, "reviewer", http.MethodPatch, bug.ID, map[string]any{
		"status": "reopened",
		"note":   "首页仍然报错",
		"regression_materials": []map[string]any{
			{"url": "https://objects.example.test/regression.png"},
		},
	})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["status"] != "reopened" ||
		!containsString(actionStrings(data["allowed_actions"]), "submit_review") {
		t.Fatalf("reopened response=%v", data)
	}

	activity := latestQualityActivity(t, fix, bug.ID)
	if activity.Action != "returned" || activity.FieldKey != "status" ||
		activity.OldValue != "review" || activity.NewValue != "reopened" ||
		!strings.Contains(activity.Summary, "附件：1个") ||
		!strings.Contains(activity.Summary, "材料：https://objects.example.test/regression.png") ||
		!strings.Contains(activity.Summary, "备注：首页仍然报错") {
		t.Fatalf("regression activity=%+v", activity)
	}
}

func TestQualityBugReopenRecordsRichTextAndMaterials(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 913, "review", false)
	note := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"复核后仍可复现"}]},{"type":"image","attrs":{"src":"https://objects.example.test/reopen.png"}}]}`
	response := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"status":          "reopened",
		"regression_note": note,
		"regression_materials": []map[string]any{
			{"url": "https://objects.example.test/reopen-log.txt"},
		},
	})
	requireStatus(t, response, http.StatusOK)

	activity := latestQualityActivity(t, fix, bug.ID)
	if activity.Action != "returned" || activity.FieldKey != "status" ||
		activity.OldValue != "review" || activity.NewValue != "reopened" ||
		!strings.Contains(activity.Summary, "附件：1个") ||
		!strings.Contains(activity.Summary, "材料：https://objects.example.test/reopen-log.txt") ||
		!strings.Contains(activity.Summary, "备注："+note) {
		t.Fatalf("review reopen activity=%+v", activity)
	}
}

func TestQualityBugCloseRecordsRichTextAndMaterialsOnce(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 914, "review", false)
	note := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"关闭前复核通过"}]},{"type":"image","attrs":{"src":"https://objects.example.test/close.png"}}]}`
	response := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"status":            "done",
		"resolution_reason": "fixed",
		"resolution_note":   note,
		"resolution_materials": []map[string]any{
			{"url": "https://objects.example.test/close-log.txt"},
		},
	})
	requireStatus(t, response, http.StatusOK)

	activity := latestQualityActivity(t, fix, bug.ID)
	if activity.Action != "verified" || activity.FieldKey != "status" ||
		activity.OldValue != "review" || activity.NewValue != "done" ||
		!strings.Contains(activity.Summary, "附件：1个") ||
		strings.Count(activity.Summary, "https://objects.example.test/close-log.txt") != 1 ||
		!strings.Contains(activity.Summary, "备注："+note) ||
		strings.Contains(activity.Summary, "备注：（空） →") {
		t.Fatalf("review close activity=%+v", activity)
	}
}

func TestQualityBugWorkflowPayloadRejectsUnsafeValues(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 912, "review", false)

	badURL := fix.request(t, "reviewer", http.MethodPatch, bug.ID, map[string]any{
		"status": "reopened",
		"regression_materials": []map[string]any{
			{"url": "javascript:alert(1)"},
		},
	})
	requireStatus(t, badURL, http.StatusBadRequest)

	badNote := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"assignment_note": strings.Repeat("缺", qualityBugNoteMaxRunes+1),
	})
	requireStatus(t, badNote, http.StatusUnprocessableEntity)
	if !strings.Contains(badNote.Body.String(), `"code":"`+apiErrorQualityAssignmentNoteTooLong+`"`) {
		t.Fatalf("assignment note error=%s", badNote.Body.String())
	}
}

func TestQualityBugReopenCountsVisibleRichTextCharacters(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 915, "review", false)
	withinLimit := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` +
		strings.Repeat("复", qualityBugNoteMaxRunes) + `"}]}]}`
	if len(withinLimit) <= qualityBugNoteMaxRunes {
		t.Fatalf("fixture must exceed the serialized limit: %d", len(withinLimit))
	}

	accepted := fix.request(t, "reviewer", http.MethodPatch, bug.ID, map[string]any{
		"status": "reopened", "regression_note": withinLimit,
	})
	requireStatus(t, accepted, http.StatusOK)

	tooLong := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` +
		strings.Repeat("复", qualityBugNoteMaxRunes+1) + `"}]}]}`
	rejected := fix.request(t, "reviewer", http.MethodPatch, bug.ID, map[string]any{
		"status": "reopened", "regression_note": tooLong,
	})
	requireStatus(t, rejected, http.StatusUnprocessableEntity)
	if !strings.Contains(rejected.Body.String(), `"code":"`+apiErrorQualityRegressionNoteTooLong+`"`) ||
		!strings.Contains(rejected.Body.String(), "5000 visible characters") {
		t.Fatalf("regression note error=%s", rejected.Body.String())
	}
}

func latestQualityActivity(
	t *testing.T,
	fix qualityPermissionFixture,
	bugID uint,
) qualitygorm.QualityBugActivity {
	t.Helper()
	var activity qualitygorm.QualityBugActivity
	if err := fix.app.db.Where(
		"workspace_id = ? AND quality_bug_id = ?", fix.workspaceID, bugID,
	).Order("id DESC").First(&activity).Error; err != nil {
		t.Fatal(err)
	}
	return activity
}
