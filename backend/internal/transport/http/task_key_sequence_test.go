package httpapi

import (
	"net/http"
	"testing"

	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestCreateTaskSkipsOccupiedImportedTaskKeys(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "sequence-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)

	for _, row := range []any{
		&taskgorm.KeySequence{
			WorkspaceID: 1, ProjectSetID: 11, Prefix: "A", NextValue: 2,
		},
		&taskgorm.WorkTask{
			WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-2",
			Title: "Imported task two", Status: "draft", Priority: "normal",
		},
		&taskgorm.WorkTask{
			WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-3",
			Title: "Imported task three", Status: "draft", Priority: "normal",
		},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", app.token(t, admin), 1,
		map[string]any{"requirement_id": 13, "title": "New task"})
	requireStatus(t, created, http.StatusCreated)
	if key := decodeData(t, created).(map[string]any)["task_key"]; key != "A-4" {
		t.Fatalf("created task_key=%v want=A-4", key)
	}

	var sequence taskgorm.KeySequence
	if err := app.db.Where(
		"workspace_id = ? AND project_set_id = ? AND prefix = ?", 1, 11, "A",
	).First(&sequence).Error; err != nil {
		t.Fatal(err)
	}
	if sequence.NextValue != 5 {
		t.Fatalf("next_value=%d want=5", sequence.NextValue)
	}
}
