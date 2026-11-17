package httpapi

import (
	"errors"
	"net/http"
	"testing"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestSubjectBoundaryNarrowingReturnsStructuredConflictForUnavailableWorkflow(t *testing.T) {
	for _, subject := range []string{"project", "plan_unit"} {
		t.Run(subject, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			if err := app.db.Model(&project).Updates(map[string]any{
				"planned_start_at": scheduleTestTime("2026-08-01"),
				"planned_end_at":   scheduleTestTime("2026-08-31"),
			}).Error; err != nil {
				t.Fatal(err)
			}
			if subject == "plan_unit" {
				if err := app.db.Model(&unit).Updates(map[string]any{
					"planned_start_at": scheduleTestTime("2026-08-01"),
					"planned_end_at":   scheduleTestTime("2026-08-31"),
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "不可用流程收窄", scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "不可用流程收窄"})
			requirementID := uint(created["id"].(float64))
			instanceID := uint(created["workflow_instance_id"].(float64))
			if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
				Update("node_plans_json", "").Error; err != nil {
				t.Fatal(err)
			}
			beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)
			path := "/api/v1/platform/project-sets/" + uintString(project.ID)
			if subject == "plan_unit" {
				path = "/api/v1/platform/project-new/" + uintString(unit.ID)
			}
			response := app.request(t, http.MethodPatch, path, token, workspace.ID,
				map[string]any{"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31"})
			requireStatus(t, response, http.StatusConflict)
			assertScheduleConflictSubject(t, response, "SUBJECT_SCHEDULE_UNAVAILABLE", "requirement", requirementID)
			afterAudits, afterEvents := allScheduleMutationCounts(t, app)
			if afterAudits != beforeAudits || afterEvents != beforeEvents {
				t.Fatalf("unavailable workflow conflict wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
			}
			if subject == "project" {
				var stored projectgorm.ProjectSet
				if err := app.db.First(&stored, project.ID).Error; err != nil {
					t.Fatal(err)
				}
				assertScheduleDate(t, stored.PlannedEndAt, "2026-08-31")
			} else {
				var stored projectgorm.PlanUnit
				if err := app.db.First(&stored, unit.ID).Error; err != nil {
					t.Fatal(err)
				}
				assertScheduleDate(t, stored.PlannedEndAt, "2026-08-31")
			}
		})
	}
}

func TestRequirementScheduleReturnsStructuredConflictForUnavailableWorkflow(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "目标版本", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
		PlannedEndAt: scheduleTestTime("2026-08-31"),
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "不可用流程换版本", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "不可用流程换版本"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	version := uint(created["version"].(float64))
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
		Update("node_plans_json", "").Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version,
		})
	requireStatus(t, response, http.StatusConflict)
	assertScheduleConflictSubject(t, response, "SUBJECT_SCHEDULE_UNAVAILABLE", "requirement", requirementID)
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("unavailable move wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func TestInvalidRecordedConditionTargetReturnsStructuredScheduleConflict(t *testing.T) {
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
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "无效历史目标", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "无效历史目标"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "condition_yes", "node_id": "choice"}), http.StatusOK)
	if err := app.db.Model(&workflowgorm.WorkflowNodeRun{}).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "choice",
	).Update("output_json", encodeWorkflowRunDecision(1, "", true, "", "missing-node")).Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-30", "expected_planned_end_at": "2026-08-31"})
	requireStatus(t, response, http.StatusConflict)
	assertScheduleConflictSubject(t, response, "SUBJECT_SCHEDULE_UNAVAILABLE", "requirement", requirementID)
	var stored projectgorm.PlanUnit
	if err := app.db.First(&stored, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, stored.PlannedEndAt, "2026-08-31")
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("invalid recorded target wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func TestWorkflowScheduleDatabaseReadFailureRemainsInternalError(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "排期数据库错误", scheduleSingleDefinition())
	createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "排期数据库错误"})
	triggered := false
	callback := "test:fail_schedule_run_database_read"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if triggered || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "WorkflowNodeRun" {
			return
		}
		triggered = true
		tx.AddError(errors.New("forced workflow run database failure"))
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-30", "expected_planned_end_at": "2026-08-31"})
	requireStatus(t, response, http.StatusInternalServerError)
	if !triggered {
		t.Fatal("workflow run database failure callback did not run")
	}
	var stored projectgorm.PlanUnit
	if err := app.db.First(&stored, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, stored.PlannedEndAt, "2026-08-31")
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("database failure wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}
