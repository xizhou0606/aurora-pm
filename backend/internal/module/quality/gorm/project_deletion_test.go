package qualitygorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestProjectDeletionHardDeletesExactWorkspaceBugsAndDependents(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&QualityBug{}, &QualityBugComment{}, &QualityBugActivity{})
	db.Create(&[]QualityBug{{ID: 1, WorkspaceID: 7, ProjectSetID: 11, BugKey: "A"}, {ID: 2, WorkspaceID: 8, ProjectSetID: 11, BugKey: "B"}})
	db.Create(&QualityBugComment{WorkspaceID: 7, QualityBugID: 1, UserID: 3, Body: "x"})
	repository := NewProjectDeletionRepository(db)
	for range 2 {
		if err := repository.DeleteProjectData(context.Background(), 7, 11); err != nil {
			t.Fatal(err)
		}
	}
	var local, foreign, comments int64
	db.Unscoped().Model(&QualityBug{}).Where("workspace_id = ?", 7).Count(&local)
	db.Unscoped().Model(&QualityBug{}).Where("workspace_id = ?", 8).Count(&foreign)
	db.Model(&QualityBugComment{}).Count(&comments)
	if local != 0 || foreign != 1 || comments != 0 {
		t.Fatalf("local=%d foreign=%d comments=%d", local, foreign, comments)
	}
}
