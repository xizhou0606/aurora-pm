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

func TestListTasksFiltersByPlanUnit(t *testing.T) {
	app := newTestApp(t, nil)
	actor := app.createUser(t, 1, "plan-unit-reader", "user", "")
	actor.DisplayName = "计划单负责人"
	actor.AvatarURL = "https://cdn.example.test/plan-unit-reader.png"
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", actor.ID).Updates(map[string]any{
		"display_name": actor.DisplayName,
		"avatar_url":   actor.AvatarURL,
	}).Error; err != nil {
		t.Fatal(err)
	}
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "多版本项目", Code: "MULTI",
	}
	otherProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "其他项目", Code: "OTHER",
	}
	firstUnit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1",
	}
	secondUnit := projectgorm.PlanUnit{
		ID: 31, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V2",
	}
	otherUnit := projectgorm.PlanUnit{
		ID: 32, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: otherProject.ID, Title: "Other V1",
	}
	firstRequirement := requirementgorm.Requirement{
		ID: 40, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: firstUnit.ID, Title: "V1 需求",
	}
	secondRequirement := requirementgorm.Requirement{
		ID: 41, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: secondUnit.ID, Title: "V2 需求",
	}
	for _, row := range []any{
		&workspace,
		&team,
		&project,
		&otherProject,
		&firstUnit,
		&secondUnit,
		&otherUnit,
		&firstRequirement,
		&secondRequirement,
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: actor.ID, Role: "member", Enabled: true,
		},
		&taskgorm.WorkTask{
			ID: 50, WorkspaceID: workspace.ID, TeamID: team.ID,
			ProjectSetID: project.ID, ProjectID: firstUnit.ID,
			RequirementID: firstRequirement.ID, Kind: taskgorm.WorkTaskKindTask,
			TaskKey: "MULTI-1", Title: "V1 任务", AssigneeID: &actor.ID,
		},
		&taskgorm.WorkTask{
			ID: 51, WorkspaceID: workspace.ID, TeamID: team.ID,
			ProjectSetID: project.ID, ProjectID: secondUnit.ID,
			RequirementID: secondRequirement.ID, Kind: taskgorm.WorkTaskKindTask,
			TaskKey: "MULTI-2", Title: "V2 任务",
		},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := app.token(t, actor)

	projectList := app.request(t, http.MethodGet,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20",
		token, workspace.ID, nil)
	requireStatus(t, projectList, http.StatusOK)
	var projectBody struct {
		Data  []taskgorm.WorkTask `json:"data"`
		Total int64               `json:"total"`
	}
	if err := json.Unmarshal(projectList.Body.Bytes(), &projectBody); err != nil {
		t.Fatal(err)
	}
	if projectBody.Total != 2 || len(projectBody.Data) != 2 {
		t.Fatalf("project task list changed: %s", projectList.Body.String())
	}

	list := app.request(t, http.MethodGet,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20&version_task_new_id=30",
		token, workspace.ID, nil)
	requireStatus(t, list, http.StatusOK)
	var body struct {
		Data  []taskgorm.WorkTask `json:"data"`
		Total int64               `json:"total"`
		Users []listUserProfile   `json:"users"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Data) != 1 || body.Data[0].ID != 50 {
		t.Fatalf("plan-unit task filter returned %s", list.Body.String())
	}
	if len(body.Users) != 1 || body.Users[0].ID != actor.ID ||
		body.Users[0].DisplayName != actor.DisplayName || body.Users[0].AvatarURL != actor.AvatarURL {
		t.Fatalf("plan-unit task user profiles=%+v", body.Users)
	}

	projectMismatch := app.request(t, http.MethodGet,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20&version_task_new_id=32",
		token, workspace.ID, nil)
	requireStatus(t, projectMismatch, http.StatusBadRequest)

	requirementMismatch := app.request(t, http.MethodGet,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20&version_task_new_id=30&requirement_new_id=41",
		token, workspace.ID, nil)
	requireStatus(t, requirementMismatch, http.StatusBadRequest)
}
