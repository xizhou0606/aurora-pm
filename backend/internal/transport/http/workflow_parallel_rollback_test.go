package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
	"time"
)

func TestParallelBranchRollbackPreservesActiveSiblingAndCompletesJoin(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "prepare", "type": "form", "name": "准备"},
			{"id": "split", "type": "parallel_split", "name": "并行开始"},
			{"id": "left_prepare", "type": "form", "name": "左侧准备"},
			{"id": "left_work", "type": "human_task", "name": "左侧执行"},
			{"id": "right_prepare", "type": "form", "name": "右侧准备"},
			{"id": "right_ai", "type": "ai_check", "name": "右侧风险检查", "capability": "delivery.risk_monitor"},
			{"id": "right_recovery", "type": "human_task", "name": "右侧风险处理"},
			{"id": "join", "type": "parallel_join", "name": "并行汇聚"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "prepare"},
			{"id": "e2", "source": "prepare", "target": "split"},
			{"id": "e3", "source": "split", "target": "left_prepare"},
			{"id": "e4", "source": "left_prepare", "target": "left_work"},
			{"id": "e5", "source": "left_work", "target": "join"},
			{"id": "e6", "source": "split", "target": "right_prepare"},
			{"id": "e7", "source": "right_prepare", "target": "right_ai"},
			{"id": "e8", "source": "right_ai", "target": "join", "kind": "success"},
			{"id": "e9", "source": "right_ai", "target": "right_recovery", "kind": "failure"},
			{"id": "e10", "source": "right_recovery", "target": "join"},
			{"id": "e11", "source": "join", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "并行分支回退", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "并行回退计划", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	instanceID := uint(decodeData(t, createdUnit).(map[string]any)["workflow_instance_id"].(float64))

	for _, nodeID := range []string{"prepare", "left_prepare", "right_prepare"} {
		requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
			"action": "complete", "node_id": nodeID,
		}), http.StatusOK)
	}
	var before workflowgorm.WorkflowInstance
	if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&before).Error; err != nil {
		t.Fatal(err)
	}
	if current := currentWorkflowNodes(before.CurrentNodeIDs); len(current) != 2 || !containsString(current, "left_work") || !containsString(current, "right_ai") {
		t.Fatalf("parallel setup current=%v", current)
	}
	var leftToken, rightToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "left_work", "active").First(&leftToken).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "right_ai", "active").First(&rightToken).Error; err != nil {
		t.Fatal(err)
	}
	var aiRun aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "plan_unit", before.SubjectID).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	var aiNodeRun workflowgorm.WorkflowNodeRun
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "right_ai", "waiting").First(&aiNodeRun).Error; err != nil {
		t.Fatal(err)
	}
	var aiJob workflowgorm.WorkflowJob
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_run_id = ?", workspace.ID, instanceID, aiNodeRun.ID).First(&aiJob).Error; err != nil {
		t.Fatal(err)
	}
	var beforeRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ?", workspace.ID, instanceID).Count(&beforeRuns).Error; err != nil {
		t.Fatal(err)
	}

	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	if targets := decodeData(t, detail).(map[string]any)["rollback_targets"].([]any); len(targets) != 0 {
		t.Fatalf("multi-active DTO unexpectedly advertised a source-ambiguous rollback: %v", targets)
	}

	assertNoMutation := func(label string) {
		t.Helper()
		var instance workflowgorm.WorkflowInstance
		if err := app.db.Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).First(&instance).Error; err != nil {
			t.Fatal(err)
		}
		if instance.Status != before.Status || instance.CurrentNodeIDs != before.CurrentNodeIDs || instance.CompletedAt != nil {
			t.Fatalf("%s changed instance: before=%+v after=%+v", label, before, instance)
		}
		for _, expected := range []workflowgorm.WorkflowToken{leftToken, rightToken} {
			var actual workflowgorm.WorkflowToken
			if err := app.db.First(&actual, expected.ID).Error; err != nil {
				t.Fatal(err)
			}
			if actual.Status != "active" || actual.NodeID != expected.NodeID {
				t.Fatalf("%s changed token: before=%+v after=%+v", label, expected, actual)
			}
		}
		var runCount int64
		if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ?", workspace.ID, instanceID).Count(&runCount).Error; err != nil || runCount != beforeRuns {
			t.Fatalf("%s changed run history: count=%d want=%d err=%v", label, runCount, beforeRuns, err)
		}
		var actualAIRun aigorm.AIRun
		var actualNodeRun workflowgorm.WorkflowNodeRun
		var actualJob workflowgorm.WorkflowJob
		if err := app.db.First(&actualAIRun, aiRun.ID).Error; err != nil {
			t.Fatal(err)
		}
		if err := app.db.First(&actualNodeRun, aiNodeRun.ID).Error; err != nil {
			t.Fatal(err)
		}
		if err := app.db.First(&actualJob, aiJob.ID).Error; err != nil {
			t.Fatal(err)
		}
		if actualAIRun.Status != "queued" || actualNodeRun.Status != "waiting" || actualJob.Status != "pending" {
			t.Fatalf("%s changed sibling AI: ai=%q node_run=%q job=%q", label, actualAIRun.Status, actualNodeRun.Status, actualJob.Status)
		}
	}

	for label, targetNodeID := range map[string]string{
		"cross-sibling rollback":   "right_prepare",
		"shared-ancestor rollback": "prepare",
	} {
		response := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
			"action": "rollback", "node_id": "left_work", "target_node_id": targetNodeID, "comment": label,
		})
		requireStatus(t, response, http.StatusConflict)
		assertNoMutation(label)
	}

	rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "left_work", "target_node_id": "left_prepare", "comment": "左侧返工",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	rollbackData := decodeData(t, rolledBack).(map[string]any)
	rollbackCurrent := rollbackData["current_node_ids"].([]any)
	if len(rollbackCurrent) != 2 || !containsAny(rollbackCurrent, "left_prepare") || !containsAny(rollbackCurrent, "right_ai") {
		t.Fatalf("branch-local rollback lost current sibling: %v", rollbackCurrent)
	}
	var rolledLeft, preservedRight, freshLeft workflowgorm.WorkflowToken
	if err := app.db.First(&rolledLeft, leftToken.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&preservedRight, rightToken.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "left_prepare", "active").Order("id DESC").First(&freshLeft).Error; err != nil {
		t.Fatal(err)
	}
	if rolledLeft.Status != "rolled_back" || preservedRight.Status != "active" || freshLeft.ID == 0 {
		t.Fatalf("branch token result: rolled=%+v preserved=%+v fresh=%+v", rolledLeft, preservedRight, freshLeft)
	}
	var preservedAIRun aigorm.AIRun
	var preservedNodeRun workflowgorm.WorkflowNodeRun
	var preservedJob workflowgorm.WorkflowJob
	if err := app.db.First(&preservedAIRun, aiRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&preservedNodeRun, aiNodeRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&preservedJob, aiJob.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preservedAIRun.Status != "queued" || preservedNodeRun.Status != "waiting" || preservedJob.Status != "pending" {
		t.Fatalf("branch-local rollback cancelled sibling AI: ai=%q node_run=%q job=%q", preservedAIRun.Status, preservedNodeRun.Status, preservedJob.Status)
	}

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "left_prepare",
	}), http.StatusOK)
	if err := app.db.Model(&aiRun).Updates(map[string]any{
		"status": "completed", "output_json": `{"summary":"risk check passed"}`, "completed_at": time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	joinedSibling := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "right_ai",
	})
	requireStatus(t, joinedSibling, http.StatusOK)
	joinedData := decodeData(t, joinedSibling).(map[string]any)
	joinedCurrent := joinedData["current_node_ids"].([]any)
	if len(joinedCurrent) != 2 || !containsAny(joinedCurrent, "left_work") || !containsAny(joinedCurrent, "join") {
		t.Fatalf("completed sibling did not wait at join: %v", joinedCurrent)
	}
	if joinNode := workflowTimelineNode(t, joinedData, "join"); joinNode["status"] != "waiting" {
		t.Fatalf("incomplete parallel join status=%v", joinNode)
	}
	var waitingJoin workflowgorm.WorkflowToken
	if err := app.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status IN ?",
		workspace.ID, instanceID, "join", []string{"active", "waiting"},
	).First(&waitingJoin).Error; err != nil {
		t.Fatal(err)
	}
	secondRollback := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "left_work", "target_node_id": "left_prepare", "comment": "左侧再次返工",
	})
	requireStatus(t, secondRollback, http.StatusOK)
	secondData := decodeData(t, secondRollback).(map[string]any)
	secondCurrent := secondData["current_node_ids"].([]any)
	if len(secondCurrent) != 2 || !containsAny(secondCurrent, "left_prepare") || !containsAny(secondCurrent, "join") {
		t.Fatalf("rollback lost sibling waiting join: %v", secondCurrent)
	}
	if joinNode := workflowTimelineNode(t, secondData, "join"); joinNode["status"] != "waiting" {
		t.Fatalf("rollback changed incomplete join timeline: %v", joinNode)
	}
	var preservedJoin workflowgorm.WorkflowToken
	if err := app.db.First(&preservedJoin, waitingJoin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preservedJoin.Status != waitingJoin.Status {
		t.Fatalf("rollback changed sibling waiting join token: %+v", preservedJoin)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "left_prepare",
	}), http.StatusOK)
	finished := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "left_work",
	})
	requireStatus(t, finished, http.StatusOK)
	finishedData := decodeData(t, finished).(map[string]any)
	if finishedData["status"] != "completed" || len(finishedData["current_node_ids"].([]any)) != 0 {
		t.Fatalf("parallel workflow did not complete after branch rollback: %v", finishedData)
	}
	for _, nodeID := range []string{"join", "end"} {
		var count int64
		if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where(
			"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
			workspace.ID, instanceID, nodeID, "completed",
		).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("completed %s runs=%d err=%v", nodeID, count, err)
		}
	}
}

