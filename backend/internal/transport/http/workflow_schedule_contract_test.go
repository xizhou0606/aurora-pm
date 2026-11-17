package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestWorkflowScheduleStatesAndParallelOrdering(t *testing.T) {
	start, end := scheduleTestTime("2026-08-01"), scheduleTestTime("2026-08-31")
	emptyDefinition := workflowDefinition{
		Nodes: []workflowNode{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
		Edges: []workflowEdge{{ID: "e1", Source: "start", Target: "end"}},
	}
	selected := map[string]bool{"start": true, "end": true}
	scheduled := workflowScheduleForParts(
		emptyDefinition, selected, []string{"e1"}, map[string]workflowNodePlan{},
		workflowScheduleScopeForBoundary(workflowScheduleBoundary{PlannedStartAt: start, PlannedEndAt: end}, true),
	)
	if scheduled.State != "scheduled" || scheduled.Suggested != nil || scheduled.CanConfirm {
		t.Fatalf("zero-business scheduled state=%+v", scheduled)
	}
	unscheduled := workflowScheduleForParts(
		emptyDefinition, selected, []string{"e1"}, map[string]workflowNodePlan{},
		workflowScheduleScopeForBoundary(workflowScheduleBoundary{}, true),
	)
	if unscheduled.State != "unscheduled" || unscheduled.Suggested != nil || unscheduled.CanConfirm {
		t.Fatalf("zero-business unscheduled state=%+v", unscheduled)
	}

	parallel := scheduleParallelDefinition()
	allSelected := map[string]bool{}
	edges := make([]string, 0, len(parallel.Edges))
	for _, node := range parallel.Nodes {
		allSelected[node.ID] = true
	}
	for _, edge := range parallel.Edges {
		edges = append(edges, edge.ID)
	}
	plans := map[string]workflowNodePlan{
		"left":  {PlannedStartAt: scheduleTestTime("2026-08-10"), PlannedEndAt: scheduleTestTime("2026-08-15")},
		"right": {PlannedStartAt: scheduleTestTime("2026-08-11"), PlannedEndAt: scheduleTestTime("2026-08-14")},
		"after": {PlannedStartAt: scheduleTestTime("2026-08-16"), PlannedEndAt: scheduleTestTime("2026-08-18")},
	}
	valid := workflowScheduleForParts(
		parallel, allSelected, edges, plans,
		workflowScheduleScopeForBoundary(workflowScheduleBoundary{PlannedStartAt: start, PlannedEndAt: end}, true),
	)
	if valid.State != "scheduled" || len(valid.Conflicts) != 0 {
		t.Fatalf("parallel siblings were treated as serial: %+v", valid)
	}
	plans["after"] = workflowNodePlan{
		PlannedStartAt: scheduleTestTime("2026-08-14"), PlannedEndAt: scheduleTestTime("2026-08-18"),
	}
	invalid := workflowScheduleForParts(
		parallel, allSelected, edges, plans,
		workflowScheduleScopeForBoundary(workflowScheduleBoundary{PlannedStartAt: start, PlannedEndAt: end}, true),
	)
	assertScheduleConflictCode(t, invalid.Conflicts, "SERIAL_NODE_ORDER")
}

func TestWorkflowNodePlanConfirmationIsAtomic(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := scheduleSerialDefinition()
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "排期确认流程", definition)
	requirement := createUnassignedRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "排期确认"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	body := map[string]any{"nodes": []map[string]any{
		{"node_id": "design", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-12", "expected_planned_end_at": nil},
		{"node_id": "delivery", "planned_start_at": "2026-08-13", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil},
	}}
	beforeAudit, beforeOutbox := scheduleMutationCounts(t, app, instanceID)

	unconfirmed := app.request(t, http.MethodPatch, path, token, workspace.ID, body)
	requireStatus(t, unconfirmed, http.StatusConflict)
	envelope := scheduleErrorEnvelope(t, unconfirmed)
	if envelope["code"] != apiErrorWorkflowScheduleConfirmation {
		t.Fatalf("confirmation code=%v body=%s", envelope["code"], unconfirmed.Body.String())
	}
	schedule := envelope["schedule"].(map[string]any)
	if schedule["can_confirm"] != true || schedule["state"] != "partial" {
		t.Fatalf("confirmation schedule=%v", schedule)
	}
	assertWorkflowScheduleUnchanged(t, app, instanceID, unit.ID, "{}", beforeAudit, beforeOutbox)

	body["confirm_subject_schedule"] = true
	confirmed := app.request(t, http.MethodPatch, path, token, workspace.ID, body)
	requireStatus(t, confirmed, http.StatusOK)
	assignWorkflowStarterToDefinitionNodes(t, app, token, workspace.ID, instanceID)
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedUnit.PlannedStartAt, "2026-08-10")
	assertScheduleDate(t, storedUnit.PlannedEndAt, "2026-08-20")
	data := decodeData(t, confirmed).(map[string]any)
	responseSchedule := data["subject_schedule"].(map[string]any)
	if responseSchedule["state"] != "scheduled" || responseSchedule["actual_dates_independent"] != true {
		t.Fatalf("confirmed schedule=%v", responseSchedule)
	}
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "design",
	}), http.StatusOK)
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "delivery",
	}), http.StatusOK)
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedUnit.ActualStartAt != nil || storedUnit.ActualCompletedAt != nil {
		t.Fatalf("workflow completion rewrote independent actual dates: %+v", storedUnit)
	}
}

