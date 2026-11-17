package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkflowNodePlanPartialDatesDoNotInferSubject(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "部分节点排期", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "部分节点排期"})
	instanceID := uint(created["workflow_instance_id"].(float64))

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil,
		}}})
	requireStatus(t, response, http.StatusOK)
	schedule := decodeData(t, response).(map[string]any)["subject_schedule"].(map[string]any)
	if schedule["state"] != "partial" || schedule["suggested"] != nil || schedule["can_confirm"] != false {
		t.Fatalf("partial node schedule=%v", schedule)
	}
	var stored projectgorm.PlanUnit
	if err := app.db.First(&stored, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PlannedStartAt != nil || stored.PlannedEndAt != nil {
		t.Fatalf("partial node plan inferred subject dates: %+v", stored)
	}
}

func TestWorkflowConfirmationFillsOnlyMissingPlanUnitBoundary(t *testing.T) {
	tests := []struct {
		name          string
		existingStart *string
		existingEnd   *string
		wantStart     string
		wantEnd       string
	}{
		{name: "preserve existing start", existingStart: scheduleString("2026-08-01"), wantStart: "2026-08-01", wantEnd: "2026-08-20"},
		{name: "preserve existing end", existingEnd: scheduleString("2026-08-31"), wantStart: "2026-08-10", wantEnd: "2026-08-31"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			if err := app.db.Model(&project).Updates(map[string]any{
				"planned_start_at": scheduleTestTime("2026-08-01"),
				"planned_end_at":   scheduleTestTime("2026-08-31"),
			}).Error; err != nil {
				t.Fatal(err)
			}
			updates := map[string]any{"planned_start_at": nil, "planned_end_at": nil}
			if test.existingStart != nil {
				updates["planned_start_at"] = scheduleTestTime(*test.existingStart)
			}
			if test.existingEnd != nil {
				updates["planned_end_at"] = scheduleTestTime(*test.existingEnd)
			}
			if err := app.db.Model(&unit).Updates(updates).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, test.name, scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": test.name})
			instanceID := uint(created["workflow_instance_id"].(float64))
			path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
			body := map[string]any{"nodes": []map[string]any{{
				"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
			}}}
			requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, body), http.StatusConflict)
			body["confirm_subject_schedule"] = true
			requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, body), http.StatusOK)

			var stored projectgorm.PlanUnit
			if err := app.db.First(&stored, unit.ID).Error; err != nil {
				t.Fatal(err)
			}
			assertScheduleDate(t, stored.PlannedStartAt, test.wantStart)
			assertScheduleDate(t, stored.PlannedEndAt, test.wantEnd)
		})
	}
}

func TestProjectSetEmptyAndPartialBoundariesConstrainPlanUnits(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	unitPath := "/api/v1/platform/project-new/" + uintString(unit.ID)
	projectPath := "/api/v1/platform/project-sets/" + uintString(project.ID)

	requireStatus(t, app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-08-01", "expected_planned_start_at": nil,
	}), http.StatusOK)
	beforeStart := app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-07-31", "expected_planned_start_at": "2026-08-10",
	})
	requireStatus(t, beforeStart, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, beforeStart, "PLAN_UNIT_BEFORE_PROJECT_START")

	requireStatus(t, app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_start_at": nil, "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-31", "expected_planned_end_at": nil,
	}), http.StatusOK)
	afterEnd := app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"planned_end_at": "2026-09-01", "expected_planned_end_at": "2026-08-20",
	})
	requireStatus(t, afterEnd, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, afterEnd, "PLAN_UNIT_AFTER_PROJECT_END")

	narrowParent := app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31",
	})
	requireStatus(t, narrowParent, http.StatusConflict)
	assertScheduleConflictSubject(t, narrowParent, "PLAN_UNIT_AFTER_PROJECT_END", "plan_unit", unit.ID)
	reversedParent := app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-09-01", "expected_planned_start_at": nil,
	})
	requireStatus(t, reversedParent, http.StatusConflict)
	assertScheduleConflictSubject(t, reversedParent, "PROJECT_START_AFTER_END", "project_set", project.ID)
}

