package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestCompletedTaskPlanCanOnlyBeRecheckedBeforeTasksExist(t *testing.T) {
	server := &Server{}
	for _, test := range []struct {
		name      string
		execution gin.H
		want      string
	}{
		{name: "draft", execution: gin.H{"status": "completed", "output": map[string]any{"taskPlan": map[string]any{}}}, want: "complete,recheck_ai"},
		{name: "legacy materialized", execution: gin.H{"status": "completed", "output": map[string]any{"createdTasks": []any{map[string]any{"id": 1}}}}, want: "complete"},
		{name: "applied", execution: gin.H{"status": "applied", "output": map[string]any{"taskPlan": map[string]any{}}}, want: "complete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := server.workflowNodeAllowedActions(
				workflowgorm.WorkflowInstance{Status: "running"}, workflowDefinition{},
				workflowNode{ID: "task-plan", Type: "ai_plan", Capability: "requirement.task_plan"},
				"active", test.execution, true, false, false,
			)
			if got := strings.Join(actions, ","); got != test.want {
				t.Fatalf("task plan actions=%q, want %q", got, test.want)
			}
		})
	}
}

func TestFailedOrCancelledWorkflowAIOnlyAllowsRetry(t *testing.T) {
	for _, aiStatus := range []string{"failed", "cancelled"} {
		t.Run(aiStatus, func(t *testing.T) {
			fixture := newWorkflowAIActionFixture(t, aiStatus, "")
			detail := fixture.app.request(t, http.MethodGet, fixture.actionPath, fixture.token, fixture.workspaceID, nil)
			requireStatus(t, detail, http.StatusOK)
			data := decodeData(t, detail).(map[string]any)
			current := data["current_nodes"].([]any)[0].(map[string]any)
			if got := strings.Join(actionStrings(current["allowed_actions"]), ","); got != "retry_ai" {
				t.Fatalf("%s AI node actions=%q current=%v", aiStatus, got, current)
			}
			if got := strings.Join(actionStrings(data["allowed_actions"]), ","); got != "retry_ai" {
				t.Fatalf("%s AI instance actions=%q data=%v", aiStatus, got, data)
			}

			var waiting workflowgorm.WorkflowNodeRun
			if err := fixture.app.db.Where(
				"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
				fixture.workspaceID, fixture.instance.ID, "ai", "waiting",
			).First(&waiting).Error; err != nil {
				t.Fatal(err)
			}
			var job workflowgorm.WorkflowJob
			if err := fixture.app.db.Where(
				"workspace_id = ? AND instance_id = ? AND node_run_id = ?",
				fixture.workspaceID, fixture.instance.ID, waiting.ID,
			).First(&job).Error; err != nil {
				t.Fatal(err)
			}

			for _, action := range []string{"complete", "advance", "failure", "recheck_ai"} {
				response := fixture.app.workflowAction(t, fixture.workspaceID, fixture.token, fixture.instance.ID, map[string]any{
					"action": action, "node_id": "ai",
				})
				requireStatus(t, response, http.StatusConflict)
			}
			assertWorkflowAIActionState(t, fixture, waiting.ID, job.ID, "waiting", "pending", 1)

			retried := fixture.app.workflowAction(t, fixture.workspaceID, fixture.token, fixture.instance.ID, map[string]any{
				"action": "retry_ai", "node_id": "ai",
			})
			requireStatus(t, retried, http.StatusOK)
			assertWorkflowAIActionState(t, fixture, waiting.ID, job.ID, aiStatus, aiStatus, 2)

			staleClaim := claimWorkflowAIRecheckTx(fixture.app.db, fixture.instance, waiting, job, time.Now().UTC())
			if !errors.Is(staleClaim, errWorkflowAIRecheckChanged) {
				t.Fatalf("stale %s retry claim error=%v", aiStatus, staleClaim)
			}
			assertWorkflowAIActionState(t, fixture, waiting.ID, job.ID, aiStatus, aiStatus, 2)

			repeated := fixture.app.workflowAction(t, fixture.workspaceID, fixture.token, fixture.instance.ID, map[string]any{
				"action": "retry_ai", "node_id": "ai",
			})
			requireStatus(t, repeated, http.StatusConflict)
			assertWorkflowAIActionState(t, fixture, waiting.ID, job.ID, aiStatus, aiStatus, 2)
		})
	}
}

