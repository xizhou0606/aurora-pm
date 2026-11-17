package httpapi

import (
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestWorkflowScheduleInheritsMissingPlanUnitBoundaryFromProject(t *testing.T) {
	tests := []struct {
		name          string
		projectStart  *string
		projectEnd    *string
		field         string
		inside        string
		outside       string
		conflictCode  string
		constraintKey string
	}{
		{
			name: "start", projectStart: scheduleString("2026-08-01"), field: "planned_start_at",
			inside: "2026-08-02", outside: "2026-07-31", conflictCode: "NODE_BEFORE_SUBJECT_START",
			constraintKey: "planned_start_at",
		},
		{
			name: "end", projectEnd: scheduleString("2026-08-31"), field: "planned_end_at",
			inside: "2026-08-30", outside: "2026-09-01", conflictCode: "NODE_AFTER_SUBJECT_END",
			constraintKey: "planned_end_at",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			updates := map[string]any{"planned_start_at": nil, "planned_end_at": nil}
			if test.projectStart != nil {
				updates["planned_start_at"] = scheduleTestTime(*test.projectStart)
			}
			if test.projectEnd != nil {
				updates["planned_end_at"] = scheduleTestTime(*test.projectEnd)
			}
			if err := app.db.Model(&project).Updates(updates).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "继承项目"+test.name, scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "继承项目" + test.name})
			instanceID := uint(created["workflow_instance_id"].(float64))
			path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
			inside := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
				"nodes": []map[string]any{{"node_id": "delivery", test.field: test.inside, "expected_" + test.field: nil}},
			})
			requireStatus(t, inside, http.StatusOK)
			insideSchedule := decodeData(t, inside).(map[string]any)["subject_schedule"].(map[string]any)
			if insideSchedule["state"] != "partial" || insideSchedule["can_confirm"] != false {
				t.Fatalf("inherited partial schedule=%v", insideSchedule)
			}
			if insideSchedule["subject"].(map[string]any)[test.constraintKey] != nil ||
				insideSchedule["constraints"].(map[string]any)[test.constraintKey] == nil {
				t.Fatalf("subject/constraints were blended: %v", insideSchedule)
			}
			var before workflowgorm.WorkflowInstance
			if err := app.db.First(&before, instanceID).Error; err != nil {
				t.Fatal(err)
			}

			outside := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
				"nodes": []map[string]any{{"node_id": "delivery", test.field: test.outside, "expected_" + test.field: test.inside}},
			})
			requireStatus(t, outside, http.StatusConflict)
			assertScheduleConflictSubject(t, outside, test.conflictCode, "project_set", project.ID)
			errorSchedule := scheduleErrorEnvelope(t, outside)["schedule"].(map[string]any)
			if errorSchedule["subject"].(map[string]any)[test.constraintKey] != nil ||
				errorSchedule["constraints"].(map[string]any)[test.constraintKey] == nil {
				t.Fatalf("error subject/constraints were blended: %v", errorSchedule)
			}
			assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
		})
	}
}

func TestUnassignedRequirementUsesProjectConstraintButRemainsUnscheduled(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	if err := app.db.Model(&project).Updates(map[string]any{
		"planned_start_at": scheduleTestTime("2026-08-01"),
		"planned_end_at":   scheduleTestTime("2026-08-31"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "未分配继承项目", scheduleSingleDefinition())
	created := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "未分配继承项目", "workflow_template_id": templateID,
	})
	requireStatus(t, created, http.StatusCreated)
	data := decodeData(t, created).(map[string]any)
	clearRequirementPlanUnit(t, app, uint(data["id"].(float64)))
	instanceID := uint(data["workflow_instance_id"].(float64))
	path := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID) + "/node-plans"
	inside := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil}},
	})
	requireStatus(t, inside, http.StatusOK)
	schedule := decodeData(t, inside).(map[string]any)["subject_schedule"].(map[string]any)
	if schedule["state"] != "unscheduled" || schedule["can_confirm"] != false || schedule["constraints"] == nil {
		t.Fatalf("unassigned inherited schedule=%v", schedule)
	}
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	out := app.request(t, http.MethodPatch, path, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_end_at": "2026-09-01", "expected_planned_end_at": "2026-08-20"}},
	})
	requireStatus(t, out, http.StatusConflict)
	assertScheduleConflictSubject(t, out, "NODE_AFTER_SUBJECT_END", "project_set", project.ID)
	if scheduleErrorEnvelope(t, out)["schedule"].(map[string]any)["state"] != "conflict" {
		t.Fatalf("fatal inherited conflict was not surfaced: %s", out.Body.String())
	}
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
}

