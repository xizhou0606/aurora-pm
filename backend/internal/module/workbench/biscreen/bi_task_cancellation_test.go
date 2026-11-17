package biscreen

import (
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func TestCancelledTasksDoNotEnterBIDelayMetrics(t *testing.T) {
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	ownerID := uint(7)
	agg := biAggregation{
		Now: now,
		Tasks: []task.Snapshot{
			{ID: 1, RequirementID: 11, PlanUnitID: 12, Status: "cancelled", AssigneeID: &ownerID, DueAt: &past},
			{ID: 2, RequirementID: 11, PlanUnitID: 12, Status: " CANCELED ", AssigneeID: &ownerID, DueAt: &past},
			{ID: 3, RequirementID: 11, PlanUnitID: 12, Status: "in_dev", AssigneeID: &ownerID, DueAt: &past},
			{ID: 4, RequirementID: 11, PlanUnitID: 12, Status: "draft"},
		},
		Ctx: biWorkspaceContext{Users: map[uint]identity.BIUser{
			ownerID: {ID: ownerID, DisplayName: "Owner"},
		}},
	}

	metrics := agg.overviewMetrics()
	if metrics["overdue_tasks"] != 1 || metrics["in_flight_tasks"] != 2 || metrics["unassigned_tasks"] != 1 {
		t.Fatalf("cancelled tasks leaked into active BI metrics: %+v", metrics)
	}
	rows := agg.dueDistribution()
	if biTaskCancellationDistributionCount(rows, "overdue") != 1 || biTaskCancellationDistributionCount(rows, "unset") != 1 {
		t.Fatalf("cancelled tasks leaked into due distribution: %+v", rows)
	}
	progress := agg.requirementProgress()
	if len(progress) != 1 || progress[0]["overdue_tasks"] != 1 {
		t.Fatalf("cancelled tasks leaked into requirement progress: %+v", progress)
	}
	owners := agg.ownerBoard()
	if len(owners) != 2 || owners[0]["overdue_tasks"] != 1 {
		t.Fatalf("cancelled tasks leaked into owner delay count: %+v", owners)
	}
}

func TestCancelledTasksDoNotEnterTeamOverdueRateSamples(t *testing.T) {
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	ownerID := uint(7)
	agg := biTeamAggregation{
		dimension: biTeamDimensionMonth, dimensionKey: "2026-09", location: time.UTC, now: now,
		rawTasks: []task.Snapshot{
			{ID: 1, PlanUnitID: 12, Status: "cancelled", AssigneeID: &ownerID, CreatedAt: past, DueAt: &past},
			{ID: 2, PlanUnitID: 12, Status: "in_dev", AssigneeID: &ownerID, CreatedAt: past, DueAt: &past},
		},
	}
	role := newBITeamRawRole()
	agg.applyDevelopmentRawMetrics(role)
	if role.total.taskCount != 2 || role.total.overdueSamples != 1 || role.total.overdue != 1 {
		t.Fatalf("cancelled task changed historical workload or overdue samples: %+v", role.total)
	}
}

func biTaskCancellationDistributionCount(rows []map[string]any, key string) int {
	for _, row := range rows {
		if row["key"] == key {
			return row["count"].(int)
		}
	}
	return 0
}
