package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestRequirementWorkflowRequiresExplicitNextNodeAssignee(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "下一节点负责人流程", definition)
	requirement := createUnassignedRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "指定下一节点"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if requirement["product_owner_user_id"] != float64(1) {
		t.Fatalf("created requirement product owner=%v", requirement["product_owner_user_id"])
	}

	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), ownerToken, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	data := decodeData(t, detail).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	if current["id"] != "input" || current["assignee_user_id"] != float64(1) {
		t.Fatalf("entry node should belong to creator: %v", current)
	}
	for _, raw := range data["timeline_nodes"].([]any) {
		node := raw.(map[string]any)
		if node["id"] == "review" && node["assignee_user_id"] != nil {
			t.Fatalf("review should not fall back to a default assignee: %v", node)
		}
	}

	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	missing := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	})
	requireStatus(t, missing, http.StatusConflict)
	if !strings.Contains(missing.Body.String(), "next node assignee is required") {
		t.Fatalf("missing next assignee error=%s", missing.Body.String())
	}

	completed := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": reviewer.ID}},
	})
	requireStatus(t, completed, http.StatusOK)
	var reviewToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").First(&reviewToken).Error; err != nil {
		t.Fatal(err)
	}
	if reviewToken.AssigneeUserID == nil || *reviewToken.AssigneeUserID != reviewer.ID {
		t.Fatalf("review token assignee=%v", reviewToken.AssigneeUserID)
	}

	events := notificationEventsForWorkflow(t, app, instanceID)
	if len(events) != 2 {
		t.Fatalf("assignment notifications=%d want 2 (entry + review)", len(events))
	}
	reviewIntent := decodeNotificationIntent(t, events[1])
	if reviewIntent.Scene != notification.SceneWorkflowNodeAssigned ||
		len(reviewIntent.RecipientUserIDs) != 1 || reviewIntent.RecipientUserIDs[0] != reviewer.ID ||
		!strings.Contains(reviewIntent.FallbackPlainText, "产品评审") {
		t.Fatalf("review assignment intent=%+v text=%q", reviewIntent, reviewIntent.FallbackPlainText)
	}

	approved := app.request(t, http.MethodPost, actionPath, app.token(t, reviewer), workspace.ID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "通过",
	})
	requireStatus(t, approved, http.StatusOK)
	if decodeData(t, approved).(map[string]any)["status"] != "completed" {
		t.Fatalf("workflow should complete without a next assignee: %s", approved.Body.String())
	}
}

func TestRequirementWorkflowRejectReusesExistingAssignee(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "驳回沿用负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "驳回沿用"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": reviewer.ID}},
	}), http.StatusOK)
	rejected := app.request(t, http.MethodPost, actionPath, app.token(t, reviewer), workspace.ID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "退回补充",
	})
	requireStatus(t, rejected, http.StatusOK)
	var inputToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").First(&inputToken).Error; err != nil {
		t.Fatal(err)
	}
	if inputToken.AssigneeUserID == nil || *inputToken.AssigneeUserID != 1 {
		t.Fatalf("reject should reuse the entry owner, got=%v", inputToken.AssigneeUserID)
	}
}

func TestRequirementWorkflowConditionNextAssigneeOverridesPlan(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	designer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "designer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "ui", "type": "condition", "name": "是否需要UI设计"},
			{"id": "design", "type": "human_task", "name": "UI/UX设计"},
			{"id": "develop", "type": "human_task", "name": "研发实现"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "ui"},
			{"id": "e2", "source": "ui", "target": "design", "condition": "subject.needs_ui == true"},
			{"id": "e3", "source": "ui", "target": "develop", "condition": "subject.needs_ui == false"},
			{"id": "e4", "source": "design", "target": "end"},
			{"id": "e5", "source": "develop", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "条件下一负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "覆盖计划负责人"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"design":{"assignee_user_id":1}}`).Error; err != nil {
		t.Fatal(err)
	}

	chosen := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/actions", ownerToken, workspace.ID, map[string]any{
		"action": "condition_yes", "node_id": "ui",
		"next_node_assignees": []map[string]any{{"node_id": "design", "assignee_user_id": designer.ID}},
	})
	requireStatus(t, chosen, http.StatusOK)
	var designToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "design", "active").First(&designToken).Error; err != nil {
		t.Fatal(err)
	}
	if designToken.AssigneeUserID == nil || *designToken.AssigneeUserID != designer.ID {
		t.Fatalf("design token should use the chosen assignee, got=%v", designToken.AssigneeUserID)
	}
	data := decodeData(t, chosen).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	if current["id"] != "design" || current["assignee_user_id"] != float64(designer.ID) {
		t.Fatalf("design current assignee=%v", current)
	}
}