func TestWorkflowNodePlanConfirmationOnlyRestoresClearedDates(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "确认专用请求", scheduleSingleDefinition())
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "确认专用请求"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"confirm_subject_schedule": true,
		"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
		}},
	}), http.StatusOK)
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}

	cleared := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_start_at": nil, "expected_planned_start_at": "2026-08-10", "planned_end_at": nil, "expected_planned_end_at": "2026-08-20",
	})
	requireStatus(t, cleared, http.StatusOK)
	var clearedUnit projectgorm.PlanUnit
	if err := app.db.First(&clearedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if clearedUnit.PlannedStartAt != nil || clearedUnit.PlannedEndAt != nil {
		t.Fatalf("plan unit dates were not cleared: %+v", clearedUnit)
	}

	restored := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []any{}, "confirm_subject_schedule": true,
	})
	requireStatus(t, restored, http.StatusOK)
	var after workflowgorm.WorkflowInstance
	if err := app.db.First(&after, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if after.NodePlansJSON != before.NodePlansJSON {
		t.Fatalf("confirmation-only CAS instance before=%+v after=%+v", before, after)
	}
	if err := app.db.First(&clearedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, clearedUnit.PlannedStartAt, "2026-08-10")
	assertScheduleDate(t, clearedUnit.PlannedEndAt, "2026-08-20")
	repeatAudits, repeatEvents := scheduleMutationCounts(t, app, instanceID)
	repeated := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []any{}, "confirm_subject_schedule": true,
	})
	requireStatus(t, repeated, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, repeated, "SCHEDULE_CONFIRMATION_NOT_AVAILABLE")
	afterAudits, afterEvents := scheduleMutationCounts(t, app, instanceID)
	if afterAudits != repeatAudits || afterEvents != repeatEvents {
		t.Fatalf("repeated confirmation wrote side effects audits=%d/%d events=%d/%d", afterAudits, repeatAudits, afterEvents, repeatEvents)
	}

	requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []any{},
	}), http.StatusBadRequest)

	requireStatus(t, app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_start_at": nil, "expected_planned_start_at": "2026-08-10", "planned_end_at": nil, "expected_planned_end_at": "2026-08-20",
	}), http.StatusOK)
	fired := false
	callback := "test:change_instance_before_confirmation_lock"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if fired || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "WorkflowInstance" {
			return
		}
		if _, locked := tx.Statement.Clauses["FOR"]; !locked {
			return
		}
		fired = true
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Model(&workflowgorm.WorkflowInstance{}).
			Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
			Update("status", "completed").Error)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })
	stale := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []any{}, "confirm_subject_schedule": true,
	})
	requireStatus(t, stale, http.StatusConflict)
	if !fired {
		t.Fatal("confirmation CAS mismatch callback did not fire")
	}
	var staleUnit projectgorm.PlanUnit
	if err := app.db.First(&staleUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if staleUnit.PlannedStartAt != nil || staleUnit.PlannedEndAt != nil {
		t.Fatalf("stale confirmation partially restored plan unit: %+v", staleUnit)
	}
}

