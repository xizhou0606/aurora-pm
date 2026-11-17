package httpapi

import (
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBITeamDataCalculatesRoleMetricsFromDeterministicFacts(t *testing.T) {
	productID, developerID, testerID := uint(1), uint(2), uint(3)
	requirementCreated := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	taskCreated := requirementCreated.Add(time.Hour)
	taskDue := requirementCreated.Add(16 * time.Hour)
	taskCompleted := taskDue.Add(-time.Hour)
	resolvedAt := requirementCreated.Add(10 * time.Hour)
	reviewedAt := resolvedAt.Add(2 * time.Hour)
	bugDue := reviewedAt.Add(time.Hour)

	agg := biTeamAggregation{
		dimension: biTeamDimensionVersion, dimensionKey: "12",
		options: []biTeamDimensionOption{{Key: "12", Label: "V1"}},
		ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			productID:   {ID: productID, DisplayName: "Product"},
			developerID: {ID: developerID, DisplayName: "Developer"},
			testerID:    {ID: testerID, DisplayName: "Tester"},
		}},
		requirements: []requirement.Snapshot{{
			ID: 21, PlanUnitID: biTestUintPointer(12), ProductOwnerUserIDs: []uint{productID},
			Status: "done", CreatedAt: requirementCreated,
		}},
		tasks: []task.Snapshot{{
			ID: 31, PlanUnitID: 12, RequirementID: 21, AssigneeID: &developerID,
			Status: "completed", CreatedAt: taskCreated, DueAt: &taskDue, CompletedAt: &taskCompleted,
		}},
		bugs: []quality.BugSnapshot{{
			ID: 41, PlanUnitID: 12, RequirementID: 21, AssigneeUserID: &developerID,
			ReporterUserID: &testerID, ReviewerUserID: &testerID,
			Status: "done", ResolutionReason: "fixed", CreatedAt: taskCreated,
			ResolvedAt: &resolvedAt, ReviewedAt: &reviewedAt, CompletedAt: &reviewedAt, DueAt: &bugDue,
		}},
		taskActivity: []task.ActivitySnapshot{
			{WorkTaskID: 31, FieldKey: "status", OldValue: "draft", NewValue: "in_dev", CreatedAt: taskCreated.Add(time.Hour)},
			{WorkTaskID: 31, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: taskCreated.Add(2 * time.Hour)},
			{WorkTaskID: 31, FieldKey: "status", OldValue: "pending_test", NewValue: "in_dev", CreatedAt: taskCreated.Add(3 * time.Hour)},
			{WorkTaskID: 31, FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: taskCreated.Add(4 * time.Hour)},
		},
		bugActivity: []quality.BugActivitySnapshot{
			{ID: 1, QualityBugID: 41, Action: "assigned", FieldKey: "assignee_user_id", NewValue: "2", CreatedAt: taskCreated},
			{ID: 2, QualityBugID: 41, Action: "confirmed", FieldKey: "status", NewValue: "in_progress", CreatedAt: taskCreated.Add(time.Hour)},
			{ID: 3, QualityBugID: 41, Action: "returned", FieldKey: "status", OldValue: "review", NewValue: "in_progress", CreatedAt: reviewedAt},
		},
	}

	data := agg.data()
	if data.Dimension != "version" || data.DimensionKey != "12" || len(data.MetricNotes) == 0 {
		t.Fatalf("team data contract=%+v", data)
	}
	product := biTeamSummaryTestRow(t, data.SummaryRows, "product")
	if product.Workload != 1 || product.CompletedWorkload != 1 || product.AvgResponseHours != 1 ||
		product.ResponseSampleSize != 1 || product.OnTimeRatePct != 100 || product.QualityRiskRatePct != 100 ||
		product.OnTimeSampleSize != 1 || product.QualityRiskSampleSize != 1 || product.ReworkSampleSize != 1 ||
		product.ReworkCount != 1 || product.ReworkRatePct != 100 || product.FirstPassRatePct != 0 {
		t.Fatalf("product metrics=%+v", product)
	}
	development := biTeamSummaryTestRow(t, data.SummaryRows, "development")
	if development.Workload != 1 || development.CompletedWorkload != 1 || development.AvgResponseHours != 1 ||
		development.ResponseSampleSize != 1 || development.OnTimeRatePct != 100 ||
		development.QualityRiskRatePct != 100 || development.ReworkCount != 1 ||
		development.OnTimeSampleSize != 1 || development.QualityRiskSampleSize != 1 ||
		development.ReworkSampleSize != 1 ||
		development.ReworkRatePct != 100 || development.FirstPassRatePct != 0 ||
		development.BugResponseRatePct != 100 || development.BugResponseSampleSize != 1 {
		t.Fatalf("development metrics=%+v", development)
	}
	testing := biTeamSummaryTestRow(t, data.SummaryRows, "testing")
	if testing.Workload != 1 || testing.CompletedWorkload != 1 || testing.AvgResponseHours != 2 ||
		testing.ResponseSampleSize != 1 || testing.OnTimeRatePct != 100 || testing.QualityRiskRatePct != 0 ||
		testing.OnTimeSampleSize != 1 || testing.QualityRiskSampleSize != 1 || testing.ReworkSampleSize != 1 ||
		testing.ReworkCount != 1 || testing.ReworkRatePct != 100 || testing.FirstPassRatePct != 0 {
		t.Fatalf("testing metrics=%+v", testing)
	}
	if len(data.MemberGroups) != 3 || data.MemberGroups[1].Members[0].UserName != "Developer" ||
		data.MemberGroups[1].Members[0].ResponseSampleSize != 1 ||
		data.MemberGroups[1].Members[0].OnTimeSampleSize != 1 ||
		data.MemberGroups[1].Members[0].QualityRiskSampleSize != 1 ||
		data.MemberGroups[1].Members[0].ReworkSampleSize != 1 {
		t.Fatalf("member groups=%+v", data.MemberGroups)
	}
}

