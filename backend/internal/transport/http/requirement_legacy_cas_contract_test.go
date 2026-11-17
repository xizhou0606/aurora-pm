package httpapi

import (
	"net/http"
	"sync"
	"testing"

	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type legacyRequirementCASFixture struct {
	app         *testApp
	token       string
	workspace   identitygorm.Workspace
	project     projectgorm.ProjectSet
	sourceUnit  projectgorm.PlanUnit
	targetUnit  projectgorm.PlanUnit
	requirement requirementgorm.Requirement
	task        taskgorm.WorkTask
	bug         qualitygorm.QualityBug
}

func setupLegacyRequirementCASFixture(t *testing.T) legacyRequirementCASFixture {
	t.Helper()
	app := newTestApp(t, nil)
	actor := app.createUser(t, 1, "legacy-requirement-editor", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "产品"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "PM", Code: "PM", OwnerUserID: &actor.ID, Status: "active",
	}
	sourceUnit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1", Type: "version", Status: "planning",
	}
	targetUnit := projectgorm.PlanUnit{
		ID: 31, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V2", Type: "version", Status: "planning",
	}
	requirement := requirementgorm.Requirement{
		ID: 40, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, PlanUnitID: sourceUnit.ID, Version: 1,
		Title: "初始需求", Description: "初始描述", Priority: "normal", Status: "backlog",
	}
	task := taskgorm.WorkTask{
		ID: 50, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, ProjectID: sourceUnit.ID, RequirementID: requirement.ID,
		Kind: taskgorm.WorkTaskKindTask, TaskKey: "PM-50", Title: "关联任务", Status: "draft", Priority: "normal",
	}
	bug := qualitygorm.QualityBug{
		ID: 60, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, ProjectID: sourceUnit.ID, RequirementID: requirement.ID,
		BugKey: "PMB-60", Title: "关联缺陷", Status: "triage", Priority: "medium", SeverityKey: "p2",
	}
	for _, row := range []any{
		&workspace,
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: actor.ID, Role: "owner", Enabled: true},
		&team,
		&identitygorm.TeamMember{WorkspaceID: workspace.ID, TeamID: team.ID, UserID: actor.ID, Role: "owner"},
		&project, &sourceUnit, &targetUnit, &requirement, &task, &bug,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return legacyRequirementCASFixture{
		app: app, token: app.token(t, actor), workspace: workspace, project: project,
		sourceUnit: sourceUnit, targetUnit: targetUnit, requirement: requirement, task: task, bug: bug,
	}
}

func TestLegacyRequirementPatchRequiresPositiveExpectedVersion(t *testing.T) {
	fix := setupLegacyRequirementCASFixture(t)
	path := "/api/v1/platform/product-requirements-new/" + uintString(fix.requirement.ID)
	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "missing", body: map[string]any{"title": "不应写入"}},
		{name: "null", body: map[string]any{"title": "不应写入", "expected_version": nil}},
		{name: "zero", body: map[string]any{"title": "不应写入", "expected_version": 0}},
		{name: "negative", body: map[string]any{"title": "不应写入", "expected_version": -1}},
		{name: "string", body: map[string]any{"title": "不应写入", "expected_version": "1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := fix.app.request(t, http.MethodPatch, path, fix.token, fix.workspace.ID, test.body)
			requireStatus(t, rec, http.StatusBadRequest)
		})
	}

	var stored requirementgorm.Requirement
	if err := fix.app.db.First(&stored, fix.requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Title != fix.requirement.Title || stored.Version != 1 {
		t.Fatalf("invalid version payload changed requirement: %+v", stored)
	}
	assertLegacyRequirementMutationCounts(t, fix, 0, 0)
}

func TestLegacyRequirementPatchRejectsConfirmationWithoutMoveAtomically(t *testing.T) {
	tests := []struct {
		name      string
		includeID bool
	}{
		{name: "same plan unit", includeID: true},
		{name: "target omitted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fix := setupLegacyRequirementCASFixture(t)
			body := map[string]any{
				"title": "不应保存的标题", "description": "不应保存的描述",
				"expected_version": 1, "confirm_subject_schedule": true,
			}
			if test.includeID {
				body["version_task_new_id"] = fix.sourceUnit.ID
			}
			response := fix.app.request(t, http.MethodPatch,
				"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID),
				fix.token, fix.workspace.ID, body,
			)
			requireStatus(t, response, http.StatusConflict)
			assertScheduleEnvelopeConflict(t, response, "SCHEDULE_CONFIRMATION_NOT_AVAILABLE")
			assertLegacyRequirementState(t, fix, fix.requirement.Title, fix.requirement.Description, 1)
			assertLegacyRequirementMutationCounts(t, fix, 0, 0)
		})
	}
}