func TestWorkflowScheduleBoundsSerialAndParallelContract(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	definition := scheduleParallelDefinitionMap()
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "并行排期", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "并行排期"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	valid := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{
		{"node_id": "left", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-15", "expected_planned_end_at": nil},
		{"node_id": "right", "planned_start_at": "2026-08-11", "expected_planned_start_at": nil, "planned_end_at": "2026-08-14", "expected_planned_end_at": nil},
		{"node_id": "after", "planned_start_at": "2026-08-16", "expected_planned_start_at": nil, "planned_end_at": "2026-08-18", "expected_planned_end_at": nil},
	}})
	requireStatus(t, valid, http.StatusOK)
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}

	serialConflict := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{
		{"node_id": "after", "planned_start_at": "2026-08-14", "expected_planned_start_at": "2026-08-16"},
	}})
	requireStatus(t, serialConflict, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, serialConflict, "SERIAL_NODE_ORDER")
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)

	boundaryConflict := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{
		{"node_id": "left", "planned_start_at": "2026-07-31", "expected_planned_start_at": "2026-08-10"},
	}})
	requireStatus(t, boundaryConflict, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, boundaryConflict, "NODE_BEFORE_SUBJECT_START")
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)

	reversed := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{
		{"node_id": "right", "planned_start_at": "2026-08-20", "expected_planned_start_at": "2026-08-11", "planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-14"},
	}})
	requireStatus(t, reversed, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, reversed, "NODE_START_AFTER_END")
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
}

func TestPlanUnitAndProjectSetDateMutationsFailClosed(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "周期收窄", scheduleSingleDefinition())
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "周期收窄"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{{
		"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
	}}}), http.StatusOK)
	reversedPlan := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_start_at": "2026-09-01", "expected_planned_start_at": "2026-08-01",
	})
	requireStatus(t, reversedPlan, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, reversedPlan, "PLAN_UNIT_START_AFTER_END")
	reversedProject := app.request(t, http.MethodPatch, "/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID, map[string]any{
		"planned_start_at": "2026-09-01", "expected_planned_start_at": "2026-08-01",
	})
	requireStatus(t, reversedProject, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, reversedProject, "PROJECT_START_AFTER_END")

	narrowPlan := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31",
	})
	requireStatus(t, narrowPlan, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, narrowPlan, "NODE_AFTER_SUBJECT_END")
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedUnit.PlannedEndAt, "2026-08-31")

	narrowProject := app.request(t, http.MethodPatch, "/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID, map[string]any{
		"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31",
	})
	requireStatus(t, narrowProject, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, narrowProject, "PLAN_UNIT_AFTER_PROJECT_END")
	var storedProject projectgorm.ProjectSet
	if err := app.db.First(&storedProject, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedProject.PlannedEndAt, "2026-08-31")
	corruptPlans := `{"delivery":{"planned_start_at":"2026-08-20T00:00:00Z","planned_end_at":"2026-08-10T00:00:00Z"}}`
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
		Update("node_plans_json", corruptPlans).Error; err != nil {
		t.Fatal(err)
	}

	cleared := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_start_at": nil, "expected_planned_start_at": "2026-08-01", "planned_end_at": nil, "expected_planned_end_at": "2026-08-31",
	})
	requireStatus(t, cleared, http.StatusOK)
	assertInstancePlansJSON(t, app, instanceID, corruptPlans)

	outside := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
		"planned_start_at": "2026-07-31", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
	})
	requireStatus(t, outside, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, outside, "PLAN_UNIT_BEFORE_PROJECT_START")
}

