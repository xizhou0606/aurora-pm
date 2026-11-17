package httpapi

import (
	"encoding/json"
	"fmt"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWorkbenchTodoIncludesAssignedWorkflowNodes(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	now := time.Now().UTC()
	overdue := now.Add(-24 * time.Hour)
	future := now.Add(72 * time.Hour)

	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 701, SubjectType: "plan_unit", SubjectID: 12,
		NodeID: "implement", NodeType: "human_task", NodeName: "研发实现",
		Assignee: &admin.ID, DueAt: &overdue, Status: "running",
	})
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 702, SubjectType: "requirement", SubjectID: 13,
		NodeID: "review", NodeType: "approval", NodeName: "产品评审",
		Assignee: &admin.ID, DueAt: &future, Status: "running",
	})
	completedAI := seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 703, SubjectType: "requirement", SubjectID: 13,
		NodeID: "ai-check", NodeType: "ai_check", NodeName: "AI 完整性检查",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoAIRun(t, app, completedAI, "ai-check", 803, "completed")
	failedAI := seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 704, SubjectType: "requirement", SubjectID: 13,
		NodeID: "ai-risk", NodeType: "ai_check", NodeName: "AI 风险检查",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoAIRun(t, app, failedAI, "ai-risk", 804, "failed")
	queuedAI := seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 705, SubjectType: "requirement", SubjectID: 13,
		NodeID: "ai-wait", NodeType: "ai_check", NodeName: "AI 排队中",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoAIRun(t, app, queuedAI, "ai-wait", 805, "queued")
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 706, SubjectType: "requirement", SubjectID: 13,
		NodeID: "manual", TokenNodeID: "start", NodeType: "human_task", NodeName: "不会展示的开始节点",
		Assignee: &admin.ID, Status: "running",
	})
	otherUser := uint(999)
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 707, SubjectType: "requirement", SubjectID: 13,
		NodeID: "other", NodeType: "human_task", NodeName: "其他人的节点",
		Assignee: &otherUser, Status: "running",
	})
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 708, SubjectType: "requirement", SubjectID: 13,
		NodeID: "stopped", NodeType: "human_task", NodeName: "已结束实例节点",
		Assignee: &admin.ID, Status: "completed",
	})
	cancelledAI := seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 709, SubjectType: "requirement", SubjectID: 13,
		NodeID: "ai-cancelled", NodeType: "ai_check", NodeName: "AI 已取消",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoAIRun(t, app, cancelledAI, "ai-cancelled", 809, "cancelled")
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 710, SubjectType: "requirement", SubjectID: 13,
		NodeID: "publish", NodeType: "outbox_action", NodeName: "发布动作",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 711, SubjectType: "requirement", SubjectID: 13,
		NodeID: "timer", NodeType: "timer", NodeName: "到时确认",
		Assignee: &admin.ID, Status: "running",
	})
	seedWorkflowTodoInstance(t, app, workflowTodoSeed{
		ID: 712, WorkspaceID: 2, SubjectType: "requirement", SubjectID: 23,
		NodeID: "foreign", NodeType: "human_task", NodeName: "跨空间秘密节点",
		Assignee: &admin.ID, Status: "running",
	})

	summaryResponse := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/summary?domains=workflow", token, 1, nil)
	requireStatus(t, summaryResponse, http.StatusOK)
	summary := decodeData(t, summaryResponse).(map[string]any)["summary"].(map[string]any)
	if summary["total"] != float64(4) || summary["workflows"] != float64(4) ||
		summary["tasks"] != float64(0) || summary["bugs"] != float64(0) ||
		summary["overdue"] != float64(1) || summary["workflow_ai_failed"] != float64(0) {
		t.Fatalf("workflow todo summary=%v", summary)
	}

	feed := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=workflow&limit=20", token, 1, nil)
	requireStatus(t, feed, http.StatusOK)
	var envelope map[string]any
	if err := json.Unmarshal(feed.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["total"] != float64(4) {
		t.Fatalf("workflow todo total=%v body=%s", envelope["total"], feed.Body.String())
	}
	rows := envelope["data"].([]any)
	byTitle := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		byTitle[row["title"].(string)] = row
	}
	if _, leaked := byTitle["跨空间秘密节点"]; leaked {
		t.Fatalf("foreign workspace workflow leaked: %s", feed.Body.String())
	}
	// AI nodes run automatically and take no assignee, so they never become a
	// personal todo regardless of the seeded token or AI run status.
	for _, title := range []string{"AI 完整性检查", "AI 风险检查", "AI 排队中", "AI 已取消"} {
		if _, leaked := byTitle[title]; leaked {
			t.Fatalf("AI node leaked into personal todos: %s", feed.Body.String())
		}
	}
	implement := byTitle["研发实现"]
	if implement["source_type"] != "workflow_node" ||
		implement["todo_id"] != "workflow:701:implement:701" ||
		implement["assignee_user_id"] != float64(admin.ID) ||
		implement["detail_path"] != "/workspace/projects/11/items/12?workflow_node=implement" {
		t.Fatalf("historical plan todo=%v", implement)
	}
	review := byTitle["产品评审"]
	if review["action_kind"] != "approval" ||
		fmt.Sprint(review["allowed_actions"]) != "[approve reject]" ||
		review["detail_path"] != "/workspace/requirements/13?workflow_node=review" {
		t.Fatalf("approval todo=%v", review)
	}
	for _, title := range []string{"发布动作", "到时确认"} {
		if row := byTitle[title]; row["action_kind"] != "complete" ||
			fmt.Sprint(row["allowed_actions"]) != "[complete]" {
			t.Fatalf("default workflow todo %q=%v", title, row)
		}
	}

	requirementScope := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=workflow&requirement_ids=13", token, 1, nil)
	requireStatus(t, requirementScope, http.StatusOK)
	if decodeTodoFeedTotal(t, requirementScope) != 3 {
		t.Fatalf("requirement-scoped workflow todos=%s", requirementScope.Body.String())
	}
	search := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=workflow&q=产品评审", token, 1, nil)
	requireStatus(t, search, http.StatusOK)
	if decodeTodoFeedTotal(t, search) != 1 {
		t.Fatalf("searched workflow todos=%s", search.Body.String())
	}
}

