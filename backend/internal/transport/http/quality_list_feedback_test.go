package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestQualityBugFeedbackFiltersUseActorAndStatusIndependentFacets(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	triage := fix.createBug(t, 920, "triage", false)
	fix.createBug(t, 921, "in_progress", false)
	other := fix.createBug(t, 922, "done", false)
	fix.createBug(t, 923, "review", false)
	fix.createBug(t, 924, "canceled", false)
	managerID := factUserID(fix.users["manager"])
	if err := fix.app.db.Model(&other).Updates(map[string]any{
		"reporter_user_id":  managerID,
		"severity_key":      "p1",
		"found_environment": "production",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Model(&triage).Updates(map[string]any{
		"severity_key":      "p0",
		"found_environment": "staging",
	}).Error; err != nil {
		t.Fatal(err)
	}

	token := fix.app.token(t, fix.users["reporter"])
	list := func(query string) *httptest.ResponseRecorder {
		return fix.app.request(t, http.MethodGet,
			"/api/v1/platform/quality/bugs?"+query,
			token, fix.workspaceID, nil)
	}
	mine := list("reported_by_me=1&status=triage")
	requireStatus(t, mine, http.StatusOK)
	data := decodeData(t, mine).(map[string]any)
	if _, exists := data["summary"]; exists {
		t.Fatalf("paginated list must not repeat aggregate summary: %s", mine.Body.String())
	}
	if data["total"] != float64(1) {
		t.Fatalf("reported_by_me total=%v body=%s", data["total"], mine.Body.String())
	}
	statistics := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/quality/bugs/statistics?reported_by_me=1&status=triage",
		token, fix.workspaceID, nil)
	requireStatus(t, statistics, http.StatusOK)
	statusFacets := decodeData(t, statistics).(map[string]any)["facets"].(map[string]any)["status"].(map[string]any)
	if statusFacets["triage"] != float64(1) || statusFacets["in_progress"] != float64(1) ||
		statusFacets["review"] != float64(1) || statusFacets["canceled"] != float64(1) ||
		statusFacets["closed"] != float64(1) {
		t.Fatalf("status facets must exclude active status filter: %v", statusFacets)
	}
	reviewerID := factUserID(fix.users["reviewer"])
	reviewerOwned := list(fmt.Sprintf("current_owner_user_id=%d", reviewerID))
	requireStatus(t, reviewerOwned, http.StatusOK)
	if total := decodeData(t, reviewerOwned).(map[string]any)["total"]; total != float64(3) {
		t.Fatalf("reviewer-owned terminal total=%v body=%s", total, reviewerOwned.Body.String())
	}
	assigneeID := factUserID(fix.users["assignee"])
	developerOwned := list(fmt.Sprintf("current_owner_user_id=%d", assigneeID))
	requireStatus(t, developerOwned, http.StatusOK)
	if total := decodeData(t, developerOwned).(map[string]any)["total"]; total != float64(2) {
		t.Fatalf("developer-owned active total=%v body=%s", total, developerOwned.Body.String())
	}
	multipleOwners := list(fmt.Sprintf("current_owner_user_id=%d,%d", reviewerID, assigneeID))
	requireStatus(t, multipleOwners, http.StatusOK)
	if total := decodeData(t, multipleOwners).(map[string]any)["total"]; total != float64(5) {
		t.Fatalf("multiple owners total=%v body=%s", total, multipleOwners.Body.String())
	}
	multipleReporters := list(fmt.Sprintf("reporter_user_id=%d,%d", factUserID(fix.users["reporter"]), managerID))
	requireStatus(t, multipleReporters, http.StatusOK)
	if total := decodeData(t, multipleReporters).(map[string]any)["total"]; total != float64(5) {
		t.Fatalf("multiple reporters total=%v body=%s", total, multipleReporters.Body.String())
	}
	multipleSeverityAndEnvironment := list("severity_key=p0,p1&found_environment=staging,production")
	requireStatus(t, multipleSeverityAndEnvironment, http.StatusOK)
	if total := decodeData(t, multipleSeverityAndEnvironment).(map[string]any)["total"]; total != float64(2) {
		t.Fatalf("multiple severity/environment total=%v body=%s", total, multipleSeverityAndEnvironment.Body.String())
	}
	developerTriage := list(fmt.Sprintf("current_owner_user_id=%d&status=triage", assigneeID))
	requireStatus(t, developerTriage, http.StatusOK)
	if total := decodeData(t, developerTriage).(map[string]any)["total"]; total != float64(1) {
		t.Fatalf("developer-owned triage total=%v body=%s", total, developerTriage.Body.String())
	}
	assigneeToken := fix.app.token(t, fix.users["assignee"])
	actorList := func(query string) *httptest.ResponseRecorder {
		return fix.app.request(t, http.MethodGet,
			"/api/v1/platform/quality/bugs?"+query,
			assigneeToken, fix.workspaceID, nil)
	}
	assignedToMe := actorList("assigned_to_me=1")
	requireStatus(t, assignedToMe, http.StatusOK)
	if total := decodeData(t, assignedToMe).(map[string]any)["total"]; total != float64(2) {
		t.Fatalf("assigned_to_me total=%v body=%s", total, assignedToMe.Body.String())
	}
	responsibleByMe := actorList("responsible_by_me=1")
	requireStatus(t, responsibleByMe, http.StatusOK)
	if total := decodeData(t, responsibleByMe).(map[string]any)["total"]; total != float64(5) {
		t.Fatalf("responsible_by_me total=%v body=%s", total, responsibleByMe.Body.String())
	}
	resolvedByMe := actorList("resolved_by_me=1")
	requireStatus(t, resolvedByMe, http.StatusOK)
	if total := decodeData(t, resolvedByMe).(map[string]any)["total"]; total != float64(2) {
		t.Fatalf("resolved_by_me total=%v body=%s", total, resolvedByMe.Body.String())
	}
	invalidActorFilter := actorList("resolved_by_me=yes")
	requireStatus(t, invalidActorFilter, http.StatusBadRequest)

	conflicting := list(fmt.Sprintf("reported_by_me=1&reporter_user_id=%d", managerID))
	requireStatus(t, conflicting, http.StatusOK)
	if total := decodeData(t, conflicting).(map[string]any)["total"]; total != float64(0) {
		t.Fatalf("conflicting reporter filters total=%v", total)
	}

	invalidRange := list("created_from=2026-08-13&created_to=2026-08-01")
	requireStatus(t, invalidRange, http.StatusBadRequest)
	if triage.ID == 0 {
		t.Fatal("fixture did not create triage bug")
	}
}

func TestProjectQualityOverviewAggregatesWithoutDownloadingBugRows(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	critical := fix.createBug(t, 940, "triage", false)
	normal := fix.createBug(t, 941, "in_progress", false)
	fix.createBug(t, 942, "done", false)
	if err := fix.app.db.Model(&critical).Update("severity_key", "p0").Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Model(&normal).Update("severity_key", "p2").Error; err != nil {
		t.Fatal(err)
	}
	token := fix.app.token(t, fix.users["reporter"])
	response := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/project-sets/%d/quality-overview", fix.project.ID),
		token, fix.workspaceID, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["total"] != float64(2) {
		t.Fatalf("overview total=%v body=%s", data["total"], response.Body.String())
	}
	items := data["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("overview items=%v", items)
	}
	counts := map[string]float64{}
	for _, item := range items {
		row := item.(map[string]any)
		counts[row["severity"].(string)] += row["count"].(float64)
		if row["requirement_id"] != float64(fix.requirement.ID) {
			t.Fatalf("unexpected requirement aggregate: %v", row)
		}
	}
	if counts["critical"] != 1 || counts["normal"] != 1 {
		t.Fatalf("overview counts=%v", counts)
	}
}

func TestQualityBugOrderUsesWhitelist(t *testing.T) {
	for _, test := range []struct {
		query string
		want  string
	}{
		{"sort_by=created_at&sort_order=asc", "created_at ASC"},
		{"sort_by=status_changed_at", "status_changed_at DESC"},
		{"sort_by=priority&sort_order=desc", "CASE LOWER(priority)"},
	} {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "/?"+test.query, nil)
		got, err := qualityBugOrder(context)
		if err != nil || !strings.Contains(got, test.want) {
			t.Fatalf("order(%q)=%q, err=%v", test.query, got, err)
		}
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/?sort_by=id%20desc%3Bdrop%20table", nil)
	if _, err := qualityBugOrder(context); err == nil {
		t.Fatal("unlisted sort expression must be rejected")
	}
}

func TestQualityBugStatusFacetsExposeExactAndCompatibleCounts(t *testing.T) {
	rows := []qualityBugStatusCount{
		{Status: "todo", Count: 1}, {Status: "triage", Count: 1},
		{Status: "in_progress", Count: 1}, {Status: "reopened", Count: 1},
		{Status: "review", Count: 1}, {Status: "done", Count: 1},
		{Status: "closed", Count: 1}, {Status: "canceled", Count: 1},
	}
	facets := qualityBugStatusFacetsFromCounts(rows)
	for key, want := range map[string]int64{
		"todo": 1, "triage": 1, "in_progress": 1, "reopened": 1, "review": 1,
		"done": 2, "canceled": 1, "closed": 3,
	} {
		if facets[key] != want {
			t.Fatalf("facet %s=%v want=%d; facets=%v", key, facets[key], want, facets)
		}
	}
}

func TestQualityListCurrentOwnerUsesDevelopmentAndTestingStages(t *testing.T) {
	developerID, testerID, reporterID := uint(7), uint(9), uint(11)
	for _, test := range []struct {
		status string
		want   uint
	}{
		{status: "todo", want: developerID},
		{status: "triage", want: developerID},
		{status: "in_progress", want: developerID},
		{status: "reopened", want: developerID},
		{status: "review", want: testerID},
		{status: "done", want: testerID},
		{status: "canceled", want: testerID},
	} {
		bug := qualitygorm.QualityBug{
			Status: test.status, AssigneeUserID: &developerID,
			ReviewerUserID: &testerID, ReporterUserID: &reporterID,
		}
		owner := qualityListCurrentOwner(bug)
		if owner == nil || *owner != test.want {
			t.Fatalf("status=%s owner=%v want=%d", test.status, owner, test.want)
		}
	}
}

func TestApplyQualityStatusTimestampsOnlyChangesForRealTransition(t *testing.T) {
	original := time.Date(2026, 8, 1, 1, 2, 3, 0, time.UTC)
	bug := qualitygorm.QualityBug{Status: "triage", StatusChangedAt: original}
	ordinary := map[string]any{"title": "updated"}
	applyQualityStatusTimestamps(&bug, ordinary, 1)
	if _, exists := ordinary["status_changed_at"]; exists {
		t.Fatal("ordinary edit refreshed status_changed_at")
	}
	same := map[string]any{"status": "triage"}
	applyQualityStatusTimestamps(&bug, same, 1)
	if _, exists := same["status_changed_at"]; exists {
		t.Fatal("same status refreshed status_changed_at")
	}
	changed := map[string]any{"status": "in_progress"}
	applyQualityStatusTimestamps(&bug, changed, 1)
	if _, exists := changed["status_changed_at"].(time.Time); !exists {
		t.Fatal("real transition did not set status_changed_at")
	}
}

func TestQualityStatusCatalogRejectsRetiredDuplicateState(t *testing.T) {
	if validQualityStatus("duplicate") {
		t.Fatal("duplicate must not remain a writable quality status")
	}
	if !validQualityStatus("review") || !validQualityResolutionReason("duplicate") {
		t.Fatal("review must remain writable and duplicate must remain a resolution reason")
	}
}
