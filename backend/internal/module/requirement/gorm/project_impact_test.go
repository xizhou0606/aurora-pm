package requirementgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectImpactReturnsExactRequirementIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Requirement{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []Requirement{
		{ID: 1, WorkspaceID: 7, ProjectSetID: 11, Title: "A", Status: "backlog", Version: 1},
		{ID: 2, WorkspaceID: 8, ProjectSetID: 11, Title: "B", Status: "backlog", Version: 1},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	impact, err := NewProjectImpactRepository(db).InspectProjectImpact(context.Background(), 7, 11)
	if err != nil || impact.Count != 1 || len(impact.RequirementIDs) != 1 || impact.RequirementIDs[0] != 1 {
		t.Fatalf("impact=%+v err=%v", impact, err)
	}
}