func TestBITeamTestingQualityRiskUsesInvalidResolutionReasons(t *testing.T) {
	testerID := uint(3)
	now := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	agg := biTeamAggregation{
		bugs: []quality.BugSnapshot{
			{ID: 1, ReporterUserID: &testerID, Status: "done", ResolutionReason: "fixed", CompletedAt: &now},
			{ID: 2, ReporterUserID: &testerID, Status: "done", ResolutionReason: "duplicate", CompletedAt: &now, UpdatedAt: now},
			{ID: 3, ReporterUserID: &testerID, Status: "done", ResolutionReason: "as_designed", CompletedAt: &now},
			{ID: 4, ReporterUserID: &testerID, Status: "review", ResolutionReason: "external", ResolvedAt: &now},
		},
	}

	row := biTeamSummaryTestRow(t, agg.data().SummaryRows, "testing")
	if row.Workload != 4 || row.CompletedWorkload != 3 || row.QualityRiskRatePct != 75 {
		t.Fatalf("testing invalid-resolution metrics=%+v", row)
	}
}

func TestBITeamBugResponseAcceptsInProgressOrResolvedAndExplainsOpenSamples(t *testing.T) {
	ownerID := uint(7)
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	created := now.Add(-48 * time.Hour)
	resolved := created.Add(3 * time.Hour)
	rows := []quality.BugSnapshot{
		{ID: 1, AssigneeUserID: &ownerID, Status: "review", CreatedAt: created, ResolvedAt: &resolved},
		{ID: 2, AssigneeUserID: &ownerID, Status: "triage", CreatedAt: created},
		{ID: 3, AssigneeUserID: &ownerID, Status: "triage", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: 4, AssigneeUserID: &ownerID, Status: "in_progress", CreatedAt: created},
		{ID: 5, AssigneeUserID: &ownerID, Status: "resolved", CreatedAt: created},
	}
	facts := biTeamBugResponseFacts(rows, []quality.BugActivitySnapshot{
		{QualityBugID: 1, FieldKey: "status", OldValue: "in_progress", NewValue: "review", CreatedAt: resolved},
		{QualityBugID: 4, FieldKey: "status", OldValue: "todo", NewValue: "in_progress", CreatedAt: created.Add(5 * time.Hour)},
		{QualityBugID: 5, FieldKey: "status", OldValue: "in_progress", NewValue: "resolved", CreatedAt: created.Add(6 * time.Hour)},
	}, now)
	if !facts[1].eligible || !facts[1].timely {
		t.Fatalf("resolved response fact=%+v", facts[1])
	}
	if !facts[2].eligible || facts[2].timely {
		t.Fatalf("overdue open response fact=%+v", facts[2])
	}
	if _, exists := facts[3]; exists {
		t.Fatalf("open SLA window must not enter the sample: %+v", facts[3])
	}
	if !facts[4].eligible || !facts[4].timely || !facts[5].eligible || !facts[5].timely {
		t.Fatalf("in-progress and resolved activity must both count as response: %+v", facts)
	}
}

