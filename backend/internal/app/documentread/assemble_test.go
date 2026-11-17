package documentread

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func sourceDatabase(t *testing.T) (*gorm.DB, *docsource.Service, docsource.ReadContext, map[string]any) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	create := func(value any) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	create(&identitygorm.Workspace{ID: 7, Name: "test", Slug: "test", Enabled: true})
	for _, id := range []uint{11, 12, 13} {
		create(&identitygorm.User{ID: id, ExternalSubject: fmt.Sprint(id), Username: fmt.Sprint(id), Active: true, Status: "active"})
		create(&identitygorm.WorkspaceMember{WorkspaceID: 7, UserID: id, Enabled: true})
	}
	create(&identitygorm.AccessRole{ID: 1, Code: "document-test", Name: "document-test", Enabled: true})
	for _, id := range []uint{11, 12, 13} {
		create(&identitygorm.AccessUserRole{UserID: id, RoleID: 1})
	}
	for i, action := range []accesscatalog.ActionCode{accesscatalog.ActionDocumentsContentView, accesscatalog.ActionAiRunsRunsCreate} {
		method, route := "GET", "/api/v1/platform/documents/:id"
		if i == 1 {
			method, route = "POST", "/api/v1/platform/ai/runs"
		}
		create(&identitygorm.AccessServiceInterface{ID: uint(i + 1), Code: fmt.Sprintf("test-%d", i), Name: "test", ActionCode: string(action), HTTPMethod: method, RouteTemplate: route, Enabled: true})
		create(&identitygorm.AccessRoleServiceInterface{RoleID: 1, ServiceInterfaceID: uint(i + 1)})
	}
	fact := uint(11)
	create(&projectgorm.ProjectSet{ID: 1, WorkspaceID: 7, Name: "project", CreatedByUserID: &fact, Status: "active"})
	create(&projectgorm.PlanUnit{ID: 1, WorkspaceID: 7, ProjectSetID: 1, Title: "version"})
	create(&projectgorm.PlanUnitParticipant{WorkspaceID: 7, PlanUnitID: 1, UserID: 12})
	create(&requirementgorm.Requirement{ID: 1, WorkspaceID: 7, ProjectSetID: 1, PlanUnitID: 1, Title: "requirement", Version: 1})
	raw, hash, plain, err := document.NormalizeContent(json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"原始正文"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	create(&documentgorm.Document{ID: 1, WorkspaceID: 7, NodeID: 1, ContentJSON: string(raw), ContentHash: hash, PlainText: plain, SchemaVersion: 1, Revision: 1, OwnerUserID: 13})
	create(&documentlibrarygorm.ResourceRecord{ID: "doc-1", WorkspaceID: 7, TypeKey: "rich_text", Name: "文档", OwnerUserID: 13, Revision: 1, Lifecycle: "active"})
	create(&documentlibrarygorm.ContentBindingRecord{WorkspaceID: 7, ResourceID: "doc-1", TypeKey: "rich_text", PayloadRef: "1"})
	create(&resourceaccessgorm.PolicyRecord{WorkspaceID: 7, ResourceID: "doc-1", Mode: "restricted", Revision: 1})
	create(&requirementgorm.ResourceLinkRecord{WorkspaceID: 7, RequirementID: 1, ResourceID: "doc-1"})
	service, err := NewSources(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	return db, service, docsource.ReadContext{WorkspaceID: 7, UserID: 11, FactUserID: 11, Method: "POST", Route: "/api/v1/platform/ai/runs"}, map[string]any{"capability": "requirement.completeness", "subject": map[string]any{"id": 1}}
}
func TestAssociatedDocumentAvailableToProjectParticipantsWithoutPersonalGrants(t *testing.T) {
	db, s, actor, input := sourceDatabase(t)
	for _, id := range []uint{11, 12} {
		actor.UserID = id
		actor.FactUserID = id
		m, err := s.Prepare(context.Background(), actor, input)
		if err != nil {
			t.Fatalf("participant %d: %v", id, err)
		}
		snapshots, err := s.Read(context.Background(), m)
		if err != nil || len(snapshots) != 1 || !strings.Contains(snapshots[0].Content, "原始正文") {
			t.Fatalf("read %d: %+v %v", id, snapshots, err)
		}
	}
	var count int64
	db.Model(&resourceaccessgorm.RuleRecord{}).Count(&count)
	if count != 0 {
		t.Fatal("personal grants created")
	}
	actor.UserID = 12
	actor.FactUserID = 12
	m, err := s.Prepare(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("workspace_id = ? AND plan_unit_id = ? AND user_id = ?", 7, 1, 12).Delete(&projectgorm.PlanUnitParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(context.Background(), m); err == nil {
		t.Fatal("removed member retained document access")
	}
	actor.WorkspaceID = 8
	if _, err = s.Prepare(context.Background(), actor, input); err == nil {
		t.Fatal("cross-workspace access")
	}
}
func TestDurableCollaborationProjectionAndFreshness(t *testing.T) {
	db, s, actor, input := sourceDatabase(t)
	now := time.Now().UTC()
	raw, hash, _, err := document.NormalizeContentForSchema(json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"协作新正文"}]}]}`), 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&documentgorm.Document{}).Where("id = ?", 1).Update("collaboration_cutover_at", now).Error; err != nil {
		t.Fatal(err)
	}
	state := collaborationgorm.DocumentStateRecord{WorkspaceID: 7, ResourceID: "doc-1", SourceRevision: 1, SourceHash: hash, DurableRevision: 5, Snapshot: []byte{1}, SnapshotHash: collaboration.SHA256Hex([]byte{1}), StateVector: []byte{2}, StateVectorHash: collaboration.SHA256Hex([]byte{2}), Projection: raw, ProjectionHash: hash, PolicyRevision: 1, SeededAt: now}
	if err := db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	m, err := s.Prepare(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := s.Read(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	if snapshots[0].Revision != "5" || !strings.Contains(snapshots[0].Content, "协作新正文") || strings.Contains(snapshots[0].Content, "原始正文") {
		t.Fatalf("stale projection %+v", snapshots[0])
	}
	if err := db.Model(&collaborationgorm.DocumentStateRecord{}).Where("workspace_id = ? AND resource_id = ?", 7, "doc-1").Update("durable_revision", 6).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(context.Background(), m, snapshots, true); err == nil {
		t.Fatal("concurrent edit accepted")
	}
	if err := db.Where("workspace_id = ? AND resource_id = ?", 7, "doc-1").Delete(&collaborationgorm.DocumentStateRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(context.Background(), m); err == nil {
		t.Fatal("missing projection fell back to stale document columns")
	}
}

func TestAssociatedDocumentStillEnforcesLiveAuthorization(t *testing.T) {
	for _, scenario := range []string{"action revoked", "account banned", "policy private", "resource archived", "unlinked"} {
		t.Run(scenario, func(t *testing.T) {
			db, s, actor, input := sourceDatabase(t)
			m, err := s.Prepare(context.Background(), actor, input)
			if err != nil {
				t.Fatal(err)
			}
			var mutation *gorm.DB
			switch scenario {
			case "action revoked":
				mutation = db.Where("role_id = ? AND service_interface_id = ?", 1, 1).Delete(&identitygorm.AccessRoleServiceInterface{})
			case "account banned":
				mutation = db.Model(&identitygorm.User{}).Where("id = ?", actor.UserID).Update("status", "banned")
			case "policy private":
				mutation = db.Model(&resourceaccessgorm.PolicyRecord{}).Where("resource_id = ?", "doc-1").Update("mode", "private")
			case "resource archived":
				mutation = db.Model(&documentlibrarygorm.ResourceRecord{}).Where("id = ?", "doc-1").Update("lifecycle", "archived")
			case "unlinked":
				mutation = db.Where("workspace_id = ? AND requirement_id = ?", 7, 1).Delete(&requirementgorm.ResourceLinkRecord{})
			}
			if mutation.Error != nil {
				t.Fatal(mutation.Error)
			}
			if _, err := s.Read(context.Background(), m); err == nil {
				t.Fatal("accepted revoked access")
			}
		})
	}
}
