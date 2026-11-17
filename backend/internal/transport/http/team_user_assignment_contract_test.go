package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

type teamUserAssignmentFixture struct {
	app         *testApp
	token       string
	workspaceID uint
	teamID      uint
	projectID   uint
	planID      uint
	requirement uint
	users       map[string]uint
}

func TestProjectPeopleDirectoryUsesGlobalActivePMUsers(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	readerToken := fix.app.token(t, teamUserAssignmentUser(t, fix.app, fix.users["reader"]))
	want := []uint{fix.users["actor"], fix.users["member"], fix.users["outsider"],
		fix.users["disabled"], fix.users["foreign"], fix.users["reader"], fix.users["ordinary"]}
	assertUserScopeIDs(t, fix.app, readerToken, "/api/v1/core/users?team_id=10", want)
	ordinaryToken := fix.app.token(t, teamUserAssignmentUser(t, fix.app, fix.users["ordinary"]))
	assertUserScopeIDs(t, fix.app, ordinaryToken, "/api/v1/core/users?project_id=200", want)
}

func TestProjectPeopleFieldsPersistActivePMIDsWithoutMembershipLookup(t *testing.T) {
	// Create always forces the authenticated user as owner. owner_user_id in
	// the request body is ignored for create.
	t.Run("create stores actor as owner", func(t *testing.T) {
		fix := setupTeamUserAssignmentFixture(t)
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/project-sets",
			fix.token, fix.workspaceID, map[string]any{
				"team_id": fix.teamID, "name": "Actor owner",
			})
		requireStatus(t, response, http.StatusCreated)
		// The actor (user 1) is the owner.
		if decodeData(t, response).(map[string]any)["owner_user_id"] != float64(1) {
			t.Fatalf("owner should be actor: %s", response.Body.String())
		}
	})

	t.Run("create ignores arbitrary owner_user_id", func(t *testing.T) {
		fix := setupTeamUserAssignmentFixture(t)
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/project-sets",
			fix.token, fix.workspaceID, map[string]any{
				"team_id": fix.teamID, "name": "Ignored owner", "owner_user_id": 98765,
			})
		requireStatus(t, response, http.StatusCreated)
		if decodeData(t, response).(map[string]any)["owner_user_id"] != float64(1) {
			t.Fatalf("owner should be actor (1), not arbitrary id: %s", response.Body.String())
		}
	})

	t.Run("create inactive user still works because actor is owner", func(t *testing.T) {
		fix := setupTeamUserAssignmentFixture(t)
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/project-sets",
			fix.token, fix.workspaceID, map[string]any{
				"team_id": fix.teamID, "name": "Inactive owner but actor is owner",
			})
		// The body's owner_user_id is ignored; actor is always owner.
		requireStatus(t, response, http.StatusCreated)
		if decodeData(t, response).(map[string]any)["owner_user_id"] != float64(1) {
			t.Fatalf("owner must be actor: %s", response.Body.String())
		}
	})

	// Plain PATCH rejects owner changes; transfer-owner remains the single
	// owner-mutation path (removal checks + audit + consistency).
	for _, target := range []string{"outsider", "foreign"} {
		t.Run("patch owner rejects "+target, func(t *testing.T) {
			fix := setupTeamUserAssignmentFixture(t)
			patched := fix.app.request(t, http.MethodPatch,
				"/api/v1/platform/project-sets/"+uintString(fix.projectID),
				fix.token, fix.workspaceID, map[string]any{"owner_user_id": fix.users[target]})
			requireStatus(t, patched, http.StatusBadRequest)
		})

		t.Run("transfer owner stores "+target, func(t *testing.T) {
			fix := setupTeamUserAssignmentFixture(t)
			transferred := fix.app.request(t, http.MethodPost,
				"/api/v1/platform/project-sets/"+uintString(fix.projectID)+"/transfer-owner-new",
				fix.token, fix.workspaceID, map[string]any{"new_owner_user_id": fix.users[target]})
			requireStatus(t, transferred, http.StatusOK)
		})
	}
}

