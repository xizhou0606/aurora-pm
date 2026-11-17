package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestWorkflowTemplateStripsAssigneesOnCreate(t *testing.T) {
	app, token, workspace, _, _ := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始", "config": map[string]any{"assignee_user_id": 99}},
			{"id": "input", "type": "form", "name": "信息录入", "config": map[string]any{"assignee_user_id": 0, "requirement_input": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "无负责人模板", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	createdData := decodeData(t, created).(map[string]any)
	templateID := uint(createdData["id"].(float64))
	createdDefinitionRaw, _ := json.Marshal(createdData["definition"])
	var createdDefinition workflowDefinition
	if err := json.Unmarshal(createdDefinitionRaw, &createdDefinition); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, createdDefinition)

	var template workflowgorm.WorkflowTemplate
	if err := app.db.Where("id = ? AND workspace_id = ?", templateID, workspace.ID).First(&template).Error; err != nil {
		t.Fatal(err)
	}
	var stored workflowDefinition
	if err := json.Unmarshal([]byte(template.DraftDefinition), &stored); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, stored)
	input, _ := workflowNodeByID(stored, "input")
	if required, _ := input.Config["requirement_input"].(bool); !required {
		t.Fatalf("template cleanup removed semantic config: %v", input.Config)
	}
}

func TestWorkflowNodeAssigneeSnapshotsDoNotAuthorizeTransitions(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	reviewer := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reviewer")
	reviewerToken := app.token(t, reviewer)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "信息录入"},
			{"id": "review", "type": "approval", "name": "人工评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "节点交接流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "指定负责人需求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"

	ownerDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), ownerToken, workspace.ID, nil)
	requireStatus(t, ownerDetail, http.StatusOK)
	ownerCurrent := decodeData(t, ownerDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if ownerCurrent["assignee_user_id"] != float64(1) || ownerCurrent["assignee_name"] != "owner" || ownerCurrent["can_act"] != true || !containsString(actionStrings(ownerCurrent["allowed_actions"]), "complete") {
		t.Fatalf("owner current node action contract=%v", ownerCurrent)
	}

	reviewerDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), reviewerToken, workspace.ID, nil)
	requireStatus(t, reviewerDetail, http.StatusOK)
	reviewerCurrent := decodeData(t, reviewerDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if reviewerCurrent["assignee_user_id"] != float64(1) || reviewerCurrent["can_act"] != true || !containsString(actionStrings(reviewerCurrent["allowed_actions"]), "complete") {
		t.Fatalf("entry-authorized current node action contract=%v", reviewerCurrent)
	}

	planPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	managedPlan := app.request(t, http.MethodPatch, planPath, ownerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{
			{"node_id": "input", "planned_end_at": "2026-08-31", "expected_planned_end_at": nil},
			{"node_id": "review", "planned_end_at": "2026-09-01", "expected_planned_end_at": nil, "assignee_user_id": reviewer.ID},
		},
	})
	requireStatus(t, managedPlan, http.StatusOK)
	if decodeData(t, managedPlan).(map[string]any)["can_manage_node_plans"] != true {
		t.Fatalf("workspace member cannot manage workflow plans: %s", managedPlan.Body.String())
	}
	allowedPlan := app.request(t, http.MethodPatch, planPath, reviewerToken, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "review", "planned_end_at": "2026-09-01", "expected_planned_end_at": "2026-09-01"}},
	})
	requireStatus(t, allowedPlan, http.StatusOK)

	requireStatus(t, app.request(t, http.MethodPost, actionPath, reviewerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)

	var reviewToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").First(&reviewToken).Error; err != nil {
		t.Fatal(err)
	}
	if reviewToken.AssigneeUserID == nil || *reviewToken.AssigneeUserID != reviewer.ID {
		t.Fatalf("review token assignee snapshot=%v", reviewToken.AssigneeUserID)
	}
	approved := app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review",
	})
	requireStatus(t, approved, http.StatusOK)
	if decodeData(t, approved).(map[string]any)["status"] != "completed" {
		t.Fatalf("reviewer did not complete workflow: %s", approved.Body.String())
	}
}

