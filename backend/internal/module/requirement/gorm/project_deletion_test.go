package requirementgorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestProjectDeletionRequiresExactSnapshotAndKeepsForeignWorkspace(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&Requirement{}, &RequirementCover{}, &RequirementMaterial{})
	db.Create(&[]Requirement{{ID: 1, WorkspaceID: 7, ProjectSetID: 11, Title: "A"}, {ID: 2, WorkspaceID: 8, ProjectSetID: 11, Title: "B"}})
	db.Create(&RequirementMaterial{WorkspaceID: 7, RequirementID: 1, Kind: "test", Source: "upload", Name: "x", ObjectKey: "k"})
	repository := NewProjectDeletionRepository(db)
	for range 2 {
		if err := repository.DeleteProjectRequirements(context.Background(), 7, 11, []uint{1}); err != nil {
			t.Fatal(err)
		}
	}
	var local, foreign, materials int64
	db.Model(&Requirement{}).Where("workspace_id = ?", 7).Count(&local)
	db.Model(&Requirement{}).Where("workspace_id = ?", 8).Count(&foreign)
	db.Model(&RequirementMaterial{}).Count(&materials)
	if local != 0 || foreign != 1 || materials != 0 {
		t.Fatalf("local=%d foreign=%d materials=%d", local, foreign, materials)
	}
}
