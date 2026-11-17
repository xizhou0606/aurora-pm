package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	fixtureWS    = uint(640)
	fixtureTeam  = uint(9640)
	fixtureSetID = uint(9640)
	fixtureUnit  = uint(9641)
	fixtureOwner = uint(640)
	fixtureUser2 = uint(641)
)

func TestPlanUnitMemberEndpointsContract(t *testing.T) {
	t.Parallel()

	// Positive: GET members, GET member-options, PUT members.
	t.Run("happy path lists options and replaces members", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)

		getResp := app.request(t, http.MethodGet,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, nil)
		requireStatus(t, getResp, http.StatusOK)
		data := decodeData(t, getResp).(map[string]any)
		items, ok := data["items"].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("GET members items missing: %v", data)
		}

		optsResp := app.request(t, http.MethodGet,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/member-options",
			token, workspace.ID, nil)
		requireStatus(t, optsResp, http.StatusOK)

		// PUT members: transfer owner to user 641 (owner cannot be a participant).
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"owner_user_id": fixtureUser2,
				"user_ids":      []uint{},
				"reason":        "test transfer ownership",
			})
		requireStatus(t, putResp, http.StatusOK)
		memberData := decodeData(t, putResp).(map[string]any)
		if memberData["owner_user_id"] != float64(fixtureUser2) {
			t.Fatalf("expected owner %d, got %v", fixtureUser2, memberData["owner_user_id"])
		}
		if memberData["plan_unit_id"] != float64(fixtureUnit) {
			t.Fatalf("expected plan_unit_id %d, got %v", fixtureUnit, memberData["plan_unit_id"])
		}
	})

	// Negative: PUT members rejects missing owner_user_id (400).
	t.Run("put without owner_user_id returns 400", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"user_ids": []uint{},
				"reason":   "missing owner",
			})
		requireStatus(t, putResp, http.StatusBadRequest)
	})

	// P4: Reject removal of participant with outstanding open tasks inside
	// the unit.
	t.Run("put members rejects removing participant with open unit tasks", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)
		if err := app.db.Create(&projectgorm.PlanUnitParticipant{
			WorkspaceID: workspace.ID, PlanUnitID: fixtureUnit, UserID: fixtureUser2, CreatedByUserID: fixtureOwner,
		}).Error; err != nil {
			t.Fatal(err)
		}
		task := taskgorm.WorkTask{
			WorkspaceID: workspace.ID, TeamID: fixtureTeam, ProjectSetID: fixtureSetID,
			ProjectID: fixtureUnit, Kind: "task", TaskKey: "TK-1",
			Title: "open task", Status: "open", AssigneeID: ptrUint(fixtureUser2),
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := app.db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"owner_user_id": fixtureOwner,
				"user_ids":      []uint{},
				"reason":        "remove participant with open task",
			})
		requireStatus(t, putResp, http.StatusConflict)
	})

	// P4: Allow removal of clean participant with no outstanding
	// responsibilities inside the unit.
	t.Run("put members allows removing clean participant", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)
		if err := app.db.Create(&projectgorm.PlanUnitParticipant{
			WorkspaceID: workspace.ID, PlanUnitID: fixtureUnit, UserID: fixtureUser2, CreatedByUserID: fixtureOwner,
		}).Error; err != nil {
			t.Fatal(err)
		}
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"owner_user_id": fixtureOwner,
				"user_ids":      []uint{},
				"reason":        "remove clean participant",
			})
		requireStatus(t, putResp, http.StatusOK)
	})

	// P4: Owner can transfer to another user.
	t.Run("put members allows owner transfer", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"owner_user_id": fixtureUser2,
				"user_ids":      []uint{},
				"reason":        "transfer ownership",
			})
		requireStatus(t, putResp, http.StatusOK)
		memberData := decodeData(t, putResp).(map[string]any)
		if memberData["owner_user_id"] != float64(fixtureUser2) {
			t.Fatalf("expected owner %d, got %v", fixtureUser2, memberData["owner_user_id"])
		}
	})

	// Structure contract: GET /members returns the paged member envelope.
	t.Run("member list response structure", func(t *testing.T) {
		t.Parallel()
		app, token, workspace := newPlanUnitMemberFixture(t)
		getResp := app.request(t, http.MethodGet,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, nil)
		requireStatus(t, getResp, http.StatusOK)
		data := decodeData(t, getResp).(map[string]any)
		for _, key := range []string{"items", "page", "page_size", "total", "has_more"} {
			if _, ok := data[key]; !ok {
				t.Fatalf("GET /members response missing key %q: %s", key, getResp.Body.String())
			}
		}
	})
}

