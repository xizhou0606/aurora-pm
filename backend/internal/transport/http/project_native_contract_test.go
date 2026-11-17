package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"testing"
)

func TestProjectAndPlanDetailAreWorkspaceScoped(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "project-contract-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	project := app.request(t, http.MethodGet, "/api/v1/platform/project-sets/11", token, 1, nil)
	requireStatus(t, project, http.StatusOK)
	projectData := decodeData(t, project).(map[string]any)
	if projectData["code"] != "A" || projectData["name"] != "Project A" {
		t.Fatalf("project detail=%s", project.Body.String())
	}
	plan := app.request(t, http.MethodGet, "/api/v1/platform/project-new/12", token, 1, nil)
	requireStatus(t, plan, http.StatusOK)
	if decodeData(t, plan).(map[string]any)["title"] != "Version A" {
		t.Fatalf("plan detail=%s", plan.Body.String())
	}

	foreignProject := app.request(t, http.MethodGet, "/api/v1/platform/project-sets/11", token, 2, nil)
	requireStatus(t, foreignProject, http.StatusNotFound)
	foreignPlan := app.request(t, http.MethodGet, "/api/v1/platform/project-new/12", token, 2, nil)
	requireStatus(t, foreignPlan, http.StatusNotFound)
}

func TestProjectParticipantsAggregateSourcesWithoutGrantingAccess(t *testing.T) {
	app := newTestApp(t, nil)
	owner := app.createUser(t, 1, "project-owner", "super_admin", "")
	developer := app.createUser(t, 2, "project-developer", "user", "")
	reporter := app.createUser(t, 3, "project-reporter", "user", "")
	seedNativeWorkbenchScopes(t, app, owner)
	for _, user := range []identitygorm.User{developer, reporter} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 1, UserID: user.ID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	developerID, reporterID := developer.ID, reporter.ID
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", 12).
		Update("owner_user_id", developerID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).Updates(map[string]any{
		"product_owner_user_id": reporterID, "design_owner_user_id": developerID,
		"developer_user_ids": requirementgorm.UintList{developerID},
	}).Error; err != nil {
		t.Fatal(err)
	}
	task := taskgorm.WorkTask{
		ID: 701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-701", Title: "Participant task",
		Status: "draft", Priority: "normal", AssigneeID: &developerID, ReporterUserID: &reporterID,
	}
	bug := qualitygorm.QualityBug{
		ID: 801, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, BugKey: "BUG-801", Title: "Participant bug",
		Status: "triage", Priority: "medium", SeverityKey: "p2",
		AssigneeUserID: &reporterID, ReporterUserID: &developerID,
	}
	for _, row := range []any{&task, &bug} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Retired permission data must not influence either participant membership or access facts.
	if err := app.db.Create(&projectgorm.ProjectPermission{
		WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: 99, AccessLevel: "admin",
	}).Error; err != nil {
		t.Fatal(err)
	}

	token := app.token(t, owner)
	response := app.request(t, http.MethodGet,
		"/api/v1/platform/project-sets/11/participants", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	authorization := data["authorization"].(map[string]any)
	if authorization["informational_only"] != true || authorization["grants_access"] != false {
		t.Fatalf("participant authorization semantics=%s", response.Body.String())
	}
	participants := data["participants"].([]any)
	if len(participants) != 3 {
		t.Fatalf("participants=%s", response.Body.String())
	}
	byID := map[float64]map[string]any{}
	for _, raw := range participants {
		participant := raw.(map[string]any)
		byID[participant["user_id"].(float64)] = participant
	}
	if _, leaked := byID[99]; leaked {
		t.Fatalf("retired ProjectPermission leaked into participants=%s", response.Body.String())
	}
	developerRow := byID[float64(developerID)]
	if developerRow["username"] != developer.Username || len(developerRow["sources"].([]any)) != 5 {
		t.Fatalf("developer participant=%v", developerRow)
	}
	reporterRow := byID[float64(reporterID)]
	if reporterRow["username"] != reporter.Username || len(reporterRow["sources"].([]any)) != 3 {
		t.Fatalf("reporter participant=%v", reporterRow)
	}
	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/project-sets/11/participants", token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)
}

func TestEffectiveProjectAccessReportsActorAndOwnerWithoutLegacyPermission(t *testing.T) {
	app := newTestApp(t, nil)
	owner := app.createUser(t, 1, "effective-owner", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, owner)
	token := app.token(t, owner)

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/project-access/effective?project_set_id=11", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["effective"] != true {
		t.Fatalf("effective access=%s", response.Body.String())
	}
	actor := data["actor"].(map[string]any)
	if actor["user_id"] != float64(owner.ID) || actor["pm_user_id"] != float64(owner.ID) || actor["username"] != owner.Username {
		t.Fatalf("actor facts=%s", response.Body.String())
	}
	if _, found := actor["role"]; found {
		t.Fatalf("legacy actor role leaked into access response=%s", response.Body.String())
	}
	project := data["project_set"].(map[string]any)
	if project["actor_is_owner"] != true || project["code"] != "A" {
		t.Fatalf("project access=%s", response.Body.String())
	}
	basis := data["access_basis"].(map[string]any)
	if basis["model"] != "global_role_interface_grants" || basis["workspace_is_data_context"] != true ||
		basis["legacy_project_permission_used"] != false {
		t.Fatalf("legacy permission restored=%s", response.Body.String())
	}
	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/project-access/effective?project_set_id=11", token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)
}

