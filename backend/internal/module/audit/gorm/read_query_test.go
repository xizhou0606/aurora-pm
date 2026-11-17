package auditgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"gorm.io/gorm"
)

func TestProjectAuditReadIsWorkspaceScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:project_audit_read?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]AuditLog{
		{ID: 1, WorkspaceID: 7, ProjectID: 11, Resource: "project_set", Action: "updated"},
		{ID: 2, WorkspaceID: 8, ProjectID: 11, Resource: "project_set", Action: "secret"},
		{ID: 3, WorkspaceID: 7, ProjectID: 12, Resource: "project_set", Action: "other"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	service, err := audit.NewReadService(NewReadRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListProjectEvents(context.Background(), 7, 11, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != 1 || page.Items[0].WorkspaceID != 7 {
		t.Fatalf("page=%+v", page)
	}
}