// TestPlanUnitMemberPermissionContract proves the member-management gate: a
// workspace member with data access but no manage fact gets 403, an access
// administrator gets through, and a user without unit access gets 404 on
// every member surface (indistinguishable from missing).
func TestPlanUnitMemberPermissionContract(t *testing.T) {
	t.Parallel()

	t.Run("non-manager put members returns 403", func(t *testing.T) {
		t.Parallel()
		// Viewer (399) has access to unit 9641 but is NOT an access
		// administrator. Owner 640 is admin in this fixture.
		app := newTestAppWithRoleAccessAndProjects(t, []uint{fixtureOwner}, []uint{fixtureOwner, 399}, map[uint][]uint{
			fixtureOwner: {fixtureUnit}, 399: {fixtureUnit},
		}, nil)
		owner := app.createUser(t, fixtureOwner, "member-actor", "super_admin", "")
		viewer := app.createUser(t, 399, "viewer", "user", "")
		workspace, project := seedPlanUnitWorkspace(t, app, owner.ID)
		if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: viewer.ID, Role: "member", Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
		viewerToken := app.token(t, viewer)
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			viewerToken, workspace.ID, map[string]any{
				"owner_user_id": owner.ID,
				"user_ids":      []uint{},
				"reason":        "non-manager attempt",
			})
		requireStatus(t, putResp, http.StatusForbidden)
		_ = project
	})

	// P4: Admin (access-administrator, non-owner) can manage members.
	t.Run("admin non-owner can put members", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithRoleAccess(t, []uint{642}, []uint{fixtureOwner, 642}, nil)
		owner := app.createUser(t, fixtureOwner, "member-actor", "super_admin", "")
		admin := app.createUser(t, 642, "admin-user", "super_admin", "")
		workspace, _ := seedPlanUnitWorkspace(t, app, owner.ID)
		if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: admin.ID, Role: "member", Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
		adminToken := app.token(t, admin)
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			adminToken, workspace.ID, map[string]any{
				"owner_user_id": admin.ID,
				"user_ids":      []uint{},
				"reason":        "admin takeover",
			})
		requireStatus(t, putResp, http.StatusOK)
	})

	t.Run("outsider member surfaces return 404", func(t *testing.T) {
		t.Parallel()
		app := newTestAppWithRoleAccess(t, []uint{fixtureOwner}, []uint{fixtureOwner}, nil)
		owner := app.createUser(t, fixtureOwner, "member-actor", "super_admin", "")
		outsider := app.createUser(t, 398, "outsider", "user", "")
		workspace, _ := seedPlanUnitWorkspace(t, app, owner.ID)
		if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: outsider.ID, Role: "member", Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
		outsiderToken := app.token(t, outsider)
		for _, path := range []string{
			"/api/v1/platform/project-new/" + fmt.Sprint(fixtureUnit) + "/members",
			"/api/v1/platform/project-new/" + fmt.Sprint(fixtureUnit) + "/members/398/removal-impact",
		} {
			rec := app.request(t, http.MethodGet, path, outsiderToken, workspace.ID, nil)
			requireStatus(t, rec, http.StatusNotFound)
		}
		putResp := app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			outsiderToken, workspace.ID, map[string]any{
				"owner_user_id": fixtureOwner,
				"user_ids":      []uint{},
			})
		requireStatus(t, putResp, http.StatusNotFound)
	})
}

// TestPlanUnitAndSetDTOContract verifies the DTO surface: the plan unit
// carries can_manage_participants; the project set carries only
// created_by_user_id (project-level members are retired).
func TestPlanUnitAndSetDTOContract(t *testing.T) {
	t.Parallel()
	app, token, workspace := newPlanUnitMemberFixture(t)

	unitResp := app.request(t, http.MethodGet,
		"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit),
		token, workspace.ID, nil)
	requireStatus(t, unitResp, http.StatusOK)
	unitData := decodeData(t, unitResp).(map[string]any)
	if manage, ok := unitData["can_manage_participants"].(bool); !ok || !manage {
		t.Fatalf("GET /project-new/:id missing/true can_manage_participants: %s", unitResp.Body.String())
	}

	setResp := app.request(t, http.MethodGet,
		"/api/v1/platform/project-sets/"+fmt.Sprint(fixtureSetID),
		token, workspace.ID, nil)
	requireStatus(t, setResp, http.StatusOK)
	setData := decodeData(t, setResp).(map[string]any)
	if _, ok := setData["created_by_user_id"]; !ok {
		t.Fatalf("GET /project-sets/:id missing created_by_user_id: %s", setResp.Body.String())
	}
	if _, ok := setData["members_snapshot"]; ok {
		t.Fatal("GET /project-sets/:id still returns retired members_snapshot")
	}
}

