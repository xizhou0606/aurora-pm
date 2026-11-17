package projectgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestImpactRepositoryReturnsOnlyWorkspaceScopedProjectOwnedReferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &DeliveryNode{}, &DeliveryNodeLabel{}, &ProjectPermission{}, &ProjectNews{}); err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&ProjectSet{ID: 1, WorkspaceID: 7, TeamID: 1, Name: "A", Status: "active"},
		&ProjectSet{ID: 2, WorkspaceID: 8, TeamID: 2, Name: "B", Status: "active"},
		&PlanUnit{ID: 11, WorkspaceID: 7, TeamID: 1, ProjectSetID: 1, Title: "V1", Type: "version", Status: "planning"},
		&PlanUnit{ID: 12, WorkspaceID: 8, TeamID: 2, ProjectSetID: 2, Title: "V2", Type: "version", Status: "planning"},
		&DeliveryNodeLabel{ID: 21, WorkspaceID: 7, ProjectSetID: 1, PlanUnitID: 11, Name: "Release"},
		&DeliveryNode{ID: 31, WorkspaceID: 7, PlanUnitID: 11},
		&ProjectPermission{ID: 41, WorkspaceID: 7, TeamID: 1, ProjectSetID: 1, UserID: 3},
		&ProjectNews{ID: 51, WorkspaceID: 7, ProjectSetID: 1, Title: "News"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	impact, err := NewImpactRepository(db).InspectLocalProjectImpact(context.Background(), 7, 1)
	if err != nil || len(impact.PlanUnitIDs) != 1 || impact.PlanUnitIDs[0] != 11 || impact.DeliveryNodeCount != 1 ||
		impact.DeliveryLabelCount != 1 || impact.PermissionCount != 1 || impact.NewsCount != 1 {
		t.Fatalf("impact=%+v err=%v", impact, err)
	}
}