func TestCompletedWorkflowAIKeepsAdvanceAliasAndBusinessFailurePath(t *testing.T) {
	fixture := newWorkflowAIActionFixture(t, "completed", `{
		"summary":"information is incomplete",
		"completenessReport":{"canProceed":false}
	}`)
	detail := fixture.app.request(t, http.MethodGet, fixture.actionPath, fixture.token, fixture.workspaceID, nil)
	requireStatus(t, detail, http.StatusOK)
	current := decodeData(t, detail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if !containsString(actionStrings(current["allowed_actions"]), "complete") ||
		containsString(actionStrings(current["allowed_actions"]), "advance") {
		t.Fatalf("completed AI canonical actions=%v", current["allowed_actions"])
	}

	hiddenRetry := fixture.app.workflowAction(t, fixture.workspaceID, fixture.token, fixture.instance.ID, map[string]any{
		"action": "retry_ai", "node_id": "ai",
	})
	requireStatus(t, hiddenRetry, http.StatusConflict)

	advanced := fixture.app.workflowAction(t, fixture.workspaceID, fixture.token, fixture.instance.ID, map[string]any{
		"action": "advance", "node_id": "ai",
		"next_node_assignees": []map[string]any{{"node_id": "repair", "assignee_user_id": 1}},
	})
	requireStatus(t, advanced, http.StatusOK)
	data := decodeData(t, advanced).(map[string]any)
	if !containsAny(data["current_node_ids"].([]any), "repair") || containsAny(data["current_node_ids"].([]any), "ai") {
		t.Fatalf("advance alias did not consume completed low-score AI result: %v", data["current_node_ids"])
	}
	var aiRun workflowgorm.WorkflowNodeRun
	if err := fixture.app.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?",
		fixture.workspaceID, fixture.instance.ID, "ai",
	).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	if aiRun.Status != "completed" {
		t.Fatalf("completed low-score AI node run status=%q", aiRun.Status)
	}
}

type workflowAIActionFixture struct {
	app         *testApp
	token       string
	workspaceID uint
	actionPath  string
	instance    workflowgorm.WorkflowInstance
}

func newWorkflowAIActionFixture(t *testing.T, aiStatus, outputJSON string) workflowAIActionFixture {
	t.Helper()
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "ai", "type": "ai_check", "name": "AI 完整性检查", "capability": "requirement.completeness"},
			{"id": "repair", "type": "human_task", "name": "人工补充"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "ai"},
			{"id": "e2", "source": "ai", "target": "end", "kind": "success"},
			{"id": "e3", "source": "ai", "target": "repair", "kind": "failure"},
			{"id": "e4", "source": "repair", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "AI 动作合同", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "AI 动作合同"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requirementID := uint(requirement["id"].(float64))
	var aiRun aigorm.AIRun
	if err := app.db.Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ? AND capability = ?",
		workspace.ID, "requirement", requirementID, "requirement.completeness",
	).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	updates := map[string]any{
		"status": aiStatus, "completed_at": time.Now().UTC(),
		"error_message": "provider failed",
	}
	if outputJSON != "" {
		updates["output_json"] = outputJSON
		updates["error_message"] = ""
	}
	if err := app.db.Model(&aiRun).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	return workflowAIActionFixture{
		app: app, token: token, workspaceID: workspace.ID,
		actionPath: "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID),
		instance:   instance,
	}
}

func assertWorkflowAIActionState(
	t *testing.T,
	fixture workflowAIActionFixture,
	nodeRunID, jobID uint,
	wantNodeRunStatus, wantJobStatus string,
	wantAIRuns int64,
) {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := fixture.app.db.Where("id = ? AND workspace_id = ?", fixture.instance.ID, fixture.workspaceID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || !containsString(currentWorkflowNodes(instance.CurrentNodeIDs), "ai") {
		t.Fatalf("AI action changed instance: %+v", instance)
	}
	var token workflowgorm.WorkflowToken
	if err := fixture.app.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?",
		fixture.workspaceID, fixture.instance.ID, "ai",
	).Order("id DESC").First(&token).Error; err != nil {
		t.Fatal(err)
	}
	if token.Status != "active" {
		t.Fatalf("AI action token status=%q", token.Status)
	}
	var nodeRun workflowgorm.WorkflowNodeRun
	if err := fixture.app.db.Where("id = ? AND workspace_id = ?", nodeRunID, fixture.workspaceID).First(&nodeRun).Error; err != nil {
		t.Fatal(err)
	}
	if nodeRun.Status != wantNodeRunStatus {
		t.Fatalf("AI node run status=%q want=%q", nodeRun.Status, wantNodeRunStatus)
	}
	var job workflowgorm.WorkflowJob
	if err := fixture.app.db.Where("id = ? AND workspace_id = ?", jobID, fixture.workspaceID).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != wantJobStatus {
		t.Fatalf("AI job status=%q want=%q", job.Status, wantJobStatus)
	}
	var aiRuns int64
	if err := fixture.app.db.Model(&aigorm.AIRun{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", fixture.workspaceID, fixture.instance.SubjectType, fixture.instance.SubjectID).
		Count(&aiRuns).Error; err != nil || aiRuns != wantAIRuns {
		t.Fatalf("AI run count=%d want=%d err=%v", aiRuns, wantAIRuns, err)
	}
}
