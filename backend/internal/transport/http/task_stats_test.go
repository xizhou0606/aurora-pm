package httpapi

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"testing"
)

func TestLoadTaskStatsGroupsStatusesAndExcludesNonPMKinds(t *testing.T) {
	app := newTestApp(t, nil)
	tasks := []taskgorm.WorkTask{
		taskStatsTestTask("TASK-1", "task", "completed"),
		taskStatsTestTask("TASK-2", "subtask", "done"),
		taskStatsTestTask("TASK-3", "task", "cancelled"),
		taskStatsTestTask("TASK-4", "task", "in_dev"),
		taskStatsTestTask("TASK-5", "subtask", "pending_test"),
		taskStatsTestTask("TASK-6", "task", "draft"),
		taskStatsTestTask("TASK-7", "task", "blocked"),
		taskStatsTestTask("TASK-8", " TASK ", "completed"),
		taskStatsTestTask("BUG-1", "bug", "completed"),
	}
	if err := app.db.Create(&tasks).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}

	stats := loadTaskStats(
		app.db,
		"workspace_id = ? AND requirement_id = ?",
		uint(1),
		uint(30),
	)
	want := taskStats{
		Total:      8,
		Completed:  3,
		Open:       4,
		InProgress: 2,
		Todo:       1,
	}
	if stats != want {
		t.Fatalf("task stats = %+v, want %+v", stats, want)
	}
}

func taskStatsTestTask(key, kind, status string) taskgorm.WorkTask {
	return taskgorm.WorkTask{
		WorkspaceID:   1,
		TeamID:        2,
		ProjectSetID:  10,
		ProjectID:     20,
		RequirementID: 30,
		Kind:          kind,
		TaskKey:       key,
		Title:         key,
		Status:        status,
		Priority:      "normal",
	}
}
