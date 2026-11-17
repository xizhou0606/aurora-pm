package httpapi

import (
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestRequirementWorkflowZeroToOneMainline(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "choice", "type": "condition", "name": "是否进入并行交付"},
			{"id": "split", "type": "parallel_split", "name": "并行开始"},
			{"id": "develop", "type": "human_task", "name": "研发实现"},
			{"id": "qa", "type": "human_task", "name": "QA 测试"},
			{"id": "join", "type": "parallel_join", "name": "并行汇聚"},
			{"id": "acceptance", "type": "approval", "name": "产品验收", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review", "loop": true},
			{"id": "e3", "source": "review", "target": "choice", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
			{"id": "e5", "source": "choice", "target": "split", "kind": "yes"},
			{"id": "e6", "source": "choice", "target": "acceptance", "kind": "no"},
			{"id": "e7", "source": "split", "target": "develop"},
			{"id": "e8", "source": "split", "target": "qa"},
			{"id": "e9", "source": "develop", "target": "join"},
			{"id": "e10", "source": "qa", "target": "join"},
			{"id": "e11", "source": "join", "target": "acceptance"},
			{"id": "e12", "source": "acceptance", "target": "end", "kind": "approve"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "需求 0-1 主流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "0-1 完整回归"})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))

	atReview := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input", "comment": "首次录入",
	})
	requireStatus(t, atReview, http.StatusOK)
	if edges := decodeData(t, atReview).(map[string]any)["selected_edge_ids"].([]any); containsAny(edges, "e4") {
		t.Fatalf("untraversed reject edge remained visible: %v", edges)
	}
	rejected := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "补充边界条件",
	})
	requireStatus(t, rejected, http.StatusOK)
	rejectedData := decodeData(t, rejected).(map[string]any)
	assertWorkflowCurrentNode(t, rejectedData, "input")
	rejectedEdges := rejectedData["selected_edge_ids"].([]any)
	if !containsAny(rejectedEdges, "e4") || !containsAny(rejectedEdges, "e3") {
		t.Fatalf("retry loop did not preserve rollback and normal paths: %v", rejectedEdges)
	}

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input", "comment": "边界条件已补齐",
	}), http.StatusOK)
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "评审通过",
	}), http.StatusOK)
	parallel := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_yes", "node_id": "choice", "comment": "进入并行交付",
	})
	requireStatus(t, parallel, http.StatusOK)
	parallelData := decodeData(t, parallel).(map[string]any)
	current := parallelData["current_node_ids"].([]any)
	if len(current) != 2 || !containsAny(current, "develop") || !containsAny(current, "qa") {
		t.Fatalf("parallel current nodes=%v", current)
	}
	selectedNodes := parallelData["selected_node_ids"].([]any)
	selectedEdges := parallelData["selected_edge_ids"].([]any)
	for _, nodeID := range []string{"develop", "qa"} {
		if !containsAny(selectedNodes, nodeID) {
			t.Fatalf("selected parallel branch node %q missing from %v", nodeID, selectedNodes)
		}
	}
	for _, edgeID := range []string{"e5", "e7", "e8"} {
		if !containsAny(selectedEdges, edgeID) {
			t.Fatalf("selected parallel edge %q missing from %v", edgeID, selectedEdges)
		}
	}
	if containsAny(selectedEdges, "e6") {
		t.Fatalf("unselected condition edge remained in parallel topology: %v", selectedEdges)
	}

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "develop", "comment": "研发完成",
	}), http.StatusOK)
	joined := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "qa", "comment": "测试通过",
	})
	requireStatus(t, joined, http.StatusOK)
	assertWorkflowCurrentNode(t, decodeData(t, joined).(map[string]any), "acceptance")

	rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "acceptance", "target_node_id": "qa", "comment": "补充回归测试",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	rolledBackData := decodeData(t, rolledBack).(map[string]any)
	assertWorkflowCurrentNode(t, rolledBackData, "qa")
	if status := workflowTimelineNode(t, rolledBackData, "develop")["status"]; status != "completed" {
		t.Fatalf("completed parallel sibling status=%v", status)
	}
	if status := workflowTimelineNode(t, rolledBackData, "qa")["status"]; status != "active" {
		t.Fatalf("rollback target status=%v", status)
	}

	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "qa", "comment": "回归测试通过",
	}), http.StatusOK)
	completed := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "approve", "node_id": "acceptance", "comment": "验收通过",
	})
	requireStatus(t, completed, http.StatusOK)
	completedData := decodeData(t, completed).(map[string]any)
	if completedData["status"] != "completed" || completedData["progress"] != float64(100) || len(completedData["current_node_ids"].([]any)) != 0 {
		t.Fatalf("completed workflow=%v", completedData)
	}

	var endRuns, approvals int64
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "end", "completed").Count(&endRuns).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowApproval{}).Where("workspace_id = ? AND instance_id = ?", workspace.ID, instanceID).Count(&approvals).Error; err != nil {
		t.Fatal(err)
	}
	if endRuns != 1 || approvals != 3 {
		t.Fatalf("terminal/approval rows: end=%d approvals=%d", endRuns, approvals)
	}
	var requirement requirementgorm.Requirement
	if err := app.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).First(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.Status != "done" {
		t.Fatalf("completed requirement status=%q", requirement.Status)
	}
}