// newPlanUnitMemberFixture creates a test app (allow-all access fake) with an
// owner user, a second user, a workspace, team, project set, and plan unit
// seeded; returns app, owner token, and workspace.
func newPlanUnitMemberFixture(t *testing.T) (*testApp, string, identitygorm.Workspace) {
	t.Helper()
	app := newTestAppWithProjectAccess(t, nil)
	owner := app.createUser(t, fixtureOwner, "member-actor", "super_admin", "")
	app.createUser(t, fixtureUser2, "member-target", "user", "")
	workspace, _ := seedPlanUnitWorkspace(t, app, owner.ID)
	token := app.token(t, owner)
	return app, token, workspace
}

// seedPlanUnitWorkspace seeds workspace/team/project set/plan unit for the
// given owner inside an existing test app.
func seedPlanUnitWorkspace(t *testing.T, app *testApp, ownerID uint) (identitygorm.Workspace, projectgorm.ProjectSet) {
	t.Helper()
	workspace := identitygorm.Workspace{ID: fixtureWS, Name: "成员项目", Slug: "member", Enabled: true}
	team := identitygorm.Team{ID: fixtureTeam, WorkspaceID: workspace.ID, Name: "成员团队"}
	for _, row := range []any{
		&workspace,
		&team,
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: ownerID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: fixtureUser2, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	creator := ownerID
	project := projectgorm.ProjectSet{
		ID: fixtureSetID, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "成员测试项目",
		CreatedByUserID: &creator, Status: "active",
	}
	if err := app.db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	unitOwner := ownerID
	unit := projectgorm.PlanUnit{
		ID: fixtureUnit, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
		Title: "成员测试版本", OwnerUserID: &unitOwner, Status: "planning",
	}
	if err := app.db.Create(&unit).Error; err != nil {
		t.Fatal(err)
	}
	return workspace, project
}

func ptrUint(u uint) *uint { return &u }

// TestPlanUnitMemberAssignmentNotification proves a member replacement
// enqueues one IM notification per newly assigned owner / added participant,
// and nothing for members whose roles did not change.
func TestPlanUnitMemberAssignmentNotification(t *testing.T) {
	t.Parallel()
	app := newTestAppWithProjectAccess(t, func(cfg *config.Config) {
		cfg.Notification.Enabled = true
		cfg.Notification.PublicBaseURL = "http://localhost:5175"
	})
	owner := app.createUser(t, fixtureOwner, "member-actor", "super_admin", "")
	user2 := app.createUser(t, fixtureUser2, "member-target", "user", "")
	workspace, _ := seedPlanUnitWorkspace(t, app, owner.ID)
	token := app.token(t, owner)

	put := func(ownerID uint, userIDs []uint) *httptest.ResponseRecorder {
		return app.request(t, http.MethodPut,
			"/api/v1/platform/project-new/"+fmt.Sprint(fixtureUnit)+"/members",
			token, workspace.ID, map[string]any{
				"owner_user_id": ownerID,
				"user_ids":      userIDs,
				"reason":        "notification contract",
			})
	}

	outboxRecipients := func() map[uint]string {
		var rows []auditgorm.OutboxEvent
		if err := app.db.Where("aggregate_type = ?", "plan_unit_notification").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		result := map[uint]string{}
		for _, row := range rows {
			var payload struct {
				RecipientUserIDs []uint `json:"recipient_user_ids"`
				Scene            string `json:"scene"`
			}
			if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
				t.Fatalf("payload: %v", err)
			}
			if payload.Scene != "plan_unit.member_assigned" {
				t.Fatalf("unexpected scene %q", payload.Scene)
			}
			for _, id := range payload.RecipientUserIDs {
				result[id] = row.EventType
			}
		}
		return result
	}

	// Transfer the owner to user2: exactly one notification for user2.
	if rec := put(fixtureUser2, []uint{}); rec.Code != http.StatusOK {
		t.Fatalf("owner transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := outboxRecipients(); got[fixtureUser2] != "pm.notification.plan_unit.member_assigned" {
		t.Fatalf("owner transfer notifications=%v", got)
	}

	// Add user1 (previous owner) back as participant: exactly one notification
	// for user1, and no duplicate for user2.
	if rec := put(fixtureUser2, []uint{fixtureOwner}); rec.Code != http.StatusOK {
		t.Fatalf("add participant status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := outboxRecipients()
	if got[fixtureOwner] != "pm.notification.plan_unit.member_assigned" {
		t.Fatalf("added participant notifications=%v", got)
	}
	var user2Events int64
	app.db.Model(&auditgorm.OutboxEvent{}).
		Where("aggregate_type = ? AND payload LIKE ?", "plan_unit_notification", "%\"recipient_user_ids\":["+fmt.Sprint(fixtureUser2)+"]%").
		Count(&user2Events)
	if user2Events != 1 {
		t.Fatalf("user2 received %d notification(s), want 1 (unchanged role must not re-notify)", user2Events)
	}
	_ = user2
}
