package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"gorm.io/gorm"
)

func TestNewBlankDocumentCollaborationIsAtomic(t *testing.T) {
	for _, scenario := range []struct {
		name                         string
		failSeed, nonempty, disabled bool
	}{
		{name: "success"}, {name: "rollback", failSeed: true}, {name: "nonempty", nonempty: true}, {name: "disabled", disabled: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			records := append(documentgorm.MigrationRecords(), documentlibrarygorm.MigrationRecords()...)
			records = append(records, resourceaccessgorm.MigrationRecords()...)
			records = append(records, collaborationgorm.MigrationRecords()...)
			if err = db.AutoMigrate(records...); err != nil {
				t.Fatal(err)
			}
			repository := documentgorm.NewRepository(db)
			repository.ConfigureResourceLifecycleFactory(newDocumentResourceLifecycle)
			if !scenario.disabled {
				repository.ConfigureNewDocumentCollaboration(newDocumentCollaborationFactory)
			}
			space, err := repository.EnsureDefaultSpace(context.Background(), 7)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.failSeed {
				if err = db.Callback().Create().Before("gorm:create").Register("test_seed_failure", func(tx *gorm.DB) {
					if tx.Statement.Table == "collaboration_document_states" {
						tx.AddError(errors.New("seed failed"))
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			raw := document.DefaultContent()
			if scenario.nonempty {
				raw = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"imported content"}]}]}`)
			}
			content, hash, plain, err := document.NormalizeContent(raw)
			if err != nil {
				t.Fatal(err)
			}
			create := repository.CreateCollaborativeNode
			if scenario.disabled {
				create = repository.CreateNode
			}
			_, body, err := create(context.Background(), document.Node{WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument, Title: "New", CreatedBy: 5, UpdatedBy: 5}, &document.Document{WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 1, ContentHash: hash, PlainText: plain, OwnerUserID: 5, CreatedBy: 5, UpdatedBy: 5})
			if scenario.failSeed {
				if err == nil {
					t.Fatal("seed failure must fail creation")
				}
				for _, table := range []string{"document_nodes", "documents", "document_resources", "document_resource_access_policies", "collaboration_document_states"} {
					var count int64
					if countErr := db.Table(table).Count(&count).Error; countErr != nil || count != 0 {
						t.Fatalf("%s rows=%d error=%v", table, count, countErr)
					}
				}
				return
			}
			if scenario.disabled || scenario.nonempty {
				if err != nil || body == nil || body.CollaborationCutoverAt != nil {
					t.Fatalf("legacy creation changed: %+v err=%v", body, err)
				}
				var count int64
				if err := db.Table("collaboration_document_states").Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("unexpected seed count=%d err=%v", count, err)
				}
				return
			}
			if err != nil || body == nil || body.CollaborationCutoverAt == nil {
				t.Fatalf("body=%+v err=%v", body, err)
			}
			stored, err := repository.GetDocument(context.Background(), 7, body.ID)
			if err != nil || stored.CollaborationCutoverAt == nil {
				t.Fatalf("persisted marker missing: %v", err)
			}
			states, _ := collaborationgorm.NewRepository(db)
			seed, err := states.LoadDocumentState(context.Background(), collaboration.Room{WorkspaceID: 7, ResourceID: body.ResourceID})
			if err != nil || seed.DurableRevision != 1 || seed.ProjectionHash != hash {
				t.Fatalf("seed=%+v err=%v", seed, err)
			}
		})
	}
}