func TestWorkflowConditionSkipsUnselectedParallelBranchAndKeepsSharedDownstream(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "是否进入并行交付"},
			{"id": "split", "type": "parallel_split", "name": "并行开始"},
			{"id": "develop", "type": "human_task", "name": "研发实现"},
			{"id": "qa", "type": "approval", "name": "QA 测试", "config": map[string]any{"critical_gate": true}},
			{"id": "recovery", "type": "human_task", "name": "测试驳回处理"},
			{"id": "join", "type": "parallel_join", "name": "并行汇聚"},
			{"id": "delivery", "type": "human_task", "name": "共享交付"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "split", "kind": "yes"},
			{"id": "e3", "source": "choice", "target": "delivery", "kind": "yes"},
			{"id": "e4", "source": "choice", "target": "delivery", "kind": "no"},
			{"id": "e5", "source": "split", "target": "develop"},
			{"id": "e6", "source": "split", "target": "qa"},
			{"id": "e7", "source": "develop", "target": "join"},
			{"id": "e8", "source": "qa", "target": "join", "kind": "approve"},
			{"id": "e9", "source": "qa", "target": "recovery", "kind": "reject"},
			{"id": "e10", "source": "recovery", "target": "join"},
			{"id": "e11", "source": "join", "target": "delivery"},
			{"id": "e12", "source": "delivery", "target": "end"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "条件跳过并行交付", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "跳过并行交付"})
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	initial := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, initial, http.StatusOK)
	initialData := decodeData(t, initial).(map[string]any)
	initialNodes := initialData["selected_node_ids"].([]any)
	initialEdges := initialData["selected_edge_ids"].([]any)
	if len(initialNodes) != 8 {
		t.Fatalf("unresolved condition display node count=%d, want 8: %v", len(initialNodes), initialNodes)
	}
	for _, nodeID := range []string{"start", "choice", "split", "develop", "qa", "join", "delivery", "end"} {
		if !containsAny(initialNodes, nodeID) {
			t.Fatalf("unresolved condition omitted future node %q: %v", nodeID, initialNodes)
		}
	}
	if containsAny(initialNodes, "recovery") {
		t.Fatalf("untaken recovery node appeared before failure: %v", initialNodes)
	}
	if len(initialEdges) != 10 {
		t.Fatalf("unresolved condition display edge count=%d, want 10: %v", len(initialEdges), initialEdges)
	}
	for _, edgeID := range []string{"e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8", "e11", "e12"} {
		if !containsAny(initialEdges, edgeID) {
			t.Fatalf("unresolved condition omitted future edge %q: %v", edgeID, initialEdges)
		}
	}
	if containsAny(initialEdges, "e9") || containsAny(initialEdges, "e10") {
		t.Fatalf("untaken recovery path appeared before failure: %v", initialEdges)
	}
	if status := workflowTimelineNode(t, initialData, "recovery")["status"]; status != "pending" {
		t.Fatalf("display filtering changed recovery node runtime status=%v", status)
	}
	for _, nodeID := range []string{"develop", "qa", "delivery"} {
		if status := workflowTimelineNode(t, initialData, nodeID)["status"]; status != "pending" {
			t.Fatalf("unresolved future node %q status=%v, want pending", nodeID, status)
		}
	}
	if initialData["completed_steps"] != float64(0) || initialData["total_steps"] != float64(4) || initialData["progress"] != float64(0) {
		t.Fatalf("unresolved display topology progress=%v/%v (%v)", initialData["completed_steps"], initialData["total_steps"], initialData["progress"])
	}

	result := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_no", "node_id": "choice", "comment": "跳过并行交付",
	})
	requireStatus(t, result, http.StatusOK)
	data := decodeData(t, result).(map[string]any)
	assertWorkflowCurrentNode(t, data, "delivery")
	for _, nodeID := range []string{"split", "develop", "qa", "recovery", "join"} {
		if status := workflowTimelineNode(t, data, nodeID)["status"]; status != "skipped" {
			t.Fatalf("unselected parallel node %q status=%v, want skipped", nodeID, status)
		}
	}
	if status := workflowTimelineNode(t, data, "delivery")["status"]; status != "active" {
		t.Fatalf("shared downstream status=%v, want active", status)
	}
	selectedNodes := data["selected_node_ids"].([]any)
	selectedEdges := data["selected_edge_ids"].([]any)
	if len(selectedNodes) != 4 {
		t.Fatalf("selected display node count=%d, want 4: %v", len(selectedNodes), selectedNodes)
	}
	for _, nodeID := range []string{"start", "choice", "delivery", "end"} {
		if !containsAny(selectedNodes, nodeID) {
			t.Fatalf("selected display node %q missing from %v", nodeID, selectedNodes)
		}
	}
	if containsAny(selectedEdges, "e2") || containsAny(selectedEdges, "e3") {
		t.Fatalf("unselected condition edges remained selected: %v", selectedEdges)
	}
	for _, nodeID := range []string{"split", "develop", "qa", "recovery", "join"} {
		if containsAny(selectedNodes, nodeID) {
			t.Fatalf("unselected branch node %q remained in display topology: %v", nodeID, selectedNodes)
		}
	}
	for _, edgeID := range []string{"e1", "e4", "e12"} {
		if !containsAny(selectedEdges, edgeID) {
			t.Fatalf("selected display edge %q missing from %v", edgeID, selectedEdges)
		}
	}
	if len(selectedEdges) != 3 {
		t.Fatalf("unreachable edges remained in display topology: %v", selectedEdges)
	}
	if data["completed_steps"] != float64(1) || data["total_steps"] != float64(2) || data["progress"] != float64(50) {
		t.Fatalf("display topology progress=%v/%v (%v)", data["completed_steps"], data["total_steps"], data["progress"])
	}
}

