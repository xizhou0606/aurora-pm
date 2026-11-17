package projectgorm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func openMemberDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&ProjectSet{}, &PlanUnit{}, &ProjectMemberMutationAudit{},
		&ProjectSetMembership{}, &PlanUnitParticipant{}, &PlanUnitMemberMutationAudit{},
		&identitygorm.User{}, &identitygorm.WorkspaceMember{},
	); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		"CREATE TABLE work_tasks (id integer PRIMARY KEY, workspace_id integer, project_id integer, assignee_id integer, status text, title text)",
		"CREATE TABLE quality_bugs (id integer PRIMARY KEY, workspace_id integer, requirement_id integer, assignee_user_id integer, status text, title text)",
		"CREATE TABLE requirements (id integer PRIMARY KEY, version_task_new_id integer)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedActiveUser(t *testing.T, db *gorm.DB, id uint, username string) {
	t.Helper()
	if err := db.Create(&identitygorm.User{ID: id, ExternalSubject: username, Username: username, Active: true, Status: "active"}).Error; err != nil {
		t.Fatalf("seed user %d: %v", id, err)
	}
}

func seedWorkspaceMember(t *testing.T, db *gorm.DB, workspaceID, userID uint) {
	t.Helper()
	if err := db.Create(&identitygorm.WorkspaceMember{WorkspaceID: workspaceID, UserID: userID, Enabled: true}).Error; err != nil {
		t.Fatalf("seed workspace member %d/%d: %v", workspaceID, userID, err)
	}
}

func createCreatorProject(t *testing.T, db *gorm.DB, workspaceID, creatorID uint) ProjectSet {
	t.Helper()
	creator := creatorID
	proj := ProjectSet{WorkspaceID: workspaceID, TeamID: 1, Name: "Test Project", CreatedByUserID: &creator, Status: "active"}
	if err := db.Create(&proj).Error; err != nil {
		t.Fatal(err)
	}
	return proj
}

func createUnit(t *testing.T, db *gorm.DB, workspaceID, projectSetID, ownerID uint) PlanUnit {
	t.Helper()
	owner := ownerID
	unit := PlanUnit{WorkspaceID: workspaceID, TeamID: 1, ProjectSetID: projectSetID, Title: "Unit", OwnerUserID: &owner, Status: "planning"}
	if err := db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	return unit
}

