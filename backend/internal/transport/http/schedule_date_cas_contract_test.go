package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestProjectAndPlanUnitDateCASRejectsMissingAndStaleExpectations(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	projectPath := "/api/v1/platform/project-sets/" + uintString(project.ID)
	unitPath := "/api/v1/platform/project-new/" + uintString(unit.ID)

	projectAuditBeforeMissing, projectOutboxBeforeMissing := mutationCountsForResource(t, app, "project_set", project.ID)
	missing := app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-08-01", "name": "不应保存的缺失期待值项目名",
	})
	requireStatus(t, missing, http.StatusBadRequest)
	var projectAfterMissing projectgorm.ProjectSet
	if err := app.db.First(&projectAfterMissing, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if projectAfterMissing.Name != project.Name || projectAfterMissing.PlannedStartAt != nil {
		t.Fatalf("missing project expectation changed row: %+v", projectAfterMissing)
	}
	assertMutationCountsForResource(
		t, app, "project_set", project.ID, projectAuditBeforeMissing, projectOutboxBeforeMissing,
	)
	expectedOnly := app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"expected_planned_end_at": nil, "name": "不应保存的单独期待值项目名",
	})
	requireStatus(t, expectedOnly, http.StatusBadRequest)
	if err := app.db.First(&projectAfterMissing, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if projectAfterMissing.Name != project.Name || projectAfterMissing.PlannedEndAt != nil {
		t.Fatalf("expected-only project request changed row: %+v", projectAfterMissing)
	}
	assertMutationCountsForResource(
		t, app, "project_set", project.ID, projectAuditBeforeMissing, projectOutboxBeforeMissing,
	)

	projectStart := scheduleTestTime("2026-08-01")
	if err := app.db.Model(&projectgorm.ProjectSet{}).
		Where("id = ? AND workspace_id = ?", project.ID, workspace.ID).
		Update("planned_start_at", projectStart).Error; err != nil {
		t.Fatal(err)
	}
	projectAudit, projectOutbox := mutationCountsForResource(t, app, "project_set", project.ID)
	staleProject := app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"name": "不应保存的项目名", "planned_start_at": "2026-08-01", "expected_planned_start_at": nil,
	})
	assertScheduleStaleResponse(t, staleProject, "planned_start_at", "")
	var storedProject projectgorm.ProjectSet
	if err := app.db.First(&storedProject, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedProject.Name != project.Name || !sameWorkflowNodePlanTime(storedProject.PlannedStartAt, projectStart) {
		t.Fatalf("stale project request changed row: %+v", storedProject)
	}
	assertMutationCountsForResource(t, app, "project_set", project.ID, projectAudit, projectOutbox)

	actualStart := scheduleTestTime("2026-08-02")
	if err := app.db.Model(&projectgorm.PlanUnit{}).
		Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
		Update("actual_start_at", actualStart).Error; err != nil {
		t.Fatal(err)
	}
	unitAudit, unitOutbox := mutationCountsForResource(t, app, "plan_unit", unit.ID)
	missingActualExpected := app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"actual_start_at": "2026-08-03", "title": "不应保存的缺失期待值版本名",
	})
	requireStatus(t, missingActualExpected, http.StatusBadRequest)
	var unitAfterMissing projectgorm.PlanUnit
	if err := app.db.First(&unitAfterMissing, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unitAfterMissing.Title != unit.Title || !sameWorkflowNodePlanTime(unitAfterMissing.ActualStartAt, actualStart) {
		t.Fatalf("missing plan unit expectation changed row: %+v", unitAfterMissing)
	}
	assertMutationCountsForResource(t, app, "plan_unit", unit.ID, unitAudit, unitOutbox)
	staleUnit := app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"title": "不应保存的版本名", "actual_start_at": "2026-08-02", "expected_actual_start_at": nil,
	})
	assertScheduleStaleResponse(t, staleUnit, "actual_start_at", "")
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedUnit.Title != unit.Title || !sameWorkflowNodePlanTime(storedUnit.ActualStartAt, actualStart) {
		t.Fatalf("stale plan unit request changed row: %+v", storedUnit)
	}
	assertMutationCountsForResource(t, app, "plan_unit", unit.ID, unitAudit, unitOutbox)
}

func TestProjectAndPlanUnitClearDateResponsesUsePersistedNulls(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")

	projectResponse := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID, map[string]any{
			"planned_start_at": nil, "expected_planned_start_at": "2026-08-01",
			"planned_end_at": nil, "expected_planned_end_at": "2026-08-31",
		})
	requireStatus(t, projectResponse, http.StatusOK)
	projectData := decodeData(t, projectResponse).(map[string]any)
	if projectData["planned_start_at"] != nil || projectData["planned_end_at"] != nil {
		t.Fatalf("project clear response retained stale dates: %v", projectData)
	}

	unitResponse := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID, map[string]any{
			"planned_start_at": nil, "expected_planned_start_at": "2026-08-01",
			"planned_end_at": nil, "expected_planned_end_at": "2026-08-31",
		})
	requireStatus(t, unitResponse, http.StatusOK)
	unitData := decodeData(t, unitResponse).(map[string]any)
	if unitData["planned_start_at"] != nil || unitData["planned_end_at"] != nil {
		t.Fatalf("plan unit clear response retained stale dates: %v", unitData)
	}
	var storedProject projectgorm.ProjectSet
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedProject, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedProject.PlannedStartAt != nil || storedProject.PlannedEndAt != nil ||
		storedUnit.PlannedStartAt != nil || storedUnit.PlannedEndAt != nil {
		t.Fatalf("cleared dates did not persist as null: project=%+v unit=%+v", storedProject, storedUnit)
	}
}

