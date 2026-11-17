package httpapi

import (
	"context"
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

func TestPMUserRegistryListsActiveUsersByInternalID(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "registry-admin", "super_admin", "")
	active := app.createUser(t, 2, "active-user", "user", "")
	if err := app.db.Model(&active).Updates(map[string]any{
		"external_user_id": 77, "display_name": "Active User",
	}).Error; err != nil {
		t.Fatal(err)
	}
	disabled := app.createUser(t, 3, "disabled-user", "user", "")
	if err := app.db.Model(&disabled).Updates(map[string]any{
		"external_user_id": 88, "active": false,
	}).Error; err != nil {
		t.Fatal(err)
	}

	rec := app.request(t, http.MethodGet, "/api/v1/core/users?ids=2,3,77,88", app.token(t, admin), 0, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != float64(2) {
		t.Fatalf("rows=%v", rows)
	}
}

func TestPMUserRegistryUsesPMPrimaryKeyForTeamFacts(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "registry-admin", "super_admin", "")
	user := app.createUser(t, 2, "team-user", "user", "")
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "Workspace", Slug: "registry-team", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: user.ID, Role: "member"}).Error; err != nil {
		t.Fatal(err)
	}
	rec := app.request(t, http.MethodGet, "/api/v1/core/teams/10/members", app.token(t, admin), 1, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != float64(2) {
		t.Fatalf("rows=%v", rows)
	}
}

func TestPMUserRegistryDoesNotResolveExternalIDs(t *testing.T) {
	app := newTestApp(t, nil)
	primary := app.createUser(t, 31, "primary-user", "user", "")
	external := app.createUser(t, 32, "external-user", "user", "")
	if err := app.db.Model(&external).Update("external_user_id", 77).Error; err != nil {
		t.Fatal(err)
	}

	users, err := findActivePMUsersByReferenceIDs(app.db, []uint{primary.ID, external.ID, 77})
	if err != nil {
		t.Fatal(err)
	}
	if users[primary.ID].ID != primary.ID || users[external.ID].ID != external.ID || users[77].ID != 0 {
		t.Fatalf("resolved users=%+v", users)
	}
}

func TestPMUserRegistryIgnoresExternalReferenceCollision(t *testing.T) {
	app := newTestApp(t, nil)
	app.createUser(t, 77, "primary-77", "user", "")
	external := app.createUser(t, 88, "external-77", "user", "")
	if err := app.db.Model(&external).Update("external_user_id", 77).Error; err != nil {
		t.Fatal(err)
	}

	if user, err := (&Server{db: app.db}).activePMUserByReferenceID(77); err != nil || user.ID != 77 {
		t.Fatalf("primary reference changed: %+v %v", user, err)
	}
}

func TestStrictWorkspaceContextPrefersInternalID(t *testing.T) {
	app := newTestApp(t, nil)
	primaryCollision := app.createUser(t, 85, "primary-85", "user", "")
	if err := app.db.Model(&primaryCollision).Update("external_user_id", 88).Error; err != nil {
		t.Fatal(err)
	}
	importedFact := app.createUser(t, 102, "external-85", "user", "")
	if err := app.db.Model(&importedFact).Update("external_user_id", 85).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: 1, UserID: importedFact.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	workspaceContext, err := (&Server{db: app.db}).loadStrictWorkspaceContext(
		context.Background(),
		1,
		[]uint{85},
	)
	if err != nil {
		t.Fatal(err)
	}
	if user := workspaceContext.Users[85]; user.ID != primaryCollision.ID {
		t.Fatalf("business user 85 resolved to PM user %d, want %d", user.ID, primaryCollision.ID)
	}

	if user, err := (&Server{db: app.db}).activePMUserByReferenceID(85); err != nil || user.ID != primaryCollision.ID {
		t.Fatalf("mutation lookup failed internal identity: %+v %v", user, err)
	}
}

func TestWorkspaceContextKeepsDisplayNamesWhenReferencesCollide(t *testing.T) {
	app := newTestApp(t, nil)
	local := app.createUser(t, 85, "local-85", "user", "")
	if err := app.db.Model(&local).Updates(map[string]any{
		"external_user_id": 88, "display_name": "阿奎",
	}).Error; err != nil {
		t.Fatal(err)
	}
	imported := app.createUser(t, 102, "imported-85", "user", "")
	if err := app.db.Model(&imported).Updates(map[string]any{
		"external_user_id": 85, "display_name": "Roli",
	}).Error; err != nil {
		t.Fatal(err)
	}
	unreferenced := app.createUser(t, 103, "unreferenced", "user", "")
	for _, user := range []identitygorm.User{local, imported, unreferenced} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 1, UserID: user.ID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctx := (&Server{db: app.db}).loadWorkspaceContext(
		1,
		workspaceContextReferences{UserIDs: []uint{85, 102}},
	)
	if got := userDisplayName(ctx, uintPtr(85)); got != "阿奎" {
		t.Fatalf("internal id 85 display=%q", got)
	}
	if got := userDisplayName(ctx, uintPtr(102)); got != "Roli" {
		t.Fatalf("internal id 102 display=%q", got)
	}
	if _, loaded := ctx.Users[factUserID(unreferenced)]; loaded {
		t.Fatal("workspace context loaded an unreferenced workspace member")
	}
	if got := userDisplayName(ctx, uintPtr(999)); got != "用户 #999" {
		t.Fatalf("unknown fact display=%q", got)
	}
}

