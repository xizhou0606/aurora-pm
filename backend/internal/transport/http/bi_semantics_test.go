package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func TestBIEndpointKeepsBothTrendsAtFourteenDays(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "bi-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	for _, period := range []string{"week", "month"} {
		response := app.request(t, http.MethodGet,
			"/api/v1/platform/executive-dashboard/bi-screen?period="+period, token, 1, nil)
		requireStatus(t, response, http.StatusOK)
		data := decodeData(t, response).(map[string]any)
		if got := len(data["completion_trend"].([]any)); got != 14 {
			t.Fatalf("period=%s completion trend points=%d want=14", period, got)
		}
		if got := len(data["quality_trend"].([]any)); got != 14 {
			t.Fatalf("period=%s quality trend points=%d want=14", period, got)
		}
	}
}

func TestBIDueDistributionUsesUnfinishedTaskDenominator(t *testing.T) {
	now := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	overdue := now.Add(-time.Hour)
	agg := biAggregation{Now: now, Tasks: []task.Snapshot{
		{Status: "in_dev", DueAt: &overdue},
		{Status: "draft"},
		{Status: "completed"},
	}}

	rows := agg.dueDistribution()
	if got := biDistributionTestRow(rows, "overdue")["percent"]; got != float64(50) {
		t.Fatalf("overdue percent=%v want 50", got)
	}
	if got := biDistributionTestRow(rows, "unset")["percent"]; got != float64(50) {
		t.Fatalf("unset percent=%v want 50", got)
	}
}

func TestBIOverviewCountsBlockedTaskStatus(t *testing.T) {
	agg := biAggregation{Tasks: []task.Snapshot{
		{Status: "in_dev", Priority: "urgent"},
		{Status: "blocked", Priority: "normal"},
	}}

	metrics := agg.overviewMetrics()
	if got := metrics["blocked_tasks"]; got != 1 {
		t.Fatalf("blocked_tasks=%v want 1", got)
	}
}

func TestBIOverviewRoundsCompletionRateLikeSource(t *testing.T) {
	agg := biAggregation{Tasks: []task.Snapshot{
		{Status: "completed"},
		{Status: "draft"},
		{Status: "draft"},
	}}

	metrics := agg.overviewMetrics()
	if got := metrics["completion_rate_pct"]; got != 33 {
		t.Fatalf("completion_rate_pct=%v want 33", got)
	}
	if got := metrics["milestone_progress_pct"]; got != 33 {
		t.Fatalf("milestone_progress_pct=%v want 33", got)
	}
}

func TestBIOwnerBoardsExposeDirectoryAvatar(t *testing.T) {
	ownerID := uint(7)
	agg := biAggregation{
		Tasks: []task.Snapshot{{AssigneeID: &ownerID}},
		Bugs:  []quality.BugSnapshot{{Status: "triage", AssigneeUserID: &ownerID}},
		Ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			ownerID: {ID: ownerID, DisplayName: "Owner", AvatarURL: "/avatars/owner.png"},
		}},
	}

	if got := agg.ownerBoard()[0]["owner_avatar_url"]; got != "/avatars/owner.png" {
		t.Fatalf("owner board avatar=%v want directory avatar", got)
	}
	if got := agg.qualityOwnerLoad()[0]["owner_avatar_url"]; got != "/avatars/owner.png" {
		t.Fatalf("quality owner avatar=%v want directory avatar", got)
	}
}

