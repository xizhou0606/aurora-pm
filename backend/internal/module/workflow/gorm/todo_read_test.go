package workflowgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWorkflowTodoReadScopesActiveAssigneeAndDerivesRealEdgeActions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkflowTemplate{}, &WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowToken{}, &WorkflowJob{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	definition := `{"nodes":[{"id":"start","type":"start"},{"id":"review","type":"approval","name":"Review"},{"id":"end","type":"end"}],"edges":[{"source":"start","target":"review"},{"source":"review","target":"end","kind":"approve"},{"source":"review","target":"end","kind":"reject"}]}`
	template := WorkflowTemplate{WorkspaceID: 1, Name: "Requirement flow", SubjectType: "requirement", DraftDefinition: definition, PublishedVersion: 1, CreatedByUserID: 1, UpdatedByUserID: 1}
	if err := db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	version := WorkflowTemplateVersion{WorkspaceID: 1, TemplateID: template.ID, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 1, PublishedAt: now}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	instances := []WorkflowInstance{
		{ID: 301, WorkspaceID: 1, TemplateID: template.ID, TemplateVersionID: version.ID, SubjectType: "requirement", SubjectID: 13, Status: "running", CurrentNodeIDs: `["review"]`, NodePlansJSON: `{"review":{"assignee_user_id":9}}`, StartedByUserID: 1, StartedAt: now, UpdatedAt: now},
		{ID: 302, WorkspaceID: 1, TemplateID: template.ID, TemplateVersionID: version.ID, SubjectType: "requirement", SubjectID: 14, Status: "running", CurrentNodeIDs: `["review"]`, NodePlansJSON: `{"review":{"assignee_user_id":10}}`, StartedByUserID: 1, StartedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&instances).Error; err != nil {
		t.Fatal(err)
	}
	owner, other := uint(9), uint(10)
	tokens := []WorkflowToken{
		{ID: 401, WorkspaceID: 1, InstanceID: 301, NodeID: "review", Status: "active", AssigneeUserID: &owner, CreatedAt: now, UpdatedAt: now},
		{ID: 402, WorkspaceID: 1, InstanceID: 302, NodeID: "review", Status: "active", AssigneeUserID: &other, CreatedAt: now, UpdatedAt: now},
		{ID: 403, WorkspaceID: 2, InstanceID: 301, NodeID: "review", Status: "active", AssigneeUserID: &owner, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	items, err := NewReadRepository(db).ListPrincipalTodos(context.Background(), 1, owner)
	if err != nil || len(items) != 1 || items[0].TokenID != 401 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].ActionKind != "approval" || len(items[0].AllowedActions) != 2 || items[0].AllowedActions[0] != "approve" || items[0].AllowedActions[1] != "reject" {
		t.Fatalf("actions=%s/%v", items[0].ActionKind, items[0].AllowedActions)
	}
}
