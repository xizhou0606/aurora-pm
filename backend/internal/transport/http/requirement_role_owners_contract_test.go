package httpapi

import (
	"net/http"
	"testing"
)

func TestCreateRequirementDefaultsProductOwnersToCreator(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)

	created := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, workspace.ID, map[string]any{
		"priority":            "p1",
		"version_task_new_id": unit.ID, "workflow_template_id": templateID, "title": "默认产品负责人",
	})
	requireStatus(t, created, http.StatusCreated)
	data := decodeData(t, created).(map[string]any)
	if data["product_owner_user_id"] != float64(1) {
		t.Fatalf("scalar product owner=%v", data["product_owner_user_id"])
	}
	assertUintIDs(t, data["product_owner_user_ids"], 1)

	pool := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "workflow_template_id": templateID, "title": "池内默认产品负责人",
	})
	requireStatus(t, pool, http.StatusCreated)
	poolData := decodeData(t, pool).(map[string]any)
	if poolData["product_owner_user_id"] != float64(1) {
		t.Fatalf("pool scalar product owner=%v", poolData["product_owner_user_id"])
	}
	assertUintIDs(t, poolData["product_owner_user_ids"], 1)
}

func TestRequirementRoleOwnersPersistMultiSelectAndSyncScalars(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	ui := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "ui")
	front := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "front")
	frontTwo := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 4, "front-two")
	back := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 5, "back")
	tester := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 6, "tester")
	productTwo := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 7, "product-two")
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)

	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title":                   "多人角色",
		"design_owner_user_ids":   []uint{ui.ID},
		"frontend_owner_user_ids": []uint{front.ID, frontTwo.ID},
		"backend_owner_user_ids":  []uint{back.ID},
		"tester_user_ids":         []uint{tester.ID},
		"product_owner_user_ids":  []uint{1, productTwo.ID},
	})
	if created["design_owner_user_id"] != float64(ui.ID) ||
		created["frontend_owner_user_id"] != float64(front.ID) ||
		created["backend_owner_user_id"] != float64(back.ID) {
		t.Fatalf("scalar owners were not synced: %v", created)
	}
	assertUintIDs(t, created["design_owner_user_ids"], ui.ID)
	assertUintIDs(t, created["frontend_owner_user_ids"], front.ID, frontTwo.ID)
	assertUintIDs(t, created["backend_owner_user_ids"], back.ID)
	assertUintIDs(t, created["tester_user_ids"], tester.ID)
	assertUintIDs(t, created["product_owner_user_ids"], 1, productTwo.ID)

	patched := app.request(t, http.MethodPatch, "/api/v1/platform/requirements/"+uintString(uint(created["id"].(float64)))+"/role-owners",
		token, workspace.ID, map[string]any{
			"expected_version":        created["version"],
			"frontend_owner_user_ids": []uint{frontTwo.ID},
		})
	requireStatus(t, patched, http.StatusOK)
	saved := decodeData(t, patched).(map[string]any)
	if saved["frontend_owner_user_id"] != float64(frontTwo.ID) {
		t.Fatalf("patched scalar frontend owner=%v", saved["frontend_owner_user_id"])
	}
	assertUintIDs(t, saved["frontend_owner_user_ids"], frontTwo.ID)
}

func TestDesignAndTestNodesKeepRoleOwnerFactsWithoutAuthorization(t *testing.T) {
	app, creatorToken, workspace, _, unit := newAIWorkflowFixture(t)
	productOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "product-owner")
	uiOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 3, "ui-owner")
	tester := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 4, "tester")
	uiToken := app.token(t, uiOwner)
	testerToken := app.token(t, tester)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "需求录入"},
			{"id": "design", "type": "human_task", "name": "UI 设计", "config": map[string]any{"require_design_image": true}},
			{"id": "cases", "type": "human_task", "name": "测试用例", "config": map[string]any{"test_case_artifact": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "design"},
			{"id": "e3", "source": "design", "target": "cases"},
			{"id": "e4", "source": "cases", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, creatorToken, workspace.ID, "角色分派流程", definition)
	requirement := createRequirementWithWorkflow(t, app, creatorToken, workspace.ID, unit.ID, templateID, map[string]any{
		"title":                  "设计与测试分派",
		"product_owner_user_ids": []uint{productOwner.ID},
		"design_owner_user_ids":  []uint{uiOwner.ID},
		"tester_user_ids":        []uint{tester.ID},
	})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, creatorToken, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
		"next_node_assignees": []map[string]any{{"node_id": "design", "assignee_user_id": uiOwner.ID}},
	}), http.StatusOK)

	designDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), uiToken, workspace.ID, nil)
	requireStatus(t, designDetail, http.StatusOK)
	designCurrent := decodeData(t, designDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if designCurrent["id"] != "design" || designCurrent["assignee_user_id"] != float64(uiOwner.ID) || designCurrent["can_act"] != true {
		t.Fatalf("design node should use the explicitly assigned UI owner: %v", designCurrent)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, creatorToken, instanceID, map[string]any{
		"action": "complete", "node_id": "design",
	}), http.StatusConflict)
	requireStatus(t, app.workflowAction(t, workspace.ID, uiToken, instanceID, map[string]any{
		"action": "complete", "node_id": "design", "design_url": "https://www.figma.com/design/role-owners/PM",
		"next_node_assignees": []map[string]any{{"node_id": "cases", "assignee_user_id": tester.ID}},
	}), http.StatusOK)

	casesDetail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), testerToken, workspace.ID, nil)
	requireStatus(t, casesDetail, http.StatusOK)
	casesCurrent := decodeData(t, casesDetail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if casesCurrent["id"] != "cases" || casesCurrent["assignee_user_id"] != float64(tester.ID) || casesCurrent["can_act"] != true {
		t.Fatalf("test node should use the explicitly assigned tester: %v", casesCurrent)
	}
}

func assertUintIDs(t *testing.T, raw any, want ...uint) {
	t.Helper()
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("ids type=%T value=%v", raw, raw)
	}
	if len(items) != len(want) {
		t.Fatalf("ids=%v want=%v", raw, want)
	}
	for index, id := range want {
		if items[index] != float64(id) {
			t.Fatalf("ids[%d]=%v want=%d in %v", index, items[index], id, raw)
		}
	}
}
