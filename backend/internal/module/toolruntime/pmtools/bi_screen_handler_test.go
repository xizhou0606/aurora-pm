package pmtools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
)

type biScreenFake struct{ query workbench.BIScreenQuery }

func (fake *biScreenFake) Get(_ context.Context, query workbench.BIScreenQuery) (workbench.BIScreenResult, error) {
	fake.query = query
	return emptyBIScreenResult(query.WorkspaceID, query.Period), nil
}

type biScreenAuthorizer struct{}

func (biScreenAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{BIScreenGet}, []string{"workbench:read"})
}

func TestBIScreenHandlerUsesServerWorkspaceThroughApplication(t *testing.T) {
	fake := &biScreenFake{}
	handlers, err := NewBIScreenReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: BIScreenReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, biScreenAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	result, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: BIScreenGet, Input: json.RawMessage(`{"project_set_id":11,"project_new_id":12,"requirement_id":13,"period":"month"}`),
		Authorization: toolruntime.ExecutionAuthorization{
			PrincipalID: 9, WorkspaceID: 7, GrantID: 8, ClientID: "bi-test",
			Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != BIScreenGet || fake.query.WorkspaceID != 7 || fake.query.Period != "month" ||
		len(fake.query.ProjectSetIDs) != 1 || fake.query.ProjectSetIDs[0] != 11 ||
		len(fake.query.PlanUnitIDs) != 1 || fake.query.PlanUnitIDs[0] != 12 ||
		len(fake.query.RequirementIDs) != 1 || fake.query.RequirementIDs[0] != 13 {
		t.Fatalf("result=%+v query=%+v", result, fake.query)
	}
}

func emptyBIScreenResult(workspaceID uint, period string) workbench.BIScreenResult {
	return workbench.BIScreenResult{
		Period: period, WorkspaceID: workspaceID, WorkspaceName: "研发", Scope: map[string]any{}, OverviewMetrics: map[string]any{},
		UrgencyDistribution: []map[string]any{}, DueDistribution: []map[string]any{}, PriorityDistribution: []map[string]any{},
		TypeDistribution: []map[string]any{}, StatusProgress: []map[string]any{}, CompletionTrend: []map[string]any{},
		VersionProgress: []map[string]any{}, RequirementProgress: []map[string]any{}, VersionRankings: []map[string]any{},
		OwnerBoard: []map[string]any{}, MilestoneOptions: []any{}, QualitySeverityDistribution: []map[string]any{},
		QualityStatusDistribution: []map[string]any{}, QualityTrend: []map[string]any{}, VersionQualityRisks: []map[string]any{},
		QualityOwnerLoad: []map[string]any{}, TeamData: map[string]any{}, GeneratedAt: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		Compatibility: map[string]any{
			"summary": map[string]any{}, "kpi": map[string]any{}, "teams": []any{}, "projects": []any{}, "risks": []any{},
			"risks_by_team": []any{}, "risks_by_kind": []any{}, "release_train": []any{}, "sparklines": []any{},
			"light_distribution": map[string]any{}, "status_mix": []any{}, "data_quality": map[string]any{},
			"project_total": 0, "project_exception_count": 0, "pm_pipeline": map[string]any{}, "team_clusters": []any{},
			"pm_narrative": map[string]any{},
		},
	}
}
