package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func newTaskTestingTestApp(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	return newTestApp(t, mutate)
}

func seedTaskTestingScopes(t *testing.T, app *testApp) (identitygorm.User, string) {
	t.Helper()
	admin := app.createUser(t, 1, "testing-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []taskgorm.WorkTask{
		{ID: 100, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-100", Title: "Task Parent",
			Status: "in_progress", Priority: "high"},
		{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-101", Title: "Task A",
			Status: "in_progress", Priority: "high", ParentID: uintPtr(100)},
		{ID: 201, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, Kind: taskgorm.WorkTaskKindTask, TaskKey: "B-201", Title: "Task B",
			Status: "draft", Priority: "normal"},
	} {
		if err := app.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return admin, app.token(t, admin)
}

func testDefinitionBody(version uint) map[string]any {
	return map[string]any{
		"version": version, "purpose": "验证任务", "preconditions": "已部署",
		"environment_guidance": "staging",
		"cases": []map[string]any{
			{"key": "TC-1", "title": "主路径", "expected_result": "成功", "steps": []map[string]any{
				{"action": "打开页面", "expected_result": "页面可见"},
			}},
			{"key": "TC-2", "title": "异常路径", "expected_result": "提示错误", "steps": []map[string]any{
				{"action": "提交非法值", "expected_result": "展示校验"},
			}},
		},
	}
}

func TestTaskTestingDefinitionContractAndOptimisticConcurrency(t *testing.T) {
	app := newTaskTestingTestApp(t, nil)
	_, token := seedTaskTestingScopes(t, app)

	createdDefinition := app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(0))
	requireStatus(t, createdDefinition, http.StatusCreated)
	definition := decodeData(t, createdDefinition).(map[string]any)
	definitionTask := definition["task"].(map[string]any)
	if definition["version"] != float64(1) || len(definition["cases"].([]any)) != 2 ||
		definitionTask["scope"].(map[string]any)["requirement"].(map[string]any)["id"] != float64(13) ||
		definitionTask["hierarchy"].(map[string]any)["depth"] != float64(1) {
		t.Fatalf("created definition=%s", createdDefinition.Body.String())
	}

	updatedBody := testDefinitionBody(1)
	updatedBody["purpose"] = "验证任务 v2"
	updated := app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, updatedBody)
	requireStatus(t, updated, http.StatusOK)
	if decodeData(t, updated).(map[string]any)["version"] != float64(2) {
		t.Fatalf("updated definition=%s", updated.Body.String())
	}
	stale := app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(1))
	requireStatus(t, stale, http.StatusConflict)

	contextResponse := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/101/test-context", token, 1, nil)
	requireStatus(t, contextResponse, http.StatusOK)
	context := decodeData(t, contextResponse).(map[string]any)
	taskContext := context["task"].(map[string]any)
	if context["definition"].(map[string]any)["version"] != float64(2) ||
		taskContext["id"] != float64(101) ||
		taskContext["scope"].(map[string]any)["requirement"].(map[string]any)["id"] != float64(13) ||
		taskContext["hierarchy"].(map[string]any)["depth"] != float64(1) {
		t.Fatalf("test context=%s", contextResponse.Body.String())
	}

	var auditCount, outboxCount int64
	app.db.Model(&auditgorm.AuditLog{}).Where("resource = ?", "task_test_definition").Count(&auditCount)
	app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_type = ?", "task_test_definition").Count(&outboxCount)
	if auditCount != 2 || outboxCount != 2 {
		t.Fatalf("mutation records audit=%d outbox=%d", auditCount, outboxCount)
	}
}

