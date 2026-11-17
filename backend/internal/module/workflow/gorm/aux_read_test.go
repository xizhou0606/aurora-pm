package workflowgorm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAuxReadRepositoryEnforcesWorkspaceInstanceNodeAndPurpose(t *testing.T) {
	db := auxReadDB(t)
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"pre-review","type":"approval","name":"需求预审"},{"id":"cases","type":"human_task","name":"测试用例"},{"id":"report","type":"human_task","name":"测试验收"},{"id":"end","type":"end","name":"结束"}],"edges":[]}`
	version := WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "requirement", SubjectID: 41, Status: "running", CurrentNodeIDs: `[]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now, UpdatedAt: now}
	opinion := WorkflowNodeOpinion{ID: 51, WorkspaceID: 1, InstanceID: 31, NodeID: "pre-review", AuthorUserID: 8, Body: "通过", CreatedAt: now, UpdatedAt: now.Add(time.Minute)}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&opinion).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewAuxReadRepository(db)
	for name, purpose := range map[string]workflow.AuxPurpose{
		"pre-review": workflow.AuxPurposeOpinion,
		"cases":      workflow.AuxPurposeTestCase,
		"report":     workflow.AuxPurposeTestReport,
	} {
		got, err := repository.ResolveAuxNode(context.Background(), 1, 31, name, purpose)
		if err != nil || got.SubjectType != "requirement" || got.SubjectID != 41 || got.NodeID != name {
			t.Fatalf("node=%s got=%+v err=%v", name, got, err)
		}
	}
	if _, err := repository.ResolveAuxNode(context.Background(), 2, 31, "pre-review", workflow.AuxPurposeOpinion); !errors.Is(err, workflow.ErrWorkflowInstanceNotFound) {
		t.Fatalf("cross-workspace err=%v", err)
	}
	if _, err := repository.ResolveAuxNode(context.Background(), 1, 31, "cases", workflow.AuxPurposeOpinion); !errors.Is(err, workflow.ErrWorkflowNodeUnsupported) {
		t.Fatalf("wrong purpose err=%v", err)
	}
	items, err := repository.ListNodeOpinions(context.Background(), 1, 31, "pre-review")
	if err != nil || len(items) != 1 || !items[0].Edited || items[0].AuthorUserID != 8 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	foreign, err := repository.ListNodeOpinions(context.Background(), 2, 31, "pre-review")
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign=%+v err=%v", foreign, err)
	}
}

func TestAuxReadRepositoryFailsClosedForInvalidDefinition(t *testing.T) {
	db := auxReadDB(t)
	now := time.Now().UTC()
	if err := db.Create(&WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: `{"nodes":"invalid"}`, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "requirement", SubjectID: 41, Status: "running", CurrentNodeIDs: `[]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewAuxReadRepository(db).ResolveAuxNode(context.Background(), 1, 31, "review", workflow.AuxPurposeOpinion); !errors.Is(err, workflow.ErrWorkflowAuxDataInvalid) {
		t.Fatalf("err=%v", err)
	}
}

func auxReadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowNodeOpinion{}); err != nil {
		t.Fatal(err)
	}
	return db
}
