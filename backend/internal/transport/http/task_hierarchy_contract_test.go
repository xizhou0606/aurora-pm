package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"testing"
	"time"
)

func TestTaskHierarchyScopeDueAndActivityContracts(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "hierarchy-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []any{
		&projectgorm.ProjectSet{ID: 15, WorkspaceID: 1, TeamID: 10, Name: "Project target", Code: "T", OwnerUserID: &admin.ID},
		&projectgorm.PlanUnit{ID: 16, WorkspaceID: 1, TeamID: 10, ProjectSetID: 15, Title: "Version target"},
		&requirementgorm.Requirement{ID: 17, WorkspaceID: 1, TeamID: 10, ProjectSetID: 15, PlanUnitID: 16, Title: "Requirement target"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	due := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	parent := taskgorm.WorkTask{
		ID: 100, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-100", Title: "Parent",
		Status: "draft", Priority: "normal", DueAt: &due,
	}
	foreign := taskgorm.WorkTask{
		ID: 200, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
		RequirementID: 23, Kind: "task", TaskKey: "B-200", Title: "Foreign",
		Status: "draft", Priority: "normal",
	}
	if err := app.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	token := app.token(t, admin)

	created := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, 1, map[string]any{
		"parent_id": parent.ID, "kind": "subtask", "title": "Inherited child",
	})
	requireStatus(t, created, http.StatusCreated)
	childData := decodeData(t, created).(map[string]any)
	childID := uint(childData["id"].(float64))
	if childData["requirement_id"] != float64(13) || childData["due_at"] == nil ||
		childData["reporter_user_id"] != float64(1) {
		t.Fatalf("child did not inherit scope/due: %s", created.Body.String())
	}

	related := app.request(t, http.MethodGet, "/api/v1/platform/tasks/100/related", token, 1, nil)
	requireStatus(t, related, http.StatusOK)
	items := decodeData(t, related).(map[string]any)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["relation"] != "child" {
		t.Fatalf("related contract=%s", related.Body.String())
	}
	foreignRelated := app.request(t, http.MethodGet, "/api/v1/platform/tasks/200/related", token, 1, nil)
	requireStatus(t, foreignRelated, http.StatusNotFound)

	later := "2026-09-02"
	updatedChild := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(childID), token, 1, map[string]any{"due_at": later})
	requireStatus(t, updatedChild, http.StatusOK)
	conflicts := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/100/due-at-descendant-conflicts?due_at=2026-08-31", token, 1, nil)
	requireStatus(t, conflicts, http.StatusOK)
	conflictItems := decodeData(t, conflicts).(map[string]any)["items"].([]any)
	if len(conflictItems) != 1 || conflictItems[0].(map[string]any)["id"] != float64(childID) {
		t.Fatalf("due conflicts=%s", conflicts.Body.String())
	}

	moved := app.request(t, http.MethodPatch, "/api/v1/platform/tasks/100", token, 1,
		map[string]any{"requirement_id": 17})
	requireStatus(t, moved, http.StatusOK)
	if decodeData(t, moved).(map[string]any)["project_set_id"] != float64(15) {
		t.Fatalf("moved task=%s", moved.Body.String())
	}
	var storedChild taskgorm.WorkTask
	if err := app.db.First(&storedChild, childID).Error; err != nil {
		t.Fatal(err)
	}
	if storedChild.ProjectSetID != 15 || storedChild.ProjectID != 16 || storedChild.RequirementID != 17 {
		t.Fatalf("child scope was not cascaded: %+v", storedChild)
	}
	moveChild := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(childID), token, 1, map[string]any{"requirement_id": 13})
	requireStatus(t, moveChild, http.StatusBadRequest)

	completed := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(childID), token, 1, map[string]any{"status": "completed"})
	requireStatus(t, completed, http.StatusOK)
	var storedParent taskgorm.WorkTask
	if err := app.db.First(&storedParent, parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedParent.Status != "completed" || storedParent.CompletedAt == nil {
		t.Fatalf("parent was not auto-completed: %+v", storedParent)
	}
	blocked := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(childID), token, 1, map[string]any{"status": "blocked"})
	requireStatus(t, blocked, http.StatusBadRequest)

	activity := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/"+uintString(childID)+"/activities?limit=1&offset=0", token, 1, nil)
	requireStatus(t, activity, http.StatusOK)
	activityData := decodeData(t, activity).(map[string]any)
	if activityData["total"].(float64) < 2 || len(activityData["items"].([]any)) != 1 {
		t.Fatalf("activity pagination=%s", activity.Body.String())
	}
	row := activityData["items"].([]any)[0].(map[string]any)
	if row["field_key"] == "" || row["summary"] == "" || row["display_name"] == "" {
		t.Fatalf("structured activity=%v", row)
	}
}

