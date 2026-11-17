package httpapi

import (
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

func TestRecordedConditionTargetSurvivesSubjectChangesForSchedule(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "按优先级分支"},
			{"id": "high", "type": "human_task", "name": "高优先级路径"},
			{"id": "normal", "type": "human_task", "name": "普通路径"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "high", "kind": "yes", "condition": `subject.priority == "p2"`},
			{"id": "e3", "source": "choice", "target": "normal", "kind": "yes", "condition": `subject.priority != "p2"`},
			{"id": "e4", "source": "choice", "target": "normal", "kind": "no"},
			{"id": "e5", "source": "high", "target": "end"},
			{"id": "e6", "source": "normal", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "条件历史固定", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "条件历史固定", "priority": "p2",
	})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	instancePath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID)
	planPath := instancePath + "/node-plans"
	requireStatus(t, app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{
			{"node_id": "choice", "planned_start_at": "2026-08-01", "expected_planned_start_at": nil, "planned_end_at": "2026-08-02", "expected_planned_end_at": nil},
			{"node_id": "high", "planned_start_at": "2026-08-03", "expected_planned_start_at": nil, "planned_end_at": "2026-08-04", "expected_planned_end_at": nil},
			{"node_id": "normal", "planned_start_at": "2026-08-20", "expected_planned_start_at": nil, "planned_end_at": "2026-08-25", "expected_planned_end_at": nil},
		},
	}), http.StatusOK)
	action := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_yes", "node_id": "choice",
	})
	requireStatus(t, action, http.StatusOK)
	assertSelectedScheduleBranch(t, decodeData(t, action).(map[string]any), "high", "normal")

	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-31"}), http.StatusOK)
	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(requirementID), token, workspace.ID,
		map[string]any{"priority": "p1", "expected_version": requirement.Version}), http.StatusOK)

	current := app.request(t, http.MethodGet, instancePath, token, workspace.ID, nil)
	requireStatus(t, current, http.StatusOK)
	currentData := decodeData(t, current).(map[string]any)
	assertSelectedScheduleBranch(t, currentData, "high", "normal")
	if currentData["subject_schedule"].(map[string]any)["state"] == "conflict" {
		t.Fatalf("subject change rewrote historical schedule topology: %s", current.Body.String())
	}

	requireStatus(t, app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "high", "planned_end_at": "2026-08-05", "expected_planned_end_at": "2026-08-04"}},
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-06", "expected_planned_end_at": "2026-08-10"}), http.StatusOK)
}

func assertSelectedScheduleBranch(t *testing.T, data map[string]any, selected, excluded string) {
	t.Helper()
	nodeIDs := data["selected_node_ids"].([]any)
	if !containsAny(nodeIDs, selected) || containsAny(nodeIDs, excluded) {
		t.Fatalf("selected nodes=%v, want %s without %s", nodeIDs, selected, excluded)
	}
}
