package aigorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
)

func TestProjectImpactCountsOnlyExactWorkspaceSubjects(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AIRun{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []AIRun{
		{ID: 1, WorkspaceID: 7, Capability: "x", SubjectType: "plan_unit", SubjectID: 11, Status: "queued"},
		{ID: 2, WorkspaceID: 8, Capability: "x", SubjectType: "plan_unit", SubjectID: 11, Status: "queued"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	count, err := NewProjectImpactRepository(db).CountProjectSubjectReferences(
		context.Background(), 7, ai.ProjectSubjects{PlanUnitIDs: []uint{11}},
	)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
