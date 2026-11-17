package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

var policyBatchQueryCallbackID atomic.Uint64

type policyBatchQueryFixture struct {
	app         *testApp
	actor       identitygorm.User
	workspace   identitygorm.Workspace
	project     projectgorm.ProjectSet
	unit        projectgorm.PlanUnit
	requirement requirementgorm.Requirement
}

func TestBoardPolicyQueriesDoNotGrowWithTaskRows(t *testing.T) {
	fix := setupPolicyBatchQueryFixture(t)
	fix.createTasks(t, 1)

	oneRow, oneRowQueries := requestWithSelectCount(
		t,
		fix.app,
		"/api/v1/platform/project-management-board?project_new_id=20&include=tasks",
		fix.app.token(t, fix.actor),
		fix.workspace.ID,
	)
	requireStatus(t, oneRow, http.StatusOK)
	if got := len(decodeData(t, oneRow).(map[string]any)["tasks"].([]any)); got != 1 {
		t.Fatalf("board task rows=%d, want 1", got)
	}

	fix.createTasks(t, 9)
	manyRows, manyRowQueries := requestWithSelectCount(
		t,
		fix.app,
		"/api/v1/platform/project-management-board?project_new_id=20&include=tasks",
		fix.app.token(t, fix.actor),
		fix.workspace.ID,
	)
	requireStatus(t, manyRows, http.StatusOK)
	tasks := decodeData(t, manyRows).(map[string]any)["tasks"].([]any)
	if len(tasks) != 10 {
		t.Fatalf("board task rows=%d, want 10", len(tasks))
	}
	if manyRowQueries != oneRowQueries {
		t.Fatalf(
			"board SELECT count grew with task rows: one=%d many=%d",
			oneRowQueries,
			manyRowQueries,
		)
	}
}

func TestListTaskPolicyQueriesDoNotGrowWithPageSize(t *testing.T) {
	fix := setupPolicyBatchQueryFixture(t)
	fix.createTasks(t, 10)
	token := fix.app.token(t, fix.actor)

	oneRow, oneRowQueries := requestWithSelectCount(
		t,
		fix.app,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20&limit=1",
		token,
		fix.workspace.ID,
	)
	requireStatus(t, oneRow, http.StatusOK)
	if got := len(decodeData(t, oneRow).([]any)); got != 1 {
		t.Fatalf("limited task rows=%d, want 1", got)
	}

	manyRows, manyRowQueries := requestWithSelectCount(
		t,
		fix.app,
		"/api/v1/platform/scrum-tasks-new?project_new_id=20&limit=50",
		token,
		fix.workspace.ID,
	)
	requireStatus(t, manyRows, http.StatusOK)
	if got := len(decodeData(t, manyRows).([]any)); got != 10 {
		t.Fatalf("full task rows=%d, want 10", got)
	}
	if manyRowQueries != oneRowQueries {
		t.Fatalf(
			"task list SELECT count grew with page size: one=%d many=%d",
			oneRowQueries,
			manyRowQueries,
		)
	}
}

func setupPolicyBatchQueryFixture(t *testing.T) policyBatchQueryFixture {
	t.Helper()
	app := newTestApp(t, nil)
	actor := app.createUser(t, 1, "developer", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "批量权限", Code: "BATCH", Status: "active",
	}
	unit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1", Type: "version", Status: "planning",
	}
	requirement := requirementgorm.Requirement{
		ID: 40, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: "批量权限需求",
	}
	for _, row := range []any{
		&workspace,
		&team,
		&project,
		&unit,
		&requirement,
		&identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID,
			UserID:      actor.ID,
			Role:        "member",
			Enabled:     true,
		},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	permission := projectgorm.ProjectPermission{
		WorkspaceID:  workspace.ID,
		TeamID:       team.ID,
		ProjectSetID: project.ID,
		UserID:       actor.ID,
		AccessLevel:  "member",
	}
	if err := app.db.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&projectgorm.ProjectMemberRole{
		WorkspaceID:         workspace.ID,
		ProjectPermissionID: permission.ID,
		RoleKey:             "developer",
		ScopeType:           "requirement",
		ScopeID:             requirement.ID,
		SourceType:          "manual",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return policyBatchQueryFixture{
		app:         app,
		actor:       actor,
		workspace:   workspace,
		project:     project,
		unit:        unit,
		requirement: requirement,
	}
}

func (f policyBatchQueryFixture) createTasks(t *testing.T, count int) {
	t.Helper()
	var existing int64
	f.app.db.Model(&taskgorm.WorkTask{}).Count(&existing)
	for index := 0; index < count; index++ {
		taskNumber := int(existing) + index + 1
		if err := f.app.db.Create(&taskgorm.WorkTask{
			WorkspaceID:   f.workspace.ID,
			TeamID:        f.project.TeamID,
			ProjectSetID:  f.project.ID,
			ProjectID:     f.unit.ID,
			RequirementID: f.requirement.ID,
			Kind:          taskgorm.WorkTaskKindTask,
			TaskKey:       fmt.Sprintf("BATCH-%d", taskNumber),
			Title:         fmt.Sprintf("任务 %d", taskNumber),
			Status:        "draft",
			Priority:      "normal",
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func requestWithSelectCount(
	t *testing.T,
	app *testApp,
	path string,
	token string,
	workspaceID uint,
) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	var selectCount atomic.Int64
	callbackName := fmt.Sprintf(
		"count_policy_batch_selects_%d",
		policyBatchQueryCallbackID.Add(1),
	)
	if err := app.db.Callback().Query().After("gorm:query").Register(
		callbackName,
		func(tx *gorm.DB) {
			sql := strings.ToUpper(strings.TrimSpace(tx.Statement.SQL.String()))
			if strings.HasPrefix(sql, "SELECT") {
				selectCount.Add(1)
			}
		},
	); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = app.db.Callback().Query().Remove(callbackName)
	}()
	rec := app.request(t, http.MethodGet, path, token, workspaceID, nil)
	return rec, selectCount.Load()
}