func TestRequirementEntryNodeAssigneeIsDataNotAuthorization(t *testing.T) {
	app, creatorToken, workspace, _, unit := newAIWorkflowFixture(t)
	productOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "product-owner")
	productOwnerToken := app.token(t, productOwner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "review", "type": "approval", "name": "产品评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, creatorToken, workspace.ID, "创建人录入流程", definition)
	requirement := createUnassignedRequirementWithWorkflow(t, app, creatorToken, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "创建人负责录入", "product_owner_user_id": productOwner.ID,
	})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"

	creatorDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), creatorToken, workspace.ID, nil)
	requireStatus(t, creatorDetail, http.StatusOK)
	creatorData := decodeData(t, creatorDetail).(map[string]any)
	creatorCurrent := creatorData["current_nodes"].([]any)[0].(map[string]any)
	if creatorCurrent["id"] != "input" || creatorCurrent["assignee_user_id"] != float64(1) || creatorCurrent["assignee_name"] != "owner" || creatorCurrent["can_act"] != true {
		t.Fatalf("requirement entry should be assigned to creator: %v", creatorCurrent)
	}
	for _, raw := range creatorData["timeline_nodes"].([]any) {
		node := raw.(map[string]any)
		if node["id"] == "input" {
			continue
		}
		if node["assignee_user_id"] != nil {
			t.Fatalf("later node %v should not display a fallback assignee", node)
		}
	}
	productOwnerDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), productOwnerToken, workspace.ID, nil)
	requireStatus(t, productOwnerDetail, http.StatusOK)
	productOwnerCurrent := decodeData(t, productOwnerDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if productOwnerCurrent["assignee_user_id"] != float64(1) || productOwnerCurrent["can_act"] != true {
		t.Fatalf("product owner should see the same entry action contract: %v", productOwnerCurrent)
	}
	requireStatus(t, app.request(t, http.MethodPost, actionPath, productOwnerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusConflict)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, creatorToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusConflict)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, creatorToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": productOwner.ID}},
	}), http.StatusOK)

	reviewDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), productOwnerToken, workspace.ID, nil)
	requireStatus(t, reviewDetail, http.StatusOK)
	reviewCurrent := decodeData(t, reviewDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if reviewCurrent["id"] != "review" || reviewCurrent["assignee_user_id"] != float64(productOwner.ID) || reviewCurrent["can_act"] != true {
		t.Fatalf("review should use the explicitly assigned next owner: %v", reviewCurrent)
	}
	requireStatus(t, app.request(t, http.MethodPost, actionPath, creatorToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, productOwnerToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "重复评审",
	}), http.StatusConflict)
}

func TestStaleRequirementEntryTokenDoesNotAuthorizeByAssignee(t *testing.T) {
	app, creatorToken, workspace, _, unit := newAIWorkflowFixture(t)
	productOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "product-owner")
	productOwnerToken := app.token(t, productOwner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入", "config": map[string]any{"assignee_user_id": productOwner.ID}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, creatorToken, workspace.ID, "旧快照录入流程", definition)
	requirement := createUnassignedRequirementWithWorkflow(t, app, creatorToken, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "旧 token 仍应归创建人", "product_owner_user_id": productOwner.ID,
	})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "input", "active").
		Update("assignee_user_id", productOwner.ID).Error; err != nil {
		t.Fatal(err)
	}

	creatorDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), creatorToken, workspace.ID, nil)
	requireStatus(t, creatorDetail, http.StatusOK)
	creatorCurrent := decodeData(t, creatorDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if creatorCurrent["assignee_user_id"] != float64(1) || creatorCurrent["can_act"] != true {
		t.Fatalf("stale token/template assignee leaked into requirement entry: %v", creatorCurrent)
	}
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, productOwnerToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, creatorToken, workspace.ID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusConflict)
	var completed workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "input").Order("id DESC").First(&completed).Error; err != nil {
		t.Fatal(err)
	}
	if completed.Status != "completed" {
		t.Fatalf("stale-token action did not complete node: %+v", completed)
	}
}

