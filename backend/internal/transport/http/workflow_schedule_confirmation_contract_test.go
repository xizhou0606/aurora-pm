package httpapi

import (
	"errors"
	"net/http"
	"testing"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestRequirementScheduleExplicitConfirmationFailsWithoutSuggestion(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "无建议显式确认", scheduleSingleDefinition())
	createdResponse := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "无建议显式确认", "workflow_template_id": templateID,
	})
	requireStatus(t, createdResponse, http.StatusCreated)
	created := decodeData(t, createdResponse).(map[string]any)
	requirementID := uint(created["id"].(float64))
	clearRequirementPlanUnit(t, app, requirementID)
	version := uint(created["version"].(float64))
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": unit.ID, "expected_version": version, "confirm_subject_schedule": true,
		})
	requireStatus(t, response, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, response, "SCHEDULE_CONFIRMATION_NOT_AVAILABLE")
	assertRequirementScheduleFacts(t, app, requirementID, 0, version)
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("invalid explicit confirmation wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func TestLegacyRequirementMoveExplicitConfirmationFailsWithoutSuggestion(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "V2", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
		PlannedEndAt: scheduleTestTime("2026-08-31"),
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "legacy 无建议确认", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "legacy 无建议确认"})
	requirementID := uint(created["id"].(float64))
	version := uint(created["version"].(float64))
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(requirementID),
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version, "confirm_subject_schedule": true,
		})
	requireStatus(t, response, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, response, "SCHEDULE_CONFIRMATION_NOT_AVAILABLE")
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("invalid legacy confirmation wrote side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func TestRequirementMovesIgnoreHistoricalIntrinsicScheduleConflicts(t *testing.T) {
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
				Title: "V2", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
				PlannedEndAt: scheduleTestTime("2026-08-31"),
			}
			if err := app.db.Create(&target).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(
				t, app, token, workspace.ID, "历史节点冲突换版本", scheduleSingleDefinition(),
			)
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID,
				map[string]any{"title": "历史节点冲突换版本"})
			requirementID := uint(created["id"].(float64))
			instanceID := uint(created["workflow_instance_id"].(float64))
			version := uint(created["version"].(float64))
			plans := `{"delivery":{"planned_start_at":"2026-08-20T00:00:00Z","planned_end_at":"2026-08-10T00:00:00Z"}}`
			if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
				Update("node_plans_json", plans).Error; err != nil {
				t.Fatal(err)
			}

			path := "/api/v1/platform/requirements/" + uintString(requirementID) + "/schedule"
			method := http.MethodPost
			if legacy {
				path = "/api/v1/platform/product-requirements-new/" + uintString(requirementID)
				method = http.MethodPatch
			}
			response := app.request(t, method, path, token, workspace.ID, map[string]any{
				"version_task_new_id": target.ID, "expected_version": version,
			})
			requireStatus(t, response, http.StatusOK)
			assertRequirementScheduleFacts(t, app, requirementID, target.ID, version+1)
			assertInstancePlansJSON(t, app, instanceID, plans)
		})
	}
}

func TestRequirementScheduleMoveHandlesExistingBoundaryConflictIncrementally(t *testing.T) {
	for _, test := range []struct {
		name      string
		targetEnd string
		want      int
	}{
		{name: "wider target succeeds", targetEnd: "2026-08-15", want: http.StatusOK},
		{name: "stricter target conflicts", targetEnd: "2026-08-05", want: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, token, workspace, project, source := newAIWorkflowFixture(t)
			setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-10")
			target := projectgorm.PlanUnit{
				ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
				Title: "目标版本", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
				PlannedEndAt: scheduleTestTime(test.targetEnd),
			}
			if err := app.db.Create(&target).Error; err != nil {
				t.Fatal(err)
			}
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "已存越界换版本", scheduleSingleDefinition())
			created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "已存越界换版本"})
			requirementID := uint(created["id"].(float64))
			instanceID := uint(created["workflow_instance_id"].(float64))
			version := uint(created["version"].(float64))
			plans := `{"delivery":{"planned_end_at":"2026-08-20T00:00:00Z"}}`
			if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where("id = ?", instanceID).
				Update("node_plans_json", plans).Error; err != nil {
				t.Fatal(err)
			}

			response := app.request(t, http.MethodPost,
				"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
				token, workspace.ID, map[string]any{
					"version_task_new_id": target.ID, "expected_version": version,
				})
			requireStatus(t, response, test.want)
			wantUnit, wantVersion := source.ID, version
			if test.want == http.StatusOK {
				wantUnit, wantVersion = target.ID, version+1
			} else {
				assertScheduleEnvelopeConflict(t, response, "NODE_AFTER_SUBJECT_END")
			}
			assertRequirementScheduleFacts(t, app, requirementID, wantUnit, wantVersion)
			assertInstancePlansJSON(t, app, instanceID, plans)
		})
	}
}

func TestRequirementScheduleConfirmationRollsBackAfterRequirementWriteFailure(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "待推导版本", Type: "version",
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "需求确认原子回滚", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "需求确认原子回滚"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	version := uint(created["version"].(float64))
	requireStatus(t, app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
		}}}), http.StatusOK)
	var beforeInstance workflowgorm.WorkflowInstance
	if err := app.db.First(&beforeInstance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeEvents := allScheduleMutationCounts(t, app)
	triggered := false
	callback := "test:fail_requirement_after_schedule_inference"
	if err := app.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if triggered || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Requirement" {
			return
		}
		triggered = true
		tx.AddError(errors.New("forced requirement schedule failure"))
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Update().Remove(callback) })

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version,
			"confirm_subject_schedule": true,
		})
	requireStatus(t, response, http.StatusInternalServerError)
	if !triggered {
		t.Fatal("requirement failure callback did not run")
	}
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	assertInstancePlansJSON(t, app, instanceID, beforeInstance.NodePlansJSON)
	var storedTarget projectgorm.PlanUnit
	if err := app.db.First(&storedTarget, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedTarget.PlannedStartAt != nil || storedTarget.PlannedEndAt != nil {
		t.Fatalf("failed requirement write leaked inferred dates: %+v", storedTarget)
	}
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterEvents != beforeEvents {
		t.Fatalf("failed requirement write leaked side effects audits=%d/%d events=%d/%d", afterAudits, beforeAudits, afterEvents, beforeEvents)
	}
}

func assertRequirementScheduleFacts(
	t *testing.T,
	app *testApp,
	requirementID, planUnitID, version uint,
) {
	t.Helper()
	var requirement requirementgorm.Requirement
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.PlanUnitID != planUnitID || requirement.Version != version {
		t.Fatalf("requirement facts changed: %+v want plan=%d version=%d", requirement, planUnitID, version)
	}
}