func TestWorkspaceTaskExactFiltersAndNestedContext(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-context-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-701", Title: "Parent exact",
		Status: "draft", Priority: "normal",
	}
	child := parent
	child.ID, child.Kind, child.TaskKey, child.Title, child.ParentID =
		702, "subtask", "A-702", "Child exact", &parent.ID
	grandchild := parent
	grandchild.ID, grandchild.Kind, grandchild.TaskKey, grandchild.Title, grandchild.ParentID =
		703, "subtask", "A-703", "Unique Exact Title", &child.ID
	similar := parent
	similar.ID, similar.TaskKey, similar.Title = 704, "A-704", "Unique Exact Title extended"
	for _, row := range []*taskgorm.WorkTask{&parent, &child, &grandchild, &similar} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	byKey := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?task_key=a-703", token, 1, nil)
	requireStatus(t, byKey, http.StatusOK)
	keyItems := decodeData(t, byKey).(map[string]any)["items"].([]any)
	if len(keyItems) != 1 || keyItems[0].(map[string]any)["task_key"] != "A-703" {
		t.Fatalf("task_key filter=%s", byKey.Body.String())
	}
	row := keyItems[0].(map[string]any)
	scope := row["scope"].(map[string]any)
	if scope["project_set"].(map[string]any)["code"] != "A" ||
		scope["project_version"].(map[string]any)["title"] != "Version A" ||
		scope["requirement"].(map[string]any)["title"] != "Requirement A" {
		t.Fatalf("nested scope=%s", byKey.Body.String())
	}
	hierarchy := row["hierarchy"].(map[string]any)
	if hierarchy["depth"] != float64(2) || hierarchy["children_count"] != float64(0) ||
		len(hierarchy["ancestors"].([]any)) != 2 ||
		hierarchy["parent"].(map[string]any)["task_key"] != "A-702" {
		t.Fatalf("nested hierarchy=%s", byKey.Body.String())
	}

	byTitle := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?title_exact=unique%20exact%20title", token, 1, nil)
	requireStatus(t, byTitle, http.StatusOK)
	titleItems := decodeData(t, byTitle).(map[string]any)["items"].([]any)
	if len(titleItems) != 1 || titleItems[0].(map[string]any)["id"] != float64(grandchild.ID) {
		t.Fatalf("title_exact filter=%s", byTitle.Body.String())
	}

	foreign := app.request(t, http.MethodGet, "/api/v1/platform/tasks/703", token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)
}