func TestTaskTestingExecutionRunContractAndBugLink(t *testing.T) {
	app := newTaskTestingTestApp(t, nil)
	_, token := seedTaskTestingScopes(t, app)
	definition := app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(0))
	requireStatus(t, definition, http.StatusCreated)
	bug := qualitygorm.QualityBug{
		ID: 301, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, BugKey: "A-BUG-301", Title: "测试失败", Status: "triage",
		Priority: "high", SeverityKey: "p1",
	}
	if err := app.db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}

	createdExecution := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/test-executions", token, 1, map[string]any{
			"definition_version": 1, "environment": "staging", "build_version": "2026.08.11",
			"executor_user_id": 1, "summary": "回归验证",
		})
	requireStatus(t, createdExecution, http.StatusCreated)
	createdData := decodeData(t, createdExecution).(map[string]any)
	execution := createdData["execution"].(map[string]any)
	runs := createdData["runs"].([]any)
	if execution["status"] != "running" || len(runs) != 2 {
		t.Fatalf("created execution=%s", createdExecution.Body.String())
	}
	createdTask := createdData["task"].(map[string]any)
	if createdTask["scope"].(map[string]any)["requirement"].(map[string]any)["id"] != float64(13) ||
		createdTask["hierarchy"].(map[string]any)["depth"] != float64(1) {
		t.Fatalf("created execution task context=%s", createdExecution.Body.String())
	}
	executionID := uint(execution["id"].(float64))
	firstRun := runs[0].(map[string]any)
	runID := uint(firstRun["id"].(float64))

	patched := app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/tasks/101/test-execution-runs/%d", runID), token, 1,
		map[string]any{
			"version": 1, "status": "failed", "actual_result": "500",
			"failure_reason": "服务异常", "quality_bug_id": 301,
			"evidence": []map[string]any{{"kind": "url", "reference": "https://evidence.example/run/1"}},
			"step_results": []map[string]any{{
				"sequence": 1, "status": "failed", "actual_result": "页面 500",
				"failure_reason": "服务异常",
			}},
		})
	requireStatus(t, patched, http.StatusOK)
	patchedData := decodeData(t, patched).(map[string]any)
	if patchedData["run"].(map[string]any)["quality_bug_id"] != float64(301) ||
		patchedData["execution"].(map[string]any)["status"] != "failed" ||
		len(patchedData["run"].(map[string]any)["step_results"].([]any)) != 1 {
		t.Fatalf("patched run=%s", patched.Body.String())
	}

	listed := app.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/platform/tasks/101/test-execution-runs?execution_id=%d", executionID),
		token, 1, nil)
	requireStatus(t, listed, http.StatusOK)
	listedData := decodeData(t, listed).(map[string]any)
	if listedData["total"] != float64(2) ||
		listedData["execution"].(map[string]any)["status"] != "failed" {
		t.Fatalf("listed runs=%s", listed.Body.String())
	}
	for _, aggregate := range []string{"task_test_execution", "task_test_execution_run"} {
		var auditCount, outboxCount int64
		app.db.Model(&auditgorm.AuditLog{}).Where("resource = ?", aggregate).Count(&auditCount)
		app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_type = ?", aggregate).Count(&outboxCount)
		if auditCount != 1 || outboxCount != 1 {
			t.Fatalf("%s mutation records audit=%d outbox=%d", aggregate, auditCount, outboxCount)
		}
	}
}

func TestTaskTestingRowsCascadeWhenTaskIsDeleted(t *testing.T) {
	app := newTaskTestingTestApp(t, nil)
	_, token := seedTaskTestingScopes(t, app)
	requireStatus(t, app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(0)), http.StatusCreated)
	requireStatus(t, app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/test-executions", token, 1, map[string]any{
			"definition_version": 1, "environment": "staging",
		}), http.StatusCreated)
	requireStatus(t, app.request(t, http.MethodDelete,
		"/api/v1/platform/tasks/101?cascade=true", token, 1, nil), http.StatusOK)
	for name, target := range map[string]any{
		"definition": &taskgorm.TaskTestDefinition{},
		"execution":  &taskgorm.TaskTestExecution{},
		"run":        &taskgorm.TaskTestExecutionRun{},
	} {
		var count int64
		if err := app.db.Model(target).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("task delete left %s rows count=%d err=%v", name, count, err)
		}
	}
}

func TestTaskTestingRejectsCrossWorkspaceAndCrossHierarchyLinks(t *testing.T) {
	app := newTaskTestingTestApp(t, nil)
	_, token := seedTaskTestingScopes(t, app)
	requireStatus(t, app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(0)), http.StatusCreated)
	createdExecution := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/test-executions", token, 1, map[string]any{
			"definition_version": 1, "environment": "staging",
		})
	requireStatus(t, createdExecution, http.StatusCreated)
	run := decodeData(t, createdExecution).(map[string]any)["runs"].([]any)[0].(map[string]any)
	runID := uint(run["id"].(float64))

	crossWorkspace := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/101/test-context", token, 2, nil)
	requireStatus(t, crossWorkspace, http.StatusNotFound)
	otherBug := qualitygorm.QualityBug{
		ID: 401, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
		RequirementID: 23, BugKey: "B-BUG-401", Title: "Other", Status: "triage",
		Priority: "normal", SeverityKey: "p2",
	}
	if err := app.db.Create(&otherBug).Error; err != nil {
		t.Fatal(err)
	}
	invalidLink := app.request(t, http.MethodPatch,
		fmt.Sprintf("/api/v1/platform/tasks/101/test-execution-runs/%d", runID), token, 1,
		map[string]any{"version": 1, "quality_bug_id": 401})
	requireStatus(t, invalidLink, http.StatusBadRequest)
	var stored taskgorm.TaskTestExecutionRun
	if err := app.db.First(&stored, runID).Error; err != nil || stored.QualityBugID != nil || stored.Version != 1 {
		t.Fatalf("cross-workspace link changed run: %+v err=%v", stored, err)
	}
}

func TestTaskTestingRejectsInactivePMExecutor(t *testing.T) {
	app := newTaskTestingTestApp(t, nil)
	_, token := seedTaskTestingScopes(t, app)
	requireStatus(t, app.request(t, http.MethodPut,
		"/api/v1/platform/tasks/101/test-definition", token, 1, testDefinitionBody(0)), http.StatusCreated)
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/test-executions", token, 1, map[string]any{
			"definition_version": 1, "environment": "staging", "executor_user_id": 999,
		})
	requireStatus(t, response, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), "PM user reference ID 999 is not active") {
		t.Fatalf("invalid executor response=%s", response.Body.String())
	}
}