func TestDeleteTaskRequiresExplicitCascadeAndDeletesDescendantTree(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "cascade-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 301, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-301", Title: "Parent",
		Status: "draft", Priority: "normal",
	}
	child := taskgorm.WorkTask{
		ID: 302, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "subtask", TaskKey: "A-302", Title: "Child",
		Status: "draft", Priority: "normal", ParentID: &parent.ID,
	}
	grandchild := taskgorm.WorkTask{
		ID: 303, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "subtask", TaskKey: "A-303", Title: "Grandchild",
		Status: "draft", Priority: "normal", ParentID: &child.ID,
	}
	bug := qualitygorm.QualityBug{
		ID: 304, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, BugKey: "A-B-304", Title: "References descendant",
		Status: "triage", Priority: "medium", SeverityKey: "p2",
		SourceExecutionID: &grandchild.ID,
	}
	for _, row := range []any{&parent, &child, &grandchild, &bug} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	blocked := app.request(t, http.MethodDelete, "/api/v1/platform/tasks/301", token, 1, nil)
	requireStatus(t, blocked, http.StatusConflict)
	var retained int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{301, 302, 303}).Count(&retained)
	if retained != 3 {
		t.Fatalf("unconfirmed cascade removed tasks: %d", retained)
	}

	deleted := app.request(t, http.MethodDelete, "/api/v1/platform/tasks/301?cascade=true", token, 1, nil)
	requireStatus(t, deleted, http.StatusOK)
	data := decodeData(t, deleted).(map[string]any)
	if data["deleted_count"] != float64(3) {
		t.Fatalf("cascade response=%s", deleted.Body.String())
	}
	var remaining int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{301, 302, 303}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("cascade left %d tasks", remaining)
	}
	var storedBug qualitygorm.QualityBug
	if err := app.db.First(&storedBug, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedBug.SourceExecutionID != nil {
		t.Fatalf("cascade left quality source reference: %v", storedBug.SourceExecutionID)
	}
}

func TestPatchTaskClearsStartAndDueDates(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "clear-task-dates-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	startsAt := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	dueAt := time.Date(2026, 8, 12, 23, 59, 59, 0, time.UTC)
	task := taskgorm.WorkTask{
		ID: 401, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-401", Title: "Clear dates",
		Status: "draft", Priority: "normal", StartsAt: &startsAt, DueAt: &dueAt,
	}
	if err := app.db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPatch, "/api/v1/platform/tasks/401", token, 1,
		map[string]any{"starts_at": nil, "due_at": nil})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["starts_at"] != nil || data["due_at"] != nil {
		t.Fatalf("clear response retained task dates: %s", response.Body.String())
	}

	var stored taskgorm.WorkTask
	if err := app.db.First(&stored, task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.StartsAt != nil || stored.DueAt != nil {
		t.Fatalf("cleared task dates did not persist as null: %+v", stored)
	}
	var activities []taskgorm.WorkTaskActivity
	if err := app.db.Where("work_task_id = ?", task.ID).Order("id ASC").Find(&activities).Error; err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 || activities[0].NewValue != "" || activities[1].NewValue != "" {
		t.Fatalf("date clear activities were not recorded as empty values: %+v", activities)
	}
}