func TestUnscheduledRequirementCanPlanNodesAndRequiresScheduleConfirmation(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "未排版本需求", scheduleSingleDefinition())
	created := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "未排版本需求", "workflow_template_id": templateID,
	})
	requireStatus(t, created, http.StatusCreated)
	requirement := decodeData(t, created).(map[string]any)
	requirementID := uint(requirement["id"].(float64))
	clearRequirementPlanUnit(t, app, requirementID)
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	version := uint(requirement["version"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	planned := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{"nodes": []map[string]any{{
		"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
	}}})
	requireStatus(t, planned, http.StatusOK)
	schedule := decodeData(t, planned).(map[string]any)["subject_schedule"].(map[string]any)
	if schedule["state"] != "unscheduled" || schedule["can_confirm"] != false {
		t.Fatalf("unassigned requirement schedule=%v", schedule)
	}
	assertScheduleConflictCodeFromAny(t, schedule["conflicts"].([]any), "SUBJECT_PLAN_UNIT_REQUIRED")

	schedulePath := "/api/v1/platform/requirements/" + uintString(requirementID) + "/schedule"
	unconfirmed := app.request(t, http.MethodPost, schedulePath, token, workspace.ID, map[string]any{
		"version_task_new_id": unit.ID, "expected_version": version,
	})
	requireStatus(t, unconfirmed, http.StatusConflict)
	if scheduleErrorEnvelope(t, unconfirmed)["code"] != apiErrorWorkflowScheduleConfirmation {
		t.Fatalf("schedule confirmation body=%s", unconfirmed.Body.String())
	}
	assertRequirementPlanUnit(t, app, requirementID, 0)

	confirmed := app.request(t, http.MethodPost, schedulePath, token, workspace.ID, map[string]any{
		"version_task_new_id": unit.ID, "expected_version": version, "confirm_subject_schedule": true,
	})
	requireStatus(t, confirmed, http.StatusOK)
	assertRequirementPlanUnit(t, app, requirementID, unit.ID)
	var scheduledUnit projectgorm.PlanUnit
	if err := app.db.First(&scheduledUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, scheduledUnit.PlannedStartAt, "2026-08-10")
	assertScheduleDate(t, scheduledUnit.PlannedEndAt, "2026-08-20")

	foreign := projectgorm.PlanUnit{ID: 99, WorkspaceID: 2, TeamID: 20, ProjectSetID: 98, Title: "foreign"}
	if err := app.db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	isolation := app.request(t, http.MethodPost, schedulePath, token, workspace.ID, map[string]any{
		"version_task_new_id": foreign.ID, "expected_version": version + 1,
	})
	assertScheduleStaleResponse(t, isolation, "version_task_new_id", "")
}