func TestWorkflowNodeAssigneeUsesInternalUserID(t *testing.T) {
	app, _, workspace, project, unit := newAIWorkflowFixture(t)
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", 1).Update("external_user_id", 77).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&projectgorm.ProjectSet{}).Where("id = ?", project.ID).Update("owner_user_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	var owner identitygorm.User
	if err := app.db.First(&owner, 1).Error; err != nil {
		t.Fatal(err)
	}
	ownerToken := app.token(t, owner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "confirm", "type": "human_task", "name": "确认"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "confirm"}, {"id": "e2", "source": "confirm", "target": "end"}},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "平台用户事实流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "平台用户事实"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	var active workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "confirm").First(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active.AssigneeUserID == nil || *active.AssigneeUserID != owner.ID {
		t.Fatalf("token did not store internal user id: %+v", active)
	}
	requireStatus(t, app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/actions",
		ownerToken, workspace.ID, map[string]any{"action": "complete", "node_id": "confirm"},
	), http.StatusOK)
}

func TestNullTokenFallsBackToRequirementOwnerAndPersistsOnAction(t *testing.T) {
	app, starterToken, workspace, _, unit := newAIWorkflowFixture(t)
	productOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "product-owner")
	productOwnerToken := app.token(t, productOwner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "历史评审"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, starterToken, workspace.ID, "历史负责人流程", definition)

	requirement := createRequirementWithWorkflow(t, app, starterToken, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "历史负责人回退", "product_owner_user_id": productOwner.ID,
	})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").
		Update("assignee_user_id", nil).Error; err != nil {
		t.Fatal(err)
	}

	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), starterToken, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	current := decodeData(t, detail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if current["assignee_user_id"] != float64(1) || current["assignee_name"] != "owner" || current["can_act"] != true {
		t.Fatalf("first node should stay assigned to creator: %v", current)
	}
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, productOwnerToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, starterToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review",
	}), http.StatusConflict)
	var completedToken workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "review").Order("id DESC").First(&completedToken).Error; err != nil {
		t.Fatal(err)
	}
	if completedToken.AssigneeUserID != nil || completedToken.Status != "completed" {
		t.Fatalf("null assignee should remain a data fact after entry-authorized action: %+v", completedToken)
	}
}

func TestQueuedAINodeTakesNoAssigneeAndNoPrematureActions(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	aiOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "ai-owner")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "ai", "type": "ai_check", "name": "AI 检查", "capability": "requirement.completeness"},
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
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "AI 免负责人流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "AI 节点免负责人"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	assigneeRejected := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		ownerToken, workspace.ID, map[string]any{"nodes": []map[string]any{{"node_id": "ai", "assignee_user_id": aiOwner.ID}}},
	)
	requireStatus(t, assigneeRejected, http.StatusBadRequest)
	if !strings.Contains(assigneeRejected.Body.String(), "workflow node ai cannot take an assignee") {
		t.Fatalf("AI node assignee must be rejected: %s", assigneeRejected.Body.String())
	}
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		ownerToken, workspace.ID, map[string]any{
			"confirm_subject_schedule": true,
			"nodes": []map[string]any{{
				"node_id": "ai", "planned_start_at": "2026-09-03", "expected_planned_start_at": nil,
				"planned_end_at": "2026-09-04", "expected_planned_end_at": nil,
			}},
		},
	), http.StatusOK)

	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), ownerToken, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	data := decodeData(t, detail).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	if current["assignee_user_id"] != nil || current["can_act"] != false || len(actionStrings(current["allowed_actions"])) != 0 || data["can_act"] != false {
		t.Fatalf("queued AI ownership/action contract=%v instance_can_act=%v", current, data["can_act"])
	}
}

