package httpapi

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestRequirementScheduleStaleVersionReturnsScheduleEnvelopeWithoutWrites(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "V2", Type: "version", Status: "planning",
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(
		t, app, token, workspace.ID, "需求排期版本 CAS", scheduleSingleDefinition(),
	)
	created := createRequirementWithWorkflow(
		t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "需求排期版本 CAS"},
	)
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	staleVersion := uint(created["version"].(float64))
	currentVersion := staleVersion + 1
	if err := app.db.Model(&requirementgorm.Requirement{}).
		Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).
		Update("version", currentVersion).Error; err != nil {
		t.Fatal(err)
	}

	var beforeRequirement requirementgorm.Requirement
	var beforeSource, beforeTarget projectgorm.PlanUnit
	var beforeInstance workflowgorm.WorkflowInstance
	for _, load := range []struct {
		destination any
		id          uint
	}{
		{destination: &beforeRequirement, id: requirementID},
		{destination: &beforeSource, id: source.ID},
		{destination: &beforeTarget, id: target.ID},
		{destination: &beforeInstance, id: instanceID},
	} {
		if err := app.db.First(load.destination, load.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	beforeAudits, beforeOutbox := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": staleVersion,
		},
	)
	assertScheduleStaleResponse(t, response, "version", "")
	envelope := scheduleErrorEnvelope(t, response)
	schedule := envelope["schedule"].(map[string]any)
	subject := schedule["subject"].(map[string]any)
	if subject["planned_start_at"] != "2026-08-01T00:00:00Z" ||
		subject["planned_end_at"] != "2026-08-31T00:00:00Z" {
		t.Fatalf("stale response used the wrong current requirement schedule: %v", subject)
	}
	conflict := schedule["conflicts"].([]any)[0].(map[string]any)
	if conflict["subject_type"] != "requirement" || conflict["subject_id"] != float64(requirementID) {
		t.Fatalf("stale requirement conflict=%v", conflict)
	}

	var afterRequirement requirementgorm.Requirement
	var afterSource, afterTarget projectgorm.PlanUnit
	var afterInstance workflowgorm.WorkflowInstance
	for _, load := range []struct {
		destination any
		id          uint
	}{
		{destination: &afterRequirement, id: requirementID},
		{destination: &afterSource, id: source.ID},
		{destination: &afterTarget, id: target.ID},
		{destination: &afterInstance, id: instanceID},
	} {
		if err := app.db.First(load.destination, load.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(afterRequirement, beforeRequirement) ||
		!reflect.DeepEqual(afterSource, beforeSource) ||
		!reflect.DeepEqual(afterTarget, beforeTarget) ||
		!reflect.DeepEqual(afterInstance, beforeInstance) {
		t.Fatalf(
			"stale schedule request changed state: requirement=%+v/%+v source=%+v/%+v target=%+v/%+v instance=%+v/%+v",
			afterRequirement, beforeRequirement, afterSource, beforeSource,
			afterTarget, beforeTarget, afterInstance, beforeInstance,
		)
	}
	afterAudits, afterOutbox := allScheduleMutationCounts(t, app)
	if afterAudits != beforeAudits || afterOutbox != beforeOutbox {
		t.Fatalf(
			"stale schedule request emitted side effects: audit=%d/%d outbox=%d/%d",
			afterAudits, beforeAudits, afterOutbox, beforeOutbox,
		)
	}
}

func TestRequirementScheduleMissingTargetReturnsStaleWithoutWrites(t *testing.T) {
	app, token, workspace, _, source, target, requirementID, version :=
		newPlanUnitDeleteScheduleFixture(t)
	deleted := app.request(t, http.MethodDelete,
		"/api/v1/platform/project-new/"+uintString(target.ID), token, workspace.ID, nil,
	)
	requireStatus(t, deleted, http.StatusOK)

	var beforeRequirement requirementgorm.Requirement
	var beforeSource projectgorm.PlanUnit
	if err := app.db.First(&beforeRequirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&beforeSource, source.ID).Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeOutbox := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version,
		},
	)
	assertScheduleStaleResponse(t, response, "version_task_new_id", "")

	var afterRequirement requirementgorm.Requirement
	var afterSource projectgorm.PlanUnit
	if err := app.db.First(&afterRequirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&afterSource, source.ID).Error; err != nil {
		t.Fatal(err)
	}
	var targetCount int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", target.ID).Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterRequirement, beforeRequirement) ||
		!reflect.DeepEqual(afterSource, beforeSource) || targetCount != 0 {
		t.Fatalf(
			"missing-target stale request changed state: requirement=%+v/%+v source=%+v/%+v target_count=%d",
			afterRequirement, beforeRequirement, afterSource, beforeSource, targetCount,
		)
	}
	assertAllScheduleMutationCounts(t, app, beforeAudits, beforeOutbox)
}

