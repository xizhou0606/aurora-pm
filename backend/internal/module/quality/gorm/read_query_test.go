package qualitygorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"gorm.io/gorm"
)

func TestBugReadRepositoryScopesRowsAndActivitiesByWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&QualityBug{}, &QualityBugActivity{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	row := QualityBug{WorkspaceID: 7, TeamID: 1, ProjectSetID: 2, ProjectID: 3, RequirementID: 4, BugKey: "BUG-7", Title: "Login fails", Status: "triage", Priority: "high", StatusChangedAt: now}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	uiRow := QualityBug{WorkspaceID: 7, TeamID: 1, ProjectSetID: 2, ProjectID: 3, RequirementID: 4, BugKey: "BUG-8", Title: "UI overlap", Status: "triage", Priority: "high", DefectDomain: quality.BugDefectDomainUI, StatusChangedAt: now}
	functionalRow := QualityBug{WorkspaceID: 7, TeamID: 1, ProjectSetID: 2, ProjectID: 3, RequirementID: 4, BugKey: "BUG-9", Title: "Functional regression", Status: "triage", Priority: "high", DefectDomain: quality.BugDefectDomainFunctional, StatusChangedAt: now}
	if err := db.Create(&uiRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&functionalRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&QualityBugActivity{WorkspaceID: 7, QualityBugID: row.ID, UserID: 9, Action: "created"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewBugReadRepository(db)
	page, err := repository.ListBugs(context.Background(), 7, quality.BugReadFilter{Query: "login", Limit: 10})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].WorkspaceID != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].DefectDomain != quality.BugDefectDomainFunctional {
		t.Fatalf("legacy defect domain=%q", page.Items[0].DefectDomain)
	}
	functional, err := repository.ListBugs(context.Background(), 7, quality.BugReadFilter{DefectDomains: []string{quality.BugDefectDomainFunctional}, Limit: 10})
	if err != nil || functional.Total != 2 {
		t.Fatalf("functional page=%+v err=%v", functional, err)
	}
	ui, err := repository.ListBugs(context.Background(), 7, quality.BugReadFilter{DefectDomains: []string{quality.BugDefectDomainUI}, Limit: 10})
	if err != nil || ui.Total != 1 || ui.Items[0].ID != uiRow.ID {
		t.Fatalf("ui page=%+v err=%v", ui, err)
	}
	if _, err := repository.GetBug(context.Background(), 8, row.ID); !errors.Is(err, quality.ErrBugReadNotFound) {
		t.Fatalf("cross-workspace get err=%v", err)
	}
	activities, err := repository.ListBugActivities(context.Background(), 7, row.ID, 10, 0)
	if err != nil || activities.Total != 1 || len(activities.Items) != 1 {
		t.Fatalf("activities=%+v err=%v", activities, err)
	}
	if _, err := repository.ListBugActivities(context.Background(), 8, row.ID, 10, 0); !errors.Is(err, quality.ErrBugReadNotFound) {
		t.Fatalf("cross-workspace activities err=%v", err)
	}
}
