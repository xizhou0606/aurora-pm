package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestPMTaskKindBoundary(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "PM 边界", Code: "PMB", OwnerUserID: &admin.ID,
	}
	unit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1", Type: "version",
	}
	requirement := requirementgorm.Requirement{
		ID: 40, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: "任务边界",
	}
	for _, row := range []any{
		&workspace,
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: admin.ID, Role: "owner", Enabled: true,
		},
		&team, &project, &unit, &requirement,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, admin)

	rejectedCreate := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, workspace.ID,
		map[string]any{"requirement_id": requirement.ID, "kind": "bug", "title": "越界缺陷"})
	requireStatus(t, rejectedCreate, http.StatusBadRequest)

	parentTask := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, workspace.ID,
		map[string]any{"requirement_id": requirement.ID, "kind": "task", "title": "父任务"})
	requireStatus(t, parentTask, http.StatusCreated)
	parentID := uint(decodeData(t, parentTask).(map[string]any)["id"].(float64))
	createdTask := app.request(t, http.MethodPost, "/api/v1/platform/tasks", token, workspace.ID,
		map[string]any{"requirement_id": requirement.ID, "kind": " SubTask ", "title": "合法子任务", "parent_id": parentID})
	requireStatus(t, createdTask, http.StatusCreated)
	createdRow := decodeData(t, createdTask).(map[string]any)
	if createdRow["kind"] != taskgorm.WorkTaskKindSubtask {
		t.Fatalf("created kind=%v", createdRow["kind"])
	}
	createdID := uint(createdRow["id"].(float64))

	rejectedPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(createdID), token, workspace.ID,
		map[string]any{"kind": "bug"})
	requireStatus(t, rejectedPatch, http.StatusBadRequest)
	var unchanged taskgorm.WorkTask
	if err := app.db.First(&unchanged, createdID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Kind != taskgorm.WorkTaskKindSubtask {
		t.Fatalf("rejected patch changed kind to %q", unchanged.Kind)
	}

	foreign := taskgorm.WorkTask{
		WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
		ProjectID: unit.ID, RequirementID: requirement.ID, Kind: "bug",
		TaskKey: "BUG-OUTSIDE-PM", Title: "不属于 PM 的缺陷",
	}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	legacyPMTask := taskgorm.WorkTask{
		WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
		ProjectID: unit.ID, RequirementID: requirement.ID, Kind: " TASK ",
		TaskKey: "PMB-LEGACY", Title: "历史非规范任务",
	}
	if err := app.db.Create(&legacyPMTask).Error; err != nil {
		t.Fatal(err)
	}

	list := app.request(t, http.MethodGet,
		"/api/v1/platform/scrum-tasks-new?project_new_id="+uintString(project.ID),
		token, workspace.ID, nil)
	requireStatus(t, list, http.StatusOK)
	var listBody struct {
		Data  []map[string]any `json:"data"`
		Total int64            `json:"total"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if listBody.Total != 3 || len(listBody.Data) != 3 {
		t.Fatalf("foreign task leaked through list: %s", list.Body.String())
	}
	for _, task := range listBody.Data {
		if task["kind"] == "bug" {
			t.Fatalf("foreign task leaked through list: %s", list.Body.String())
		}
	}

	board := app.request(t, http.MethodGet,
		"/api/v1/platform/project-management-board?project_new_id="+uintString(project.ID)+"&include=tasks",
		token, workspace.ID, nil)
	requireStatus(t, board, http.StatusOK)
	boardTasks := decodeData(t, board).(map[string]any)["tasks"].([]any)
	if len(boardTasks) != 3 {
		t.Fatalf("foreign task leaked through board: %s", board.Body.String())
	}
	for _, task := range boardTasks {
		if task.(map[string]any)["kind"] == "bug" {
			t.Fatalf("foreign task leaked through board: %s", board.Body.String())
		}
	}

	foreignPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/tasks/"+uintString(foreign.ID), token, workspace.ID,
		map[string]any{"title": "不应可访问"})
	requireStatus(t, foreignPatch, http.StatusNotFound)
	foreignActivities := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/"+uintString(foreign.ID)+"/activities",
		token, workspace.ID, nil)
	requireStatus(t, foreignActivities, http.StatusNotFound)
}
