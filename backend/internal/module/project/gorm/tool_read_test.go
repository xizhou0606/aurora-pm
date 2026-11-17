package projectgorm

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func openToolReadDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&ProjectSet{}, &PlanUnit{}, &DeliveryNodeLabel{}, &ProjectSetMembership{}, &PlanUnitParticipant{},
		&identitygorm.User{}, &identitygorm.WorkspaceMember{},
		&identitygorm.AccessRole{}, &identitygorm.AccessUserRole{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedToolReadUser(t *testing.T, db *gorm.DB, id uint, username, role string) {
	t.Helper()
	if err := db.Create(&identitygorm.User{
		ID: id, ExternalSubject: username, Username: username, DisplayName: username,
		Role: role, Active: true, Status: "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func newToolReadAccessService(t *testing.T, db *gorm.DB) *project.AccessService {
	t.Helper()
	admin := NewAdminRoleChecker(db)
	unitAccess := NewPlanUnitAccessRepository(db)
	service, err := project.NewAccessService(admin, admin, unitAccess, unitAccess, unitAccess)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestProjectToolReadsEnforceWorkspaceAndParentHierarchy(t *testing.T) {
	db := openToolReadDB(t, "project_tool_reads_hierarchy")
	if err := db.Create(&[]ProjectSet{
		{ID: 11, WorkspaceID: 7, TeamID: 2, Name: "PM", Status: "active"},
		{ID: 12, WorkspaceID: 8, TeamID: 2, Name: "Other", Status: "active"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]PlanUnit{
		{ID: 21, WorkspaceID: 7, TeamID: 2, ProjectSetID: 11, Title: "V1", Type: "version", Status: "planning"},
		{ID: 22, WorkspaceID: 8, TeamID: 2, ProjectSetID: 12, Title: "V2", Type: "version", Status: "planning"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]DeliveryNodeLabel{
		{ID: 31, WorkspaceID: 7, ProjectSetID: 11, PlanUnitID: 21, Name: "Ready"},
		{ID: 32, WorkspaceID: 8, ProjectSetID: 12, PlanUnitID: 22, Name: "Secret"},
		{ID: 33, WorkspaceID: 7, ProjectSetID: 12, PlanUnitID: 21, Name: "Wrong parent"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedToolReadUser(t, db, 90, "super", "super_admin")
	service, err := project.NewToolReadService(NewReadRepository(db), newToolReadAccessService(t, db))
	if err != nil {
		t.Fatal(err)
	}
	labels, err := service.ListDeliveryLabels(context.Background(), 7, 21)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 || labels[0].ID != 31 {
		t.Fatalf("labels=%+v", labels)
	}
	// A super_admin takes the fast path and still sees the whole workspace.
	scope, err := service.ListScopeOptions(context.Background(), 7, 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.ProjectSets) != 1 || scope.ProjectSets[0].ID != 11 || len(scope.PlanUnits) != 1 || scope.PlanUnits[0].ID != 21 {
		t.Fatalf("scope=%+v", scope)
	}
}

func TestProjectToolScopeOptionsFollowProjectDataAccess(t *testing.T) {
	db := openToolReadDB(t, "project_tool_reads_access")
	owner := uint(101)
	participant := uint(102)
	outsider := uint(103)
	otherOwner := uint(104)
	for _, row := range []ProjectSet{
		{ID: 11, WorkspaceID: 7, TeamID: 2, Name: "Owned", CreatedByUserID: &owner, Status: "active"},
		{ID: 12, WorkspaceID: 7, TeamID: 2, Name: "Participating", CreatedByUserID: &otherOwner, Status: "active"},
		{ID: 13, WorkspaceID: 7, TeamID: 2, Name: "Inaccessible", CreatedByUserID: &otherOwner, Status: "active"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	unitOwner := owner
	otherUnitOwner := otherOwner
	if err := db.Create(&[]PlanUnit{
		{ID: 21, WorkspaceID: 7, TeamID: 2, ProjectSetID: 11, Title: "Owned V1", Type: "version", OwnerUserID: &unitOwner, Status: "planning"},
		{ID: 22, WorkspaceID: 7, TeamID: 2, ProjectSetID: 12, Title: "Participating V1", Type: "version", OwnerUserID: &otherUnitOwner, Status: "planning"},
		{ID: 23, WorkspaceID: 7, TeamID: 2, ProjectSetID: 13, Title: "Inaccessible V1", Type: "version", OwnerUserID: &otherUnitOwner, Status: "planning"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{owner, participant, outsider, otherOwner} {
		seedToolReadUser(t, db, id, fmt.Sprintf("scope-user-%d", id), "user")
		if err := db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 7, UserID: id, Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&PlanUnitParticipant{
		WorkspaceID: 7, PlanUnitID: 22, UserID: participant, CreatedByUserID: otherOwner,
	}).Error; err != nil {
		t.Fatal(err)
	}

	service, err := project.NewToolReadService(NewReadRepository(db), newToolReadAccessService(t, db))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// The owner sees the owned project set and its plan units only.
	owned, err := service.ListScopeOptions(ctx, 7, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned.ProjectSets) != 1 || owned.ProjectSets[0].ID != 11 {
		t.Fatalf("owner scope project_sets=%+v", owned.ProjectSets)
	}
	if len(owned.PlanUnits) != 1 || owned.PlanUnits[0].ID != 21 {
		t.Fatalf("owner scope plan_units=%+v", owned.PlanUnits)
	}

	// The participant sees the participated project set only.
	participated, err := service.ListScopeOptions(ctx, 7, participant)
	if err != nil {
		t.Fatal(err)
	}
	if len(participated.ProjectSets) != 1 || participated.ProjectSets[0].ID != 12 {
		t.Fatalf("participant scope project_sets=%+v", participated.ProjectSets)
	}
	if len(participated.PlanUnits) != 1 || participated.PlanUnits[0].ID != 22 {
		t.Fatalf("participant scope plan_units=%+v", participated.PlanUnits)
	}

	// A workspace member without any project membership gets no options.
	none, err := service.ListScopeOptions(ctx, 7, outsider)
	if err != nil {
		t.Fatal(err)
	}
	if len(none.ProjectSets) != 0 || len(none.PlanUnits) != 0 {
		t.Fatalf("outsider scope=%+v", none)
	}

	// A zero principal fails closed instead of enumerating projects.
	anonymous, err := service.ListScopeOptions(ctx, 7, 0)
	if err == nil && (len(anonymous.ProjectSets) != 0 || len(anonymous.PlanUnits) != 0) {
		t.Fatalf("anonymous scope=%+v", anonymous)
	}

	// Without an access formula the service must not enumerate projects.
	blind, err := project.NewToolReadService(NewReadRepository(db), nil)
	if err != nil {
		t.Fatal(err)
	}
	denied, err := blind.ListScopeOptions(ctx, 7, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(denied.ProjectSets) != 0 || len(denied.PlanUnits) != 0 {
		t.Fatalf("no-access-builder scope=%+v", denied)
	}
}
