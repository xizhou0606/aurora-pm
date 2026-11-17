package workflowgorm

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

func TestReadRepositoryScopesWorkflowQueriesByWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkflowTemplate{}, &WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowNodeRun{}, &WorkflowApproval{}); err != nil {
		t.Fatal(err)
	}
	template := WorkflowTemplate{WorkspaceID: 21, Name: "Requirement flow", SubjectType: "requirement", DraftDefinition: `{"nodes":[],"edges":[]}`, PublishedVersion: 1, CreatedByUserID: 1, UpdatedByUserID: 1}
	if err := db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	version := WorkflowTemplateVersion{WorkspaceID: 21, TemplateID: template.ID, Version: 1, Definition: `{"nodes":[],"edges":[]}`, DefinitionHash: "hash", PublishedByUser: 1}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	instance := WorkflowInstance{WorkspaceID: 21, TemplateID: template.ID, TemplateVersionID: version.ID, SubjectType: "requirement", SubjectID: 99, Status: "running", CurrentNodeIDs: `[]`, NodePlansJSON: `{}`, StartedByUserID: 1}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewReadRepository(db)
	items, err := repository.ListTemplates(context.Background(), 21, workflow.TemplateFilter{SubjectType: "requirement"})
	if err != nil || len(items) != 1 || items[0].WorkspaceID != 21 {
		t.Fatalf("templates=%+v err=%v", items, err)
	}
	otherItems, err := repository.ListTemplates(context.Background(), 22, workflow.TemplateFilter{})
	if err != nil || len(otherItems) != 0 {
		t.Fatalf("cross-workspace templates=%+v err=%v", otherItems, err)
	}
	if _, err := repository.GetInstance(context.Background(), 22, instance.ID); !errors.Is(err, workflow.ErrWorkflowInstanceNotFound) {
		t.Fatalf("cross-workspace instance err=%v", err)
	}
}