func TestLegacyRequirementMoveEnforcesTargetScheduleAndConfirmation(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "V2", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
		PlannedEndAt: scheduleTestTime("2026-08-05"),
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "legacy 移动排期", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "legacy 移动排期"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	version := uint(created["version"].(float64))
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
		}}}), http.StatusOK)
	movePath := "/api/v1/platform/product-requirements-new/" + uintString(requirementID)

	outOfBounds := app.request(t, http.MethodPatch, movePath, token, workspace.ID, map[string]any{
		"version_task_new_id": target.ID, "expected_version": version,
	})
	requireStatus(t, outOfBounds, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, outOfBounds, "NODE_AFTER_SUBJECT_END")
	assertRequirementPlanUnit(t, app, requirementID, source.ID)
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", target.ID).
		Updates(map[string]any{"planned_start_at": nil, "planned_end_at": nil}).Error; err != nil {
		t.Fatal(err)
	}

	unconfirmed := app.request(t, http.MethodPatch, movePath, token, workspace.ID, map[string]any{
		"version_task_new_id": target.ID, "expected_version": version,
	})
	requireStatus(t, unconfirmed, http.StatusConflict)
	if scheduleErrorEnvelope(t, unconfirmed)["code"] != apiErrorWorkflowScheduleConfirmation {
		t.Fatalf("legacy confirmation body=%s", unconfirmed.Body.String())
	}
	assertRequirementPlanUnit(t, app, requirementID, source.ID)

	confirmed := app.request(t, http.MethodPatch, movePath, token, workspace.ID, map[string]any{
		"version_task_new_id": target.ID, "expected_version": version, "confirm_subject_schedule": true,
	})
	requireStatus(t, confirmed, http.StatusOK)
	assertRequirementPlanUnit(t, app, requirementID, target.ID)
	if err := app.db.First(&target, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, target.PlannedStartAt, "2026-08-10")
	assertScheduleDate(t, target.PlannedEndAt, "2026-08-20")
}

func TestWorkflowScheduleConfirmationRollsBackOnPlanUnitWriteFailure(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "排期事务回滚", scheduleSingleDefinition())
	requirement := createUnassignedRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "排期事务回滚"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	beforeAudit, beforeOutbox := scheduleMutationCounts(t, app, instanceID)
	callback := "test:fail_schedule_plan_unit_update"
	if err := app.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*projectgorm.PlanUnit); ok {
			tx.AddError(errors.New("forced schedule update failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Update().Remove(callback) })

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{
			"confirm_subject_schedule": true,
			"nodes": []map[string]any{{
				"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
			}},
		})
	requireStatus(t, response, http.StatusInternalServerError)
	assertWorkflowScheduleUnchanged(t, app, instanceID, unit.ID, "{}", beforeAudit, beforeOutbox)
}

func scheduleSerialDefinition() map[string]any {
	return map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "design", "type": "human_task", "name": "设计"},
			{"id": "delivery", "type": "human_task", "name": "交付"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "design"},
			{"id": "e2", "source": "design", "target": "delivery"},
			{"id": "e3", "source": "delivery", "target": "end"},
		},
	}
}

func scheduleSingleDefinition() map[string]any {
	return map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "delivery", "type": "human_task", "name": "交付"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "delivery"},
			{"id": "e2", "source": "delivery", "target": "end"},
		},
	}
}

func scheduleParallelDefinitionMap() map[string]any {
	return map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "split", "type": "parallel_split", "name": "并行"},
			{"id": "left", "type": "human_task", "name": "左分支"},
			{"id": "right", "type": "human_task", "name": "右分支"},
			{"id": "join", "type": "parallel_join", "name": "汇聚"},
			{"id": "after", "type": "human_task", "name": "发布"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "split"},
			{"id": "e2", "source": "split", "target": "left"},
			{"id": "e3", "source": "split", "target": "right"},
			{"id": "e4", "source": "left", "target": "join"},
			{"id": "e5", "source": "right", "target": "join"},
			{"id": "e6", "source": "join", "target": "after"},
			{"id": "e7", "source": "after", "target": "end"},
		},
	}
}

func scheduleParallelDefinition() workflowDefinition {
	raw, _ := json.Marshal(scheduleParallelDefinitionMap())
	definition, issues := compileWorkflowDefinition(string(raw))
	if len(issues) > 0 {
		panic(issues)
	}
	return definition
}

func scheduleTestTime(raw string) *time.Time {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		panic(err)
	}
	return &parsed
}