func TestProjectSetNarrowingValidatesEmptyPlanUnitsAndUnassignedRequirements(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	if err := app.db.Model(&project).Updates(map[string]any{
		"planned_start_at": scheduleTestTime("2026-08-01"),
		"planned_end_at":   scheduleTestTime("2026-08-31"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "项目收窄继承校验", scheduleSingleDefinition())
	assigned := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "空版本需求"})
	unassignedResponse := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "未分配需求", "workflow_template_id": templateID,
	})
	requireStatus(t, unassignedResponse, http.StatusCreated)
	unassigned := decodeData(t, unassignedResponse).(map[string]any)
	clearRequirementPlanUnit(t, app, uint(unassigned["id"].(float64)))
	instanceIDs := []uint{
		uint(assigned["workflow_instance_id"].(float64)),
		uint(unassigned["workflow_instance_id"].(float64)),
	}
	for _, instanceID := range instanceIDs {
		requireStatus(t, app.request(t, http.MethodPatch,
			"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
			token, workspace.ID, map[string]any{"nodes": []map[string]any{{
				"node_id": "delivery", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
			}}}), http.StatusOK)
	}
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)
	narrowed := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-sets/"+uintString(project.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31"})
	requireStatus(t, narrowed, http.StatusConflict)
	assertScheduleConflictSubject(t, narrowed, "NODE_AFTER_SUBJECT_END", "requirement", uint(assigned["id"].(float64)))
	assertScheduleConflictSubject(t, narrowed, "NODE_AFTER_SUBJECT_END", "requirement", uint(unassigned["id"].(float64)))
	var storedProject projectgorm.ProjectSet
	if err := app.db.First(&storedProject, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedProject.PlannedEndAt, "2026-08-31")
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("rejected project narrowing wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func TestScheduleMetadataUpdatesIgnoreUnchangedHistoricalConflicts(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-10")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "历史冲突元数据更新", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "历史冲突元数据更新"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	corrupt := `{"delivery":{"planned_end_at":"2026-08-20T00:00:00Z"}}`
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
		Update("node_plans_json", corrupt).Error; err != nil {
		t.Fatal(err)
	}
	unitPath := "/api/v1/platform/project-new/" + uintString(unit.ID)
	requireStatus(t, app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"title": "改名后版本", "planned_start_at": "2026-08-01", "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-10",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPatch, unitPath, token, workspace.ID, map[string]any{
		"planned_start_at": "2026-08-02", "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-10",
	}), http.StatusOK)
	projectPath := "/api/v1/platform/project-sets/" + uintString(project.ID)
	requireStatus(t, app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"name": "改名后项目", "planned_start_at": "2026-08-01", "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-10", "expected_planned_end_at": "2026-08-10",
	}), http.StatusOK)
	requireStatus(t, app.request(t, http.MethodPatch, projectPath, token, workspace.ID, map[string]any{
		"planned_end_at": "2026-09-30", "expected_planned_end_at": "2026-08-10",
	}), http.StatusOK)
	assertInstancePlansJSON(t, app, instanceID, corrupt)
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedUnit.Title != "改名后版本" || storedUnit.ActualStartAt != nil || storedUnit.ActualCompletedAt != nil {
		t.Fatalf("metadata update altered independent facts: %+v", storedUnit)
	}
}

