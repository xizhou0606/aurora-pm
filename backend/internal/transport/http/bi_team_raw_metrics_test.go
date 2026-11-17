package httpapi

import (
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func TestBITeamRawMetricsUseTheirConfirmedEventMonths(t *testing.T) {
	productID, testerID, developerID := uint(1), uint(2), uint(3)
	july := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	august := time.Date(2026, time.August, 10, 8, 0, 0, 0, time.UTC)
	september := time.Date(2026, time.September, 10, 8, 0, 0, 0, time.UTC)
	duePast := august.Add(-24 * time.Hour)
	dueFuture := august.Add(10 * 24 * time.Hour)
	completedLate := duePast.Add(time.Hour)
	resolved := july.Add(24 * time.Hour)
	closed := august.Add(2 * time.Hour)
	canceled := august.Add(time.Hour)

	agg := biTeamAggregation{
		dimension: biTeamDimensionMonth, dimensionKey: "2026-08", location: time.UTC,
		now: august.Add(5 * 24 * time.Hour), scoreProfile: defaultBITeamScoreProfile(),
		ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			productID:   {ID: productID, DisplayName: "Product"},
			testerID:    {ID: testerID, DisplayName: "Tester"},
			developerID: {ID: developerID, DisplayName: "Developer"},
		}},
		rawRequirements: []requirement.Snapshot{
			{ID: 1, PlanUnitID: biTestUintPointer(10), ProductOwnerUserIDs: []uint{productID}, CreatedAt: july},
			{ID: 2, PlanUnitID: biTestUintPointer(10), ProductOwnerUserIDs: []uint{productID}, CreatedAt: august},
			{ID: 3, PlanUnitID: biTestUintPointer(10), ProductOwnerUserIDs: []uint{productID}, CreatedAt: july},
		},
		reviewDecisions: []biTeamReviewDecision{
			{ID: 1, RequirementID: 1, Status: "rejected", DecidedAt: august},
			{ID: 2, RequirementID: 1, Status: "completed", DecidedAt: august.Add(time.Hour)},
			{ID: 3, RequirementID: 3, Status: "completed", DecidedAt: august.Add(2 * time.Hour)},
		},
		rawTasks: []task.Snapshot{
			{ID: 10, PlanUnitID: 10, AssigneeID: &developerID, Status: "draft", CreatedAt: august, DueAt: &september},
			{ID: 11, PlanUnitID: 10, AssigneeID: &developerID, Status: "completed", CreatedAt: july, DueAt: &duePast, CompletedAt: &completedLate},
			{ID: 12, PlanUnitID: 10, AssigneeID: &developerID, Status: "draft", CreatedAt: july, DueAt: &dueFuture},
		},
		rawBugs: []quality.BugSnapshot{
			{ID: 20, PlanUnitID: 10, ReporterUserID: &testerID, ReviewerUserID: &testerID,
				AssigneeUserID: &developerID, Status: "done", ResolutionReason: "fixed", SeverityKey: "p0",
				CreatedAt: august, ResolvedAt: &resolved, CompletedAt: &closed},
			{ID: 21, PlanUnitID: 10, ReporterUserID: &testerID, Status: "done",
				ResolutionReason: "duplicate", CreatedAt: july, CompletedAt: &canceled, UpdatedAt: canceled},
		},
		bugActivity: []quality.BugActivitySnapshot{
			{QualityBugID: 20, FieldKey: "assignee_user_id", NewValue: "3", CreatedAt: july},
			{QualityBugID: 20, FieldKey: "status", NewValue: "in_progress", CreatedAt: august.Add(-time.Hour)},
			{QualityBugID: 21, FieldKey: "resolution_reason", NewValue: "duplicate", CreatedAt: canceled},
		},
	}

	data := agg.data()
	product := biTeamSummaryTestRow(t, data.SummaryRows, "product")
	if product.MemberCount != 1 || product.RequirementCount != 1 || product.ReviewPassSampleSize != 3 || metricValue(product.ReviewPassRatePct) != 66.67 ||
		product.FirstPassSampleSize != 2 || product.FirstPassRatePct != 50 {
		t.Fatalf("product raw metrics=%+v", product)
	}
	testing := biTeamSummaryTestRow(t, data.SummaryRows, "testing")
	if testing.MemberCount != 1 || testing.BugCount != 1 || testing.BugCloseSampleSize != 1 || metricValue(testing.BugCloseAvgHours) <= 0 ||
		testing.InvalidBugSampleSize != 1 || metricValue(testing.InvalidBugRatePct) != 100 {
		t.Fatalf("testing raw metrics=%+v", testing)
	}
	development := biTeamSummaryTestRow(t, data.SummaryRows, "development")
	if development.MemberCount != 1 || development.TaskCount != 1 || development.OverdueSampleSize != 2 || metricValue(development.OverdueRatePct) != 50 ||
		development.BugTaskRatePct == nil || *development.BugTaskRatePct != 100 ||
		development.P0BugCount != 1 || development.TeamP0SharePct == nil || *development.TeamP0SharePct != 100 ||
		development.BugResponseSampleSize != 1 || metricValue(development.BugResponseAvgHours) <= 0 {
		t.Fatalf("development raw metrics=%+v", development)
	}
	if len(data.MemberGroups) != 3 || data.MemberGroups[0].Members[0].RequirementCount != 1 ||
		data.MemberGroups[2].Members[0].BugCount != 1 {
		t.Fatalf("member raw metrics=%+v", data.MemberGroups)
	}
}

