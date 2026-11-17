package workflowgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

func TestProjectImpactCountsOnlyExactWorkspaceSubjects(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkflowInstance{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []WorkflowInstance{
		{ID: 1, WorkspaceID: 7, TemplateID: 1, TemplateVersionID: 1, SubjectType: "plan_unit", SubjectID: 11, Status: "running", CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedByUserID: 1},
		{ID: 2, WorkspaceID: 7, TemplateID: 1, TemplateVersionID: 1, SubjectType: "requirement", SubjectID: 21, Status: "running", CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedByUserID: 1},
		{ID: 3, WorkspaceID: 8, TemplateID: 1, TemplateVersionID: 1, SubjectType: "plan_unit", SubjectID: 11, Status: "running", CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedByUserID: 1},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	count, err := NewProjectImpactRepository(db).CountProjectSubjectReferences(
		context.Background(), 7, workflow.ProjectSubjects{PlanUnitIDs: []uint{11}, RequirementIDs: []uint{21}},
	)
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