func TestWorkflowDisplayScheduleUsesConditionDecisionAndRollbackDoesNotGate(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "选择"},
			{"id": "left", "type": "human_task", "name": "左分支"},
			{"id": "right", "type": "human_task", "name": "右分支"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "left", "kind": "yes"},
			{"id": "e3", "source": "choice", "target": "right", "kind": "no"},
			{"id": "e4", "source": "left", "target": "end"},
			{"id": "e5", "source": "right", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "条件排期与回退", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "条件排期与回退"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	planPath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	planned := app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{"nodes": []map[string]any{
		{"node_id": "choice", "planned_start_at": "2026-08-02", "expected_planned_start_at": nil, "planned_end_at": "2026-08-03", "expected_planned_end_at": nil},
		{"node_id": "left", "planned_start_at": "2026-08-20", "expected_planned_start_at": nil, "planned_end_at": "2026-08-25", "expected_planned_end_at": nil},
		{"node_id": "right", "planned_start_at": "2026-08-04", "expected_planned_start_at": nil, "planned_end_at": "2026-08-05", "expected_planned_end_at": nil},
	}})
	requireStatus(t, planned, http.StatusOK)
	plannedData := decodeData(t, planned).(map[string]any)
	for _, nodeID := range []string{"left", "right"} {
		if !containsAny(plannedData["selected_node_ids"].([]any), nodeID) {
			t.Fatalf("unresolved condition omitted %s: %v", nodeID, plannedData["selected_node_ids"])
		}
	}
	decided := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "condition_no", "node_id": "choice",
	})
	requireStatus(t, decided, http.StatusOK)
	var afterDecision workflowgorm.WorkflowInstance
	if err := app.db.First(&afterDecision, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if containsAny(decodeData(t, decided).(map[string]any)["selected_node_ids"].([]any), "left") {
		t.Fatalf("unselected condition branch remained visible: %s", decided.Body.String())
	}
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-31"}), http.StatusOK)

	rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "rollback", "node_id": "right", "target_node_id": "choice", "comment": "重新选择",
	})
	requireStatus(t, rolledBack, http.StatusOK)
	rollbackSchedule := decodeData(t, rolledBack).(map[string]any)["subject_schedule"].(map[string]any)
	if rollbackSchedule["state"] != "conflict" {
		t.Fatalf("rollback schedule=%v", rollbackSchedule)
	}
	assertScheduleConflictCodeFromAny(t, rollbackSchedule["conflicts"].([]any), "NODE_AFTER_SUBJECT_END")
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedUnit.PlannedStartAt, "2026-08-01")
	assertScheduleDate(t, storedUnit.PlannedEndAt, "2026-08-10")
	if storedUnit.ActualStartAt != nil || storedUnit.ActualCompletedAt != nil {
		t.Fatalf("rollback invented actual dates: %+v", storedUnit)
	}
	assertInstancePlansJSON(t, app, instanceID, afterDecision.NodePlansJSON)

	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "schedule-owner")
	assigneeOnly := app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "choice", "assignee_user_id": nextOwner.ID,
		}},
	})
	requireStatus(t, assigneeOnly, http.StatusOK)
	assigneeSchedule := decodeData(t, assigneeOnly).(map[string]any)["subject_schedule"].(map[string]any)
	if assigneeSchedule["state"] != "conflict" {
		t.Fatalf("assignee-only response hid existing schedule conflict: %v", assigneeSchedule)
	}
	assertActiveWorkflowTokenAssignee(t, app, workspace.ID, instanceID, "choice", nextOwner.ID)
	var afterAssignee workflowgorm.WorkflowInstance
	if err := app.db.First(&afterAssignee, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

	mixed := app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "choice", "planned_start_at": "2026-09-01", "expected_planned_start_at": "2026-08-02",
			"planned_end_at": "2026-08-03", "expected_planned_end_at": "2026-08-03", "assignee_user_id": uint(1),
		}},
	})
	requireStatus(t, mixed, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, mixed, "NODE_START_AFTER_END")
	assertActiveWorkflowTokenAssignee(t, app, workspace.ID, instanceID, "choice", nextOwner.ID)
	assertInstancePlansJSON(t, app, instanceID, afterAssignee.NodePlansJSON)
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("mixed rejected update wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedUnit.PlannedEndAt, "2026-08-10")
}

