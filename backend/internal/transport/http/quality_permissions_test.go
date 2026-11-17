package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type qualityPermissionFixture struct {
	app         *testApp
	workspaceID uint
	project     projectgorm.ProjectSet
	plan        projectgorm.PlanUnit
	requirement requirementgorm.Requirement
	users       map[string]identitygorm.User
}

func TestQualityBugAllowedActionsAreWorkspaceWide(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	tests := []struct {
		name    string
		actor   string
		status  string
		actions []string
	}{
		{name: "system admin", actor: "system_admin", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "team admin", actor: "team_admin", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "workspace manager", actor: "manager", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "project owner", actor: "owner", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "quality owner", actor: "quality_owner", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "assignee", actor: "assignee", status: "in_progress", actions: []string{"reset_triage", "submit_review"}},
		{name: "reopened assignee", actor: "assignee", status: "reopened", actions: []string{"reset_triage", "submit_review"}},
		{name: "reviewer", actor: "reviewer", status: "review", actions: []string{"complete", "reset_triage", "return"}},
		{name: "tester", actor: "tester", status: "review", actions: []string{"complete", "reset_triage", "return"}},
		{name: "reporter", actor: "reporter", status: "triage", actions: []string{"start", "submit_review"}},
		{name: "legacy readonly role", actor: "readonly", status: "triage", actions: []string{"start", "submit_review"}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bug := fix.createBug(t, uint(100+index), test.status, test.actor == "readonly")
			response := fix.request(t, test.actor, http.MethodGet, bug.ID, nil)
			requireStatus(t, response, http.StatusOK)
			data := decodeData(t, response).(map[string]any)
			got := actionStrings(data["allowed_actions"])
			sort.Strings(test.actions)
			if fmt.Sprint(got) != fmt.Sprint(test.actions) {
				t.Fatalf("allowed_actions=%v want=%v body=%s", got, test.actions, response.Body.String())
			}
			if _, exists := data["can_update"]; exists {
				t.Fatalf("retired authorization field leaked: %v", data)
			}
		})
	}
}

func TestQualityBugAllowedActionsHideImpossibleCompletion(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 150, "review", false)
	if err := fix.app.db.Model(&bug).Update("resolution_reason", "").Error; err != nil {
		t.Fatal(err)
	}
	response := fix.request(t, "reviewer", http.MethodGet, bug.ID, nil)
	requireStatus(t, response, http.StatusOK)
	actions := actionStrings(decodeData(t, response).(map[string]any)["allowed_actions"])
	if fmt.Sprint(actions) != fmt.Sprint([]string{"reset_triage", "return"}) {
		t.Fatalf("allowed_actions=%v body=%s", actions, response.Body.String())
	}
}

func TestQualityBugPatchIsWorkspaceWide(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	tests := []struct {
		name   string
		actor  string
		status string
		body   map[string]any
		want   int
	}{
		{name: "assignee updates fields", actor: "assignee", status: "in_progress", body: map[string]any{"title": "assignee edit", "priority": "high"}, want: http.StatusOK},
		{name: "assignee submits review", actor: "assignee", status: "in_progress", body: map[string]any{"status": "review", "resolution_reason": "fixed"}, want: http.StatusOK},
		{name: "assignee completes regression", actor: "assignee", status: "review", body: map[string]any{"status": "done"}, want: http.StatusOK},
		{name: "reviewer passes regression", actor: "reviewer", status: "review", body: map[string]any{"status": "done"}, want: http.StatusOK},
		{name: "reviewer returns regression", actor: "reviewer", status: "review", body: map[string]any{"status": "reopened"}, want: http.StatusOK},
		{name: "reviewer edits fields", actor: "reviewer", status: "review", body: map[string]any{"title": "reviewer edit"}, want: http.StatusOK},
		{name: "reviewer edits status and priority", actor: "reviewer", status: "review", body: map[string]any{"status": "done", "priority": "urgent"}, want: http.StatusOK},
		{name: "tester passes regression", actor: "tester", status: "review", body: map[string]any{"status": "done"}, want: http.StatusOK},
		{name: "tester resets triage", actor: "tester", status: "done", body: map[string]any{"status": "triage"}, want: http.StatusOK},
		{name: "reporter updates report", actor: "reporter", status: "triage", body: map[string]any{"title": "clear report", "found_version": "v2"}, want: http.StatusOK},
		{name: "reporter assigns", actor: "reporter", status: "triage", body: map[string]any{"assignee_user_id": fix.users["reporter"].ID}, want: http.StatusOK},
		{name: "reporter transitions", actor: "reporter", status: "triage", body: map[string]any{"status": "in_progress"}, want: http.StatusOK},
		{name: "legacy readonly role updates", actor: "readonly", status: "triage", body: map[string]any{"title": "readonly role edit"}, want: http.StatusOK},
		{name: "quality owner fully manages", actor: "quality_owner", status: "triage", body: map[string]any{"status": "done", "resolution_reason": "as_designed", "resolution_note": "confirmed"}, want: http.StatusOK},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bug := fix.createBug(t, uint(200+index), test.status, test.actor == "readonly")
			response := fix.request(t, test.actor, http.MethodPatch, bug.ID, test.body)
			requireStatus(t, response, test.want)
			if test.name == "tester resets triage" {
				var stored qualitygorm.QualityBug
				if err := fix.app.db.First(&stored, bug.ID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.ResolutionReason != "" || stored.ResolutionNote != "" || stored.ReviewerUserID != nil || stored.ResolvedAt != nil {
					t.Fatalf("triage did not reset resolution state: %+v", stored)
				}
			}
		})
	}
}

