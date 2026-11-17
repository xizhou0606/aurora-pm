package httpapi

import (
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

// The grouped loaders must agree with the per-row loader exactly: list
// endpoints render task progress from them.
func TestLoadTaskStatsGroupedMatchesPerRow(t *testing.T) {
	app := newTestApp(t, nil)
	workspace := identitygorm.Workspace{ID: 1, Name: "stats", Slug: "stats", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "stats-team"}
	project := projectgorm.ProjectSet{ID: 20, WorkspaceID: 1, TeamID: 10, Name: "stats-project", Code: "STAT"}
	plan := projectgorm.PlanUnit{ID: 30, WorkspaceID: 1, TeamID: 10, ProjectSetID: 20, Title: "stats-plan"}
	requirement := requirementgorm.Requirement{ID: 40, WorkspaceID: 1, TeamID: 10, ProjectSetID: 20, PlanUnitID: 30, Title: "stats-req"}
	for _, row := range []any{&workspace, &team, &project, &plan, &requirement} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	tasks := []taskgorm.WorkTask{
		{WorkspaceID: 1, TeamID: 10, ProjectSetID: 20, ProjectID: 30, RequirementID: 40, Kind: "task", TaskKey: "S-1", Title: "done", Status: "completed"},
		{WorkspaceID: 1, TeamID: 10, ProjectSetID: 20, ProjectID: 30, RequirementID: 40, Kind: "task", TaskKey: "S-2", Title: "open", Status: "draft"},
	}
	for i := range tasks {
		if err := app.db.Create(&tasks[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	byRequirement := loadTaskStatsByRequirement(app.db, 1, []uint{40})[40]
	byPlanUnit := loadTaskStatsByPlanUnit(app.db, 1, []uint{30})[30]
	byProjectSet := loadTaskStatsByProjectSet(app.db, 1, []uint{20})[20]
	perRow := loadTaskStats(app.db, "workspace_id = ? AND requirement_id = ?", uint(1), uint(40))
	for name, stats := range map[string]taskStats{
		"byRequirement": byRequirement, "byPlanUnit": byPlanUnit, "byProjectSet": byProjectSet,
	} {
		if stats != perRow {
			t.Fatalf("%s = %+v, want per-row %+v", name, stats, perRow)
		}
	}
	if perRow.Total != 2 || perRow.Completed != 1 {
		t.Fatalf("per-row = %+v, want total=2 completed=1", perRow)
	}
}
