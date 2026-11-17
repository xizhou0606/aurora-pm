package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestWorkflowNodePlanDisjointAssigneesMergeLatestPlans(t *testing.T) {
	app, leftToken, workspace, project, unit := newAIWorkflowFixture(t)
	rightOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "right-owner")
	rightToken := app.token(t, rightOwner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "split", "type": "parallel_split", "name": "并行开始"},
			{"id": "left", "type": "human_task", "name": "左分支"},
			{"id": "right", "type": "human_task", "name": "右分支"},
			{"id": "join", "type": "parallel_join", "name": "并行汇聚"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "split"},
			{"id": "e2", "source": "split", "target": "left"},
			{"id": "e3", "source": "split", "target": "right"},
			{"id": "e4", "source": "left", "target": "join"},
			{"id": "e5", "source": "right", "target": "join"},
			{"id": "e6", "source": "join", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", leftToken, workspace.ID, map[string]any{
		"name": "并行负责人流程", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", leftToken, workspace.ID, nil), http.StatusOK)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", leftToken, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "并行计划", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	instanceID := uint(decodeData(t, createdUnit).(map[string]any)["workflow_instance_id"].(float64))

	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	leftResponse := app.request(t, http.MethodPatch, path, leftToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "left", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
		}},
	})
	requireStatus(t, leftResponse, http.StatusOK)
	rightResponse := app.request(t, http.MethodPatch, path, rightToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "right", "planned_end_at": "2026-08-21", "expected_planned_end_at": nil,
		}},
	})
	requireStatus(t, rightResponse, http.StatusOK)

	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	plans := workflowNodePlans(instance)
	if plans["left"].PlannedEndAt == nil || plans["right"].PlannedEndAt == nil {
		t.Fatalf("disjoint updates did not merge both assignees' plans: %+v", plans)
	}
}

func TestPendingAssigneeBatchWinsRaceAgainstAdvanceWithoutStaleToken(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "pending-next-owner")
	definition := workflowNodePlanRaceDefinition()
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "pending assignee race", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "pending race"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	setWorkflowConcurrencyConnections(t, app)

	ready, release, remove := blockFirstWorkflowTokenUpdate(t, app, "test:block-pending-assignee-advance")
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	actionBody, _ := json.Marshal(map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": 1}},
	})
	actionDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		actionDone <- sendWorkflowConcurrencyRequest(app, http.MethodPost, actionPath, ownerToken, workspace.ID, actionBody)
	}()
	waitForNodePlanTestSignal(t, ready, "advance token claim")

	batch := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", ownerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "review", "assignee_user_id": nextOwner.ID}},
	})
	requireStatus(t, batch, http.StatusOK)
	release()
	action := waitForNodePlanTestResponse(t, actionDone, "stale advance response")
	requireStatus(t, action, http.StatusConflict)
	remove()

	var instance workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
		t.Fatal(err)
	}
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil || plans["review"].AssigneeUserID == nil || *plans["review"].AssigneeUserID != nextOwner.ID {
		t.Fatalf("winning pending override was not retained: plans=%+v err=%v", plans, err)
	}
	var completedRuns, reviewTokens int64
	app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "completed").Count(&completedRuns)
	app.db.Model(&workflowgorm.WorkflowToken{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "review").Count(&reviewTokens)
	if completedRuns != 0 || reviewTokens != 0 {
		t.Fatalf("stale advance partially committed: completed_runs=%d review_tokens=%d", completedRuns, reviewTokens)
	}
	var inputToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").First(&inputToken).Error; err != nil {
		t.Fatalf("active input token was not rolled back: %v", err)
	}

	retry := app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{"action": "complete", "node_id": "input"})
	requireStatus(t, retry, http.StatusOK)
	var reviewToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").First(&reviewToken).Error; err != nil {
		t.Fatal(err)
	}
	if reviewToken.AssigneeUserID == nil || *reviewToken.AssigneeUserID != nextOwner.ID {
		t.Fatalf("fresh advance did not inherit pending override: %+v", reviewToken)
	}
}

