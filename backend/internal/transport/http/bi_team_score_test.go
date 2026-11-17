package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	"net/http"
	"testing"
)

func TestBITeamScoreUsesDynamicWeightsAndSampleCoverage(t *testing.T) {
	profile := defaultBITeamScoreProfile()
	input := biTeamScoreInput{
		workload: 10, completedWorkload: 8,
		avgResponseHours: 12, responseSampleSize: 5,
		onTimeRatePct: 90, onTimeSampleSize: 5,
		qualityRiskRatePct: 10, qualityRiskSampleSize: 5,
		firstPassRatePct: 75, reworkSampleSize: 5,
		workloadSharePct: 20, workloadShareScorePct: 100, workloadShareSamples: 10,
		bugResponseRatePct: 80, bugResponseSamples: 5,
	}
	result := biTeamScoreFor("development", input, profile)
	if !result.eligible || result.compositeScore == nil || *result.compositeScore != 89.5 ||
		result.coveragePct != 100 || len(result.breakdown) != 5 {
		t.Fatalf("score result=%+v", result)
	}

	input.onTimeSampleSize = 0
	result = biTeamScoreFor("development", input, profile)
	if !result.eligible || result.compositeScore == nil || result.coveragePct != 85 || *result.compositeScore != 89.41 {
		t.Fatalf("renormalized result=%+v", result)
	}

	input.workload = 2
	result = biTeamScoreFor("development", input, profile)
	if result.eligible || result.compositeScore != nil ||
		len(result.ineligibleReasons) == 0 || result.ineligibleReasons[0] != "workload_below_minimum" {
		t.Fatalf("low workload must be ineligible: %+v", result)
	}
}

func TestBITeamScoreDefaultsKeepWorkloadAndCompletionAsCoreMetrics(t *testing.T) {
	profile := defaultBITeamScoreProfile()
	for _, roleKey := range biTeamScoreRoles {
		coreWeight := 0
		for _, item := range profile.Items {
			if item.RoleKey == roleKey && item.Enabled &&
				(item.MetricKey == "workload_share" || item.MetricKey == "completion_rate") {
				coreWeight += item.WeightBPS
			}
		}
		if coreWeight < biTeamScoreCoreMinimumBPS {
			t.Fatalf("role=%s core weight=%d want >=%d", roleKey, coreWeight, biTeamScoreCoreMinimumBPS)
		}
	}
}

func TestBITeamScoreProfileRejectsRoleConfigWithoutCoreWeight(t *testing.T) {
	defaults := defaultBITeamScoreProfile()
	items := make([]biTeamScoreProfileItemRequest, 0, len(defaults.Items))
	for _, item := range defaults.Items {
		weight := item.WeightBPS
		if item.RoleKey == "development" {
			switch item.MetricKey {
			case "workload_share":
				weight = 1_000
			case "on_time_rate":
				weight = 4_000
			}
		}
		items = append(items, biTeamScoreProfileItemRequest{
			MetricKey: item.MetricKey, RoleKey: item.RoleKey, Enabled: item.Enabled,
			WeightBPS: weight, MissingPolicy: item.MissingPolicy, MinimumSamples: item.MinimumSamples,
		})
	}
	if _, err := validateBITeamScoreProfileRequest(biTeamScoreProfileRequest{Items: items}); err == nil {
		t.Fatal("role-specific profile without at least 50% core weight must be rejected")
	}
}

func TestBITeamScoreExcludesMetricsThatDoNotApplyToTheRole(t *testing.T) {
	profile := defaultBITeamScoreProfile()
	input := biTeamScoreInput{
		workload: 10, completedWorkload: 10,
		avgResponseHours: 1, responseSampleSize: 10,
		onTimeRatePct: 100, onTimeSampleSize: 10,
		qualityRiskRatePct: 0, qualityRiskSampleSize: 10,
		firstPassRatePct: 100, reworkSampleSize: 10,
		workloadSharePct: 50, workloadShareScorePct: 100, workloadShareSamples: 10,
		bugResponseRatePct: 100, bugResponseSamples: 10,
	}
	result := biTeamScoreFor("development", input, profile)
	if !result.eligible || result.compositeScore == nil || *result.compositeScore != 100 ||
		result.coveragePct != 100 || len(result.breakdown) != 5 {
		t.Fatalf("role-specific result=%+v", result)
	}
}

func TestBITeamScoreProfileIsWorkspaceScopedAndVersioned(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "score-admin", "super_admin", "")
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Score", Slug: "score", Enabled: true},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	items := []map[string]any{
		{"metric_key": "completion_rate", "enabled": true, "weight_bps": 4_000, "missing_policy": "renormalize", "minimum_samples": 1},
		{"metric_key": "on_time_rate", "enabled": true, "weight_bps": 2_000, "missing_policy": "renormalize", "minimum_samples": 1},
		{"metric_key": "first_pass_rate", "enabled": true, "weight_bps": 2_000, "missing_policy": "renormalize", "minimum_samples": 1},
		{"metric_key": "quality_health", "enabled": true, "weight_bps": 1_000, "missing_policy": "renormalize", "minimum_samples": 1},
		{"metric_key": "response_efficiency", "enabled": true, "weight_bps": 1_000, "missing_policy": "renormalize", "minimum_samples": 1},
	}
	path := "/api/v1/platform/executive-dashboard/bi-screen/team-score-profile"
	created := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "items": items,
	})
	requireStatus(t, created, http.StatusOK)
	profile := decodeData(t, created).(map[string]any)
	if profile["version"] != float64(1) || profile["source"] != "workspace" ||
		profile["total_weight_bps"] != float64(10_000) {
		t.Fatalf("created profile=%v", profile)
	}

	conflict := app.request(t, http.MethodPut, path, app.token(t, admin), 1, map[string]any{
		"expected_version": 0, "items": items,
	})
	requireStatus(t, conflict, http.StatusConflict)
	if conflict.Code != http.StatusConflict || conflict.Body.String() == "" {
		t.Fatalf("conflict=%s", conflict.Body.String())
	}

	var profiles int64
	if err := app.db.Model(&workbenchgorm.BITeamScoreProfile{}).Where("workspace_id = ?", 1).Count(&profiles).Error; err != nil {
		t.Fatal(err)
	}
	if profiles != 1 {
		t.Fatalf("conflict wrote profile count=%d", profiles)
	}
}

func TestBITeamScoreProfileRejectsInvalidTotal(t *testing.T) {
	_, err := validateBITeamScoreProfileRequest(biTeamScoreProfileRequest{Items: []biTeamScoreProfileItemRequest{
		{MetricKey: "completion_rate", Enabled: true, WeightBPS: 1},
		{MetricKey: "on_time_rate", Enabled: false},
		{MetricKey: "first_pass_rate", Enabled: false},
		{MetricKey: "quality_health", Enabled: false},
		{MetricKey: "response_efficiency", Enabled: false},
	}})
	if err == nil {
		t.Fatal("invalid total must be rejected")
	}
}