func TestBIQualityLegacySeverityMatchesSourceSemantics(t *testing.T) {
	ownerID := uint(7)
	bugs := []quality.BugSnapshot{
		{PlanUnitID: 12, Status: "triage", Priority: "normal", SeverityKey: "p0", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "triage", Priority: "normal", SeverityKey: "p1", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "triage", Priority: "normal", SeverityKey: " FATAL ", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "triage", Priority: "normal", SeverityKey: "Critical", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "triage", Priority: "normal", SeverityKey: "blocker", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "triage", Priority: "urgent", SeverityKey: "p3", AssigneeUserID: &ownerID},
		{PlanUnitID: 12, Status: "closed", Priority: "urgent", SeverityKey: "p0", AssigneeUserID: &ownerID},
	}
	agg := biAggregation{
		Bugs: bugs,
		Ctx: biWorkspaceContext{
			Projects: map[uint]project.PlanUnitSnapshot{12: {ID: 12, Title: "Version A"}},
			Users:    map[uint]identity.BIUser{ownerID: {ID: ownerID, DisplayName: "Owner"}},
		},
	}

	metrics := agg.overviewMetrics()
	if metrics["open_bugs"] != 6 || metrics["high_severity_bugs"] != 4 || metrics["blocker_bugs"] != 5 {
		t.Fatalf("quality overview=%v, want open=6 high=4 blocker=5", metrics)
	}

	risks := agg.versionQualityRisks()
	if len(risks) != 1 || risks[0]["open_bugs"] != 6 || risks[0]["high_severity_bugs"] != 4 {
		t.Fatalf("version quality risks=%v, want one row open=6 high=4", risks)
	}
	owners := agg.qualityOwnerLoad()
	if len(owners) != 1 || owners[0]["open_bugs"] != 6 || owners[0]["high_severity_bugs"] != 4 {
		t.Fatalf("quality owner load=%v, want one row open=6 high=4", owners)
	}

	for _, test := range []struct {
		value string
		want  bool
	}{
		{"p0", true}, {"p1", true}, {" fatal ", true}, {"CRITICAL", true},
		{"blocker", false}, {"p2", false}, {"", false},
	} {
		if got := isHighSeverity(test.value); got != test.want {
			t.Errorf("isHighSeverity(%q)=%v want=%v", test.value, got, test.want)
		}
	}
	for _, status := range []string{"done", "closed", "completed", "canceled", "cancelled"} {
		if isBugOpen(status) {
			t.Errorf("terminal status %q was treated as open", status)
		}
	}
}

func TestBIQualityTrendKeepsLegacyFatalOnlyUntilTerminalDay(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	created := time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)
	completed := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	agg := biAggregation{
		Now: now, Location: time.UTC, Days: 2,
		Bugs: []quality.BugSnapshot{
			{Status: "completed", SeverityKey: "fatal", CreatedAt: created, CompletedAt: &completed},
			{Status: "triage", SeverityKey: "blocker", CreatedAt: created},
		},
	}

	trend := agg.qualityTrend()
	if trend[0]["open_bugs"] != 2 || trend[0]["high_severity_bugs"] != 1 {
		t.Fatalf("pre-terminal trend=%v, want open=2 high=1", trend[0])
	}
	if trend[1]["open_bugs"] != 1 || trend[1]["high_severity_bugs"] != 0 {
		t.Fatalf("post-terminal trend=%v, want open=1 high=0", trend[1])
	}
}

func TestBIQualityLegacySeverityIsWorkspaceIsolated(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "bi-isolation-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	rows := []qualitygorm.QualityBug{
		{ID: 501, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-FATAL", Title: "Workspace A fatal", Status: "triage", Priority: "normal",
			SeverityKey: "fatal", AssigneeUserID: &admin.ID},
		{ID: 502, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-BLOCKER", Title: "Workspace A legacy blocker", Status: "triage", Priority: "normal",
			SeverityKey: "blocker", AssigneeUserID: &admin.ID},
		{ID: 503, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "A-CLOSED", Title: "Workspace A closed", Status: "completed", Priority: "urgent",
			SeverityKey: "p0", AssigneeUserID: &admin.ID},
		{ID: 601, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22, RequirementID: 23,
			BugKey: "B-SECRET", Title: "Workspace B secret", Status: "triage", Priority: "urgent",
			SeverityKey: "p0", AssigneeUserID: &admin.ID},
	}
	for i := range rows {
		if err := app.db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodGet, "/api/v1/platform/executive-dashboard/bi-screen", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	metrics := data["overview_metrics"].(map[string]any)
	if metrics["open_bugs"] != float64(2) || metrics["high_severity_bugs"] != float64(1) {
		t.Fatalf("workspace-scoped quality overview=%v, want open=2 high=1", metrics)
	}
	if _, exists := metrics["story_points_total"]; exists {
		t.Fatalf("overview still exposes retired story points: %v", metrics)
	}
	risks := data["version_quality_risks"].([]any)
	if len(risks) != 1 {
		t.Fatalf("workspace-scoped version risks=%v, want one row", risks)
	}
	risk := risks[0].(map[string]any)
	if risk["version_id"] != float64(12) || risk["open_bugs"] != float64(2) || risk["high_severity_bugs"] != float64(1) {
		t.Fatalf("workspace-scoped version risk=%v", risk)
	}
	owners := data["quality_owner_load"].([]any)
	if len(owners) != 1 || owners[0].(map[string]any)["open_bugs"] != float64(2) ||
		owners[0].(map[string]any)["high_severity_bugs"] != float64(1) {
		t.Fatalf("workspace-scoped owner load=%v", owners)
	}
	severity := data["quality_severity_distribution"].([]any)
	if len(severity) != 4 || biJSONDistributionCount(severity, "fatal") != 1 ||
		biJSONDistributionCount(severity, "minor") != 1 || biJSONDistributionTotal(severity) != 2 {
		t.Fatalf("workspace-scoped severity distribution=%v", severity)
	}
	trend := data["quality_trend"].([]any)
	latest := trend[len(trend)-1].(map[string]any)
	if latest["open_bugs"] != float64(2) || latest["high_severity_bugs"] != float64(1) {
		t.Fatalf("workspace-scoped latest quality trend=%v", latest)
	}
}

