package biscreen

import (
	"context"
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type fakeBIReads struct {
	workspace    identity.BIWorkspace
	projects     project.BIContext
	requirements []requirement.Snapshot
	tasks        []task.Snapshot
	bugs         []quality.BugSnapshot
	users        map[uint]identity.BIUser
}

func (fake *fakeBIReads) ListBIFacts(_ context.Context, _ uint) ([]task.Snapshot, []task.ActivitySnapshot, error) {
	return fake.tasks, nil, nil
}

type fakeBIQuality struct{ source *fakeBIReads }

func (fake fakeBIQuality) ListBIFacts(_ context.Context, _ uint) ([]quality.BugSnapshot, []quality.BugActivitySnapshot, error) {
	return fake.source.bugs, nil, nil
}

func (fake *fakeBIReads) LoadBIContext(_ context.Context, _ uint) (project.BIContext, error) {
	return fake.projects, nil
}

func (fake *fakeBIReads) GetBIWorkspace(_ context.Context, _ uint) (identity.BIWorkspace, error) {
	return fake.workspace, nil
}

func (fake *fakeBIReads) FindActiveBIUsersByFactIDs(_ context.Context, _ []uint) (map[uint]identity.BIUser, error) {
	return fake.users, nil
}

func (fake *fakeBIReads) GetBITeamMetricSettings(_ context.Context, _ uint) (workbench.BITeamMetricSettingsSnapshot, error) {
	return workbench.BITeamMetricSettingsSnapshot{}, nil
}

func (fake *fakeBIReads) GetBITeamScoreProfile(_ context.Context, _ uint) (workbench.BITeamScoreProfileSnapshot, error) {
	return workbench.BITeamScoreProfileSnapshot{}, nil
}

func (fake *fakeBIReads) ListBIReviewDecisions(context.Context, uint, uint, string, []uint) ([]workflow.BIReviewDecision, error) {
	return nil, nil
}

func (fake *fakeBIReads) ListBIMetricWorkflowOptions(context.Context, uint) ([]workflow.BIMetricWorkflowOption, error) {
	return nil, nil
}

type fakeBIRequirements struct{ source *fakeBIReads }

func (fake fakeBIRequirements) ListBIFacts(_ context.Context, _ uint) ([]requirement.Snapshot, error) {
	return fake.source.requirements, nil
}

func TestBackendBuildsFromPublicDomainFacts(t *testing.T) {
	ownerID := uint(7)
	planUnitID := uint(12)
	now := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	facts := &fakeBIReads{
		workspace: identity.BIWorkspace{ID: 1, Name: "Workspace"},
		projects: project.BIContext{
			ProjectSets: []project.ProjectSetSnapshot{{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "Set"}},
			PlanUnits:   []project.PlanUnitSnapshot{{ID: planUnitID, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "V1"}},
		},
		requirements: []requirement.Snapshot{{ID: 13, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
			PlanUnitID: &planUnitID, Title: "Requirement", ProductOwnerUserIDs: []uint{ownerID}, CreatedAt: now.Add(-24 * time.Hour)}},
		tasks: []task.Snapshot{{ID: 21, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
			PlanUnitID: planUnitID, RequirementID: 13, Status: "completed", AssigneeID: &ownerID,
			CreatedAt: now.Add(-time.Hour), CompletedAt: &now}},
		bugs: []quality.BugSnapshot{{ID: 31, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
			PlanUnitID: planUnitID, RequirementID: 13, Status: "triage", SeverityKey: "p1", CreatedAt: now.Add(-time.Hour)}},
		users: map[uint]identity.BIUser{ownerID: {ID: ownerID, DisplayName: "Owner"}},
	}
	backend, err := NewBIScreenBackend(workbench.BIScreenReadDependencies{
		Tasks: facts, Quality: fakeBIQuality{facts}, Projects: facts, Requirements: fakeBIRequirements{facts},
		Identities: facts, Configuration: facts, Workflows: facts,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := backend.BuildBIScreen(context.Background(), workbench.BIScreenQuery{
		WorkspaceID: 1, Period: "week", ProjectSetIDs: []uint{11}, PlanUnitIDs: []uint{planUnitID},
		RequirementIDs: []uint{13}, TeamDimension: "version", Location: time.UTC, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkspaceName != "Workspace" || result.TeamCount != 1 || result.OverviewMetrics["total_tasks"] != 1 ||
		result.OverviewMetrics["open_bugs"] != 1 || len(result.VersionProgress) != 1 {
		t.Fatalf("unexpected BI result: %+v", result)
	}
	weekResult, err := backend.BuildBIScreen(context.Background(), workbench.BIScreenQuery{
		WorkspaceID: 1, Period: "week", ProjectSetIDs: []uint{11}, PlanUnitIDs: []uint{planUnitID},
		RequirementIDs: []uint{13}, TeamDimension: "week", TeamDimensionKey: "2026-W35",
		Location: time.UTC, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	weekData, ok := weekResult.TeamData.(biTeamData)
	if !ok || weekData.Dimension != "week" || weekData.DimensionKey != "2026-W35" ||
		len(weekData.DimensionOptions) == 0 {
		t.Fatalf("unexpected weekly team data: %+v", weekResult.TeamData)
	}
}

func TestBackendRejectsScopeOutsideWorkspace(t *testing.T) {
	facts := &fakeBIReads{workspace: identity.BIWorkspace{ID: 1, Name: "Workspace"}}
	backend, err := NewBIScreenBackend(workbench.BIScreenReadDependencies{
		Tasks: facts, Quality: fakeBIQuality{facts}, Projects: facts, Requirements: fakeBIRequirements{facts},
		Identities: facts, Configuration: facts, Workflows: facts,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.BuildBIScreen(context.Background(), workbench.BIScreenQuery{WorkspaceID: 1, ProjectSetIDs: []uint{99}, Location: time.UTC, Now: time.Now()})
	if err == nil {
		t.Fatal("expected out-of-workspace scope rejection")
	}
}

func TestRetiredUserReferencesUseTheSharedDisplayName(t *testing.T) {
	retiredID := uint(78)
	ownerRows := (biAggregation{
		Tasks: []task.Snapshot{{ID: 1, AssigneeID: &retiredID}},
		Ctx:   biWorkspaceContext{Users: map[uint]identity.BIUser{}},
	}).ownerBoard()
	if len(ownerRows) != 1 || ownerRows[0]["owner_name"] != "用户 #78" {
		t.Fatalf("owner board retired user=%+v", ownerRows)
	}

	role := &biTeamRoleAccumulator{
		key: "product", label: "产品", total: newBITeamAccumulator(),
		members: map[uint]*biTeamAccumulator{retiredID: newBITeamAccumulator()},
	}
	group := role.group(biWorkspaceContext{Users: map[uint]identity.BIUser{}}, defaultBITeamScoreProfile())
	if len(group.Members) != 1 || group.Members[0].UserName != "用户 #78" {
		t.Fatalf("team member retired user=%+v", group.Members)
	}
}

func TestTeamWeekDimensionUsesISOWeekInWorkspaceTimezone(t *testing.T) {
	dimension, key, err := normalizeBITeamDimension("week", "2026-W35")
	if err != nil || dimension != biTeamDimensionWeek || key != "2026-W35" {
		t.Fatalf("normalize week=(%q,%q,%v)", dimension, key, err)
	}
	if _, _, err := normalizeBITeamDimension("week", "2025-W53"); err == nil {
		t.Fatal("2025 does not contain ISO week 53")
	}
	location := time.FixedZone("UTC+8", 8*60*60)
	lateSundayUTC := time.Date(2026, time.August, 30, 16, 30, 0, 0, time.UTC)
	if got := biTeamTimeKey(lateSundayUTC, biTeamDimensionWeek, location); got != "2026-W36" {
		t.Fatalf("workspace week key=%q", got)
	}
	if !biTeamRecordMatches(0, lateSundayUTC, biTeamDimensionWeek, "2026-W36", location) {
		t.Fatal("record should match the workspace-local ISO week")
	}
}