func TestBITeamMonthDimensionUsesRequestTimezoneForCreatedFacts(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.July, 31, 16, 30, 0, 0, time.UTC)
	tasks := []task.Snapshot{{ID: 1, CreatedAt: createdAt}}
	filtered := biFilterTeamTasks(tasks, biTeamDimensionMonth, "2026-08", location)
	if len(filtered) != 1 {
		t.Fatalf("timezone month filter rows=%d want=1", len(filtered))
	}
	options := biTeamDimensionOptions(
		biTeamDimensionMonth, workspaceScope{}, biWorkspaceContext{}, nil, tasks, nil, location,
	)
	if len(options) != 1 || options[0].Key != "2026-08" || options[0].Label != "2026 年 08 月" {
		t.Fatalf("month options=%+v", options)
	}
}

func TestBITeamDataEndpointReusesWorkspaceScopeAndResponseEnvelope(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "bi-team-admin", "super_admin", "")
	developer := app.createUser(t, 2, "bi-team-developer", "user", "")
	tester := app.createUser(t, 3, "bi-team-tester", "user", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []identitygorm.WorkspaceMember{
		{WorkspaceID: 1, UserID: developer.ID, Role: "member", Enabled: true},
		{WorkspaceID: 1, UserID: tester.ID, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	createdAt := time.Date(2026, time.August, 3, 8, 0, 0, 0, time.UTC)
	completedAt := createdAt.Add(4 * time.Hour)
	dueAt := completedAt.Add(time.Hour)
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).Updates(map[string]any{
		"product_owner_user_id": admin.ID, "status": "done", "created_at": createdAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&taskgorm.WorkTask{ID: 701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-TEAM-1", Title: "Scoped task", Status: "completed",
			AssigneeID: &developer.ID, CreatedAt: createdAt.Add(time.Hour), CompletedAt: &completedAt, DueAt: &dueAt},
		&taskgorm.WorkTaskActivity{WorkspaceID: 1, WorkTaskID: 701, UserID: developer.ID,
			Action: "status_change", FieldKey: "status", OldValue: "draft", NewValue: "in_dev", CreatedAt: createdAt.Add(2 * time.Hour)},
		&taskgorm.WorkTaskActivity{WorkspaceID: 1, WorkTaskID: 701, UserID: developer.ID,
			Action: "status_change", FieldKey: "status", OldValue: "in_dev", NewValue: "pending_test", CreatedAt: createdAt.Add(3 * time.Hour)},
		&qualitygorm.QualityBug{ID: 702, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, BugKey: "A-TEAM-BUG", Title: "Scoped bug", Status: "done", ResolutionReason: "fixed",
			ReporterUserID: &tester.ID, ReviewerUserID: &tester.ID, CreatedAt: createdAt,
			ResolvedAt: &createdAt, ReviewedAt: &completedAt, CompletedAt: &completedAt, DueAt: &dueAt},
		&taskgorm.WorkTaskActivity{WorkspaceID: 2, WorkTaskID: 999, UserID: developer.ID,
			Action: "status_change", FieldKey: "status", OldValue: "pending_test", NewValue: "in_dev", CreatedAt: createdAt},
		&qualitygorm.QualityBugActivity{WorkspaceID: 2, QualityBugID: 999, UserID: tester.ID,
			Action: "returned", FieldKey: "status", OldValue: "review", NewValue: "in_progress", CreatedAt: createdAt},
	}
	for _, row := range rows {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen?team_dimension=version&team_dimension_key=12",
		app.token(t, admin), 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	teamData := data["team_data"].(map[string]any)
	if teamData["dimension"] != "version" || teamData["dimension_key"] != "12" ||
		len(teamData["summary_rows"].([]any)) != 3 || len(teamData["member_groups"].([]any)) != 3 ||
		len(teamData["metric_notes"].([]any)) == 0 {
		t.Fatalf("team data response contract=%v", teamData)
	}
	options := teamData["dimension_options"].([]any)
	if len(options) != 1 || options[0].(map[string]any)["key"] != "12" ||
		strings.Contains(response.Body.String(), "Version B") {
		t.Fatalf("dimension options leaked workspace data: %s", response.Body.String())
	}
	development := biTeamJSONSummaryTestRow(t, teamData["summary_rows"].([]any), "development")
	if development["workload"] != float64(1) || development["rework_count"] != float64(0) ||
		development["first_pass_rate_pct"] != float64(100) ||
		development["on_time_sample_size"] != float64(1) ||
		development["quality_risk_sample_size"] != float64(1) ||
		development["rework_sample_size"] != float64(1) {
		t.Fatalf("workspace-scoped development row=%v", development)
	}
	testing := biTeamJSONSummaryTestRow(t, teamData["summary_rows"].([]any), "testing")
	if testing["workload"] != float64(1) || testing["rework_count"] != float64(0) {
		t.Fatalf("workspace-scoped testing row=%v", testing)
	}

	productItemsResponse := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen/team-member-items?role=product&item_type=requirement&user_id=1&team_dimension=version&team_dimension_key=12",
		app.token(t, admin), 1, nil)
	requireStatus(t, productItemsResponse, http.StatusOK)
	productItems := decodeData(t, productItemsResponse).(map[string]any)
	if productItems["total"] != float64(1) || len(productItems["items"].([]any)) != 1 ||
		productItems["items"].([]any)[0].(map[string]any)["id"] != float64(13) {
		t.Fatalf("product member drilldown=%v", productItems)
	}

	developmentItemsResponse := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen/team-member-items?role=development&item_type=task&user_id=2&team_dimension=version&team_dimension_key=12",
		app.token(t, admin), 1, nil)
	requireStatus(t, developmentItemsResponse, http.StatusOK)
	developmentItems := decodeData(t, developmentItemsResponse).(map[string]any)
	developmentItem := developmentItems["items"].([]any)[0].(map[string]any)
	if developmentItems["total"] != float64(1) || developmentItem["id"] != float64(701) ||
		developmentItem["effective_due_at"] == nil || developmentItem["due_at_source"] != "self" ||
		developmentItem["due_at_source_task_id"] != float64(701) || developmentItem["delivery_at"] == nil ||
		developmentItem["overdue"] != false {
		t.Fatalf("development member drilldown=%v", developmentItems)
	}

	testingItemsResponse := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen/team-member-items?role=testing&item_type=bug&user_id=3&team_dimension=version&team_dimension_key=12",
		app.token(t, admin), 1, nil)
	requireStatus(t, testingItemsResponse, http.StatusOK)
	testingItems := decodeData(t, testingItemsResponse).(map[string]any)
	if testingItems["total"] != float64(1) || len(testingItems["items"].([]any)) != 1 ||
		testingItems["items"].([]any)[0].(map[string]any)["id"] != float64(702) {
		t.Fatalf("testing member drilldown=%v", testingItems)
	}
}

