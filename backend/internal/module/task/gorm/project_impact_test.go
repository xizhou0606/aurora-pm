package taskgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectImpactCountsExactWorkspaceTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []WorkTask{
		{ID: 1, WorkspaceID: 7, ProjectSetID: 11, TaskKey: "PM-1", Title: "A", Status: "todo"},
		{ID: 2, WorkspaceID: 8, ProjectSetID: 11, TaskKey: "PM-2", Title: "B", Status: "todo"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	count, err := NewProjectImpactRepository(db).CountProjectReferences(context.Background(), 7, 11)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
