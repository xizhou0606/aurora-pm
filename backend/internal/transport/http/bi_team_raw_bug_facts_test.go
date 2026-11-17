package httpapi

import (
	"net/http"
	"testing"
	"time"

	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestBITeamBugResponseRequiresReliableFinalAssignmentFact(t *testing.T) {
	ownerID := uint(7)
	created := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	resolved := created.Add(4 * time.Hour)
	bug := quality.BugSnapshot{
		ID: 1, AssigneeUserID: &ownerID, CreatedAt: created, ResolvedAt: &resolved,
	}
	if facts := biTeamBugResponseDurations([]quality.BugSnapshot{bug}, nil); len(facts) != 0 {
		t.Fatalf("response without assignment fact must be excluded: %+v", facts)
	}
	activities := []quality.BugActivitySnapshot{
		{ID: 1, QualityBugID: bug.ID, FieldKey: "assignee_user_id", NewValue: "7", CreatedAt: created},
		{ID: 2, QualityBugID: bug.ID, FieldKey: "status", NewValue: "in_progress", CreatedAt: created.Add(2 * time.Hour)},
	}
	fact, found := biTeamBugResponseDurations([]quality.BugSnapshot{bug}, activities)[bug.ID]
	if !found || fact.hours != 2 || !fact.respondedAt.Equal(created.Add(2*time.Hour)) {
		t.Fatalf("response fact=%+v found=%v", fact, found)
	}
}

func TestBITeamBugResolutionDecisionDoesNotUseUnrelatedUpdatedAt(t *testing.T) {
	resolved := time.Date(2026, time.July, 31, 23, 0, 0, 0, time.UTC)
	updated := resolved.Add(48 * time.Hour)
	withoutDecision := quality.BugSnapshot{ResolutionReason: "as_designed", UpdatedAt: updated}
	if decision := biTeamBugResolutionDecisionAt(withoutDecision, nil); decision != nil {
		t.Fatalf("updated_at is not a resolution decision: %v", decision)
	}

	bug := quality.BugSnapshot{ResolutionReason: "as_designed", ResolvedAt: &resolved, UpdatedAt: updated}
	ambiguous := []quality.BugActivitySnapshot{{
		ID: 1, FieldKey: "multiple_fields", CreatedAt: resolved.Add(time.Hour),
	}}
	if decision := biTeamBugResolutionDecisionAt(bug, ambiguous); decision != nil {
		t.Fatalf("ambiguous post-resolution edit must exclude decision month: %v", decision)
	}
	explicitAt := resolved.Add(2 * time.Hour)
	explicit := append(ambiguous, quality.BugActivitySnapshot{
		ID: 2, FieldKey: "resolution_reason", NewValue: "as_designed", CreatedAt: explicitAt,
	})
	decision := biTeamBugResolutionDecisionAt(bug, explicit)
	if decision == nil || !decision.Equal(explicitAt) {
		t.Fatalf("explicit resolution decision=%v", decision)
	}
}

func TestQualityStatusAndAssigneePatchWritesIndependentAssignmentFact(t *testing.T) {
	app := newTestApp(t, nil)
	ownerID := uint(7)
	before := qualitygorm.QualityBug{ID: 91, WorkspaceID: 1, Status: "triage"}
	after := before
	after.Status = "in_progress"
	after.AssigneeUserID = &ownerID
	if _, err := recordQualityPatchActivity(app.db, before, after, 3, qualityWorkflowPayload{
		AssignmentTargetChange: true,
	}); err != nil {
		t.Fatal(err)
	}
	var rows []qualitygorm.QualityBugActivity
	if err := app.db.Where("quality_bug_id = ?", before.ID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].FieldKey != "assignee_user_id" || rows[0].NewValue != "7" ||
		rows[1].FieldKey != "status" || rows[1].NewValue != "in_progress" {
		t.Fatalf("activities=%+v", rows)
	}
	fact, found := biTeamBugResponseDurations([]quality.BugSnapshot{{
		ID: before.ID, AssigneeUserID: &ownerID, CreatedAt: rows[0].CreatedAt.Add(-time.Hour),
	}}, legacyBIBugActivitySnapshots(rows))[before.ID]
	if !found || fact.hours != 0 {
		t.Fatalf("same-operation assignment response fact=%+v found=%v", fact, found)
	}
}

func TestQualityCreatePreservesCreatedActivityAndAddsAssignmentFact(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	assigneeID := factUserID(fix.users["assignee"])
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs",
		fix.app.token(t, fix.users["reporter"]), fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement.ID, "title": "可度量创建指派",
			"assignee_user_id": assigneeID,
		})
	requireStatus(t, response, http.StatusCreated)
	bugID := uint(decodeData(t, response).(map[string]any)["id"].(float64))
	var rows []qualitygorm.QualityBugActivity
	if err := fix.app.db.Where("quality_bug_id = ?", bugID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Action != "created" || rows[0].FieldKey != "" ||
		rows[1].Action != "assigned" || rows[1].FieldKey != "assignee_user_id" || rows[1].NewValue != uintString(assigneeID) {
		t.Fatalf("create activities=%+v", rows)
	}
}
