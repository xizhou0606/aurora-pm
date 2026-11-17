package api

import (
	"context"
	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	accessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestHistoricalInboxEnrichmentRemainsRecipientAndWorkspaceScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}, &identitygorm.User{}, &projectgorm.ProjectSet{}, &accessgorm.AccessRequestRecord{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&identitygorm.User{ID: 110, Username: "sysadmin", DisplayName: "PM Administrator", Active: true},
		&projectgorm.ProjectSet{ID: 17, WorkspaceID: 7, Name: "PM", Code: "PM"},
		&projectgorm.ProjectSet{ID: 18, WorkspaceID: 8, Name: "Private", Code: "PRIVATE"},
		&accessgorm.AccessRequestRecord{ID: 31, WorkspaceID: 7, TargetKind: "project_set", ApplicantUserID: 110, Status: resourceaccess.RequestPending, CreatedAt: time.Now()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	intent := notification.Intent{WorkspaceID: 7, TeamID: 1, Scene: notification.SceneProjectAccessRequested, ResourceType: "project_set", ResourceID: "17", RequestID: 31, ActorUserID: 110, RecipientUserIDs: []uint{8}, FallbackPlainText: "申请加入", SkipExternal: true, Vars: map[string]string{}}
	if err := notification.Enqueue(db, "membership_access_request", 31, intent); err != nil {
		t.Fatal(err)
	}
	repo := inboxRepository(db)
	page, err := repo.ListInbox(context.Background(), 7, 8, notification.InboxFilter{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	item := page.Items[0]
	if item.ResourceTitle != "PM" || item.ActorName != "PM Administrator" || item.RequestStatus != "pending" || item.GrantLevel != "member" {
		t.Fatalf("item=%+v", item)
	}
	if err := db.Model(&accessgorm.AccessRequestRecord{}).Where("id = ?", 31).Update("status", "approved").Error; err != nil {
		t.Fatal(err)
	}
	page, err = repo.ListInbox(context.Background(), 7, 8, notification.InboxFilter{})
	if err != nil || page.Items[0].RequestStatus != "approved" {
		t.Fatalf("stale status: %+v %v", page, err)
	}
	other, err := repo.ListInbox(context.Background(), 7, 9, notification.InboxFilter{})
	if err != nil || len(other.Items) != 0 {
		t.Fatalf("recipient leak: %+v %v", other, err)
	}
	item.ResourceID = "18"
	if title := inboxResourceTitle(context.Background(), db, 7, item); title != "" {
		t.Fatalf("workspace title leak: %s", title)
	}
}