func TestRequirementWorkflowNextAssigneeOverridesPlanOnEveryAdvance(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	developer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "developer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "develop", "type": "human_task", "name": "研发实现"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "develop", "kind": "approve"},
			{"id": "e4", "source": "develop", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "每节点指定负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "覆盖每节点计划负责人"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"review":{"assignee_user_id":1},"develop":{"assignee_user_id":1}}`).Error; err != nil {
		t.Fatal(err)
	}
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"

	completed := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": reviewer.ID}},
	})
	requireStatus(t, completed, http.StatusOK)
	assertWorkflowNodeAssignee(t, app, workspace.ID, instanceID, "review", reviewer.ID)

	approved := app.request(t, http.MethodPost, actionPath, app.token(t, reviewer), workspace.ID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "通过",
		"next_node_assignees": []map[string]any{{"node_id": "develop", "assignee_user_id": developer.ID}},
	})
	requireStatus(t, approved, http.StatusOK)
	assertWorkflowNodeAssignee(t, app, workspace.ID, instanceID, "develop", developer.ID)
}

func TestRequirementWorkflowAdvanceDoesNotReuseLastAssignee(t *testing.T) {
	app, ownerToken, workspace, _, unit := newNotifyingAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	developer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "developer")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "前进不沿用历史负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "历史负责人不可复用"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": reviewer.ID}},
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, app.token(t, reviewer), workspace.ID, map[string]any{
		"action": "reject", "node_id": "review", "comment": "退回",
	}), http.StatusOK)
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", "{}").Error; err != nil {
		t.Fatal(err)
	}

	missing := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	})
	requireStatus(t, missing, http.StatusConflict)
	if !strings.Contains(missing.Body.String(), "next node assignee is required") {
		t.Fatalf("missing next assignee after last token should not reuse history: %s", missing.Body.String())
	}

	completed := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": developer.ID}},
	})
	requireStatus(t, completed, http.StatusOK)
	assertWorkflowNodeAssignee(t, app, workspace.ID, instanceID, "review", developer.ID)
}

func TestRequirementWorkflowAINextNodeNeedsNoAssignee(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	checker := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "checker")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "ai", "type": "ai_check", "name": "完整性检查", "capability": "requirement.completeness"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai"},
			{"id": "e3", "source": "ai", "target": "end", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "AI 下一节点免负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "AI 节点不设负责人"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"

	rejected := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "ai", "assignee_user_id": checker.ID}},
	})
	requireStatus(t, rejected, http.StatusBadRequest)
	if !strings.Contains(rejected.Body.String(), "next node ai cannot take an assignee") {
		t.Fatalf("AI next assignee must be rejected: %s", rejected.Body.String())
	}

	completed := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	})
	requireStatus(t, completed, http.StatusOK)
	var aiToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "ai", "active").First(&aiToken).Error; err != nil {
		t.Fatal(err)
	}
	if aiToken.AssigneeUserID != nil {
		t.Fatalf("AI node token must stay unassigned, got=%v", aiToken.AssigneeUserID)
	}
	data := decodeData(t, completed).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	actions := actionStrings(current["allowed_actions"])
	if current["id"] != "ai" || current["assignee_user_id"] != nil {
		t.Fatalf("queued AI node should show no owner: %v", current)
	}
	for _, forbidden := range []string{"complete", "retry_ai", "recheck_ai", "override_ai"} {
		if containsString(actions, forbidden) {
			t.Fatalf("queued AI node must not expose %s: %v", forbidden, actions)
		}
	}
}

