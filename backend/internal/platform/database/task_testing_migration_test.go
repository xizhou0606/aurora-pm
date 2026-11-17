package database

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"testing"
)

func TestMigrateRegistersTaskTestingModels(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]any{
		"definition": &taskgorm.TaskTestDefinition{},
		"execution":  &taskgorm.TaskTestExecution{},
		"run":        &taskgorm.TaskTestExecutionRun{},
	} {
		if !db.Migrator().HasTable(target) {
			t.Fatalf("task testing %s table was not migrated", name)
		}
	}
	if !db.Migrator().HasColumn(&taskgorm.TaskTestDefinition{}, "version") ||
		!db.Migrator().HasColumn(&taskgorm.TaskTestExecutionRun{}, "quality_bug_id") ||
		!db.Migrator().HasColumn(&taskgorm.TaskTestExecutionRun{}, "step_results_json") {
		t.Fatal("task testing concurrency or bug link columns were not migrated")
	}
}
