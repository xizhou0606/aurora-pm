package httpapi

import (
	"net/http"
	"strings"
	"testing"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task/plan"
)

func TestTaskPlanAIRunAllowsRequirementsWithoutOwners(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 1)

	created := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": taskplan.Capability, "subject_type": "requirement", "subject_id": requirement.ID,
	})
	requireStatus(t, created, http.StatusCreated)
	requireModelCount(t, app.db, "queued task plan run", &aigorm.AIRun{}, "capability = ?", 1, taskplan.Capability)
}

func TestWorkflowTaskPlanDoesNotQueueUntilLaneOwnersAssigned(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	front := app.createUser(t, 21, "front", "user", "")
	back := app.createUser(t, 22, "back", "user", "")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "plan", "type": "ai_generate", "name": "任务拆解", "capability": taskplan.Capability},
			{"id": "repair", "type": "human_task", "name": "人工补充"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "plan"},
			{"id": "e2", "source": "plan", "target": "end", "kind": "success"},
			{"id": "e3", "source": "plan", "target": "repair", "kind": "failure"},
			{"id": "e4", "source": "repair", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "任务拆解等人", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "结算优化"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	actionPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID)

	requireModelCount(t, app.db, "no auto-queued task plan", &aigorm.AIRun{},
		"workspace_id = ? AND capability = ?", 0, workspace.ID, taskplan.Capability)

	detail := app.request(t, http.MethodGet, actionPath, token, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	current := decodeData(t, detail).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if containsString(actionStrings(current["allowed_actions"]), "retry_ai") {
		t.Fatalf("task plan started without owners: %v", current["allowed_actions"])
	}
	if containsString(actionStrings(current["allowed_actions"]), "complete") {
		t.Fatalf("task plan complete allowed before AI: %v", current["allowed_actions"])
	}

	patched := app.request(t, http.MethodPatch, "/api/v1/platform/requirements/"+uintString(requirementID), token, workspace.ID, map[string]any{
		"expected_version": created["version"], "frontend_owner_user_id": front.ID, "backend_owner_user_id": back.ID,
	})
	requireStatus(t, patched, http.StatusOK)
	saved := decodeData(t, patched).(map[string]any)
	if saved["frontend_owner_user_id"] != float64(front.ID) || saved["backend_owner_user_id"] != float64(back.ID) {
		t.Fatalf("lane owners were not echoed: %v", saved)
	}

	ready := app.request(t, http.MethodGet, actionPath, token, workspace.ID, nil)
	requireStatus(t, ready, http.StatusOK)
	readyNode := decodeData(t, ready).(map[string]any)["current_nodes"].([]any)[0].(map[string]any)
	if !containsString(actionStrings(readyNode["allowed_actions"]), "retry_ai") {
		t.Fatalf("task plan cannot start after owners: %v", readyNode["allowed_actions"])
	}

	started := app.request(t, http.MethodPost, actionPath+"/actions", token, workspace.ID, map[string]any{
		"action": "retry_ai", "node_id": "plan",
	})
	requireStatus(t, started, http.StatusOK)
	requireModelCount(t, app.db, "started task plan run", &aigorm.AIRun{},
		"workspace_id = ? AND capability = ? AND status = ?", 1, workspace.ID, taskplan.Capability, "queued")
}

func TestApplyTaskPlanWithoutAssigneesLeavesTasksUnassigned(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	front := app.createUser(t, 21, "front", "user", "")
	back := app.createUser(t, 22, "back", "user", "")
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	requirement.FrontendOwnerUserID = &front.ID
	requirement.BackendOwnerUserID = &back.ID
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", requirement.ID).Updates(map[string]any{
		"frontend_owner_user_id": front.ID, "backend_owner_user_id": back.ID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))

	// The standalone path never auto-assigns lane owners, even when the
	// requirement has them: assignees are chosen manually at apply time.
	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, nil)
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, app, requirement, 4)

	var rows []taskgorm.WorkTask
	if err := app.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.AssigneeID != nil {
			t.Fatalf("task plan row was auto-assigned without a manual pick: %+v", row)
		}
	}
}

func TestApplyTaskPlanWithManualAssignees(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	front := app.createUser(t, 21, "front", "user", "")
	back := app.createUser(t, 22, "back", "user", "")
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, map[string]any{
		"assignees": map[string]any{"frontend": front.ID, "backend": back.ID},
	})
	requireStatus(t, response, http.StatusOK)
	assertTaskPlanTree(t, app, requirement, 4)

	var rows []taskgorm.WorkTask
	if err := app.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if strings.Contains(row.Title, "前端") && (row.AssigneeID == nil || *row.AssigneeID != front.ID) {
			t.Fatalf("frontend task assignee=%v want=%d row=%+v", row.AssigneeID, front.ID, row)
		}
		if strings.Contains(row.Title, "后端") && (row.AssigneeID == nil || *row.AssigneeID != back.ID) {
			t.Fatalf("backend task assignee=%v want=%d row=%+v", row.AssigneeID, back.ID, row)
		}
	}
	// 主任务标题 = 需求标题-链路，子任务挂主任务下。
	for _, want := range []string{requirement.Title + "-前端", requirement.Title + "-后端"} {
		found := false
		for _, row := range rows {
			if row.Kind == taskgorm.WorkTaskKindTask && row.Title == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing main task %q in created rows", want)
		}
	}
}

func TestApplyTaskPlanRejectsUnknownAssignee(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 3)
	run := seedCompletedTaskPlanRun(t, app, requirement, taskPlanOutput(nil))
	unknown := uint(99999)

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs/"+uintString(run.ID)+"/apply", token, workspace.ID, map[string]any{
		"assignees": map[string]any{"frontend": unknown},
	})
	requireStatus(t, response, http.StatusBadRequest)
	requireModelCount(t, app.db, "no applied run", &aigorm.AIRun{}, "id = ? AND status = ?", 0, run.ID, "applied")
}