func TestRequirementMoveConflictKeepsActualSubjectSeparateFromInheritedConstraints(t *testing.T) {
	app, token, workspace, sourceProject, sourceUnit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &sourceProject, &sourceUnit, "2026-08-01", "2026-09-30")
	targetProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: sourceProject.TeamID, Name: "目标项目",
		OwnerUserID: sourceProject.OwnerUserID, PlannedStartAt: scheduleTestTime("2026-08-01"),
		PlannedEndAt: scheduleTestTime("2026-08-31"),
	}
	targetUnit := projectgorm.PlanUnit{
		ID: 22, WorkspaceID: workspace.ID, TeamID: sourceUnit.TeamID, ProjectSetID: targetProject.ID,
		Title: "目标空结束版本", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
	}
	if err := app.db.Create(&targetProject).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&targetUnit).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "需求迁移继承约束", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, sourceUnit.ID, templateID, map[string]any{"title": "需求迁移继承约束"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	version := uint(created["version"].(float64))
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_end_at": "2026-09-01", "expected_planned_end_at": nil,
		}}}), http.StatusOK)

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(requirementID), token, workspace.ID,
		map[string]any{"version_task_new_id": targetUnit.ID, "expected_version": version})
	requireStatus(t, response, http.StatusConflict)
	assertScheduleConflictSubject(t, response, "NODE_AFTER_SUBJECT_END", "requirement", requirementID)
	schedule := scheduleErrorEnvelope(t, response)["schedule"].(map[string]any)
	if schedule["subject"].(map[string]any)["planned_end_at"] != nil ||
		schedule["constraints"].(map[string]any)["planned_end_at"] == nil {
		t.Fatalf("move error blended actual and inherited boundaries: %v", schedule)
	}
	assertRequirementScheduleFacts(t, app, requirementID, sourceUnit.ID, version)
}

func TestClearingPlanUnitDatesValidatesNewInheritedProjectConstraints(t *testing.T) {
	for _, boundedProject := range []bool{true, false} {
		name := "unbounded project allows clear"
		want := http.StatusOK
		if boundedProject {
			name, want = "bounded project rejects newly inherited conflicts", http.StatusConflict
		}
		t.Run(name, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			projectUpdates := map[string]any{"planned_start_at": nil, "planned_end_at": nil}
			if boundedProject {
				projectUpdates["planned_start_at"] = scheduleTestTime("2026-08-10")
				projectUpdates["planned_end_at"] = scheduleTestTime("2026-08-20")
			}
			if err := app.db.Model(&project).Updates(projectUpdates).Error; err != nil {
				t.Fatal(err)
			}
			if err := app.db.Model(&unit).Updates(map[string]any{
				"planned_start_at": scheduleTestTime("2026-08-01"),
				"planned_end_at":   scheduleTestTime("2026-08-31"),
			}).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, name, scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": name})
			instanceID := uint(created["workflow_instance_id"].(float64))
			requireStatus(t, app.request(t, http.MethodPatch,
				"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
				token, workspace.ID, map[string]any{"nodes": []map[string]any{{
					"node_id": "delivery", "planned_start_at": "2026-08-05", "expected_planned_start_at": nil, "planned_end_at": "2026-08-25", "expected_planned_end_at": nil,
				}}}), http.StatusOK)
			var before workflowgorm.WorkflowInstance
			if err := app.db.First(&before, instanceID).Error; err != nil {
				t.Fatal(err)
			}
			response := app.request(t, http.MethodPatch,
				"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
				map[string]any{"planned_start_at": nil, "expected_planned_start_at": "2026-08-01", "planned_end_at": nil, "expected_planned_end_at": "2026-08-31"})
			requireStatus(t, response, want)
			var stored projectgorm.PlanUnit
			if err := app.db.First(&stored, unit.ID).Error; err != nil {
				t.Fatal(err)
			}
			if boundedProject {
				assertScheduleEnvelopeConflict(t, response, "NODE_BEFORE_SUBJECT_START")
				assertScheduleDate(t, stored.PlannedStartAt, "2026-08-01")
				assertScheduleDate(t, stored.PlannedEndAt, "2026-08-31")
			} else if stored.PlannedStartAt != nil || stored.PlannedEndAt != nil {
				t.Fatalf("unbounded project did not allow clear: %+v", stored)
			}
			assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
		})
	}
}

func allScheduleMutationCounts(t *testing.T, app *testApp) (int64, int64) {
	t.Helper()
	var audits, events int64
	if err := app.db.Model(&auditgorm.AuditLog{}).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&auditgorm.OutboxEvent{}).Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	return audits, events
}