func TestBITrendsUseBusinessTimezoneAndQualityEndOfDayStock(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 3, 16, 30, 0, 0, time.UTC)
	createdPreviousDay := time.Date(2026, time.August, 3, 15, 30, 0, 0, time.UTC)
	completedToday := time.Date(2026, time.August, 3, 16, 10, 0, 0, time.UTC)
	createdToday := time.Date(2026, time.August, 3, 16, 20, 0, 0, time.UTC)
	closedPreviousDay := time.Date(2026, time.August, 3, 15, 40, 0, 0, time.UTC)

	agg := biAggregation{
		Now: now, Location: location, Days: 2,
		Tasks: []task.Snapshot{
			{Status: "completed", CreatedAt: createdPreviousDay, CompletedAt: &completedToday},
			{Status: "in_dev", CreatedAt: createdToday},
		},
		Bugs: []quality.BugSnapshot{
			{Status: "in_progress", SeverityKey: "p1", CreatedAt: time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)},
			{Status: "done", SeverityKey: "p2", CreatedAt: time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC), CompletedAt: &closedPreviousDay, ResolvedAt: &closedPreviousDay},
			{Status: "triage", SeverityKey: "p3", CreatedAt: createdToday},
		},
	}

	completion := agg.completionTrend()
	if completion[0]["date"] != "2026-08-03" || completion[0]["created"] != 1 {
		t.Fatalf("previous business day completion point=%+v", completion[0])
	}
	if completion[1]["date"] != "2026-08-04" || completion[1]["created"] != 1 || completion[1]["completed"] != 1 {
		t.Fatalf("current business day completion point=%+v", completion[1])
	}

	quality := agg.qualityTrend()
	if quality[0]["date"] != "2026-08-03" || quality[0]["open_bugs"] != 1 || quality[0]["resolved_bugs"] != 1 {
		t.Fatalf("previous business day quality point=%+v", quality[0])
	}
	if quality[1]["date"] != "2026-08-04" || quality[1]["open_bugs"] != 2 ||
		quality[1]["created_bugs"] != 1 || quality[1]["high_severity_bugs"] != 1 {
		t.Fatalf("current business day quality point=%+v", quality[1])
	}
}

func TestBILocationReadsTimezoneHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("GET", "/", nil)
	context.Request.Header.Set("X-Timezone", "Asia/Shanghai")
	if got := biLocation(context).String(); got != "Asia/Shanghai" {
		t.Fatalf("location=%q want Asia/Shanghai", got)
	}
	context.Request.Header.Set("X-Timezone", "invalid/timezone")
	if got := biLocation(context); got != time.UTC {
		t.Fatalf("invalid timezone location=%v want UTC", got)
	}
}

func biDistributionTestRow(rows []map[string]any, key string) map[string]any {
	for _, row := range rows {
		if row["key"] == key {
			return row
		}
	}
	return nil
}

func biJSONDistributionCount(rows []any, key string) float64 {
	for _, item := range rows {
		row := item.(map[string]any)
		if row["key"] == key {
			return row["count"].(float64)
		}
	}
	return 0
}

func biJSONDistributionTotal(rows []any) float64 {
	total := float64(0)
	for _, item := range rows {
		total += item.(map[string]any)["count"].(float64)
	}
	return total
}
