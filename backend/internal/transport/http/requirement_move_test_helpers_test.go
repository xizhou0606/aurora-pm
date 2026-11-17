package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

type requirementMoveFixture struct {
	app                *testApp
	workspace          identitygorm.Workspace
	team               identitygorm.Team
	actor              identitygorm.User
	productOwner       identitygorm.User
	sourceProject      projectgorm.ProjectSet
	targetProject      projectgorm.ProjectSet
	sourceUnit         projectgorm.PlanUnit
	targetUnit         projectgorm.PlanUnit
	crossUnit          projectgorm.PlanUnit
	requirementID      uint
	requirementVersion uint
	workflowTemplateID uint
}

func setupRequirementMoveFixture(t *testing.T) requirementMoveFixture {
	t.Helper()
	app := newTestApp(t, nil)
	projectOwner := app.createUser(t, 1, "project-owner", "user", "")
	actor := app.createUser(t, 2, "project-admin", "user", "")
	productOwner := app.createUser(t, 3, "product-owner", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	sourceProject := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "源项目集", Code: "SRC", OwnerUserID: &projectOwner.ID, Status: "active",
	}
	targetProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "目标项目集", Code: "DST", OwnerUserID: &projectOwner.ID, Status: "active",
	}
	sourceUnit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: sourceProject.ID, Title: "源项目", Type: "version", Status: "planning",
	}
	targetUnit := projectgorm.PlanUnit{
		ID: 31, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: sourceProject.ID, Title: "同项目集目标", Type: "version", Status: "planning",
	}
	crossUnit := projectgorm.PlanUnit{
		ID: 32, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: targetProject.ID, Title: "跨项目集目标", Type: "version", Status: "planning",
	}
	for _, row := range []any{
		&workspace, &team, &sourceProject, &targetProject, &sourceUnit, &targetUnit, &crossUnit,
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: projectOwner.ID, Role: "member", Enabled: true,
		},
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: actor.ID, Role: "member", Enabled: true,
		},
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: productOwner.ID, Role: "member", Enabled: true,
		},
		&identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: projectOwner.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: actor.ID, Role: "member"},
		&identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: productOwner.ID, Role: "member"},
		&projectgorm.ProjectPermission{
			WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: sourceProject.ID,
			UserID: actor.ID, AccessLevel: "admin",
		},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	workflowTemplateID := createPublishedRequirementWorkflow(t, app, app.token(t, actor), workspace.ID)
	create := app.request(t, http.MethodPost,
		"/api/v1/platform/product-requirements-new", app.token(t, actor), workspace.ID,
		map[string]any{
			"version_task_new_id":   sourceUnit.ID,
			"title":                 "支付链路",
			"product_owner_user_id": productOwner.ID,
			"workflow_template_id":  workflowTemplateID,
			"priority":              "p1",
		})
	requireStatus(t, create, http.StatusCreated)
	createdRequirement := decodeData(t, create).(map[string]any)
	return requirementMoveFixture{
		app: app, workspace: workspace, team: team, actor: actor, productOwner: productOwner,
		sourceProject: sourceProject, targetProject: targetProject,
		sourceUnit: sourceUnit, targetUnit: targetUnit, crossUnit: crossUnit,
		requirementID:      uint(createdRequirement["id"].(float64)),
		requirementVersion: uint(createdRequirement["version"].(float64)),
		workflowTemplateID: workflowTemplateID,
	}
}

func (f requirementMoveFixture) move(
	t *testing.T,
	user identitygorm.User,
	targetUnitID uint,
) *httptest.ResponseRecorder {
	t.Helper()
	return f.app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(f.requirementID),
		f.app.token(t, user), f.workspace.ID,
		map[string]any{"version_task_new_id": targetUnitID, "expected_version": f.requirementVersion})
}
