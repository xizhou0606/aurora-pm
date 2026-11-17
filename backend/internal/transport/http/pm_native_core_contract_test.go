package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestPlatformUserSearchFiltersPMUserRegistry(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "directory-admin", "super_admin", "")
	for _, user := range []identitygorm.User{
		{ID: 33, ExternalSubject: "one2all|33", ExternalIssuer: "one2all", Username: "xizhou", DisplayName: "西洲", Active: true},
		{ID: 47, ExternalSubject: "one2all|47", ExternalIssuer: "one2all", Username: "tieao", DisplayName: "铁奥", Active: true},
		{ID: 48, ExternalSubject: "one2all|48", ExternalIssuer: "one2all", Username: "tieao-shadow", DisplayName: "铁奥影子", Active: true},
	} {
		if err := app.db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []any{
		&identitygorm.Workspace{ID: 1, Name: "Workspace", Slug: "user-search", Enabled: true},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rec := app.request(t, http.MethodGet,
		"/api/v1/core/users?q="+url.QueryEscape("铁奥")+"&exact=true", app.token(t, admin), 1, nil)
	requireStatus(t, rec, http.StatusOK)
	rows := decodeData(t, rec).([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != float64(47) {
		t.Fatalf("authoritative exact user search=%s", rec.Body.String())
	}
	ambiguous := app.request(t, http.MethodGet,
		"/api/v1/core/users?q=tieao&exact=false", app.token(t, admin), 1, nil)
	requireStatus(t, ambiguous, http.StatusOK)
	candidates := decodeData(t, ambiguous).([]any)
	if len(candidates) != 2 || candidates[0].(map[string]any)["id"] != float64(47) ||
		candidates[1].(map[string]any)["id"] != float64(48) {
		t.Fatalf("ambiguous user candidates=%s", ambiguous.Body.String())
	}
}

func TestWorkspaceTaskNativeCreatorFiltersAndIdentity(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-admin", "super_admin", "")
	assignee := app.createUser(t, 2, "tieao", "user", "")
	reporter := app.createUser(t, 3, "reporter", "user", "")
	if err := app.db.Model(&assignee).Update("display_name", "铁奥").Error; err != nil {
		t.Fatal(err)
	}
	seedNativeWorkbenchScopes(t, app, admin)
	for _, user := range []identitygorm.User{assignee, reporter} {
		if err := app.db.Create(&identitygorm.WorkspaceMember{
			WorkspaceID: 1, UserID: user.ID, Role: "member", Enabled: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	assigneeID, reporterID := assignee.ID, reporter.ID
	tasks := []taskgorm.WorkTask{
		{ID: 5700, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-5700", Title: "before",
			Status: "draft", Priority: "normal", AssigneeID: &assigneeID, ReporterUserID: &reporterID,
			CreatedAt: from.Add(-time.Minute)},
		{ID: 5701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-5701", Title: "111",
			Status: "draft", Priority: "normal", AssigneeID: &assigneeID, ReporterUserID: &reporterID,
			CreatedAt: from.Add(10 * time.Hour)},
		{ID: 5702, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-5702", Title: "other creator",
			Status: "draft", Priority: "normal", AssigneeID: &assigneeID, ReporterUserID: &reporterID,
			CreatedAt: from.Add(11 * time.Hour)},
	}
	if err := app.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	activities := []taskgorm.WorkTaskActivity{
		{WorkspaceID: 1, WorkTaskID: 5701, UserID: assignee.ID, Action: "created", CreatedAt: from.Add(10 * time.Hour)},
		{WorkspaceID: 1, WorkTaskID: 5702, UserID: reporter.ID, Action: "created", CreatedAt: from.Add(11 * time.Hour)},
	}
	if err := app.db.Create(&activities).Error; err != nil {
		t.Fatal(err)
	}

	rec := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?ids=5700,5701,5702&created_by_user_id=2&created_from=2026-08-10&created_to=2026-08-10&offset=0",
		app.token(t, admin), 1, nil)
	requireStatus(t, rec, http.StatusOK)
	data := decodeData(t, rec).(map[string]any)
	items := data["items"].([]any)
	if data["total"] != float64(1) || len(items) != 1 {
		t.Fatalf("native task filters=%s", rec.Body.String())
	}
	item := items[0].(map[string]any)
	for key, want := range map[string]any{
		"id": float64(5701), "created_by_user_id": float64(2), "created_by_username": "tieao",
		"assignee_id": float64(2), "assignee_username": "tieao",
		"reporter_user_id": float64(3), "reporter_username": "reporter",
	} {
		if item[key] != want {
			t.Fatalf("task identity %s=%v want=%v row=%v", key, item[key], want, item)
		}
	}
	latest := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?sort=created_at_desc&limit=1&offset=0",
		app.token(t, admin), 1, nil)
	requireStatus(t, latest, http.StatusOK)
	latestData := decodeData(t, latest).(map[string]any)
	latestItems := latestData["items"].([]any)
	if latestData["total"] != float64(3) || len(latestItems) != 1 ||
		latestItems[0].(map[string]any)["id"] != float64(5702) {
		t.Fatalf("zero offset must keep latest task=%s", latest.Body.String())
	}
}

func TestTaskMutationsValidateActivePMUserFacts(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-admin", "super_admin", "")
	assignee := app.createUser(t, 47, "task-assignee", "user", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1, map[string]any{
		"requirement_id": 13, "title": "validated", "assignee_id": 47,
	})
	requireStatus(t, created, http.StatusCreated)
	taskID := uint(decodeData(t, created).(map[string]any)["id"].(float64))

	missing := app.request(t, http.MethodPatch, "/api/v1/platform/tasks/"+uintString(taskID), token, 1,
		map[string]any{"assignee_id": 999})
	requireStatus(t, missing, http.StatusBadRequest)
	if !strings.Contains(missing.Body.String(), "PM user reference ID 999 is not active") {
		t.Fatalf("missing PM user response=%s", missing.Body.String())
	}

	if err := app.db.Model(&assignee).Update("active", false).Error; err != nil {
		t.Fatal(err)
	}
	inactive := app.request(t, http.MethodPatch, "/api/v1/platform/tasks/"+uintString(taskID), token, 1,
		map[string]any{"reporter_user_id": 47})
	requireStatus(t, inactive, http.StatusBadRequest)
}

func TestRequirementDetailAndDeletionImpactAreWorkspaceScoped(t *testing.T) {
	fix := setupQualityPermissionFixture(t)
	actorID := fix.users["owner"].ID
	for _, row := range []any{
		&identitygorm.Workspace{ID: 2, Name: "Other workspace", Slug: "other-requirement-workspace", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: actorID, Role: "member", Enabled: true},
	} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	task := taskgorm.WorkTask{
		ID: 701, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: fix.requirement.ID,
		Kind: "task", TaskKey: "QP-701", Title: "affected", Status: "draft", Priority: "normal",
	}
	bug := fix.createBug(t, 801, "triage", false)
	otherRequirement := requirementgorm.Requirement{
		ID: 802, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, PlanUnitID: fix.plan.ID, Title: "Other requirement",
		Priority: "normal", Status: "draft",
	}
	if err := fix.app.db.Create(&otherRequirement).Error; err != nil {
		t.Fatal(err)
	}
	externalReference := qualitygorm.QualityBug{
		ID: 803, WorkspaceID: fix.workspaceID, TeamID: fix.project.TeamID,
		ProjectSetID: fix.project.ID, ProjectID: fix.plan.ID, RequirementID: otherRequirement.ID,
		BugKey: "BUG-803", Title: "External source reference", Status: "triage",
		Priority: "medium", SeverityKey: "p2", SourceExecutionID: &task.ID,
	}
	for _, row := range []any{
		&task,
		&externalReference,
		&taskgorm.WorkTaskComment{ID: 711, WorkspaceID: fix.workspaceID, WorkTaskID: task.ID, UserID: actorID, Body: "comment"},
		&taskgorm.WorkTaskAttachment{ID: 712, WorkspaceID: fix.workspaceID, WorkTaskID: task.ID,
			UploaderUserID: actorID, Filename: "a.txt", Size: 1, ObjectKey: "test/a.txt", StorageBackend: "s3"},
		&taskgorm.WorkTaskWatcher{ID: 713, WorkspaceID: fix.workspaceID, WorkTaskID: task.ID, UserID: actorID},
		&taskgorm.WorkTaskActivity{ID: 714, WorkspaceID: fix.workspaceID, WorkTaskID: task.ID, UserID: actorID, Action: "created"},
		&qualitygorm.QualityBugActivity{ID: 715, WorkspaceID: fix.workspaceID, QualityBugID: bug.ID, UserID: actorID, Action: "created"},
	} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := fix.app.token(t, fix.users["owner"])
	detail := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID), token, fix.workspaceID, nil)
	requireStatus(t, detail, http.StatusOK)
	if decodeData(t, detail).(map[string]any)["id"] != float64(fix.requirement.ID) {
		t.Fatalf("requirement detail=%s", detail.Body.String())
	}
	impact := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID)+"/deletion-impact",
		token, fix.workspaceID, nil)
	requireStatus(t, impact, http.StatusOK)
	data := decodeData(t, impact).(map[string]any)
	counts := data["counts"].(map[string]any)
	for key, want := range map[string]float64{
		"requirements": 1, "tasks": 1, "bugs": 1, "task_comments": 1,
		"task_attachments": 1, "task_activities": 1, "task_watchers": 1, "bug_activities": 1,
		"quality_source_references": 1,
	} {
		if counts[key] != want {
			t.Fatalf("impact count %s=%v want=%v body=%s", key, counts[key], want, impact.Body.String())
		}
	}
	if data["affected_count"] != float64(9) || data["cascade_required"] != true {
		t.Fatalf("requirement impact=%s", impact.Body.String())
	}
	foreign := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID), token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)
	deleted := fix.app.request(t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID), token, fix.workspaceID, nil)
	requireStatus(t, deleted, http.StatusConflict)
	for name, target := range map[string]any{
		"requirement": &requirementgorm.Requirement{}, "task": &taskgorm.WorkTask{},
		"attachment": &taskgorm.WorkTaskAttachment{},
	} {
		var count int64
		if err := fix.app.db.Model(target).Where("workspace_id = ?", fix.workspaceID).Count(&count).Error; err != nil || count == 0 {
			t.Fatalf("%s mutated despite attachment conflict: count=%d err=%v", name, count, err)
		}
	}
}
