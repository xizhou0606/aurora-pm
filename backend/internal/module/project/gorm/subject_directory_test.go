package projectgorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func TestProjectSetSubjectDirectorySearchResolveAndDynamicMembership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ProjectSet{}, &PlanUnit{}, &PlanUnitParticipant{}); err != nil {
		t.Fatal(err)
	}
	creator, unitOwner, participant := uint(41), uint(42), uint(43)
	sets := []ProjectSet{
		{ID: 11, WorkspaceID: 7, TeamID: 1, Name: "Alpha", Code: "A", CreatedByUserID: &creator, Status: "active"},
		{ID: 12, WorkspaceID: 7, TeamID: 1, Name: "Alpha Two", Code: "B", Status: "active"},
		{ID: 13, WorkspaceID: 7, TeamID: 1, Name: "Archived", Code: "C", Status: "archived"},
		{ID: 21, WorkspaceID: 8, TeamID: 1, Name: "Alpha Secret", Code: "X", CreatedByUserID: &creator, Status: "active"},
	}
	if err := db.Create(&sets).Error; err != nil {
		t.Fatal(err)
	}
	units := []PlanUnit{
		{ID: 101, WorkspaceID: 7, TeamID: 1, ProjectSetID: 12, Title: "V1", OwnerUserID: &unitOwner, Status: "planning"},
		{ID: 201, WorkspaceID: 8, TeamID: 1, ProjectSetID: 21, Title: "Secret", OwnerUserID: &unitOwner, Status: "planning"},
	}
	if err := db.Create(&units).Error; err != nil {
		t.Fatal(err)
	}
	member := PlanUnitParticipant{WorkspaceID: 7, PlanUnitID: 101, UserID: participant, CreatedByUserID: creator}
	if err := db.Create(&member).Error; err != nil {
		t.Fatal(err)
	}

	service, err := project.NewProjectSetSubjectService(NewProjectSetSubjectRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.SearchProjectSetSubjects(context.Background(), 7, project.ProjectSetSubjectSearch{
		Query: "alpha", Page: 1, PageSize: 1,
	})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != "11" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	resolved, err := service.ResolveProjectSetSubjects(context.Background(), 7, []string{"12"})
	if err != nil || len(resolved) != 1 || resolved[0].DisplayName != "Alpha Two" {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	if _, err := service.ResolveProjectSetSubjects(context.Background(), 7, []string{"21"}); err == nil {
		t.Fatal("cross-workspace project resolved")
	}
	if _, err := service.ResolveProjectSetSubjects(context.Background(), 7, []string{"13"}); err == nil {
		t.Fatal("inactive project resolved")
	}

	for _, userID := range []uint{creator, unitOwner, participant} {
		facts, err := service.MatchProjectSetSubjects(context.Background(), 7, userID, []string{"11", "12", "21"})
		if err != nil {
			t.Fatal(err)
		}
		if userID == creator && (!facts["11"] || facts["12"] || facts["21"]) {
			t.Fatalf("creator facts=%+v", facts)
		}
		if userID != creator && (facts["11"] || !facts["12"] || facts["21"]) {
			t.Fatalf("member %d facts=%+v", userID, facts)
		}
	}
	if err := db.Delete(&member).Error; err != nil {
		t.Fatal(err)
	}
	facts, err := service.MatchProjectSetSubjects(context.Background(), 7, participant, []string{"12"})
	if err != nil || facts["12"] {
		t.Fatalf("removed participant facts=%+v err=%v", facts, err)
	}
}