func TestQualityBugRejectsInvalidResolutionReasonButNotLegacyRoleScope(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	bug := fix.createBug(t, 300, "in_progress", false)
	invalid := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"status": "review", "resolution_reason": "invalid_reason",
	})
	requireStatus(t, invalid, http.StatusBadRequest)
	retiredStatus := fix.request(t, "manager", http.MethodPatch, bug.ID, map[string]any{
		"status": "backlog",
	})
	requireStatus(t, retiredStatus, http.StatusBadRequest)

	otherRequirement := requirementgorm.Requirement{
		ID: 14, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, PlanUnitID: fix.plan.ID, Title: "Other requirement",
	}
	if err := fix.app.db.Create(&otherRequirement).Error; err != nil {
		t.Fatal(err)
	}
	otherBug := fix.createBugForRequirement(t, 301, "triage", otherRequirement.ID, false)
	response := fix.request(t, "quality_owner", http.MethodPatch, otherBug.ID, map[string]any{"title": "workspace-wide"})
	requireStatus(t, response, http.StatusOK)
}

func TestQualityBugDeleteIsWorkspaceWide(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	reporterBug := fix.createBug(t, 400, "triage", false)
	allowed := fix.request(t, "reporter", http.MethodDelete, reporterBug.ID, nil)
	requireStatus(t, allowed, http.StatusOK)
}

func TestQualityTransitionActionCannotEditSensitiveFieldsOrDelete(t *testing.T) {
	fix := setupQualityPermissionFixtureWithConfig(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	access := identity.NewAccessService(identitygorm.NewAccessRepository(fix.app.db), nil)
	engine := NewWithDependencies(fix.app.cfg, fix.app.db, Dependencies{
		Authorizer: access, ProjectAccess: fakeAccessAllower{},
	})
	inventory := NewRouteInventory()
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	qualityActions := map[accesscatalog.ActionCode]struct{}{
		accesscatalog.ActionQualityBugsUpdate:           {},
		accesscatalog.ActionQualityBugsTransitionStatus: {},
		accesscatalog.ActionQualityBugsDelete:           {},
	}
	for _, record := range accesscatalog.BootstrapActionRoutes() {
		if _, ok := qualityActions[record.ActionCode]; !ok {
			continue
		}
		if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
			Code: record.InterfaceCode, ActionCode: string(record.ActionCode),
			HTTPMethod: record.HTTPMethod, RouteTemplate: record.RouteTemplate,
			Name: record.ActionName, Description: record.CopyKey, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	transitioner, err := access.CreateRole(context.Background(), identity.RoleInput{
		Code: "quality-transitioner", Name: "Quality transitioner", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleActionCodes(
		context.Background(), fix.users["manager"].ID, transitioner.ID,
		[]string{string(accesscatalog.ActionQualityBugsTransitionStatus)},
	); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), fix.users["readonly"].ID, []uint{transitioner.ID}); err != nil {
		t.Fatal(err)
	}
	fix.app.engine = engine
	bug := fix.createBug(t, 450, "triage", true)
	token := fix.app.token(t, fix.users["readonly"])

	transition := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/status", bug.ID),
		token, fix.workspaceID, map[string]any{"status": "in_progress"})
	requireStatus(t, transition, http.StatusOK)
	assignment := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/assignment", bug.ID),
		token, fix.workspaceID, map[string]any{
			"assignee_user_id": fix.users["reviewer"].ID,
			"assignment_note":  "handoff",
		})
	requireStatus(t, assignment, http.StatusOK)
	batchAssignment := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/quality/bugs/batch-assignment",
		token, fix.workspaceID, map[string]any{
			"ids": []uint{bug.ID}, "assignee_user_id": fix.users["assignee"].ID,
		})
	requireStatus(t, batchAssignment, http.StatusOK)
	assignmentSensitive := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/assignment", bug.ID),
		token, fix.workspaceID, map[string]any{
			"assignee_user_id": fix.users["reviewer"].ID,
			"severity_key":     "p0",
		})
	requireStatus(t, assignmentSensitive, http.StatusBadRequest)
	sensitive := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/status", bug.ID),
		token, fix.workspaceID, map[string]any{"status": "review", "severity_key": "p0"})
	requireStatus(t, sensitive, http.StatusBadRequest)
	fullEdit := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", bug.ID),
		token, fix.workspaceID, map[string]any{"severity_key": "p0"})
	requireStatus(t, fullEdit, http.StatusForbidden)
	deleted := fix.app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d", bug.ID),
		token, fix.workspaceID, nil)
	requireStatus(t, deleted, http.StatusForbidden)
}