func TestRequirementScheduleCrossProjectTargetReturnsCurrentScopeStale(t *testing.T) {
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(
		t, app, token, workspace.ID, "需求跨项目排期 CAS", scheduleSingleDefinition(),
	)
	created := createRequirementWithWorkflow(
		t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "需求跨项目排期 CAS"},
	)
	requirementID := uint(created["id"].(float64))
	currentVersion := uint(created["version"].(float64)) + 1
	currentProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: project.TeamID,
		Name: "当前项目", Code: "CURRENT", Status: "active",
		PlannedStartAt: scheduleTestTime("2026-09-01"), PlannedEndAt: scheduleTestTime("2026-09-30"),
	}
	currentUnit := projectgorm.PlanUnit{
		ID: 22, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: currentProject.ID,
		Title: "当前版本", Type: "version", Status: "planning",
		PlannedStartAt: scheduleTestTime("2026-09-05"), PlannedEndAt: scheduleTestTime("2026-09-25"),
	}
	if err := app.db.Create(&currentProject).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&currentUnit).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&requirementgorm.Requirement{}).
		Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).
		Updates(map[string]any{
			"project_new_id": currentProject.ID, "version_task_new_id": currentUnit.ID,
			"team_id": currentUnit.TeamID, "version": currentVersion,
		}).Error; err != nil {
		t.Fatal(err)
	}

	var beforeRequirement requirementgorm.Requirement
	var beforeSource, beforeCurrentUnit projectgorm.PlanUnit
	if err := app.db.First(&beforeRequirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&beforeSource, source.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&beforeCurrentUnit, currentUnit.ID).Error; err != nil {
		t.Fatal(err)
	}
	beforeAudits, beforeOutbox := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": source.ID, "expected_version": currentVersion,
		},
	)
	assertScheduleStaleResponse(t, response, "version_task_new_id", "")
	schedule := scheduleErrorEnvelope(t, response)["schedule"].(map[string]any)
	subject := schedule["subject"].(map[string]any)
	if subject["planned_start_at"] != "2026-09-05T00:00:00Z" ||
		subject["planned_end_at"] != "2026-09-25T00:00:00Z" {
		t.Fatalf("cross-project stale response used the wrong current scope: %v", subject)
	}

	var afterRequirement requirementgorm.Requirement
	var afterSource, afterCurrentUnit projectgorm.PlanUnit
	if err := app.db.First(&afterRequirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&afterSource, source.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&afterCurrentUnit, currentUnit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterRequirement, beforeRequirement) ||
		!reflect.DeepEqual(afterSource, beforeSource) ||
		!reflect.DeepEqual(afterCurrentUnit, beforeCurrentUnit) {
		t.Fatalf(
			"cross-project stale request changed state: requirement=%+v/%+v source=%+v/%+v current=%+v/%+v",
			afterRequirement, beforeRequirement, afterSource, beforeSource,
			afterCurrentUnit, beforeCurrentUnit,
		)
	}
	assertAllScheduleMutationCounts(t, app, beforeAudits, beforeOutbox)
}

func TestRequirementScheduleTargetLookupDatabaseFailureIsNotStale(t *testing.T) {
	app, token, workspace, _, source, target, requirementID, version :=
		newPlanUnitDeleteScheduleFixture(t)
	callbackName := "test:fail_requirement_schedule_target_lookup"
	fired := false
	if err := app.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if fired || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "PlanUnit" {
			return
		}
		fired = true
		tx.AddError(errors.New("forced target plan unit lookup failure"))
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callbackName) })
	beforeAudits, beforeOutbox := allScheduleMutationCounts(t, app)

	response := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version,
		},
	)
	if !fired {
		t.Fatal("target lookup failure callback did not fire")
	}
	requireStatus(t, response, http.StatusInternalServerError)
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	assertAllScheduleMutationCounts(t, app, beforeAudits, beforeOutbox)
}

func assertAllScheduleMutationCounts(t *testing.T, app *testApp, wantAudits, wantOutbox int64) {
	t.Helper()
	audits, outbox := allScheduleMutationCounts(t, app)
	if audits != wantAudits || outbox != wantOutbox {
		t.Fatalf("schedule stale side effects audit=%d/%d outbox=%d/%d", audits, wantAudits, outbox, wantOutbox)
	}
}
