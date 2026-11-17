package aigorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunReadRepositoryScopesStatusesByWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AIRun{}); err != nil {
		t.Fatal(err)
	}
	rows := []AIRun{
		{ID: 501, WorkspaceID: 1, Capability: "requirement.completeness", SubjectType: "requirement", SubjectID: 13, SubjectVersion: 1, Status: "FAILED", Model: "test", InputJSON: `{}`, OutputJSON: `{}`, RequestedByUser: 9},
		{ID: 502, WorkspaceID: 2, Capability: "requirement.completeness", SubjectType: "requirement", SubjectID: 23, SubjectVersion: 1, Status: "completed", Model: "test", InputJSON: `{}`, OutputJSON: `{}`, RequestedByUser: 9},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	items, err := NewRunReadRepository(db).ListRunStatuses(context.Background(), 1, []uint{501, 502})
	if err != nil || len(items) != 1 || items[0].ID != 501 || items[0].Status != "FAILED" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}
