package api

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	bitablegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/bitable/gorm/native"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
)

func openBitableCoordinatorDB(t *testing.T, withPolicies bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := append(bitablegorm.MigrationRecords(), documentlibrarygorm.MigrationRecords()...)
	records = append(records, &auditgorm.AuditLog{}, &auditgorm.OutboxEvent{})
	if withPolicies {
		records = append(records, resourceaccessgorm.MigrationRecords()...)
		records = append(records, collaborationgorm.MigrationRecords()...)
	}
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBitablePrivateCoordinatorCreatesAndReplaysOneAtomicResource(t *testing.T) {
	db := openBitableCoordinatorDB(t, true)
	coordinator := bitablePrivateResourceCoordinator{db: db}
	principal := bitable.Principal{WorkspaceID: 7, UserID: 5, ActorFactUserID: 5}
	operationID := uuid.NewString()
	created, binding, err := coordinator.CreatePrivateBitable(context.Background(), principal, "Roadmap", operationID)
	if err != nil || created.ID == "" || binding.ResourceID == "" || binding.NativeResourceID != created.ID ||
		string(binding.ResourceID) == created.ID || binding.OwnerUserID != principal.UserID {
		t.Fatalf("created=%+v binding=%+v err=%v", created, binding, err)
	}
	resource, content, err := documentlibrarygorm.NewRepository(db).ResolveContent(
		context.Background(), principal.WorkspaceID, documentlibrary.TypeBitable, created.ID,
	)
	if err != nil || resource.ID != binding.ResourceID || content.ResourceID != binding.ResourceID {
		t.Fatalf("resource=%+v content=%+v err=%v", resource, content, err)
	}
	policy, err := resourceaccessgorm.NewRepository(db).GetPolicy(
		context.Background(), principal.WorkspaceID, resource.ID,
	)
	if err != nil || policy.Mode != resourceaccess.ModePrivate || policy.Revision != 1 {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	replayed, replayBinding, err := coordinator.CreatePrivateBitable(
		context.Background(), principal, "Roadmap", operationID,
	)
	if err != nil || replayed.ID != created.ID || replayBinding != binding {
		t.Fatalf("replayed=%+v binding=%+v err=%v", replayed, replayBinding, err)
	}
	var nativeCount, resourceCount, policyCount int64
	if err := db.Model(&bitablegorm.ResourceRecord{}).Count(&nativeCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&documentlibrarygorm.ResourceRecord{}).Count(&resourceCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&resourceaccessgorm.PolicyRecord{}).Count(&policyCount).Error; err != nil {
		t.Fatal(err)
	}
	if nativeCount != 1 || resourceCount != 1 || policyCount != 1 {
		t.Fatalf("native=%d resources=%d policies=%d", nativeCount, resourceCount, policyCount)
	}
}

func TestBitablePrivateCoordinatorRollsBackNativeAndResourceWhenPolicyFails(t *testing.T) {
	db := openBitableCoordinatorDB(t, false)
	coordinator := bitablePrivateResourceCoordinator{db: db}
	_, _, err := coordinator.CreatePrivateBitable(
		context.Background(), bitable.Principal{WorkspaceID: 7, UserID: 5}, "Roadmap", uuid.NewString(),
	)
	if err == nil {
		t.Fatal("expected missing policy storage to fail")
	}
	var nativeCount, resourceCount int64
	if err := db.Model(&bitablegorm.ResourceRecord{}).Count(&nativeCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&documentlibrarygorm.ResourceRecord{}).Count(&resourceCount).Error; err != nil {
		t.Fatal(err)
	}
	if nativeCount != 0 || resourceCount != 0 {
		t.Fatalf("partial creation native=%d resources=%d", nativeCount, resourceCount)
	}
}