func TestWorkflowScheduleSerialOrderIgnoresUntakenRecoveryEdges(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		normalKind string
		recovery   string
	}{
		{name: "reject", sourceType: "approval", normalKind: "approve", recovery: "reject"},
		{name: "failure", sourceType: "ai_check", normalKind: "success", recovery: "failure"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start"}, {ID: "source", Type: test.sourceType},
					{ID: "normal", Type: "human_task"}, {ID: "recovery", Type: "human_task"},
					{ID: "end", Type: "end"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "source"},
					{ID: "e2", Source: "source", Target: "normal", Kind: test.normalKind},
					{ID: "e3", Source: "source", Target: "recovery", Kind: test.recovery},
					{ID: "e4", Source: "normal", Target: "end"},
					{ID: "e5", Source: "recovery", Target: "end"},
				},
			}
			plans := map[string]workflowNodePlan{
				"source":   {PlannedEndAt: scheduleTestTime("2026-08-10")},
				"normal":   {PlannedStartAt: scheduleTestTime("2026-08-11")},
				"recovery": {PlannedStartAt: scheduleTestTime("2026-08-01")},
			}
			displayNodes, displayEdges := workflowDisplayTopology(
				definition, nil, workflowAllEdgeIDs(definition), nil,
			)
			if conflicts := workflowScheduleSerialConflicts(
				definition, scheduleSelectedNodes(displayNodes), displayEdges, plans,
			); len(conflicts) != 0 {
				t.Fatalf("untaken %s recovery affected serial order: %+v", test.recovery, conflicts)
			}
			status := "rejected"
			if test.recovery == "failure" {
				status = "failed"
			}
			run := workflowgorm.WorkflowNodeRun{
				NodeID: "source", NodeType: test.sourceType, Status: status,
				OutputJSON: encodeWorkflowRunTransition(1, "", "recovery"),
			}
			displayNodes, displayEdges = workflowDisplayTopology(
				definition, []workflowgorm.WorkflowNodeRun{run}, workflowAllEdgeIDs(definition), []string{"recovery"},
			)
			conflicts := workflowScheduleSerialConflicts(
				definition, scheduleSelectedNodes(displayNodes), displayEdges, plans,
			)
			assertScheduleConflictCode(t, conflicts, "SERIAL_NODE_ORDER")
		})
	}
}

func scheduleSelectedNodes(nodeIDs []string) map[string]bool {
	selected := make(map[string]bool, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		selected[nodeID] = true
	}
	return selected
}

func assertActiveWorkflowTokenAssignee(
	t *testing.T,
	app *testApp,
	workspaceID, instanceID uint,
	nodeID string,
	want uint,
) {
	t.Helper()
	var token workflowgorm.WorkflowToken
	if err := app.db.Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		workspaceID, instanceID, nodeID, "active",
	).Order("id DESC").First(&token).Error; err != nil {
		t.Fatal(err)
	}
	if token.AssigneeUserID == nil || *token.AssigneeUserID != want {
		t.Fatalf("active token assignee=%v, want %d", token.AssigneeUserID, want)
	}
}

func scheduleString(value string) *string { return &value }

func assertScheduleConflictSubject(
	t *testing.T,
	response *httptest.ResponseRecorder,
	code, subjectType string,
	subjectID uint,
) {
	t.Helper()
	envelope := scheduleErrorEnvelope(t, response)
	if envelope["code"] != apiErrorWorkflowScheduleConflict {
		t.Fatalf("top-level conflict code=%v body=%s", envelope["code"], response.Body.String())
	}
	conflicts := envelope["schedule"].(map[string]any)["conflicts"].([]any)
	for _, raw := range conflicts {
		conflict := raw.(map[string]any)
		if conflict["code"] == code && conflict["subject_type"] == subjectType &&
			conflict["subject_id"] == float64(subjectID) {
			return
		}
	}
	t.Fatalf("conflict %s subject=%s/%d missing: %s", code, subjectType, subjectID, response.Body.String())
}