func TestRequirementWorkflowCompletenessAssignsPreReviewOwner(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	bill := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "bill")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"requirement_input": true}},
			{"id": "ai", "type": "ai_check", "name": "完整性检查", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "需求预审审批", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai"},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e5", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "完整性指定预审负责人", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "预审应是 bill"})
	requirementID := uint(requirement["id"].(float64))
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"review":{"assignee_user_id":1}}`).Error; err != nil {
		t.Fatal(err)
	}
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)

	var aiRun aigorm.AIRun
	if err := app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", requirementID).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(workflowCompletenessOutput(true, aiRun.SubjectVersion, 90))
	if err := app.db.Model(&aiRun).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
		t.Fatal(err)
	}

	completed := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "ai",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": bill.ID}},
	})
	requireStatus(t, completed, http.StatusOK)
	assertWorkflowNodeAssignee(t, app, workspace.ID, instanceID, "review", bill.ID)

	data := decodeData(t, completed).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	if current["id"] != "review" || current["assignee_user_id"] != float64(bill.ID) || current["assignee_name"] != "bill" {
		t.Fatalf("pre-review should echo bill, got=%v", current)
	}
}

func assertWorkflowNodeAssignee(t *testing.T, app *testApp, workspaceID, instanceID uint, nodeID string, want uint) {
	t.Helper()
	var token workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspaceID, instanceID, nodeID, "active").First(&token).Error; err != nil {
		t.Fatal(err)
	}
	if token.AssigneeUserID == nil || *token.AssigneeUserID != want {
		t.Fatalf("%s token assignee=%v want %d", nodeID, token.AssigneeUserID, want)
	}
}

func TestWorkflowAssigneeRequiredNodeIDsSkipsTerminalAndConnectors(t *testing.T) {
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start", Name: "开始"},
			{ID: "split", Type: "parallel_split", Name: "并行"},
			{ID: "dev", Type: "human_task", Name: "研发"},
			{ID: "qa", Type: "human_task", Name: "测试"},
			{ID: "join", Type: "parallel_join", Name: "汇合"},
			{ID: "end", Type: "end", Name: "结束"},
		},
		Edges: []workflowEdge{
			{ID: "e1", Source: "split", Target: "dev"},
			{ID: "e2", Source: "split", Target: "qa"},
			{ID: "e3", Source: "dev", Target: "join"},
			{ID: "e4", Source: "qa", Target: "join"},
			{ID: "e5", Source: "join", Target: "end"},
		},
	}
	got := workflowAssigneeRequiredNodeIDs(definition, []string{"split", "end"})
	if strings.Join(got, ",") != "dev,qa" && strings.Join(got, ",") != "qa,dev" {
		if len(got) != 2 || !containsString(got, "dev") || !containsString(got, "qa") {
			t.Fatalf("required=%v", got)
		}
	}
}

func newNotifyingAIWorkflowFixture(t *testing.T) (*testApp, string, identitygorm.Workspace, projectgorm.ProjectSet, projectgorm.PlanUnit) {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AI.Enabled = true
		cfg.AI.Model = "qwen3.6-max-preview"
		cfg.AI.InternalSecret = "test-internal-secret-at-least-32-characters"
		enableTestNotifications(cfg)
	})
	user := app.createUser(t, 1, "owner", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "AI PM", OwnerUserID: &user.ID}
	unit := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "V1.1", Type: "version"}
	for _, row := range []any{&workspace, &team, &identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: "owner", Enabled: true}, &identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: "owner"}, &project, &unit} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return app, app.token(t, user), workspace, project, unit
}

func notificationEventsForWorkflow(t *testing.T, app *testApp, instanceID uint) []auditgorm.OutboxEvent {
	t.Helper()
	var events []auditgorm.OutboxEvent
	if err := app.db.Where(
		"aggregate_type = ? AND aggregate_id = ? AND event_type = ?",
		"workflow_node_notification", instanceID, notification.EventType(notification.SceneWorkflowNodeAssigned),
	).Order("id ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}
