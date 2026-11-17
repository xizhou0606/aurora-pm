package workflowgorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"testing"
)

func TestProjectDeletionDeletesExactWorkspaceWorkflowSubjectsAndReplays(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&WorkflowInstance{}, &WorkflowToken{}, &WorkflowNodeRun{}, &WorkflowApproval{}, &WorkflowNodeOpinion{}, &WorkflowJob{}, &WorkflowNodeArtifact{})
	db.Create(&[]WorkflowInstance{{ID: 1, WorkspaceID: 7, SubjectType: "requirement", SubjectID: 13}, {ID: 2, WorkspaceID: 8, SubjectType: "requirement", SubjectID: 13}})
	db.Create(&WorkflowToken{WorkspaceID: 7, InstanceID: 1, NodeID: "start", Status: "active"})
	repository := NewProjectDeletionRepository(db)
	for range 2 {
		if err := repository.DeleteProjectSubjects(context.Background(), 7, workflow.ProjectSubjects{RequirementIDs: []uint{13}}); err != nil {
			t.Fatal(err)
		}
	}
	var local, foreign, tokens int64
	db.Model(&WorkflowInstance{}).Where("workspace_id = ?", 7).Count(&local)
	db.Model(&WorkflowInstance{}).Where("workspace_id = ?", 8).Count(&foreign)
	db.Model(&WorkflowToken{}).Where("workspace_id = ?", 7).Count(&tokens)
	if local != 0 || foreign != 1 || tokens != 0 {
		t.Fatalf("local=%d foreign=%d tokens=%d", local, foreign, tokens)
	}
}
