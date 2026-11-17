package taskgorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestProjectDeletionDeletesExactWorkspaceTasksAndDependents(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&WorkTask{}, &WorkTaskComment{}, &WorkTaskActivity{}, &WorkTaskWatcher{}, &WorkTaskAttachment{}, &TaskTestDefinition{}, &TaskTestExecution{}, &TaskTestExecutionRun{}, &KeySequence{})
	db.Create(&[]WorkTask{{ID: 1, WorkspaceID: 7, ProjectSetID: 11, TaskKey: "A"}, {ID: 2, WorkspaceID: 8, ProjectSetID: 11, TaskKey: "B"}})
	db.Create(&WorkTaskComment{WorkspaceID: 7, WorkTaskID: 1, UserID: 3, Body: "x"})
	repository := NewProjectDeletionRepository(db)
	for range 2 {
		if err := repository.DeleteProjectData(context.Background(), 7, 11); err != nil {
			t.Fatal(err)
		}
	}
	var local, foreign, comments int64
	db.Model(&WorkTask{}).Where("workspace_id = ?", 7).Count(&local)
	db.Model(&WorkTask{}).Where("workspace_id = ?", 8).Count(&foreign)
	db.Model(&WorkTaskComment{}).Count(&comments)
	if local != 0 || foreign != 1 || comments != 0 {
		t.Fatalf("local=%d foreign=%d comments=%d", local, foreign, comments)
	}
}
