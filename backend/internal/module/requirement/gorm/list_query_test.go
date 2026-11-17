package requirementgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
)

func TestRequirementListQueryIsWorkspaceScopedAndEscapesKeywords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:requirement_list_query?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Requirement{}); err != nil {
		t.Fatal(err)
	}
	planUnitID := uint(21)
	rows := []Requirement{
		{ID: 1, WorkspaceID: 7, TeamID: 2, ProjectSetID: 11, ScheduledPlanUnitID: &planUnitID, Title: "100% ready", Priority: "normal", Status: "backlog"},
		{ID: 2, WorkspaceID: 7, TeamID: 2, ProjectSetID: 12, ScheduledPlanUnitID: &planUnitID, Title: "other", Priority: "normal", Status: "backlog"},
		{ID: 3, WorkspaceID: 8, TeamID: 2, ProjectSetID: 11, ScheduledPlanUnitID: &planUnitID, Title: "100% ready", Priority: "normal", Status: "backlog"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	service, err := requirement.NewListService(NewReadRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListRequirements(context.Background(), 7, requirement.ListFilter{ProjectSetID: 11, Query: "100%", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != 1 || page.Items[0].WorkspaceID != 7 {
		t.Fatalf("page=%+v", page)
	}
	scope, err := service.ListScopeRequirements(context.Background(), 7, []uint{11})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope) != 1 || scope[0].ID != 1 {
		t.Fatalf("scope=%+v", scope)
	}
}
