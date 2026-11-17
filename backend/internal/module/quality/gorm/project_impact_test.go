package qualitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectImpactCountsExactWorkspaceBugs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&QualityBug{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []QualityBug{
		{ID: 1, WorkspaceID: 7, ProjectSetID: 11, BugKey: "BUG-1", Title: "A", Status: "triage"},
		{ID: 2, WorkspaceID: 8, ProjectSetID: 11, BugKey: "BUG-2", Title: "B", Status: "triage"},
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