func TestBITeamDevelopmentOverdueUsesEffectivePendingTestHandoff(t *testing.T) {
	developerID := uint(7)
	due := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	now := due.Add(48 * time.Hour)
	onTime := due.Add(-time.Hour)
	late := due.Add(time.Hour)
	created := due.Add(-72 * time.Hour)

	agg := biTeamAggregation{
		dimension: biTeamDimensionMonth, dimensionKey: "2026-08", location: time.UTC,
		now: now, scoreProfile: defaultBITeamScoreProfile(),
		ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			developerID: {ID: developerID, DisplayName: "Developer"},
		}},
		rawTasks: []task.Snapshot{
			{ID: 1, PlanUnitID: 10, AssigneeID: &developerID, Status: "pending_test", CreatedAt: created, DueAt: &due},
			{ID: 2, PlanUnitID: 10, AssigneeID: &developerID, Status: "pending_test", CreatedAt: created, DueAt: &due},
			{ID: 3, PlanUnitID: 10, AssigneeID: &developerID, Status: "completed", CreatedAt: created, DueAt: &due, CompletedAt: &late},
			{ID: 4, PlanUnitID: 10, AssigneeID: &developerID, Status: "in_dev", CreatedAt: created, DueAt: &due},
			{ID: 5, PlanUnitID: 10, AssigneeID: &developerID, Status: "completed", CreatedAt: created, DueAt: &due, CompletedAt: &onTime},
			{ID: 6, PlanUnitID: 10, AssigneeID: &developerID, Status: "completed", CreatedAt: created, DueAt: &due, CompletedAt: &late},
		},
		taskActivity: []task.ActivitySnapshot{
			{ID: 1, WorkTaskID: 1, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: onTime},
			{ID: 2, WorkTaskID: 2, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: late},
			{ID: 3, WorkTaskID: 3, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: onTime},
			{ID: 4, WorkTaskID: 3, FieldKey: "status", OldValue: "pending_test", NewValue: "completed", CreatedAt: late},
			{ID: 5, WorkTaskID: 4, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: onTime},
			{ID: 6, WorkTaskID: 4, FieldKey: "status", OldValue: "pending_test", NewValue: "in_dev", CreatedAt: late},
			{ID: 7, WorkTaskID: 5, FieldKey: "status", OldValue: "in_dev", NewValue: "completed", CreatedAt: onTime},
			{ID: 8, WorkTaskID: 6, FieldKey: "status", OldValue: "in_dev", NewValue: "completed", CreatedAt: late},
		},
	}

	development := biTeamSummaryTestRow(t, agg.data().SummaryRows, "development")
	if development.OverdueSampleSize != 6 || metricValue(development.OverdueRatePct) != 50 {
		t.Fatalf("development overdue handoff metric=%+v", development)
	}
}

func TestBITeamDevelopmentOverdueInheritsNearestParentDeadline(t *testing.T) {
	developerID := uint(7)
	parentID, childID := uint(1), uint(2)
	due := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	late := due.Add(time.Hour)
	created := due.Add(-72 * time.Hour)

	agg := biTeamAggregation{
		dimension: biTeamDimensionMonth, dimensionKey: "2026-08", location: time.UTC,
		now: due.Add(48 * time.Hour), scoreProfile: defaultBITeamScoreProfile(),
		ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			developerID: {ID: developerID, DisplayName: "Developer"},
		}},
		rawTasks: []task.Snapshot{
			{ID: parentID, PlanUnitID: 10, Status: "in_dev", CreatedAt: created, DueAt: &due},
			{ID: childID, PlanUnitID: 10, AssigneeID: &developerID, ParentID: &parentID, Status: "pending_test", CreatedAt: created},
			{ID: 3, PlanUnitID: 10, AssigneeID: &developerID, ParentID: &childID, Status: "pending_test", CreatedAt: created},
			{ID: 4, PlanUnitID: 10, AssigneeID: &developerID, Status: "pending_test", CreatedAt: created},
		},
		taskActivity: []task.ActivitySnapshot{
			{ID: 1, WorkTaskID: childID, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: late},
			{ID: 2, WorkTaskID: 3, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: due.Add(-time.Hour)},
			{ID: 3, WorkTaskID: 4, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: late},
		},
	}

	development := biTeamSummaryTestRow(t, agg.data().SummaryRows, "development")
	if development.OverdueSampleSize != 2 || metricValue(development.OverdueRatePct) != 50 {
		t.Fatalf("inherited deadline metric=%+v", development)
	}
}

