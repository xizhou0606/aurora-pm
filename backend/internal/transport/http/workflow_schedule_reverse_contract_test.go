package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestInheritedReverseProjectConstraintConflictsButMetadataAndAssigneeRemainUsable(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	if err := app.db.Model(&project).Updates(map[string]any{
		"planned_start_at": scheduleTestTime("2026-08-31"),
		"planned_end_at":   scheduleTestTime("2026-08-01"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "继承反向项目约束", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{
		"title": "继承反向项目约束",
	})
	instanceID := uint(created["workflow_instance_id"].(float64))
	instancePath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID)
	current := app.request(t, http.MethodGet, instancePath, token, workspace.ID, nil)
	requireStatus(t, current, http.StatusOK)
	schedule := decodeData(t, current).(map[string]any)["subject_schedule"].(map[string]any)
	if schedule["state"] != "conflict" || schedule["subject"].(map[string]any)["planned_start_at"] != nil {
		t.Fatalf("reverse inherited schedule=%v", schedule)
	}
	assertScheduleConflictCodeFromAny(t, schedule["conflicts"].([]any), "PROJECT_START_AFTER_END")

	planPath := instancePath + "/node-plans"
	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "reverse-owner")
	requireStatus(t, app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "assignee_user_id": nextOwner.ID}},
	}), http.StatusOK)
	assertActiveWorkflowTokenAssignee(t, app, workspace.ID, instanceID, "delivery", nextOwner.ID)
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	planned := app.request(t, http.MethodPatch, planPath, token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil}},
	})
	requireStatus(t, planned, http.StatusConflict)
	assertScheduleConflictSubject(t, planned, "PROJECT_START_AFTER_END", "project_set", project.ID)
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)

	unitPath := "/api/v1/platform/project-new/" + uintString(unit.ID)
	requireStatus(t, app.request(t, http.MethodPatch, unitPath, token, workspace.ID,
		map[string]any{"title": "反向项目下仍可改名"}), http.StatusOK)
	datePatch := app.request(t, http.MethodPatch, unitPath, token, workspace.ID,
		map[string]any{"planned_start_at": "2026-08-10", "expected_planned_start_at": nil})
	requireStatus(t, datePatch, http.StatusConflict)
	assertScheduleConflictSubject(t, datePatch, "PROJECT_START_AFTER_END", "project_set", project.ID)
}

func TestRequirementMovesRejectReverseTargetPlanUnit(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "schedule endpoint"
		if legacy {
			name = "legacy patch"
		}
		t.Run(name, func(t *testing.T) {
			app, token, workspace, project, source := newAIWorkflowFixture(t)
			setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
			target := projectgorm.PlanUnit{
				ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
				Title: "历史反向版本", Type: "version",
				PlannedStartAt: scheduleTestTime("2026-08-20"), PlannedEndAt: scheduleTestTime("2026-08-10"),
			}
			if err := app.db.Create(&target).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "拒绝反向版本", scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "拒绝反向版本"})
			requirementID := uint(created["id"].(float64))
			version := uint(created["version"].(float64))
			beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

			path, method := "/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule", http.MethodPost
			if legacy {
				path, method = "/api/v1/platform/product-requirements-new/"+uintString(requirementID), http.MethodPatch
			}
			response := app.request(t, method, path, token, workspace.ID, map[string]any{
				"version_task_new_id": target.ID, "expected_version": version,
			})
			requireStatus(t, response, http.StatusConflict)
			assertScheduleEnvelopeConflict(t, response, "SUBJECT_START_AFTER_END")
			assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
			afterAudits, afterEvents := allScheduleMutationCounts(t, app)
			if afterAudits != beforeAudits || afterEvents != beforeEvents {
				t.Fatalf("reverse target wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
			}
		})
	}
}
