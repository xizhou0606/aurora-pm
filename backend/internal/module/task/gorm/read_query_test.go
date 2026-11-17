package taskgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func TestReadRepositoryResolvesExactReferenceWithinWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}); err != nil {
		t.Fatal(err)
	}
	parent := WorkTask{WorkspaceID: 3, TeamID: 30, ProjectSetID: 31, ProjectID: 32, RequirementID: 33, Kind: "task", TaskKey: "PM-1", Title: "Exact", Status: "draft", Priority: "normal"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	child := WorkTask{WorkspaceID: 3, TeamID: 30, ProjectSetID: 31, ProjectID: 32, RequirementID: 33, Kind: "subtask", ParentID: &parent.ID, TaskKey: "PM-2", Title: "Child", Status: "draft", Priority: "normal"}
	other := WorkTask{WorkspaceID: 4, TeamID: 40, ProjectSetID: 41, ProjectID: 42, RequirementID: 43, Kind: "task", TaskKey: "PM-1", Title: "Other", Status: "draft", Priority: "normal"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	deleting := WorkTask{WorkspaceID: 3, TeamID: 30, ProjectSetID: 31, ProjectID: 32, RequirementID: 33, Kind: "task", TaskKey: "PM-3", Title: "Deleting", Status: "draft", Priority: "normal", MutationState: workTaskStateDeleting}
	if err := db.Create(&deleting).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReadRepository(db)
	got, err := repository.GetTask(context.Background(), 3, task.Lookup{Reference: "PM-1"})
	if err != nil || got.ID != parent.ID || len(got.Children) != 1 || got.Children[0].ID != child.ID {
		t.Fatalf("task snapshot: got=%+v err=%v", got, err)
	}
	if _, err := repository.GetTask(context.Background(), 5, task.Lookup{Reference: "PM-1"}); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("cross-workspace task err=%v", err)
	}
	if _, err := repository.GetTask(context.Background(), 3, task.Lookup{ID: deleting.ID}); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("deleting task must be hidden: %v", err)
	}
}