func TestWorkflowArtifactMutationUsesEntryAuthorizationAndStateValidation(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	testOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "test-owner")
	testOwnerToken := app.token(t, testOwner)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "cases", "type": "human_task", "name": "测试用例", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "cases"}, {"id": "e2", "source": "cases", "target": "end"}},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "测试交付流程", definition)
	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{"title": "测试交付"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		ownerToken, workspace.ID, map[string]any{"nodes": []map[string]any{{"node_id": "cases", "assignee_user_id": testOwner.ID}}},
	), http.StatusOK)
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/nodes/cases/test-case-artifacts"
	// Entry authorization no longer consults the workflow assignee; malformed
	// multipart input is still rejected by the handler.
	requireStatus(t, app.request(t, http.MethodPost, path, ownerToken, workspace.ID, nil), http.StatusBadRequest)
	requireStatus(t, app.request(t, http.MethodPost, path, testOwnerToken, workspace.ID, nil), http.StatusBadRequest)
	requireStatus(t, app.request(t, http.MethodGet, path, ownerToken, workspace.ID, nil), http.StatusOK)
}

func TestPublishedWorkflowIgnoresLegacyTemplateAssignees(t *testing.T) {
	app, ownerToken, workspace, _, unit := newAIWorkflowFixture(t)
	productOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "product-owner")
	templatePerson := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "template-person")
	productOwnerToken := app.token(t, productOwner)
	templatePersonToken := app.token(t, templatePerson)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, ownerToken, workspace.ID, "旧模板绑人流程", definition)
	var version workflowgorm.WorkflowTemplateVersion
	if err := app.db.Where("workspace_id = ? AND template_id = ?", workspace.ID, templateID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	legacyDefinition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "评审", "config": map[string]any{"critical_gate": true, "assignee_user_id": templatePerson.ID}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": definition["edges"],
	}
	legacyRaw, err := json.Marshal(legacyDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&version).Update("definition", string(legacyRaw)).Error; err != nil {
		t.Fatal(err)
	}

	requirement := createRequirementWithWorkflow(t, app, ownerToken, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "不应继承模板负责人", "product_owner_user_id": productOwner.ID,
	})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), productOwnerToken, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	data := decodeData(t, detail).(map[string]any)
	current := data["current_nodes"].([]any)[0].(map[string]any)
	if current["assignee_user_id"] != float64(1) || current["assignee_name"] != "owner" || current["can_act"] != true {
		t.Fatalf("legacy template assignee leaked into requirement node: %v", current)
	}
	if config, _ := current["config"].(map[string]any); config != nil {
		if _, exists := config["assignee_user_id"]; exists {
			t.Fatalf("instance node config still exposed template assignee: %v", config)
		}
	}
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/actions"
	requireStatus(t, app.request(t, http.MethodPost, actionPath, templatePersonToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, productOwnerToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "需求负责人处理",
	}), http.StatusConflict)
	requireStatus(t, app.request(t, http.MethodPost, actionPath, ownerToken, workspace.ID, map[string]any{
		"action": "approve", "node_id": "review", "comment": "创建人处理",
	}), http.StatusConflict)
}