func TestRollbackTargetsUseLatestExecutedConditionBranch(t *testing.T) {
	decisionNo, decisionYes := false, true
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start"},
			{ID: "choice", Type: "condition"},
			{ID: "left", Type: "human_task"},
			{ID: "right", Type: "human_task"},
			{ID: "review", Type: "approval"},
			{ID: "end", Type: "end"},
		},
		Edges: []workflowEdge{
			{ID: "e1", Source: "start", Target: "choice"},
			{ID: "e2", Source: "choice", Target: "left", Kind: "yes"},
			{ID: "e3", Source: "choice", Target: "right", Kind: "no"},
			{ID: "e4", Source: "left", Target: "review"},
			{ID: "e5", Source: "right", Target: "review"},
			{ID: "e6", Source: "review", Target: "end", Kind: "approve"},
		},
	}
	runs := []workflowgorm.WorkflowNodeRun{
		{ID: 1, NodeID: "start", NodeType: "start", Status: "completed"},
		{ID: 2, NodeID: "choice", NodeType: "condition", Status: "completed", OutputJSON: encodeWorkflowRunDecision(1, "", decisionNo, "", "right")},
		{ID: 3, NodeID: "right", NodeType: "human_task", Status: "completed", OutputJSON: encodeWorkflowRunTransition(1, "", "review")},
		{ID: 4, NodeID: "choice", NodeType: "condition", Status: "completed", OutputJSON: encodeWorkflowRunDecision(1, "", decisionYes, "", "left")},
		{ID: 5, NodeID: "left", NodeType: "human_task", Status: "completed", OutputJSON: encodeWorkflowRunTransition(1, "", "review")},
	}
	targets := workflowRollbackTargetIDs(definition, runs, "review", true)
	if !targets["left"] || targets["right"] {
		t.Fatalf("latest condition execution targets=%v", targets)
	}
}