func TestBITeamRawRatiosRemainMissingOrUncappedAsDefined(t *testing.T) {
	developerID := uint(7)
	created := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	agg := biTeamAggregation{
		dimension: biTeamDimensionMonth, dimensionKey: "2026-08", location: time.UTC,
		rawBugs: []quality.BugSnapshot{
			{ID: 1, PlanUnitID: 10, AssigneeUserID: &developerID, Status: "triage", CreatedAt: created},
			{ID: 2, PlanUnitID: 10, AssigneeUserID: &developerID, Status: "triage", CreatedAt: created},
		},
	}
	development := biTeamSummaryTestRow(t, agg.data().SummaryRows, "development")
	if development.BugTaskRatePct != nil || development.BugTaskSampleSize != 0 || development.TeamP0SharePct != nil ||
		development.OverdueRatePct != nil || development.BugResponseAvgHours != nil {
		t.Fatalf("zero denominators must be missing: %+v", development)
	}
	for _, roleKey := range []string{"product", "testing"} {
		row := biTeamSummaryTestRow(t, agg.data().SummaryRows, roleKey)
		if row.ReviewPassRatePct != nil || row.BugCloseAvgHours != nil || row.InvalidBugRatePct != nil {
			t.Fatalf("%s zero-sample metrics must be missing: %+v", roleKey, row)
		}
	}

	agg.rawTasks = []task.Snapshot{{
		ID: 1, PlanUnitID: 10, AssigneeID: &developerID, Status: "draft", CreatedAt: created,
	}}
	development = biTeamSummaryTestRow(t, agg.data().SummaryRows, "development")
	if development.BugTaskRatePct == nil || *development.BugTaskRatePct != 200 {
		t.Fatalf("bug/task rate must remain uncapped: %+v", development)
	}
}

func TestBITeamSeveritySharesIncludeUnassignedTeamBugs(t *testing.T) {
	developerID := uint(7)
	created := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	agg := biTeamAggregation{location: time.UTC, rawBugs: []quality.BugSnapshot{
		{ID: 1, PlanUnitID: 10, AssigneeUserID: &developerID, Status: "triage", SeverityKey: "p0", CreatedAt: created},
		{ID: 2, PlanUnitID: 10, Status: "triage", SeverityKey: "p0", CreatedAt: created},
		{ID: 3, PlanUnitID: 10, AssigneeUserID: &developerID, Status: "triage", SeverityKey: "p1", CreatedAt: created},
		{ID: 4, PlanUnitID: 10, Status: "triage", SeverityKey: "p1", CreatedAt: created},
	}}
	data := agg.data()
	development := biTeamSummaryTestRow(t, data.SummaryRows, "development")
	if development.P0BugCount != 2 || development.TeamP0SharePct == nil || *development.TeamP0SharePct != 100 {
		t.Fatalf("team P0 summary=%+v", development)
	}
	if development.P1BugCount != 2 || development.TeamP1SharePct == nil || *development.TeamP1SharePct != 100 {
		t.Fatalf("team P1 summary=%+v", development)
	}
	var member biTeamMember
	for _, group := range data.MemberGroups {
		if group.RoleKey == "development" && len(group.Members) > 0 {
			member = group.Members[0]
			break
		}
	}
	if member.P0BugCount != 1 || member.TeamP0SharePct == nil || *member.TeamP0SharePct != 50 {
		t.Fatalf("member P0 share=%+v", member)
	}
	if member.P1BugCount != 1 || member.TeamP1SharePct == nil || *member.TeamP1SharePct != 50 {
		t.Fatalf("member P1 share=%+v", member)
	}
}

func TestBITeamRawMetricsPreserveExistingScoreOrder(t *testing.T) {
	newUserID := uint(5)
	data := biTeamData{MemberGroups: []biTeamMemberGroup{{
		RoleKey: "product",
		Members: []biTeamMember{{UserID: 9}, {UserID: 2}},
	}}}
	biTeamAggregation{rawRequirements: []requirement.Snapshot{{
		ID: 1, PlanUnitID: biTestUintPointer(10), ProductOwnerUserIDs: []uint{newUserID}, CreatedAt: time.Now().UTC(),
	}}}.applyRawMetrics(&data)
	rows := data.MemberGroups[0].Members
	if len(rows) != 3 || rows[0].UserID != 9 || rows[1].UserID != 2 || rows[2].UserID != newUserID {
		t.Fatalf("raw metrics changed score order: %+v", rows)
	}
}

func metricValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func biTestUintPointer(value uint) *uint { return &value }