func TestBroadQualityUpdateActionStillIncludesDedicatedTransitionRoute(t *testing.T) {
	fix := setupQualityPermissionFixtureWithConfig(t, func(cfg *config.Config) {
		cfg.RBACMode = config.RBACModeEnforce
	})
	access := identity.NewAccessService(identitygorm.NewAccessRepository(fix.app.db), nil)
	engine := NewWithDependencies(fix.app.cfg, fix.app.db, Dependencies{
		Authorizer: access, ProjectAccess: fakeAccessAllower{},
	})
	inventory := NewRouteInventory()
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	for _, record := range accesscatalog.BootstrapActionRoutes() {
		if record.ActionCode != accesscatalog.ActionQualityBugsUpdate &&
			record.ActionCode != accesscatalog.ActionQualityBugsTransitionStatus {
			continue
		}
		if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
			Code: record.InterfaceCode, ActionCode: string(record.ActionCode),
			HTTPMethod: record.HTTPMethod, RouteTemplate: record.RouteTemplate,
			Name: record.ActionName, Description: record.CopyKey, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	editor, err := access.CreateRole(context.Background(), identity.RoleInput{
		Code: "quality-editor", Name: "Quality editor", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceRoleActionCodes(
		context.Background(), fix.users["manager"].ID, editor.ID,
		[]string{string(accesscatalog.ActionQualityBugsUpdate)},
	); err != nil {
		t.Fatal(err)
	}
	if err := access.ReplaceUserRoles(context.Background(), fix.users["readonly"].ID, []uint{editor.ID}); err != nil {
		t.Fatal(err)
	}
	fix.app.engine = engine
	bug := fix.createBug(t, 451, "triage", true)
	response := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/status", bug.ID),
		fix.app.token(t, fix.users["readonly"]), fix.workspaceID,
		map[string]any{"status": "in_progress"})
	requireStatus(t, response, http.StatusOK)
	assignment := fix.app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/quality/bugs/%d/assignment", bug.ID),
		fix.app.token(t, fix.users["readonly"]), fix.workspaceID,
		map[string]any{"assignee_user_id": fix.users["assignee"].ID})
	requireStatus(t, assignment, http.StatusOK)
}

func TestQualityBugLegacyFilterAliasesAndFoundVersion(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	assigneeID := fix.users["assignee"].ID
	rows := []qualitygorm.QualityBug{
		{ID: 500, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "QP-500", Title: "Exact version", Status: "review", Priority: "high",
			SeverityKey: "p1", AssigneeUserID: &assigneeID, FoundVersion: "V2.4.0"},
		{ID: 501, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12, RequirementID: 13,
			BugKey: "QP-501", Title: "Fuzzy version", Status: "triage", Priority: "medium",
			SeverityKey: "p2", AssigneeUserID: &assigneeID, FoundVersion: "release-2.4-hotfix"},
	}
	for index := range rows {
		if err := fix.app.db.Create(&rows[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := fix.app.token(t, fix.users["manager"])
	exact := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs?statuses=review&priorities=high&assignee_ids=%d&found_version=v2.4.0", assigneeID),
		token, fix.workspaceID, nil)
	requireStatus(t, exact, http.StatusOK)
	if data := decodeData(t, exact).(map[string]any); data["total"] != float64(1) {
		t.Fatalf("legacy aliases/exact version=%s", exact.Body.String())
	}
	fuzzy := fix.app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/quality/bugs?status=triage&priority=medium&assignee_user_id=%d&found_version_like=2.4", assigneeID),
		token, fix.workspaceID, nil)
	requireStatus(t, fuzzy, http.StatusOK)
	if data := decodeData(t, fuzzy).(map[string]any); data["total"] != float64(1) {
		t.Fatalf("singular aliases/fuzzy version=%s", fuzzy.Body.String())
	}
}

func setupQualityPermissionFixture(t *testing.T) qualityPermissionFixture {
	return setupQualityPermissionFixtureWithConfig(t, nil)
}

func setupQualityPermissionFixtureWithConfig(
	t *testing.T,
	mutate func(*config.Config),
) qualityPermissionFixture {
	t.Helper()
	app := newTestApp(t, mutate)
	users := map[string]identitygorm.User{}
	roles := map[string]string{
		"system_admin": "admin", "team_admin": "user", "manager": "user",
		"owner": "user", "quality_owner": "user", "assignee": "user",
		"reviewer": "user", "tester": "user", "reporter": "user", "readonly": "readonly",
	}
	userID := uint(1)
	for name, role := range roles {
		users[name] = app.createUser(t, userID, "quality-"+name, role, "")
		userID++
	}
	workspace := identitygorm.Workspace{ID: 1, Name: "Quality workspace", Slug: "quality-workspace", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "Quality team"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "Quality project", Code: "QP", OwnerUserID: uintPtr(users["owner"].ID)}
	plan := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "Quality plan"}
	requirement := requirementgorm.Requirement{ID: 13, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, PlanUnitID: plan.ID, Title: "Quality requirement"}
	for _, row := range []any{&workspace, &team, &project, &plan, &requirement} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	permissionByName := map[string]projectgorm.ProjectPermission{}
	for name, user := range users {
		workspaceRole := "member"
		if name == "manager" {
			workspaceRole = "admin"
		}
		teamRole := "member"
		if name == "team_admin" {
			teamRole = "admin"
		}
		for _, row := range []any{
			&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: workspaceRole, Enabled: true},
			&identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: teamRole},
		} {
			if err := app.db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
		if name == "owner" || name == "manager" {
			continue
		}
		permission := projectgorm.ProjectPermission{
			WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
			UserID: user.ID, AccessLevel: "member",
		}
		if err := app.db.Create(&permission).Error; err != nil {
			t.Fatal(err)
		}
		permissionByName[name] = permission
	}
	for name, roleKey := range map[string]string{"quality_owner": "quality_owner", "tester": "tester"} {
		permission := permissionByName[name]
		if err := app.db.Create(&projectgorm.ProjectMemberRole{
			WorkspaceID: workspace.ID, ProjectPermissionID: permission.ID, RoleKey: roleKey,
			ScopeType: "requirement", ScopeID: requirement.ID, SourceType: "manual",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return qualityPermissionFixture{
		app: app, workspaceID: workspace.ID, project: project, plan: plan,
		requirement: requirement, users: users,
	}
}

func (fix qualityPermissionFixture) createBug(
	t *testing.T,
	id uint,
	status string,
	readonlyAssignee bool,
) qualitygorm.QualityBug {
	return fix.createBugForRequirement(t, id, status, fix.requirement.ID, readonlyAssignee)
}

func (fix qualityPermissionFixture) createBugForRequirement(
	t *testing.T,
	id uint,
	status string,
	requirementID uint,
	readonlyAssignee bool,
) qualitygorm.QualityBug {
	t.Helper()
	assigneeID := fix.users["assignee"].ID
	if readonlyAssignee {
		assigneeID = fix.users["readonly"].ID
	}
	reporterID := fix.users["reporter"].ID
	reviewerID := fix.users["reviewer"].ID
	bug := qualitygorm.QualityBug{
		ID: id, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: requirementID,
		BugKey: fmt.Sprintf("QP-%d", id), Title: fmt.Sprintf("Quality bug %d", id),
		Status: status, Priority: "medium", SeverityKey: "p2",
		AssigneeUserID: &assigneeID, ReporterUserID: &reporterID,
		ReviewerUserID: &reviewerID, ResolutionReason: "fixed", ResolutionNote: "resolved",
	}
	if err := fix.app.db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}
	return bug
}

func (fix qualityPermissionFixture) request(
	t *testing.T,
	actor string,
	method string,
	bugID uint,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	return fix.app.request(
		t, method, fmt.Sprintf("/api/v1/platform/quality/bugs/%d", bugID),
		fix.app.token(t, fix.users[actor]), fix.workspaceID, body,
	)
}

func actionStrings(raw any) []string {
	items, _ := raw.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, fmt.Sprint(item))
	}
	sort.Strings(result)
	return result
}