func TestPlanUnitMemberRepositoryLoadAndSave(t *testing.T) {
	db := openMemberDB(t)
	repo := NewPlanUnitMemberRepository(db)
	proj := createCreatorProject(t, db, 927, 490)
	unit := createUnit(t, db, 927, proj.ID, 7)
	ctx := context.Background()

	owner := uint(7)
	if err := db.Create(&PlanUnitParticipant{WorkspaceID: 927, PlanUnitID: unit.ID, UserID: 20, CreatedByUserID: 7, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}

	t.Run("load returns owner and participants", func(t *testing.T) {
		state, err := repo.LoadPlanUnitMemberState(ctx, 927, unit.ID)
		if err != nil {
			t.Fatalf("load: err=%v", err)
		}
		if state.OwnerUserID == nil || *state.OwnerUserID != owner {
			t.Fatalf("owner=%v", state.OwnerUserID)
		}
		if len(state.ParticipantUserIDs) != 1 || state.ParticipantUserIDs[0] != 20 {
			t.Fatalf("participants=%v", state.ParticipantUserIDs)
		}
	})

	t.Run("save replaces members and writes audit", func(t *testing.T) {
		seedActiveUser(t, db, 7, "u7")
		seedActiveUser(t, db, 8, "u8")
		seedActiveUser(t, db, 21, "u21")
		seedWorkspaceMember(t, db, 927, 7)
		seedWorkspaceMember(t, db, 927, 8)
		seedWorkspaceMember(t, db, 927, 21)

		newOwner := uint(8)
		saved, err := repo.SavePlanUnitMemberState(ctx, 927, unit.ID, project.MemberState{
			OwnerUserID:        &newOwner,
			ParticipantUserIDs: []uint{7, 21},
		}, project.MemberMutationMeta{ActorUserID: 8, Reason: "rebalance"})
		if err != nil {
			t.Fatalf("save: err=%v", err)
		}
		if saved.OwnerUserID == nil || *saved.OwnerUserID != newOwner {
			t.Fatalf("saved owner=%v", saved.OwnerUserID)
		}
		if len(saved.ParticipantUserIDs) != 2 {
			t.Fatalf("saved participants=%v", saved.ParticipantUserIDs)
		}
		var audits int64
		db.Model(&PlanUnitMemberMutationAudit{}).Where("plan_unit_id = ?", unit.ID).Count(&audits)
		if audits != 1 {
			t.Fatalf("audit rows=%d", audits)
		}
	})

	t.Run("stale expected timestamp is rejected without write", func(t *testing.T) {
		stale := time.Now().Add(-time.Hour).UTC()
		_, err := repo.SavePlanUnitMemberState(ctx, 927, unit.ID, project.MemberState{
			OwnerUserID: func() *uint { v := uint(7); return &v }(),
		}, project.MemberMutationMeta{ActorUserID: 7, ExpectedAt: &stale})
		if err != project.ErrParticipantStale {
			t.Fatalf("stale: err=%v", err)
		}
	})

	t.Run("owner cannot be participant", func(t *testing.T) {
		_, err := repo.SavePlanUnitMemberState(ctx, 927, unit.ID, project.MemberState{
			OwnerUserID:        func() *uint { v := uint(7); return &v }(),
			ParticipantUserIDs: []uint{7},
		}, project.MemberMutationMeta{ActorUserID: 7})
		if err != project.ErrParticipantIsOwner {
			t.Fatalf("owner-as-participant: err=%v", err)
		}
	})

	t.Run("unknown unit is not found", func(t *testing.T) {
		_, err := repo.LoadPlanUnitMemberState(ctx, 927, 999999)
		if err != project.ErrParticipantPlanUnitNotFound {
			t.Fatalf("unknown unit: err=%v", err)
		}
	})

	t.Run("cross workspace unit is not found", func(t *testing.T) {
		_, err := repo.LoadPlanUnitMemberState(ctx, 928, unit.ID)
		if err != project.ErrParticipantPlanUnitNotFound {
			t.Fatalf("cross workspace: err=%v", err)
		}
	})
}

func TestPlanUnitMemberRepositoryList(t *testing.T) {
	db := openMemberDB(t)
	repo := NewPlanUnitMemberRepository(db)
	proj := createCreatorProject(t, db, 930, 490)
	unit := createUnit(t, db, 930, proj.ID, 7)
	if err := db.Create(&PlanUnitParticipant{WorkspaceID: 930, PlanUnitID: unit.ID, UserID: 8, CreatedByUserID: 7, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	seedActiveUser(t, db, 7, "Feng")
	seedActiveUser(t, db, 8, "Allen")
	ctx := context.Background()

	page, err := repo.ListPlanUnitMemberUsers(ctx, 930, unit.ID, "", 1, 20)
	if err != nil {
		t.Fatalf("list: err=%v", err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page=%+v", page)
	}
	filtered, err := repo.ListPlanUnitMemberUsers(ctx, 930, unit.ID, "allen", 1, 20)
	if err != nil {
		t.Fatalf("filtered list: err=%v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].Username != "Allen" {
		t.Fatalf("filtered=%+v", filtered)
	}
}

func TestPlanUnitAccessRepositoryFacts(t *testing.T) {
	db := openMemberDB(t)
	access := NewPlanUnitAccessRepository(db)
	proj := createCreatorProject(t, db, 931, 490)
	unit := createUnit(t, db, 931, proj.ID, 7)
	if err := db.Create(&PlanUnitParticipant{WorkspaceID: 931, PlanUnitID: unit.ID, UserID: 20, CreatedByUserID: 7, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("creator resolved", func(t *testing.T) {
		creator, err := access.ProjectCreatorID(ctx, 931, proj.ID)
		if err != nil || creator != 490 {
			t.Fatalf("creator=%d err=%v", creator, err)
		}
	})

	t.Run("unit roles", func(t *testing.T) {
		cases := []struct {
			name   string
			userID uint
			access bool
			manage bool
		}{
			{"set creator", 490, true, true},
			{"unit owner", 7, true, true},
			{"unit participant", 20, true, false},
			{"outsider", 100, false, false},
		}
		for _, tc := range cases {
			gotAccess, err := access.HasPlanUnitAccess(ctx, 931, unit.ID, tc.userID)
			if err != nil || gotAccess != tc.access {
				t.Fatalf("%s access=%v err=%v", tc.name, gotAccess, err)
			}
			gotManage, err := access.CanManagePlanUnitMembers(ctx, 931, unit.ID, tc.userID)
			if err != nil || gotManage != tc.manage {
				t.Fatalf("%s manage=%v err=%v", tc.name, gotManage, err)
			}
		}
	})

	t.Run("any unit access and member fact", func(t *testing.T) {
		// Unit owner and participant hold the any-unit fact directly.
		for _, userID := range []uint{7, 20} {
			got, err := access.HasAnyPlanUnitAccess(ctx, 931, proj.ID, userID)
			if err != nil || !got {
				t.Fatalf("user %d any-access=%v err=%v", userID, got, err)
			}
		}
		// The creator holds the member fact via created_by, not via units.
		member, err := access.IsProjectSetMemberFact(ctx, 931, proj.ID, 490)
		if err != nil || !member {
			t.Fatalf("creator member-fact=%v err=%v", member, err)
		}
		anyForCreator, err := access.HasAnyPlanUnitAccess(ctx, 931, proj.ID, 490)
		if err != nil || anyForCreator {
			t.Fatalf("creator any-access=%v err=%v (creator is not a unit role)", anyForCreator, err)
		}
		got, err := access.HasAnyPlanUnitAccess(ctx, 931, proj.ID, 100)
		if err != nil || got {
			t.Fatalf("outsider any-access=%v err=%v", got, err)
		}
	})
}

func TestPlanUnitAccessRepositoryBuildMemberFilter(t *testing.T) {
	db := openMemberDB(t)
	access := NewPlanUnitAccessRepository(db)
	proj := createCreatorProject(t, db, 932, 490)
	unit := createUnit(t, db, 932, proj.ID, 7)
	if err := db.Create(&PlanUnitParticipant{WorkspaceID: 932, PlanUnitID: unit.ID, UserID: 20, CreatedByUserID: 7, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	createCreatorProject(t, db, 932, 8)
	ctx := context.Background()

	filter, err := access.BuildMemberFilter(ctx, "project_sets", "id", 932, 20)
	if err != nil {
		t.Fatalf("filter: err=%v", err)
	}
	var visible []ProjectSet
	if err := db.Where("workspace_id = ?", 932).Where(filter.SQL, filter.Args...).Find(&visible).Error; err != nil {
		t.Fatalf("apply filter: err=%v", err)
	}
	if len(visible) != 1 || visible[0].ID != proj.ID {
		t.Fatalf("visible=%+v", visible)
	}

	// The other set's creator sees only their own set.
	filterOther, err := access.BuildMemberFilter(ctx, "project_sets", "id", 932, 8)
	if err != nil {
		t.Fatalf("other creator filter: err=%v", err)
	}
	var visibleOther []ProjectSet
	if err := db.Where("workspace_id = ?", 932).Where(filterOther.SQL, filterOther.Args...).Find(&visibleOther).Error; err != nil {
		t.Fatalf("apply other creator filter: err=%v", err)
	}
	if len(visibleOther) != 1 || visibleOther[0].CreatedByUserID == nil || *visibleOther[0].CreatedByUserID != 8 {
		t.Fatalf("other creator visible=%+v", visibleOther)
	}
}

func TestPlanUnitAccessRepositoryFilterBranches(t *testing.T) {
	db := openMemberDB(t)
	access := NewPlanUnitAccessRepository(db)
	ctx := context.Background()

	cases := []struct {
		table    string
		column   string
		contains []string
	}{
		{"project_sets", "id", []string{"src.created_by_user_id", "pu.project_new_id", "plan_unit_participants"}},
		{"plan_units", "id", []string{"src.created_by_user_id", "plan_unit_participants"}},
		{"requirements", "project_new_id", []string{"version_task_new_id", "src.created_by_user_id"}},
		{"work_tasks", "project_set_id", []string{"project_id", "requirement_id", "r.version_task_new_id"}},
		{"quality_bugs", "project_set_id", []string{"requirement_id", "r.version_task_new_id"}},
	}
	for _, tc := range cases {
		filter, err := access.BuildMemberFilter(ctx, tc.table, tc.column, 932, 20)
		if err != nil {
			t.Fatalf("%s: err=%v", tc.table, err)
		}
		for _, want := range tc.contains {
			if !strings.Contains(filter.SQL, want) {
				t.Fatalf("%s: filter missing %q: %s", tc.table, want, filter.SQL)
			}
		}
		if !strings.Contains(filter.SQL, tc.table+".") {
			t.Fatalf("%s: outer column not qualified: %s", tc.table, filter.SQL)
		}
	}
}

// TestPlanUnitFilterDoesNotCrossEntityIDs proves the plan unit filter compares
// the unit's owning SET id (project_new_id) against project_sets — never the
// unit's own id, which can collide with an unrelated project set the user
// created (real-world case: unit 17 vs project set 17).
func TestPlanUnitFilterDoesNotCrossEntityIDs(t *testing.T) {
	db := openMemberDB(t)
	access := NewPlanUnitAccessRepository(db)
	proj := createCreatorProject(t, db, 933, 490)
	owner := uint(7)
	unit := PlanUnit{ID: 50, WorkspaceID: 933, TeamID: 1, ProjectSetID: proj.ID, Title: "Colliding unit", OwnerUserID: &owner, Status: "planning"}
	if err := db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	// An unrelated project set whose id equals the unit's id, created by the
	// participant user.
	participant := uint(20)
	if err := db.Create(&ProjectSet{ID: 50, WorkspaceID: 933, TeamID: 1, Name: "Colliding set", CreatedByUserID: &participant, Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	filter, err := access.BuildMemberFilter(ctx, "plan_units", "id", 933, participant)
	if err != nil {
		t.Fatalf("filter: err=%v", err)
	}
	var count int64
	if err := db.Table("plan_units").
		Where("workspace_id = ? AND project_new_id = ?", 933, proj.ID).
		Where(filter.SQL, filter.Args...).Count(&count).Error; err != nil {
		t.Fatalf("apply: %v", err)
	}
	if count != 0 {
		t.Fatalf("participant sees %d unit(s) of a set they hold no role on — id collision leaked", count)
	}

	// The set creator still sees the unit.
	filterCreator, err := access.BuildMemberFilter(ctx, "plan_units", "id", 933, 490)
	if err != nil {
		t.Fatalf("creator filter: err=%v", err)
	}
	if err := db.Table("plan_units").
		Where("workspace_id = ? AND project_new_id = ?", 933, proj.ID).
		Where(filterCreator.SQL, filterCreator.Args...).Count(&count).Error; err != nil {
		t.Fatalf("apply creator: %v", err)
	}
	if count != 1 {
		t.Fatalf("creator sees %d unit(s), want 1", count)
	}
}