func TestCompletedAssigneeBatchWinsRaceAgainstRollbackWithoutStaleToken(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "rollback-next-owner")
	definition := workflowNodePlanRaceDefinition()
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "completed assignee rollback race", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "rollback race"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{"action": "complete", "node_id": "input"}), http.StatusOK)
	setWorkflowConcurrencyConnections(t, app)

	ready, release, remove := blockFirstWorkflowTokenUpdate(t, app, "test:block-completed-assignee-rollback")
	rollbackBody, _ := json.Marshal(map[string]any{
		"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "race rollback",
	})
	rollbackDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rollbackDone <- sendWorkflowConcurrencyRequest(app, http.MethodPost, actionPath, ownerToken, workspace.ID, rollbackBody)
	}()
	waitForNodePlanTestSignal(t, ready, "rollback token claim")

	batch := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", ownerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "input", "assignee_user_id": nextOwner.ID}},
	})
	requireStatus(t, batch, http.StatusOK)
	release()
	rollback := waitForNodePlanTestResponse(t, rollbackDone, "stale rollback response")
	requireStatus(t, rollback, http.StatusConflict)
	remove()

	var reviewToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").First(&reviewToken).Error; err != nil {
		t.Fatalf("rollback token CAS was not rolled back: %v", err)
	}
	var historical workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "completed").Order("id DESC").First(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if historical.AssigneeUserID == nil || *historical.AssigneeUserID != 1 {
		t.Fatalf("completed token history was rewritten: %+v", historical)
	}
	var rollbackRuns int64
	app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND status = ?", workspace.ID, instanceID, "rolled_back").Count(&rollbackRuns)
	if rollbackRuns != 0 {
		t.Fatalf("stale rollback left history behind: %d", rollbackRuns)
	}

	retry := app.workflowAction(t, workspace.ID, ownerToken, instanceID, map[string]any{
		"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "fresh rollback",
	})
	requireStatus(t, retry, http.StatusOK)
	var fresh workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").Order("id DESC").First(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.AssigneeUserID == nil || *fresh.AssigneeUserID != nextOwner.ID {
		t.Fatalf("fresh rollback did not inherit completed override: %+v", fresh)
	}
}

func workflowNodePlanRaceDefinition() map[string]any {
	return map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "录入"},
			{"id": "review", "type": "approval", "name": "评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
}

func setWorkflowConcurrencyConnections(t *testing.T, app *testApp) {
	t.Helper()
	sqlDB, err := app.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(2)
}

func blockFirstWorkflowTokenUpdate(
	t *testing.T,
	app *testApp,
	callbackName string,
) (<-chan struct{}, func(), func()) {
	t.Helper()
	ready, release := make(chan struct{}), make(chan struct{})
	var blockOnce, releaseOnce, removeOnce sync.Once
	if err := app.db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*workflowgorm.WorkflowToken); !ok {
			return
		}
		blockOnce.Do(func() {
			close(ready)
			<-release
		})
	}); err != nil {
		t.Fatal(err)
	}
	releaseUpdate := func() { releaseOnce.Do(func() { close(release) }) }
	removeCallback := func() {
		removeOnce.Do(func() {
			if err := app.db.Callback().Update().Remove(callbackName); err != nil {
				t.Errorf("remove workflow token callback: %v", err)
			}
		})
	}
	t.Cleanup(func() {
		releaseUpdate()
		removeCallback()
	})
	return ready, releaseUpdate, removeCallback
}

func sendWorkflowConcurrencyRequest(
	app *testApp,
	method, path, token string,
	workspaceID uint,
	body []byte,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-ID", uintString(workspaceID))
	recorder := httptest.NewRecorder()
	app.engine.ServeHTTP(recorder, req)
	return recorder
}

func waitForNodePlanTestSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func waitForNodePlanTestResponse(
	t *testing.T,
	response <-chan *httptest.ResponseRecorder,
	label string,
) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case recorder := <-response:
		return recorder
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
		return nil
	}
}