func TestWorkflowTemplateStripsLegacyAssigneesOnPatchValidateAndPublish(t *testing.T) {
	app, ownerToken, workspace, _, _ := newAIWorkflowFixture(t)
	baseDefinition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "评审", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", ownerToken, workspace.ID, map[string]any{
		"name": "兼容旧负责人字段", "subject_type": "requirement", "definition": baseDefinition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	legacyDefinition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "旧评审", "config": map[string]any{"assignee_user_id": 99999, "critical_gate": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": baseDefinition["edges"],
	}
	patched := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID), ownerToken, workspace.ID,
		map[string]any{"name": "已清理旧负责人", "definition": legacyDefinition},
	)
	requireStatus(t, patched, http.StatusOK)
	patchedRaw, _ := json.Marshal(decodeData(t, patched).(map[string]any)["definition"])
	var patchedDefinition workflowDefinition
	if err := json.Unmarshal(patchedRaw, &patchedDefinition); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, patchedDefinition)
	patchedReview, _ := workflowNodeByID(patchedDefinition, "review")
	if critical, _ := patchedReview.Config["critical_gate"].(bool); !critical {
		t.Fatalf("patch cleanup removed semantic config: %v", patchedReview.Config)
	}

	legacyDraft, err := json.Marshal(legacyDefinition)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowTemplate{}).
		Where("id = ? AND workspace_id = ?", templateID, workspace.ID).
		Update("draft_definition", string(legacyDraft)).Error; err != nil {
		t.Fatal(err)
	}
	nameOnlyPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID), ownerToken, workspace.ID,
		map[string]any{"name": "仅修改名称也清理负责人"},
	)
	requireStatus(t, nameOnlyPatch, http.StatusOK)
	var nameOnlyStored workflowgorm.WorkflowTemplate
	if err := app.db.Where("id = ? AND workspace_id = ?", templateID, workspace.ID).First(&nameOnlyStored).Error; err != nil {
		t.Fatal(err)
	}
	var nameOnlyDefinition workflowDefinition
	if err := json.Unmarshal([]byte(nameOnlyStored.DraftDefinition), &nameOnlyDefinition); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, nameOnlyDefinition)
	if err := app.db.Model(&workflowgorm.WorkflowTemplate{}).
		Where("id = ? AND workspace_id = ?", templateID, workspace.ID).
		Update("draft_definition", string(legacyDraft)).Error; err != nil {
		t.Fatal(err)
	}
	validated := app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/validate", ownerToken, workspace.ID, nil,
	)
	requireStatus(t, validated, http.StatusOK)
	if decodeData(t, validated).(map[string]any)["valid"] != true {
		t.Fatalf("legacy assignee made template invalid: %s", validated.Body.String())
	}
	validatedDefinition := app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/validate-definition", ownerToken, workspace.ID,
		map[string]any{"definition": legacyDefinition},
	)
	requireStatus(t, validatedDefinition, http.StatusOK)
	if decodeData(t, validatedDefinition).(map[string]any)["valid"] != true {
		t.Fatalf("legacy assignee made submitted definition invalid: %s", validatedDefinition.Body.String())
	}
	published := app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", ownerToken, workspace.ID, nil,
	)
	requireStatus(t, published, http.StatusOK)
	var template workflowgorm.WorkflowTemplate
	if err := app.db.Where("id = ? AND workspace_id = ?", templateID, workspace.ID).First(&template).Error; err != nil {
		t.Fatal(err)
	}
	var storedDraft workflowDefinition
	if err := json.Unmarshal([]byte(template.DraftDefinition), &storedDraft); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, storedDraft)
	var version workflowgorm.WorkflowTemplateVersion
	if err := app.db.Where("workspace_id = ? AND template_id = ?", workspace.ID, templateID).First(&version).Error; err != nil {
		t.Fatal(err)
	}
	var publishedDefinition workflowDefinition
	if err := json.Unmarshal([]byte(version.Definition), &publishedDefinition); err != nil {
		t.Fatal(err)
	}
	assertWorkflowTemplateHasNoAssignees(t, publishedDefinition)
}

func addWorkflowAssigneeUser(t *testing.T, app *testApp, workspaceID, teamID, id uint, name string) identitygorm.User {
	t.Helper()
	user := app.createUser(t, id, name, "user", "")
	for _, row := range []any{
		&identitygorm.WorkspaceMember{WorkspaceID: workspaceID, UserID: user.ID, Role: "member", Enabled: true},
		&identitygorm.TeamMember{WorkspaceID: workspaceID, TeamID: teamID, UserID: user.ID, Role: "member"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return user
}

func createPublishedWorkflowWithDefinition(
	t *testing.T,
	app *testApp,
	token string,
	workspaceID uint,
	name string,
	definition map[string]any,
) uint {
	t.Helper()
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspaceID, map[string]any{
		"name": name, "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspaceID, nil), http.StatusOK)
	return templateID
}

func assertWorkflowTemplateHasNoAssignees(t *testing.T, definition workflowDefinition) {
	t.Helper()
	for _, node := range definition.Nodes {
		if _, exists := node.Config["assignee_user_id"]; exists {
			t.Fatalf("template node %s retained assignee: %v", node.ID, node.Config)
		}
	}
}
