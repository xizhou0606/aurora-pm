package httpapi

import (
	"context"
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

// scopeAccessBuilder mirrors the AccessFilterBuilder facet the transport layer
// uses for SQL scoping.
type scopeAccessBuilder interface {
	BuildMemberFilter(ctx context.Context, table, column string, workspaceID, userID uint) (project.AccessFilter, error)
}

// seedScopeAccessFixtures creates three projects in workspace 1: id 10 owned by
// user 1, id 11 with user 2 as participant, id 12 owned by an unrelated user.
// It returns nothing; assertions read the endpoint per caller.
func seedScopeAccessFixtures(t *testing.T, app *testApp) {
	t.Helper()
	stranger := uint(999)
	// plan_units/requirements scope-option queries INNER JOIN teams, so the
	// fixture team must actually exist for children to be enumerable.
	if err := app.db.Create(&identitygorm.Team{ID: 1, WorkspaceID: 1, Name: "Scope Team"}).Error; err != nil {
		t.Fatal(err)
	}
	creator := uint(1)
	projects := []projectgorm.ProjectSet{
		{ID: 10, WorkspaceID: 1, TeamID: 1, Name: "Owned", CreatedByUserID: &creator, OwnerUserID: uintPtr(1)},
		{ID: 11, WorkspaceID: 1, TeamID: 1, Name: "Participated", CreatedByUserID: &stranger},
		{ID: 12, WorkspaceID: 1, TeamID: 1, Name: "Inaccessible", CreatedByUserID: &stranger},
	}
	for i := range projects {
		if err := app.db.Create(&projects[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	plans := []projectgorm.PlanUnit{
		{ID: 100, WorkspaceID: 1, TeamID: 1, ProjectSetID: 10, Title: "Owned V1", OwnerUserID: uintPtr(1)},
		{ID: 110, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, Title: "Participated V1", OwnerUserID: &stranger},
		{ID: 120, WorkspaceID: 1, TeamID: 1, ProjectSetID: 12, Title: "Inaccessible V1", OwnerUserID: &stranger},
	}
	for i := range plans {
		if err := app.db.Create(&plans[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	requirements := []requirementgorm.Requirement{
		{ID: 1000, WorkspaceID: 1, TeamID: 1, ProjectSetID: 10, PlanUnitID: 100, Title: "Owned req"},
		{ID: 1100, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, PlanUnitID: 110, Title: "Participated req"},
		{ID: 1200, WorkspaceID: 1, TeamID: 1, ProjectSetID: 12, PlanUnitID: 120, Title: "Inaccessible req"},
	}
	for i := range requirements {
		if err := app.db.Create(&requirements[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Create(&projectgorm.PlanUnitParticipant{
		WorkspaceID: 1, PlanUnitID: 110, UserID: 2, CreatedByUserID: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func scopeOptionsFor(t *testing.T, app *testApp, userID uint) map[string]any {
	t.Helper()
	var user identitygorm.User
	if err := app.db.First(&user, userID).Error; err != nil {
		t.Fatal(err)
	}
	rec := app.request(t, http.MethodGet,
		"/api/v1/platform/project-requirement-scope-options", app.token(t, user), 1, nil)
	requireStatus(t, rec, http.StatusOK)
	return decodeData(t, rec).(map[string]any)
}

// TestScopeOptionsFollowProjectDataAccess proves the workspace-wide scope
// options endpoint only enumerates projects the caller can access: a member
// without project access must not be able to pick that project (or its
// versions and requirements) as a filter.
func TestScopeOptionsFollowProjectDataAccess(t *testing.T) {
	// User 1 owns project 10; user 2 participates in project 11; user 3 has no
	// project membership at all. Nobody here is an access administrator. The
	// per-user project mapping keeps the SQL filter (scope options) and the
	// point gate consistent with the seeded fixtures.
	projectAccess := map[uint][]uint{
		1: {10},
		2: {11},
		3: {}, // workspace member without project access
	}
	app := newTestAppWithRoleAccessAndProjects(t, nil, []uint{1, 2}, projectAccess, nil)
	app.createUser(t, 1, "scope-owner", "user", "")
	app.createUser(t, 2, "scope-participant", "user", "")
	app.createUser(t, 3, "scope-member", "user", "")
	for _, userID := range []uint{1, 2, 3} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 1, UserID: userID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Create(&identitygorm.Workspace{
		ID: 1, Name: "Workspace A", Slug: "workspace-a", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedScopeAccessFixtures(t, app)

	// The owner only sees the owned project and its version/requirement.
	owned := scopeOptionsFor(t, app, 1)
	assertJSONRowIDs(t, owned["project_sets"], []uint{10})
	assertJSONRowIDs(t, owned["projects"], []uint{100})
	assertJSONRowIDs(t, owned["requirements"], []uint{1000})

	// The participant only sees the participated project and its children.
	participated := scopeOptionsFor(t, app, 2)
	assertJSONRowIDs(t, participated["project_sets"], []uint{11})
	assertJSONRowIDs(t, participated["projects"], []uint{110})
	assertJSONRowIDs(t, participated["requirements"], []uint{1100})

	// A workspace member without project access gets an empty cascade.
	member := scopeOptionsFor(t, app, 3)
	assertJSONRowIDs(t, member["project_sets"], []uint{})
	assertJSONRowIDs(t, member["projects"], []uint{})
	assertJSONRowIDs(t, member["requirements"], []uint{})
}

// TestScopeOptionsAdminFastPathShowsAllProjects proves administrators still
// receive every workspace project in the scope options.
func TestScopeOptionsAdminFastPathShowsAllProjects(t *testing.T) {
	app := newTestAppWithRoleAccess(t, []uint{5}, []uint{5}, nil)
	app.createUser(t, 5, "scope-admin", "user", "")
	if err := app.db.Create(&identitygorm.Workspace{
		ID: 1, Name: "Workspace A", Slug: "workspace-a", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: 1, UserID: 5, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedScopeAccessFixtures(t, app)

	admin := scopeOptionsFor(t, app, 5)
	// project_sets are ordered updated_at DESC, id DESC in production.
	assertJSONRowIDs(t, admin["project_sets"], []uint{12, 11, 10})
	// plans and requirements are ordered project_new_id ASC in production.
	assertJSONRowIDs(t, admin["projects"], []uint{100, 110, 120})
	assertJSONRowIDs(t, admin["requirements"], []uint{1000, 1100, 1200})
}