func TestPlanUnitAllDateFieldsRequireExpectedAndRejectStale(t *testing.T) {
	fields := []string{
		"planned_start_at", "planned_end_at", "design_freeze_due_at",
		"development_complete_due_at", "test_acceptance_due_at",
		"actual_start_at", "actual_completed_at",
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			app, token, workspace, _, unit := newAIWorkflowFixture(t)
			current := scheduleTestTime("2026-08-01")
			if err := app.db.Model(&projectgorm.PlanUnit{}).
				Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
				Update(field, current).Error; err != nil {
				t.Fatal(err)
			}
			path := "/api/v1/platform/project-new/" + uintString(unit.ID)
			beforeAudit, beforeOutbox := mutationCountsForResource(t, app, "plan_unit", unit.ID)
			missing := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
				field: "2026-08-02", "title": "不应保存的缺失期待值标题",
			})
			requireStatus(t, missing, http.StatusBadRequest)
			var afterMissing projectgorm.PlanUnit
			if err := app.db.First(&afterMissing, unit.ID).Error; err != nil {
				t.Fatal(err)
			}
			if afterMissing.Title != unit.Title || !sameWorkflowNodePlanTime(planUnitDateValue(afterMissing, field), current) {
				t.Fatalf("field %s missing expectation changed row: %+v", field, afterMissing)
			}
			assertMutationCountsForResource(t, app, "plan_unit", unit.ID, beforeAudit, beforeOutbox)
			stale := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
				field: "2026-08-01", "expected_" + field: nil, "title": "不应保存的陈旧标题",
			})
			assertScheduleStaleResponse(t, stale, field, "")
			var stored projectgorm.PlanUnit
			if err := app.db.First(&stored, unit.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Title != unit.Title || !sameWorkflowNodePlanTime(planUnitDateValue(stored, field), current) {
				t.Fatalf("field %s stale request changed row: %+v", field, stored)
			}
			assertMutationCountsForResource(t, app, "plan_unit", unit.ID, beforeAudit, beforeOutbox)
		})
	}
}

func TestWorkflowNodePlanDateCASMergesOtherFieldAndRollsBackMixedStaleWrite(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "节点日期 CAS", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "节点日期 CAS"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
		Update("node_plans_json", `{"delivery":{"planned_end_at":"2026-08-20T00:00:00Z"}}`).Error; err != nil {
		t.Fatal(err)
	}
	beforeMissingJSON := workflowNodePlansJSON(t, app, instanceID)
	beforeMissingAudit, beforeMissingOutbox := scheduleMutationCounts(t, app, instanceID)
	missing := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_start_at": "2026-08-10"}},
	})
	requireStatus(t, missing, http.StatusBadRequest)
	if got := workflowNodePlansJSON(t, app, instanceID); got != beforeMissingJSON {
		t.Fatalf("missing node expectation changed plans: got=%s want=%s", got, beforeMissingJSON)
	}
	afterMissingAudit, afterMissingOutbox := scheduleMutationCounts(t, app, instanceID)
	if afterMissingAudit != beforeMissingAudit || afterMissingOutbox != beforeMissingOutbox {
		t.Fatalf(
			"missing node expectation emitted side effects: audit=%d/%d outbox=%d/%d",
			afterMissingAudit, beforeMissingAudit, afterMissingOutbox, beforeMissingOutbox,
		)
	}

	callbackName := "test:node_plan_other_date_commits_after_preflight"
	fired := false
	if err := app.db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if fired || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "WorkflowInstance" {
			return
		}
		row, ok := tx.Statement.Dest.(*workflowgorm.WorkflowInstance)
		if !ok || row.ID != instanceID {
			return
		}
		fired = true
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Model(&workflowgorm.WorkflowInstance{}).
			Where("id = ? AND workspace_id = ?", instanceID, workspace.ID).
			Update("node_plans_json", `{"delivery":{"planned_end_at":"2026-08-21T00:00:00Z"}}`).Error)
	}); err != nil {
		t.Fatal(err)
	}
	merged := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil,
		}},
	})
	_ = app.db.Callback().Query().Remove(callbackName)
	requireStatus(t, merged, http.StatusOK)
	if !fired {
		t.Fatal("concurrent node-plan callback did not fire")
	}
	plans := storedWorkflowNodePlans(t, app, instanceID)
	assertScheduleDate(t, plans["delivery"].PlannedStartAt, "2026-08-10")
	assertScheduleDate(t, plans["delivery"].PlannedEndAt, "2026-08-21")

	nextOwner := app.createUser(t, 991, "node-plan-cas-owner", "member", "")
	if err := app.db.Create(&identitygorm.WorkspaceMember{
		WorkspaceID: workspace.ID, UserID: nextOwner.ID, Role: "member", Enabled: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	var tokenBefore workflowgorm.WorkflowToken
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?",
		workspace.ID, instanceID, "delivery", "active").First(&tokenBefore).Error; err != nil {
		t.Fatal(err)
	}
	beforeJSON := workflowNodePlansJSON(t, app, instanceID)
	beforeAudit, beforeOutbox := scheduleMutationCounts(t, app, instanceID)
	stale := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{
			"node_id": "delivery", "planned_end_at": "2026-08-22",
			"expected_planned_end_at": "2026-08-20", "assignee_user_id": nextOwner.ID,
		}},
	})
	assertScheduleStaleResponse(t, stale, "planned_end_at", "delivery")
	if got := workflowNodePlansJSON(t, app, instanceID); got != beforeJSON {
		t.Fatalf("stale mixed request changed node plans: got=%s want=%s", got, beforeJSON)
	}
	var tokenAfter workflowgorm.WorkflowToken
	if err := app.db.First(&tokenAfter, tokenBefore.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !sameOptionalUint(tokenBefore.AssigneeUserID, tokenAfter.AssigneeUserID) {
		t.Fatalf("stale mixed request changed active token: before=%v after=%v", tokenBefore.AssigneeUserID, tokenAfter.AssigneeUserID)
	}
	afterAudit, afterOutbox := scheduleMutationCounts(t, app, instanceID)
	if afterAudit != beforeAudit || afterOutbox != beforeOutbox {
		t.Fatalf("stale mixed request emitted side effects: audit=%d/%d outbox=%d/%d", afterAudit, beforeAudit, afterOutbox, beforeOutbox)
	}
}