func TestPlanDeliveryAndRequirementPeopleFieldsPersistPlatformIDs(t *testing.T) {
	for _, target := range []string{"outsider", "foreign"} {
		t.Run("plan and delivery owners store "+target, func(t *testing.T) {
			fix := setupTeamUserAssignmentFixture(t)
			workflowTemplateID := createPublishedPlanUnitWorkflow(t, fix.app, fix.token, fix.workspaceID)
			response := fix.app.request(t, http.MethodPost, "/api/v1/platform/project-new",
				fix.token, fix.workspaceID, map[string]any{
					"project_new_id": fix.projectID, "title": "Stored plan owner",
					"owner_user_id":        fix.users[target],
					"workflow_template_id": workflowTemplateID,
					"delivery_nodes": []map[string]any{{
						"label_name": "上线", "type": "release", "owner_user_id": fix.users[target],
					}},
				})
			requireStatus(t, response, http.StatusCreated)
		})

		for _, field := range []string{
			"product_owner_user_id", "design_owner_user_id", "frontend_owner_user_id", "backend_owner_user_id",
			"developer_user_ids", "product_owner_user_ids", "design_owner_user_ids", "frontend_owner_user_ids",
			"backend_owner_user_ids", "tester_user_ids",
		} {
			t.Run("requirement "+field+" stores "+target, func(t *testing.T) {
				fix := setupTeamUserAssignmentFixture(t)
				workflowTemplateID := createPublishedRequirementWorkflow(t, fix.app, fix.token, fix.workspaceID)
				body := map[string]any{
					"version_task_new_id": fix.planID, "title": "Stored requirement assignment",
					"workflow_template_id": workflowTemplateID, "priority": "p1",
					"product_owner_user_id": fix.users["member"],
					"design_owner_user_id":  fix.users["member"],
					"developer_user_ids":    []uint{fix.users["member"]},
				}
				if field == "developer_user_ids" || field == "product_owner_user_ids" ||
					field == "design_owner_user_ids" || field == "frontend_owner_user_ids" ||
					field == "backend_owner_user_ids" || field == "tester_user_ids" {
					body[field] = []uint{fix.users[target]}
				} else {
					body[field] = fix.users[target]
				}
				response := fix.app.request(t, http.MethodPost,
					"/api/v1/platform/product-requirements-new", fix.token, fix.workspaceID, body)
				requireStatus(t, response, http.StatusCreated)
			})
		}
	}
}

func TestTaskPeopleFieldsPersistActivePMIDsWithoutMembershipLookup(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks",
		fix.token, fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement, "title": "Stored task people",
			"assignee_id": fix.users["foreign"], "reporter_user_id": fix.users["disabled"],
		})
	requireStatus(t, response, http.StatusCreated)
	row := decodeData(t, response).(map[string]any)
	if row["assignee_id"] != float64(fix.users["foreign"]) ||
		row["reporter_user_id"] != float64(fix.users["disabled"]) {
		t.Fatalf("task people IDs were not stored: %s", response.Body.String())
	}
	invalid := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks",
		fix.token, fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement, "title": "Inactive reporter",
			"assignee_id": fix.users["foreign"], "reporter_user_id": fix.users["inactive"],
		})
	requireStatus(t, invalid, http.StatusBadRequest)
}

func TestQualityPeopleFieldsPersistActivePMIDsWithoutMembershipLookup(t *testing.T) {
	for _, target := range []string{"outsider", "disabled", "foreign"} {
		for _, field := range []string{"assignee_user_id", "reviewer_user_id"} {
			t.Run(field+" stores "+target, func(t *testing.T) {
				fix := setupTeamUserAssignmentFixture(t)
				body := map[string]any{
					"requirement_id": fix.requirement, "title": "Stored quality assignment",
					"assignee_user_id": fix.users["member"], "reviewer_user_id": fix.users["member"],
				}
				body[field] = fix.users[target]
				response := fix.app.request(t, http.MethodPost,
					"/api/v1/platform/quality/bugs", fix.token, fix.workspaceID, body)
				requireStatus(t, response, http.StatusCreated)
				if decodeData(t, response).(map[string]any)[field] != float64(fix.users[target]) {
					t.Fatalf("quality user fact was not stored: %s", response.Body.String())
				}
			})
		}

		t.Run("cc stores "+target, func(t *testing.T) {
			fix := setupTeamUserAssignmentFixture(t)
			bugID := createTeamAssignmentQualityBug(t, fix)
			response := fix.app.request(t, http.MethodPatch,
				"/api/v1/platform/quality/bugs/"+uintString(bugID),
				fix.token, fix.workspaceID, map[string]any{"cc_user_ids": []uint{fix.users[target]}})
			requireStatus(t, response, http.StatusOK)
		})
	}
	t.Run("inactive assignee is rejected", func(t *testing.T) {
		fix := setupTeamUserAssignmentFixture(t)
		response := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs", fix.token, fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement, "title": "Inactive assignment",
			"assignee_user_id": fix.users["inactive"], "reviewer_user_id": fix.users["member"],
		})
		requireStatus(t, response, http.StatusBadRequest)
	})
	t.Run("inactive cc user is rejected", func(t *testing.T) {
		fix := setupTeamUserAssignmentFixture(t)
		bugID := createTeamAssignmentQualityBug(t, fix)
		response := fix.app.request(t, http.MethodPatch,
			"/api/v1/platform/quality/bugs/"+uintString(bugID), fix.token, fix.workspaceID,
			map[string]any{"cc_user_ids": []uint{fix.users["inactive"]}})
		requireStatus(t, response, http.StatusBadRequest)
	})
}