func TestWorkspaceContextResolvesAdministratorWithoutMemberRow(t *testing.T) {
	app := newTestApp(t, nil)
	if err := app.db.Create(&identitygorm.Workspace{ID: 1, Name: "test", Slug: "test", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	admin := app.createUser(t, 110, "sysadmin", "super_admin", "")
	outsider := app.createUser(t, 111, "outsider", "user", "")
	if err := app.db.Where("user_id IN ?", []uint{admin.ID, outsider.ID}).Delete(&identitygorm.WorkspaceMember{}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := (&Server{db: app.db}).loadWorkspaceContext(1, workspaceContextReferences{UserIDs: []uint{factUserID(admin), factUserID(outsider)}})
	if got := userDisplayName(ctx, uintPtr(factUserID(admin))); got != admin.DisplayName {
		t.Fatalf("administrator display = %q, want %q", got, admin.DisplayName)
	}
	if got := userDisplayName(ctx, uintPtr(factUserID(outsider))); got != outsider.DisplayName {
		t.Fatalf("referenced former member lost identity: %q", got)
	}
	if err := app.db.Model(&admin).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	ctx = (&Server{db: app.db}).loadWorkspaceContext(1, workspaceContextReferences{UserIDs: []uint{factUserID(admin)}})
	if _, ok := ctx.Users[factUserID(admin)]; ok {
		t.Fatal("inactive administrator displayed as active")
	}
}

func TestBusinessDisplayUsesInternalIDsWithoutMemberRows(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 109, "admin", "super_admin", "")
	if err := app.db.Model(&admin).Updates(map[string]any{"external_user_id": 1, "display_name": "Administrator"}).Error; err != nil {
		t.Fatal(err)
	}
	// Display is record-scoped, not membership-scoped; removed members and
	// administrators without workspace membership retain their identity.
	if err := app.db.Where("user_id = ?", admin.ID).Delete(&identitygorm.WorkspaceMember{}).Error; err != nil {
		t.Fatal(err)
	}
	server := &Server{db: app.db}
	ctx := server.loadWorkspaceContext(1, workspaceContextReferences{UserIDs: []uint{1, 109}})
	for _, id := range []uint{109} {
		if got := userDisplayName(ctx, &id); got != "Administrator" {
			t.Fatalf("reference %d name=%q", id, got)
		}
	}
	bug := qualityBugDTO(qualitygorm.QualityBug{ID: 6584, ReporterUserID: uintPtr(109)}, ctx)
	if got := bug["reporter_name"]; got != "Administrator" {
		t.Fatalf("quality reporter name=%v", got)
	}
	profiles, err := server.listUserProfiles([]uint{1, 109})
	if err != nil || len(profiles) != 1 || profiles[0].ID != 109 || profiles[0].DisplayName != "Administrator" {
		t.Fatalf("profiles=%+v err=%v", profiles, err)
	}
	// No alias that was not requested should escape in the response.
	profiles, err = server.listUserProfiles([]uint{109})
	if err != nil || len(profiles) != 1 || profiles[0].ID != 109 {
		t.Fatalf("profiles=%+v err=%v", profiles, err)
	}
}

func TestBusinessDisplayIgnoresInactiveExternalCollision(t *testing.T) {
	app := newTestApp(t, nil)
	primary := app.createUser(t, 85, "primary-85", "user", "")
	if err := app.db.Model(&primary).Update("external_user_id", 88).Error; err != nil {
		t.Fatal(err)
	}
	fact := app.createUser(t, 102, "fact-85", "user", "")
	if err := app.db.Model(&fact).Updates(map[string]any{"external_user_id": 85, "active": false}).Error; err != nil {
		t.Fatal(err)
	}
	users, err := (&Server{db: app.db}).findActivePMUsersByFactIDs([]uint{85, 88, 102})
	if err != nil {
		t.Fatal(err)
	}
	if users[85].ID != primary.ID {
		t.Fatal("inactive external collision hid active primary identity")
	}
	if _, exists := users[102]; exists {
		t.Fatal("inactive primary identity returned")
	}
	if _, exists := users[88]; exists {
		t.Fatalf("external import ID resolved as primary: %+v", users)
	}
}
