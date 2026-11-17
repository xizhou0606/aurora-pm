package aigorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	"gorm.io/gorm"
	"testing"
)

func TestProjectDeletionDeletesExactWorkspaceSubjectsAndReplays(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&AIRun{}, &AIRunSource{}, &AIArtifact{}, &AIArtifactApplication{}, &AIFeedback{})
	db.Create(&[]AIRun{{ID: 1, WorkspaceID: 7, SubjectType: "plan_unit", SubjectID: 12}, {ID: 2, WorkspaceID: 8, SubjectType: "plan_unit", SubjectID: 12}})
	db.Create(&[]AIRunSource{{WorkspaceID: 7, RunID: 1}, {WorkspaceID: 8, RunID: 2}})
	repository := NewProjectDeletionRepository(db)
	for range 2 {
		if err := repository.DeleteProjectSubjects(context.Background(), 7, ai.ProjectSubjects{PlanUnitIDs: []uint{12}}); err != nil {
			t.Fatal(err)
		}
	}
	var local, foreign int64
	db.Model(&AIRun{}).Where("workspace_id = ?", 7).Count(&local)
	db.Model(&AIRun{}).Where("workspace_id = ?", 8).Count(&foreign)
	if local != 0 || foreign != 1 {
		t.Fatalf("local=%d foreign=%d", local, foreign)
	}
}