func TestWorkflowDisplayTopologyUsesOnlyConfirmedFailurePath(t *testing.T) {
	run := workflowgorm.WorkflowNodeRun{
		NodeID: "risk", NodeType: "ai_check", Status: "completed",
		OutputJSON: encodeWorkflowRunTransition(0, "", "handle"),
	}
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start"},
			{ID: "risk", Type: "ai_check"},
			{ID: "handle", Type: "human_task"},
			{ID: "end", Type: "end"},
		},
		Edges: []workflowEdge{
			{ID: "start-risk", Source: "start", Target: "risk"},
			{ID: "success", Source: "risk", Target: "handle", Kind: "success"},
			{ID: "failure", Source: "risk", Target: "handle", Kind: "failure"},
			{ID: "handle-end", Source: "handle", Target: "end"},
		},
	}
	_, ambiguousEdges := workflowDisplayTopology(definition, []workflowgorm.WorkflowNodeRun{run}, workflowAllEdgeIDs(definition), nil)
	if !containsString(ambiguousEdges, "success") || containsString(ambiguousEdges, "failure") {
		t.Fatalf("ambiguous recovery target did not keep one normal path: %v", ambiguousEdges)
	}

	definition.Nodes = append(definition.Nodes, workflowNode{ID: "repair", Type: "human_task"})
	definition.Edges[2].Target = "repair"
	definition.Edges = append(definition.Edges, workflowEdge{ID: "repair-end", Source: "repair", Target: "end"})
	run.OutputJSON = encodeWorkflowRunTransition(0, "", "repair")
	nodes, failureEdges := workflowDisplayTopology(definition, []workflowgorm.WorkflowNodeRun{run}, workflowAllEdgeIDs(definition), nil)
	if containsString(failureEdges, "success") || !containsString(failureEdges, "failure") || containsString(nodes, "handle") || !containsString(nodes, "repair") {
		t.Fatalf("confirmed recovery path did not replace normal topology: nodes=%v edges=%v", nodes, failureEdges)
	}

	legacyDefinition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start"},
			{ID: "risk", Type: "ai_check"},
			{ID: "delivery", Type: "human_task"},
			{ID: "recovery", Type: "human_task"},
			{ID: "end", Type: "end"},
		},
		Edges: []workflowEdge{
			{ID: "start-risk", Source: "start", Target: "risk"},
			{ID: "success", Source: "risk", Target: "delivery", Kind: "success"},
			{ID: "failure", Source: "risk", Target: "recovery", Kind: "failure"},
			{ID: "delivery-end", Source: "delivery", Target: "end"},
			{ID: "recovery-end", Source: "recovery", Target: "end"},
		},
	}
	legacyRun := workflowgorm.WorkflowNodeRun{NodeID: "risk", NodeType: "ai_check", Status: "failed"}
	legacyNodes, legacyEdges := workflowDisplayTopology(legacyDefinition, []workflowgorm.WorkflowNodeRun{legacyRun}, workflowAllEdgeIDs(legacyDefinition), []string{"recovery"})
	if containsString(legacyEdges, "success") || !containsString(legacyEdges, "failure") || containsString(legacyNodes, "delivery") || !containsString(legacyNodes, "recovery") {
		t.Fatalf("current legacy recovery node did not replace stale normal topology: nodes=%v edges=%v", legacyNodes, legacyEdges)
	}
}