func setScheduleBoundaries(
	t *testing.T,
	app *testApp,
	project *projectgorm.ProjectSet,
	unit *projectgorm.PlanUnit,
	start, end string,
) {
	t.Helper()
	project.PlannedStartAt, project.PlannedEndAt = scheduleTestTime(start), scheduleTestTime(end)
	unit.PlannedStartAt, unit.PlannedEndAt = scheduleTestTime(start), scheduleTestTime(end)
	if err := app.db.Model(project).Updates(map[string]any{
		"planned_start_at": project.PlannedStartAt, "planned_end_at": project.PlannedEndAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(unit).Updates(map[string]any{
		"planned_start_at": unit.PlannedStartAt, "planned_end_at": unit.PlannedEndAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func scheduleErrorEnvelope(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func assertScheduleEnvelopeConflict(t *testing.T, response *httptest.ResponseRecorder, code string) {
	t.Helper()
	envelope := scheduleErrorEnvelope(t, response)
	if envelope["code"] != apiErrorWorkflowScheduleConflict {
		t.Fatalf("top-level conflict code=%v body=%s", envelope["code"], response.Body.String())
	}
	schedule, ok := envelope["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("schedule missing: %s", response.Body.String())
	}
	assertScheduleConflictCodeFromAny(t, schedule["conflicts"].([]any), code)
}

func assertScheduleConflictCodeFromAny(t *testing.T, conflicts []any, code string) {
	t.Helper()
	for _, raw := range conflicts {
		if raw.(map[string]any)["code"] == code {
			return
		}
	}
	t.Fatalf("conflict code %q missing from %v", code, conflicts)
}

func assertScheduleConflictCode(t *testing.T, conflicts []workflowScheduleConflictItem, code string) {
	t.Helper()
	for _, conflict := range conflicts {
		if conflict.Code == code {
			return
		}
	}
	t.Fatalf("conflict code %q missing from %+v", code, conflicts)
}

func scheduleMutationCounts(t *testing.T, app *testApp, instanceID uint) (int64, int64) {
	t.Helper()
	var audits, events int64
	if err := app.db.Model(&auditgorm.AuditLog{}).
		Where("resource = ? AND resource_id = ?", "workflow_instance", instanceID).
		Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&auditgorm.OutboxEvent{}).
		Where("aggregate_type = ? AND aggregate_id = ?", "workflow_instance", instanceID).
		Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	return audits, events
}

func assertWorkflowScheduleUnchanged(
	t *testing.T,
	app *testApp,
	instanceID, planUnitID uint,
	wantPlans string,
	wantAudits, wantEvents int64,
) {
	t.Helper()
	assertInstancePlansJSON(t, app, instanceID, wantPlans)
	var unit projectgorm.PlanUnit
	if err := app.db.First(&unit, planUnitID).Error; err != nil {
		t.Fatal(err)
	}
	if unit.PlannedStartAt != nil || unit.PlannedEndAt != nil {
		t.Fatalf("plan unit was partially scheduled: %+v", unit)
	}
	audits, events := scheduleMutationCounts(t, app, instanceID)
	if audits != wantAudits || events != wantEvents {
		t.Fatalf("schedule failure side effects audits=%d/%d events=%d/%d", audits, wantAudits, events, wantEvents)
	}
}

func assertInstancePlansJSON(t *testing.T, app *testApp, instanceID uint, want string) {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if instance.NodePlansJSON != want {
		t.Fatalf("instance plans=%q want=%q", instance.NodePlansJSON, want)
	}
}

func assertInstancePlansJSONNotEmpty(t *testing.T, app *testApp, instanceID uint) {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if instance.NodePlansJSON == "{}" || instance.NodePlansJSON == "" {
		t.Fatalf("node plans were erased: %q", instance.NodePlansJSON)
	}
}

func assertRequirementPlanUnit(t *testing.T, app *testApp, requirementID, wantPlanUnitID uint) {
	t.Helper()
	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.PlanUnitID != wantPlanUnitID {
		t.Fatalf("requirement plan unit=%d want=%d", requirement.PlanUnitID, wantPlanUnitID)
	}
}

func assertScheduleDate(t *testing.T, value *time.Time, want string) {
	t.Helper()
	if value == nil || value.UTC().Format("2006-01-02") != want {
		t.Fatalf("schedule date=%v want=%s", value, want)
	}
}