func TestRollbackTargetsRespectNestedParallelBoundary(t *testing.T) {
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start"},
			{ID: "outer_split", Type: "parallel_split"},
			{ID: "outer_left", Type: "human_task"},
			{ID: "inner_split", Type: "parallel_split"},
			{ID: "inner_left_prepare", Type: "form"},
			{ID: "inner_left_work", Type: "human_task"},
			{ID: "inner_right", Type: "human_task"},
			{ID: "inner_join", Type: "parallel_join"},
			{ID: "outer_left_end", Type: "human_task"},
			{ID: "outer_right", Type: "human_task"},
			{ID: "outer_join", Type: "parallel_join"},
			{ID: "end", Type: "end"},
		},
		Edges: []workflowEdge{
			{ID: "e1", Source: "start", Target: "outer_split"},
			{ID: "e2", Source: "outer_split", Target: "outer_left"},
			{ID: "e3", Source: "outer_left", Target: "inner_split"},
			{ID: "e4", Source: "inner_split", Target: "inner_left_prepare"},
			{ID: "e5", Source: "inner_left_prepare", Target: "inner_left_work"},
			{ID: "e6", Source: "inner_left_work", Target: "inner_join"},
			{ID: "e7", Source: "inner_split", Target: "inner_right"},
			{ID: "e8", Source: "inner_right", Target: "inner_join"},
			{ID: "e9", Source: "inner_join", Target: "outer_left_end"},
			{ID: "e10", Source: "outer_left_end", Target: "outer_join"},
			{ID: "e11", Source: "outer_split", Target: "outer_right"},
			{ID: "e12", Source: "outer_right", Target: "outer_join"},
			{ID: "e13", Source: "outer_join", Target: "end"},
		},
	}
	runs := []workflowgorm.WorkflowNodeRun{
		{ID: 1, NodeID: "start", NodeType: "start", Status: "completed"},
		{ID: 2, NodeID: "outer_split", NodeType: "parallel_split", Status: "completed"},
		{ID: 3, NodeID: "outer_left", NodeType: "human_task", Status: "completed", OutputJSON: encodeWorkflowRunTransition(1, "", "inner_split")},
		{ID: 4, NodeID: "inner_split", NodeType: "parallel_split", Status: "completed"},
		{ID: 5, NodeID: "inner_left_prepare", NodeType: "form", Status: "completed", OutputJSON: encodeWorkflowRunTransition(1, "", "inner_left_work")},
	}
	targets := workflowRollbackTargetIDs(definition, runs, "inner_left_work", true)
	if !targets["inner_left_prepare"] || targets["outer_left"] {
		t.Fatalf("nested parallel rollback targets=%v", targets)
	}
}

func TestCompletedSerialWorkflowCanRollbackAndCompleteAgain(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
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
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "已完成流程回退", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "已完成回退"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	complete := func() {
		t.Helper()
		requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "complete", "node_id": "input"}), http.StatusOK)
		requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "approve", "node_id": "review"}), http.StatusOK)
	}
	complete()
	rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "ignored-for-completed", "target_node_id": "input", "comment": "重新核对",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	data := decodeData(t, rolledBack).(map[string]any)
	if data["status"] != "running" || data["completed_at"] != nil {
		t.Fatalf("completed workflow was not reopened: %v", data)
	}
	assertWorkflowCurrentNode(t, data, "input")
	complete()
	var endRuns int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		workspace.ID, instanceID, "end", "completed",
	).Count(&endRuns).Error; err != nil || endRuns != 2 {
		t.Fatalf("completed workflow end runs=%d err=%v", endRuns, err)
	}
}
