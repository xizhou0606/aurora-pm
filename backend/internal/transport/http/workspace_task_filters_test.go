package httpapi

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"testing"
	"time"
)

func TestWorkspaceTaskLegacyVisibleFiltersAndSort(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-filter-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	rows := []taskgorm.WorkTask{
		{ID: 701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-701", Title: "First",
			Description: "contains migration needle", Status: "draft", Priority: "normal",
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		{ID: 702, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-702", Title: "Second",
			Status: "draft", Priority: "normal", DueAt: timePtr(todayStart.Add(-time.Minute)),
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: 703, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-703", Title: "Finished overdue",
			Status: "completed", Priority: "normal", DueAt: timePtr(todayStart.Add(-time.Minute)),
			CreatedAt: now, UpdatedAt: now},
		{ID: 704, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-704", Title: "Due earlier today",
			Status: "draft", Priority: "normal", DueAt: timePtr(todayStart.Add(time.Minute)),
			CreatedAt: now.Add(-30 * time.Minute), UpdatedAt: now},
	}
	for index := range rows {
		if err := app.db.Create(&rows[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	search := app.request(t, http.MethodGet, "/api/v1/platform/tasks?q=migration+needle", token, 1, nil)
	requireStatus(t, search, http.StatusOK)
	searchData := decodeData(t, search).(map[string]any)
	if searchData["total"] != float64(1) || searchData["items"].([]any)[0].(map[string]any)["id"] != float64(701) {
		t.Fatalf("description search=%s", search.Body.String())
	}

	overdue := app.request(t, http.MethodGet, "/api/v1/platform/tasks?overdue=1", token, 1, nil)
	requireStatus(t, overdue, http.StatusOK)
	overdueData := decodeData(t, overdue).(map[string]any)
	if overdueData["total"] != float64(1) || overdueData["items"].([]any)[0].(map[string]any)["id"] != float64(702) {
		t.Fatalf("overdue alias=%s", overdue.Body.String())
	}

	created := app.request(t, http.MethodGet, "/api/v1/platform/tasks?sort=created_at_desc", token, 1, nil)
	requireStatus(t, created, http.StatusOK)
	createdRows := decodeData(t, created).(map[string]any)["items"].([]any)
	if createdRows[0].(map[string]any)["id"] != float64(703) {
		t.Fatalf("created_at_desc=%s", created.Body.String())
	}
	invalid := app.request(t, http.MethodGet, "/api/v1/platform/tasks?sort=random", token, 1, nil)
	requireStatus(t, invalid, http.StatusBadRequest)
}

func TestWorkspaceTaskFilteredTreePaginatesRootsAndKeepsMatchedContext(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "task-tree-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	rows := []taskgorm.WorkTask{
		{ID: 800, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "TREE-800", Title: "Context parent",
			Status: "draft", Priority: "normal"},
		{ID: 801, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "subtask", ParentID: uintPtr(800), TaskKey: "TREE-801",
			Title: "Matching child", Status: "in_dev", Priority: "normal"},
		{ID: 802, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "subtask", ParentID: uintPtr(800), TaskKey: "TREE-802",
			Title: "Filtered sibling", Status: "completed", Priority: "normal"},
		{ID: 803, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "subtask", ParentID: uintPtr(800), TaskKey: "TREE-803",
			Title: "Second matching child", Status: "in_dev", Priority: "normal"},
		{ID: 900, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "TREE-900", Title: "Other root",
			Status: "in_dev", Priority: "normal"},
	}
	if err := app.db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?hierarchy=filtered_tree&statuses=in_dev&limit=1&offset=1&sort=created_at_desc",
		token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["total"] != float64(2) || data["match_total"] != float64(3) ||
		data["root_total"] != float64(2) || data["page_unit"] != "root_task" {
		t.Fatalf("filtered tree totals=%s", response.Body.String())
	}
	items := data["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("filtered tree items=%s", response.Body.String())
	}
	parent := items[0].(map[string]any)
	if parent["id"] != float64(800) || parent["context_only"] != true || parent["filter_match"] != false {
		t.Fatalf("parent context=%v", parent)
	}
	childIDs := map[float64]bool{}
	for _, item := range items[1:] {
		child := item.(map[string]any)
		if child["context_only"] != false || child["filter_match"] != true {
			t.Fatalf("matched child=%v", child)
		}
		childIDs[child["id"].(float64)] = true
	}
	if !childIDs[801] || !childIDs[803] {
		t.Fatalf("matched child ids=%v", childIDs)
	}

	parentSearch := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?hierarchy=filtered_tree&q=Context%20parent",
		token, 1, nil)
	requireStatus(t, parentSearch, http.StatusOK)
	parentSearchData := decodeData(t, parentSearch).(map[string]any)
	parentItems := parentSearchData["items"].([]any)
	if parentSearchData["total"] != float64(1) || parentSearchData["match_total"] != float64(1) || len(parentItems) != 4 {
		t.Fatalf("parent search tree=%s", parentSearch.Body.String())
	}
	if parentItems[0].(map[string]any)["id"] != float64(800) ||
		parentItems[0].(map[string]any)["filter_match"] != true {
		t.Fatalf("parent search root=%v", parentItems[0])
	}
	for _, item := range parentItems[1:] {
		row := item.(map[string]any)
		if row["context_only"] != true || row["filter_match"] != false {
			t.Fatalf("parent search descendant context=%v", row)
		}
	}
}
