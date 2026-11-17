package qualitygorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

func TestBugTodoReadUsesPrincipalLifecycleOwnershipAndWorkspaceScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&QualityBug{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	owner, other := uint(9), uint(10)
	rows := []QualityBug{
		{ID: 201, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, BugKey: "BUG-201", Title: "Owned triage", Status: "triage", Priority: "high", AssigneeUserID: &owner, StatusChangedAt: now},
		{ID: 202, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, BugKey: "BUG-202", Title: "Owned review", Status: "review", Priority: "normal", AssigneeUserID: &other, ReviewerUserID: &owner, StatusChangedAt: now},
		{ID: 203, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, BugKey: "BUG-203", Title: "Not review owner", Status: "review", Priority: "normal", AssigneeUserID: &owner, ReviewerUserID: &other, StatusChangedAt: now},
		{ID: 204, WorkspaceID: 2, TeamID: 2, ProjectSetID: 21, ProjectID: 22, RequirementID: 23, BugKey: "BUG-204", Title: "Foreign", Status: "triage", Priority: "urgent", AssigneeUserID: &owner, StatusChangedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewBugReadRepository(db)
	page, err := repository.ListPrincipalBugTodos(context.Background(), 1, owner, quality.BugTodoFilter{Limit: 20, Now: now})
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	ids := map[uint]bool{}
	for _, item := range page.Items {
		ids[item.ID] = true
	}
	if !ids[201] || !ids[202] || ids[203] || ids[204] {
		t.Fatalf("ids=%v", ids)
	}
}