func assertScheduleStaleResponse(t *testing.T, response *httptest.ResponseRecorder, field, nodeID string) {
	t.Helper()
	requireStatus(t, response, http.StatusConflict)
	decodeScheduleStaleResponse(t, response.Body.Bytes(), field, nodeID)
}

func storedWorkflowNodePlans(t *testing.T, app *testApp, instanceID uint) map[string]workflowNodePlan {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		t.Fatal(err)
	}
	return plans
}

func workflowNodePlansJSON(t *testing.T, app *testApp, instanceID uint) string {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	return instance.NodePlansJSON
}

func sameOptionalUint(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func planUnitDateValue(unit projectgorm.PlanUnit, field string) *time.Time {
	switch field {
	case "planned_start_at":
		return unit.PlannedStartAt
	case "planned_end_at":
		return unit.PlannedEndAt
	case "design_freeze_due_at":
		return unit.DesignFreezeDueAt
	case "development_complete_due_at":
		return unit.DevelopmentCompleteAt
	case "test_acceptance_due_at":
		return unit.TestAcceptanceDueAt
	case "actual_start_at":
		return unit.ActualStartAt
	case "actual_completed_at":
		return unit.ActualCompletedAt
	default:
		return nil
	}
}

func mutationCountsForResource(t *testing.T, app *testApp, resource string, id uint) (int64, int64) {
	t.Helper()
	var audits, outbox int64
	if err := app.db.Model(&auditgorm.AuditLog{}).
		Where("resource = ? AND resource_id = ?", resource, id).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&auditgorm.OutboxEvent{}).
		Where("aggregate_type = ? AND aggregate_id = ?", resource, id).Count(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	return audits, outbox
}

func assertMutationCountsForResource(
	t *testing.T,
	app *testApp,
	resource string,
	id uint,
	wantAudits, wantOutbox int64,
) {
	t.Helper()
	audits, outbox := mutationCountsForResource(t, app, resource, id)
	if audits != wantAudits || outbox != wantOutbox {
		t.Fatalf("mutation side effects for %s/%d audits=%d/%d outbox=%d/%d", resource, id, audits, wantAudits, outbox, wantOutbox)
	}
}

func decodeScheduleStaleResponse(t *testing.T, raw []byte, field, nodeID string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["code"] != apiErrorScheduleSubjectStale {
		t.Fatalf("top-level stale code=%v body=%s", envelope["code"], string(raw))
	}
	schedule := envelope["schedule"].(map[string]any)
	conflicts := schedule["conflicts"].([]any)
	for _, rawConflict := range conflicts {
		conflict := rawConflict.(map[string]any)
		if conflict["code"] == apiErrorScheduleSubjectStale && conflict["field"] == field &&
			(nodeID == "" || conflict["node_id"] == nodeID) {
			return
		}
	}
	t.Fatalf("stale conflict missing field=%s node=%s: %v", field, nodeID, conflicts)
}