type workflowTodoSeed struct {
	ID          uint
	WorkspaceID uint
	SubjectType string
	SubjectID   uint
	NodeID      string
	TokenNodeID string
	NodeType    string
	NodeName    string
	Assignee    *uint
	DueAt       *time.Time
	Status      string
}

func seedWorkflowTodoInstance(
	t *testing.T,
	app *testApp,
	seed workflowTodoSeed,
) workflowgorm.WorkflowInstance {
	t.Helper()
	workspaceID := seed.WorkspaceID
	if workspaceID == 0 {
		workspaceID = 1
	}
	definition := workflowTodoDefinition(seed.NodeID, seed.NodeType, seed.NodeName)
	now := time.Now().UTC()
	template := workflowgorm.WorkflowTemplate{
		ID: seed.ID, WorkspaceID: workspaceID, Name: seed.NodeName + "流程", SubjectType: seed.SubjectType,
		DraftDefinition: definition, PublishedVersion: 1, CreatedByUserID: 1, UpdatedByUserID: 1,
	}
	version := workflowgorm.WorkflowTemplateVersion{
		ID: seed.ID, WorkspaceID: workspaceID, TemplateID: seed.ID, Version: 1,
		Definition: definition, DefinitionHash: fmt.Sprintf("hash-%d", seed.ID),
		PublishedByUser: 1, PublishedAt: now,
	}
	plans := "{}"
	if seed.DueAt != nil {
		raw, err := json.Marshal(map[string]workflowNodePlan{
			seed.NodeID: {PlannedEndAt: seed.DueAt},
		})
		if err != nil {
			t.Fatal(err)
		}
		plans = string(raw)
	}
	instance := workflowgorm.WorkflowInstance{
		ID: seed.ID, WorkspaceID: workspaceID, TemplateID: seed.ID, TemplateVersionID: seed.ID,
		SubjectType: seed.SubjectType, SubjectID: seed.SubjectID, Status: seed.Status,
		CurrentNodeIDs: fmt.Sprintf("[%q]", workflowTodoSeedTokenNode(seed)), NodePlansJSON: plans,
		StartedByUserID: 1, StartedAt: now,
	}
	for _, row := range []any{&template, &version, &instance} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	token := workflowgorm.WorkflowToken{
		ID: seed.ID, WorkspaceID: workspaceID, InstanceID: seed.ID, NodeID: workflowTodoSeedTokenNode(seed),
		Status: "active", AssigneeUserID: seed.Assignee,
	}
	if err := app.db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	return instance
}

func workflowTodoSeedTokenNode(seed workflowTodoSeed) string {
	if seed.TokenNodeID != "" {
		return seed.TokenNodeID
	}
	return seed.NodeID
}

func workflowTodoDefinition(nodeID, nodeType, nodeName string) string {
	nodes := []map[string]any{
		{"id": "start", "type": "start", "name": "开始"},
		{"id": nodeID, "type": nodeType, "name": nodeName},
		{"id": "end", "type": "end", "name": "结束"},
	}
	if len(nodeType) > 3 && nodeType[:3] == "ai_" {
		nodes[1]["capability"] = "requirement.completeness"
	}
	edges := []map[string]any{
		{"id": "enter", "source": "start", "target": nodeID},
		{"id": "success", "source": nodeID, "target": "end", "kind": "success"},
	}
	if nodeType == "approval" {
		edges = append(edges, map[string]any{
			"id": "reject", "source": nodeID, "target": "end", "kind": "reject",
		})
	}
	if len(nodeType) > 3 && nodeType[:3] == "ai_" {
		edges = append(edges, map[string]any{
			"id": "failure", "source": nodeID, "target": "end", "kind": "failure",
		})
	}
	raw, _ := json.Marshal(map[string]any{"nodes": nodes, "edges": edges})
	return string(raw)
}

func seedWorkflowTodoAIRun(
	t *testing.T,
	app *testApp,
	instance workflowgorm.WorkflowInstance,
	nodeID string,
	id uint,
	status string,
) {
	t.Helper()
	now := time.Now().UTC()
	run := aigorm.AIRun{
		ID: id, WorkspaceID: 1, Capability: "requirement.completeness",
		SubjectType: instance.SubjectType, SubjectID: instance.SubjectID, SubjectVersion: 1,
		Status: status, Model: "test-model", InputJSON: `{}`, RequestedByUser: 1,
	}
	nodeRun := workflowgorm.WorkflowNodeRun{
		ID: id, WorkspaceID: 1, InstanceID: instance.ID, NodeID: nodeID,
		NodeType: "ai_check", Status: "waiting", StartedAt: now,
	}
	payload, _ := json.Marshal(workflowAIJobPayload{AIRunID: id, NodeID: nodeID})
	job := workflowgorm.WorkflowJob{
		ID: id, WorkspaceID: 1, InstanceID: instance.ID, NodeRunID: id,
		Kind: "ai_run", Status: "pending", PayloadJSON: string(payload),
		IdempotencyKey: fmt.Sprintf("todo-ai-%d", id), AvailableAt: now,
	}
	for _, row := range []any{&run, &nodeRun, &job} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func decodeTodoFeedTotal(t *testing.T, response *httptest.ResponseRecorder) int {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return int(envelope["total"].(float64))
}