func setupTeamUserAssignmentFixture(t *testing.T) teamUserAssignmentFixture {
	t.Helper()
	app := newTestAppWithProjectAccess(t, nil)
	users := map[string]identitygorm.User{
		"actor":    app.createUser(t, 1, "assignment-actor", "super_admin", ""),
		"member":   app.createUser(t, 2, "assignment-member", "user", ""),
		"outsider": app.createUser(t, 3, "assignment-outsider", "user", ""),
		"disabled": app.createUser(t, 4, "assignment-disabled", "user", ""),
		"inactive": app.createUser(t, 5, "assignment-inactive", "user", ""),
		"foreign":  app.createUser(t, 6, "assignment-foreign", "user", ""),
		"reader":   app.createUser(t, 7, "assignment-reader", "user", ""),
		"ordinary": app.createUser(t, 8, "assignment-ordinary", "user", ""),
	}
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", users["inactive"].ID).
		Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	actorID := users["actor"].ID
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Assignment workspace", Slug: "assignment-workspace", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "Foreign workspace", Slug: "assignment-foreign", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["actor"].ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["member"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["outsider"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["disabled"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["inactive"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["reader"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: users["ordinary"].ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: users["foreign"].ID, Role: "member", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Assignment team", OwnerUserID: &actorID},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Foreign team"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: users["actor"].ID, Role: "admin"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: users["member"].ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: users["disabled"].ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: users["inactive"].ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: 2, TeamID: 20, UserID: users["foreign"].ID, Role: "member"},
		&projectgorm.ProjectSet{ID: 100, WorkspaceID: 1, TeamID: 10, Name: "Assignment project", Code: "ASG", OwnerUserID: &actorID},
		&projectgorm.PlanUnit{ID: 200, WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, Title: "Assignment plan", OwnerUserID: &actorID},
		&requirementgorm.Requirement{ID: 300, WorkspaceID: 1, TeamID: 10, ProjectSetID: 100, PlanUnitID: 200, Title: "Assignment requirement"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Model(&identitygorm.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", 1, users["disabled"].ID).
		Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	permission := projectgorm.ProjectPermission{
		WorkspaceID: 1, TeamID: 10, ProjectSetID: 100,
		UserID: users["reader"].ID, AccessLevel: "viewer",
	}
	if err := app.db.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	factIDs := make(map[string]uint, len(users))
	for name, user := range users {
		factIDs[name] = factUserID(user)
	}
	token := app.token(t, users["actor"])
	return teamUserAssignmentFixture{
		app: app, token: token, workspaceID: 1, teamID: 10,
		projectID: 100, planID: 200, requirement: 300, users: factIDs,
	}
}

func teamUserAssignmentUser(t *testing.T, app *testApp, userID uint) identitygorm.User {
	t.Helper()
	var user identitygorm.User
	if err := app.db.First(&user, userID).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func createTeamAssignmentQualityBug(t *testing.T, fix teamUserAssignmentFixture) uint {
	t.Helper()
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/quality/bugs",
		fix.token, fix.workspaceID, map[string]any{
			"requirement_id": fix.requirement, "title": "CC scope bug",
			"assignee_user_id": fix.users["member"],
		})
	requireStatus(t, response, http.StatusCreated)
	return uint(decodeData(t, response).(map[string]any)["id"].(float64))
}