func TestLegacyRequirementPatchRejectsStaleWriterWithoutSideEffects(t *testing.T) {
	fix := setupLegacyRequirementCASFixture(t)
	path := "/api/v1/platform/product-requirements-new/" + uintString(fix.requirement.ID)

	winner := fix.app.request(t, http.MethodPatch, path, fix.token, fix.workspace.ID, map[string]any{
		"description": "先保存的内容", "expected_version": 1,
	})
	requireStatus(t, winner, http.StatusOK)
	data := decodeData(t, winner).(map[string]any)
	if data["version"] != float64(2) {
		t.Fatalf("successful patch version=%v, want 2", data["version"])
	}
	assertLegacyRequirementMutationCounts(t, fix, 1, 1)

	stale := fix.app.request(t, http.MethodPatch, path, fix.token, fix.workspace.ID, map[string]any{
		"title": "过期页面覆盖", "version_task_new_id": fix.targetUnit.ID, "expected_version": 1,
	})
	requireStatus(t, stale, http.StatusConflict)
	if envelope := scheduleErrorEnvelope(t, stale); envelope["code"] != "REQUIREMENT_STALE" {
		t.Fatalf("legacy stale code=%v body=%s", envelope["code"], stale.Body.String())
	}

	assertLegacyRequirementState(t, fix, "初始需求", "先保存的内容", 2)
	assertLegacyRequirementMutationCounts(t, fix, 1, 1)
}

func TestLegacyRequirementPatchCASRejectsChangeAfterInitialRead(t *testing.T) {
	fix := setupLegacyRequirementCASFixture(t)
	var mu sync.Mutex
	fired := false
	callbackName := "test:legacy_requirement_concurrent_writer"
	if err := fix.app.db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "ProjectSet" {
			return
		}
		project, ok := tx.Statement.Dest.(*projectgorm.ProjectSet)
		if !ok || project.ID != fix.project.ID {
			return
		}
		mu.Lock()
		if fired {
			mu.Unlock()
			return
		}
		fired = true
		mu.Unlock()
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Model(&requirementgorm.Requirement{}).
			Where("id = ? AND workspace_id = ? AND version = ?", fix.requirement.ID, fix.workspace.ID, 1).
			Updates(map[string]any{"title": "并发写入者", "version": 2}).Error)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fix.app.db.Callback().Query().Remove(callbackName) })

	rec := fix.app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirement.ID),
		fix.token, fix.workspace.ID, map[string]any{
			"title": "后写入者", "expected_version": 1,
		})
	requireStatus(t, rec, http.StatusConflict)
	if envelope := scheduleErrorEnvelope(t, rec); envelope["code"] != "REQUIREMENT_STALE" {
		t.Fatalf("legacy concurrent stale code=%v body=%s", envelope["code"], rec.Body.String())
	}
	mu.Lock()
	callbackFired := fired
	mu.Unlock()
	if !callbackFired {
		t.Fatal("concurrent writer callback did not fire")
	}

	assertLegacyRequirementState(t, fix, "并发写入者", "初始描述", 2)
	assertLegacyRequirementMutationCounts(t, fix, 0, 0)
}

func assertLegacyRequirementState(
	t *testing.T, fix legacyRequirementCASFixture, wantTitle, wantDescription string, wantVersion uint,
) {
	t.Helper()
	var requirement requirementgorm.Requirement
	if err := fix.app.db.First(&requirement, fix.requirement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.Title != wantTitle || requirement.Description != wantDescription ||
		requirement.Version != wantVersion || requirement.ProjectSetID != fix.project.ID ||
		requirement.PlanUnitID != fix.sourceUnit.ID {
		t.Fatalf("requirement after conflict=%+v", requirement)
	}
	var task taskgorm.WorkTask
	if err := fix.app.db.First(&task, fix.task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.ProjectSetID != fix.project.ID || task.ProjectID != fix.sourceUnit.ID {
		t.Fatalf("task scope changed after conflict: %+v", task)
	}
	var bug qualitygorm.QualityBug
	if err := fix.app.db.First(&bug, fix.bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bug.ProjectSetID != fix.project.ID || bug.ProjectID != fix.sourceUnit.ID {
		t.Fatalf("quality scope changed after conflict: %+v", bug)
	}
}

func assertLegacyRequirementMutationCounts(
	t *testing.T, fix legacyRequirementCASFixture, wantAudit, wantOutbox int64,
) {
	t.Helper()
	var auditCount int64
	if err := fix.app.db.Model(&auditgorm.AuditLog{}).Where(
		"workspace_id = ? AND resource = ? AND resource_id = ?",
		fix.workspace.ID, "requirement", fix.requirement.ID,
	).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	var outboxCount int64
	if err := fix.app.db.Model(&auditgorm.OutboxEvent{}).Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ?",
		fix.workspace.ID, "requirement", fix.requirement.ID,
	).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != wantAudit || outboxCount != wantOutbox {
		t.Fatalf(
			"mutation side effects audit=%d outbox=%d, want audit=%d outbox=%d",
			auditCount, outboxCount, wantAudit, wantOutbox,
		)
	}
}
