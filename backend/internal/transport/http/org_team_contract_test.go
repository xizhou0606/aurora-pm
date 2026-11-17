package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"net/http"
	"testing"
)

func TestTeamMembersUseActivePMUsersRegardlessOfWorkspaceMembership(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "team-admin", "super_admin", "")
	enabled := app.createUser(t, 2, "enabled-member", "user", "")
	disabled := app.createUser(t, 3, "disabled-member", "user", "")
	inactive := app.createUser(t, 4, "inactive-member", "user", "")
	foreign := app.createUser(t, 5, "foreign-member", "user", "")
	if err := app.db.Model(&inactive).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Workspace A", Slug: "team-workspace-a", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "Workspace B", Slug: "team-workspace-b", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Team B"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: enabled.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: disabled.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: inactive.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: foreign.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 2, TeamID: 20, UserID: foreign.ID, Role: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)

	membersRec := app.request(t, http.MethodGet, "/api/v1/core/teams/10/members", token, 1, nil)
	requireStatus(t, membersRec, http.StatusOK)
	members := decodeData(t, membersRec).([]any)
	if len(members) != 3 {
		t.Fatalf("members=%v want three active PM users", members)
	}
	memberIDs := map[float64]bool{}
	for _, raw := range members {
		memberIDs[raw.(map[string]any)["id"].(float64)] = true
	}
	for _, id := range []uint{enabled.ID, disabled.ID, foreign.ID} {
		if !memberIDs[float64(id)] {
			t.Fatalf("member %d missing from %v", id, members)
		}
	}

	teamsRec := app.request(t, http.MethodGet, "/api/v1/core/teams", token, 1, nil)
	requireStatus(t, teamsRec, http.StatusOK)
	teams := decodeData(t, teamsRec).([]any)
	if len(teams) != 1 || teams[0].(map[string]any)["member_count"] != float64(3) {
		t.Fatalf("teams=%v want member_count=3", teams)
	}

	foreignRec := app.request(t, http.MethodGet, "/api/v1/core/teams/20/members", token, 1, nil)
	requireStatus(t, foreignRec, http.StatusNotFound)
}

func TestCreateTeamCreatesOwnerMembershipAtomically(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "team-create-admin", "super_admin", "")
	owner := app.createUser(t, 2, "team-owner", "user", "")
	if err := app.db.Model(&owner).Update("external_user_id", 77).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Workspace", Slug: "team-create-workspace", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: owner.ID, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)

	success := app.request(t, http.MethodPost, "/api/v1/core/teams", token, 1, map[string]any{
		"name": "Atomic Team", "owner_user_id": owner.ID,
	})
	requireStatus(t, success, http.StatusCreated)
	createdTeam := decodeData(t, success).(map[string]any)
	teamID := uint(createdTeam["id"].(float64))
	var membership identitygorm.TeamMember
	if err := app.db.Where("workspace_id = ? AND team_id = ? AND user_id = ?", 1, teamID, owner.ID).
		First(&membership).Error; err != nil {
		t.Fatalf("owner membership missing: %v", err)
	}
	if membership.Role != "admin" || createdTeam["owner_user_id"] != float64(owner.ID) {
		t.Fatalf("membership=%+v team=%v", membership, createdTeam)
	}

	if err := app.db.Exec(`CREATE TRIGGER fail_admin_team_member
		BEFORE INSERT ON team_members WHEN NEW.role = 'admin'
		BEGIN SELECT RAISE(ABORT, 'forced team membership failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	failure := app.request(t, http.MethodPost, "/api/v1/core/teams", token, 1, map[string]any{
		"name": "Rolled Back Team", "owner_user_id": owner.ID,
	})
	requireStatus(t, failure, http.StatusInternalServerError)
	var count int64
	if err := app.db.Model(&identitygorm.Team{}).
		Where("workspace_id = ? AND name = ?", 1, "Rolled Back Team").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("team persisted despite owner-membership failure: count=%d", count)
	}
}
