package httpapi

import (
	"net/http"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

func TestPlanUnitDeleteWinsBeforeRequirementScheduleLock(t *testing.T) {
	app, token, workspace, project, source, target, requirementID, version :=
		newPlanUnitDeleteScheduleFixture(t)
	setScheduleConcurrencyConnections(t, app)
	triggered, deleteStatus := false, 0
	auditsAfterDelete, eventsAfterDelete := int64(-1), int64(-1)
	callback := "test:delete_plan_before_requirement_schedule_lock"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if triggered || !scheduleLockedProjectQuery(tx) {
			return
		}
		triggered = true
		deleted := app.request(t, http.MethodDelete,
			"/api/v1/platform/project-new/"+uintString(target.ID), token, workspace.ID, nil)
		deleteStatus = deleted.Code
		auditsAfterDelete, eventsAfterDelete = allScheduleMutationCounts(t, app)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })

	scheduled := app.request(t, http.MethodPost,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
		token, workspace.ID, map[string]any{
			"version_task_new_id": target.ID, "expected_version": version,
		})
	if !triggered || deleteStatus != http.StatusOK {
		t.Fatalf("delete linearization triggered=%t status=%d", triggered, deleteStatus)
	}
	requireStatus(t, scheduled, http.StatusConflict)
	assertScheduleStaleResponse(t, scheduled, "version_task_new_id", "")
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	var targetCount int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", target.ID).Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("deleted target still exists: %d", targetCount)
	}
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != auditsAfterDelete || afterEvents != eventsAfterDelete {
		t.Fatalf("rejected schedule leaked side effects audits=%d/%d events=%d/%d", afterAudits, auditsAfterDelete, afterEvents, eventsAfterDelete)
	}
	_ = project
}

func TestRequirementScheduleWinsBeforePlanUnitDeleteLock(t *testing.T) {
	app, token, workspace, _, _, target, requirementID, version :=
		newPlanUnitDeleteScheduleFixture(t)
	setScheduleConcurrencyConnections(t, app)
	triggered, scheduleStatus := false, 0
	auditsAfterSchedule, eventsAfterSchedule := int64(-1), int64(-1)
	callback := "test:schedule_requirement_before_plan_delete_lock"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if triggered || !scheduleLockedProjectQuery(tx) {
			return
		}
		triggered = true
		scheduled := app.request(t, http.MethodPost,
			"/api/v1/platform/requirements/"+uintString(requirementID)+"/schedule",
			token, workspace.ID, map[string]any{
				"version_task_new_id": target.ID, "expected_version": version,
			})
		scheduleStatus = scheduled.Code
		auditsAfterSchedule, eventsAfterSchedule = allScheduleMutationCounts(t, app)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })

	deleted := app.request(t, http.MethodDelete,
		"/api/v1/platform/project-new/"+uintString(target.ID), token, workspace.ID, nil)
	if !triggered || scheduleStatus != http.StatusOK {
		t.Fatalf("schedule linearization triggered=%t status=%d", triggered, scheduleStatus)
	}
	requireStatus(t, deleted, http.StatusConflict)
	assertRequirementScheduleFacts(t, app, requirementID, target.ID, version+1)
	var targetCount int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", target.ID).Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 {
		t.Fatalf("scheduled target was deleted: %d", targetCount)
	}
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != auditsAfterSchedule || afterEvents != eventsAfterSchedule {
		t.Fatalf("rejected delete leaked side effects audits=%d/%d events=%d/%d", afterAudits, auditsAfterSchedule, afterEvents, eventsAfterSchedule)
	}
}

func TestPlanUnitDeleteWinsBeforeLegacyRequirementMoveLock(t *testing.T) {
	app, token, workspace, _, source, target, requirementID, version :=
		newPlanUnitDeleteScheduleFixture(t)
	setScheduleConcurrencyConnections(t, app)
	triggered, deleteStatus := false, 0
	auditsAfterDelete, eventsAfterDelete := int64(-1), int64(-1)
	callback := "test:delete_plan_before_legacy_requirement_move_lock"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if triggered || !scheduleLockedProjectQuery(tx) {
			return
		}
		triggered = true
		deleted := app.request(t, http.MethodDelete,
			"/api/v1/platform/project-new/"+uintString(target.ID), token, workspace.ID, nil)
		deleteStatus = deleted.Code
		auditsAfterDelete, eventsAfterDelete = allScheduleMutationCounts(t, app)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })

	moved := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(requirementID), token, workspace.ID,
		map[string]any{"version_task_new_id": target.ID, "expected_version": version})
	if !triggered || deleteStatus != http.StatusOK {
		t.Fatalf("legacy delete linearization triggered=%t status=%d", triggered, deleteStatus)
	}
	requireStatus(t, moved, http.StatusConflict)
	assertRequirementScheduleFacts(t, app, requirementID, source.ID, version)
	var targetCount int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", target.ID).Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("legacy deleted target still exists: %d", targetCount)
	}
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != auditsAfterDelete || afterEvents != eventsAfterDelete {
		t.Fatalf("rejected legacy move leaked side effects audits=%d/%d events=%d/%d", afterAudits, auditsAfterDelete, afterEvents, eventsAfterDelete)
	}
}

func newPlanUnitDeleteScheduleFixture(
	t *testing.T,
) (*testApp, string, identitygorm.Workspace, projectgorm.ProjectSet, projectgorm.PlanUnit, projectgorm.PlanUnit, uint, uint) {
	t.Helper()
	app, token, workspace, project, source := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &source, "2026-08-01", "2026-08-31")
	target := projectgorm.PlanUnit{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: "可删除目标版本", Type: "version", PlannedStartAt: scheduleTestTime("2026-08-01"),
		PlannedEndAt: scheduleTestTime("2026-08-31"),
	}
	if err := app.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "删除与排期竞态", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, source.ID, templateID, map[string]any{"title": "删除与排期竞态"})
	return app, token, workspace, project, source, target,
		uint(created["id"].(float64)), uint(created["version"].(float64))
}

func setScheduleConcurrencyConnections(t *testing.T, app *testApp) {
	t.Helper()
	sqlDB, err := app.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(2)
}

func scheduleLockedProjectQuery(tx *gorm.DB) bool {
	if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "ProjectSet" {
		return false
	}
	_, locked := tx.Statement.Clauses["FOR"]
	return locked
}