func TestBITeamDataRejectsInvalidDimensionContract(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "bi-team-invalid", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	invalidDimension := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen?team_dimension=quarter", token, 1, nil)
	requireStatus(t, invalidDimension, http.StatusBadRequest)
	invalidMonth := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen?team_dimension=month&team_dimension_key=2026-8", token, 1, nil)
	requireStatus(t, invalidMonth, http.StatusBadRequest)
	validWeek := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen?team_dimension=week&team_dimension_key=2026-W35", token, 1, nil)
	requireStatus(t, validWeek, http.StatusOK)
	invalidWeek := app.request(t, http.MethodGet,
		"/api/v1/platform/executive-dashboard/bi-screen?team_dimension=week&team_dimension_key=2025-W53", token, 1, nil)
	requireStatus(t, invalidWeek, http.StatusBadRequest)
}

func biTeamSummaryTestRow(t *testing.T, rows []biTeamSummaryRow, key string) biTeamSummaryRow {
	t.Helper()
	for _, row := range rows {
		if row.RoleKey == key {
			return row
		}
	}
	t.Fatalf("missing team summary row %q", key)
	return biTeamSummaryRow{}
}

func biTeamJSONSummaryTestRow(t *testing.T, rows []any, key string) map[string]any {
	t.Helper()
	for _, item := range rows {
		row := item.(map[string]any)
		if row["role_key"] == key {
			return row
		}
	}
	t.Fatalf("missing team JSON summary row %q", key)
	return nil
}