func TestWorkflowDisplayTopologyKeepsNormalPathAfterRetryLoop(t *testing.T) {
	run := workflowgorm.WorkflowNodeRun{
		NodeID: "completeness", NodeType: "ai_check", Status: "completed",
		OutputJSON: encodeWorkflowRunTransition(0, "", "input"),
	}
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start"},
			{ID: "input", Type: "form"},
			{ID: "completeness", Type: "ai_check"},
			{ID: "review", Type: "approval"},
			{ID: "end", Type: "end"},
		},
		Edges: []workflowEdge{
			{ID: "start-input", Source: "start", Target: "input"},
			{ID: "input-completeness", Source: "input", Target: "completeness"},
			{ID: "completeness-review", Source: "completeness", Target: "review", Kind: "success"},
			{ID: "completeness-input", Source: "completeness", Target: "input", Kind: "failure"},
			{ID: "review-end", Source: "review", Target: "end"},
		},
	}

	nodes, edges := workflowDisplayTopology(
		definition,
		[]workflowgorm.WorkflowNodeRun{run},
		workflowAllEdgeIDs(definition),
		[]string{"input"},
	)
	for _, nodeID := range []string{"start", "input", "completeness", "review", "end"} {
		if !containsString(nodes, nodeID) {
			t.Fatalf("retry loop hid normal node %q: nodes=%v edges=%v", nodeID, nodes, edges)
		}
	}
	for _, edgeID := range []string{
		"start-input", "input-completeness", "completeness-review", "completeness-input", "review-end",
	} {
		if !containsString(edges, edgeID) {
			t.Fatalf("retry loop hid normal edge %q: nodes=%v edges=%v", edgeID, nodes, edges)
		}
	}
}

func assertWorkflowCurrentNode(t *testing.T, data map[string]any, want string) {
	t.Helper()
	current := data["current_nodes"].([]any)
	if len(current) != 1 || current[0].(map[string]any)["id"] != want {
		t.Fatalf("current nodes=%v, want %s", current, want)
	}
}
