package httpapi

import (
	"fmt"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"strings"
	"testing"
)

func TestMoveRequirementUpdatesPMTaskScopeAndAudit(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	createTask := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks",
		fix.app.token(t, fix.actor), fix.workspace.ID,
		map[string]any{"requirement_id": fix.requirementID, "title": "接口改造"})
	requireStatus(t, createTask, http.StatusCreated)
	taskID := uint(decodeData(t, createTask).(map[string]any)["id"].(float64))
	reporterID := fix.actor.ID
	bug := qualitygorm.QualityBug{
		ID: 700, WorkspaceID: fix.workspace.ID, TeamID: fix.team.ID,
		ProjectSetID: fix.sourceProject.ID, ProjectID: fix.sourceUnit.ID,
		RequirementID: fix.requirementID, BugKey: "SRCB-700", Title: "跟随需求移动",
		Status: "triage", Priority: "medium", SeverityKey: "p2", ReporterUserID: &reporterID,
	}
	if err := fix.app.db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}
	if err := fix.app.db.Create(&qualitygorm.QualityBugActivity{
		WorkspaceID: fix.workspace.ID, QualityBugID: bug.ID, UserID: reporterID,
		Action: "created", Summary: "提交缺陷",
	}).Error; err != nil {
		t.Fatal(err)
	}
	var beforeRequirementEvents int64
	if err := fix.app.db.Model(&auditgorm.OutboxEvent{}).Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ?",
		fix.workspace.ID, "requirement", fix.requirementID,
	).Count(&beforeRequirementEvents).Error; err != nil {
		t.Fatal(err)
	}

	rec := fix.move(t, fix.actor, fix.crossUnit.ID)
	requireStatus(t, rec, http.StatusOK)
	var moved requirementgorm.Requirement
	if err := fix.app.db.First(&moved, fix.requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if moved.TeamID != fix.crossUnit.TeamID ||
		moved.ProjectSetID != fix.targetProject.ID ||
		moved.PlanUnitID != fix.crossUnit.ID {
		t.Fatalf("moved requirement scope=%+v", moved)
	}
	var task taskgorm.WorkTask
	if err := fix.app.db.First(&task, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.TeamID != fix.crossUnit.TeamID ||
		task.ProjectSetID != fix.targetProject.ID ||
		task.ProjectID != fix.crossUnit.ID {
		t.Fatalf("moved task scope=%+v", task)
	}
	var movedBug qualitygorm.QualityBug
	if err := fix.app.db.First(&movedBug, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if movedBug.TeamID != fix.crossUnit.TeamID ||
		movedBug.ProjectSetID != fix.targetProject.ID ||
		movedBug.ProjectID != fix.crossUnit.ID || movedBug.RequirementID != fix.requirementID {
		t.Fatalf("moved quality bug scope=%+v", movedBug)
	}
	var activity qualitygorm.QualityBugActivity
	if err := fix.app.db.Where("quality_bug_id = ?", bug.ID).First(&activity).Error; err != nil {
		t.Fatalf("quality activity lost during move: %v", err)
	}
	var sourceAudit auditgorm.AuditLog
	if err := fix.app.db.Where(
		"workspace_id = ? AND project_id = ? AND resource = ? AND resource_id = ? AND action = ?",
		fix.workspace.ID, fix.sourceProject.ID, "requirement", fix.requirementID, "move",
	).First(&sourceAudit).Error; err != nil {
		t.Fatalf("move audit missing: %v", err)
	}
	if !strings.Contains(sourceAudit.Detail, fix.crossUnit.Title) {
		t.Fatalf("move audit detail=%q", sourceAudit.Detail)
	}
	var targetAudit auditgorm.AuditLog
	if err := fix.app.db.Where(
		"workspace_id = ? AND project_id = ? AND resource = ? AND resource_id = ? AND action = ?",
		fix.workspace.ID, fix.targetProject.ID, "requirement", fix.requirementID, "moved_in",
	).First(&targetAudit).Error; err != nil {
		t.Fatalf("target move audit missing: %v", err)
	}
	if !strings.Contains(targetAudit.Detail, fix.targetProject.Name) ||
		!strings.Contains(targetAudit.Detail, fix.crossUnit.Title) {
		t.Fatalf("target move audit detail=%q", targetAudit.Detail)
	}
	var event auditgorm.OutboxEvent
	if err := fix.app.db.Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ? AND event_type = ?",
		fix.workspace.ID, "requirement", fix.requirementID, "pm.requirement.moved",
	).First(&event).Error; err != nil {
		t.Fatalf("move outbox event missing: %v", err)
	}
	if !strings.Contains(
		event.Payload,
		fmt.Sprintf(`"to_version_task_new_id":%d`, fix.crossUnit.ID),
	) {
		t.Fatalf("move payload=%s", event.Payload)
	}
	var afterRequirementEvents int64
	if err := fix.app.db.Model(&auditgorm.OutboxEvent{}).Where(
		"workspace_id = ? AND aggregate_type = ? AND aggregate_id = ?",
		fix.workspace.ID, "requirement", fix.requirementID,
	).Count(&afterRequirementEvents).Error; err != nil {
		t.Fatal(err)
	}
	if afterRequirementEvents != beforeRequirementEvents+1 {
		t.Fatalf(
			"requirement outbox events before=%d after=%d, want exactly one move event",
			beforeRequirementEvents,
			afterRequirementEvents,
		)
	}
}

func TestMoveUnscheduledRequirementToAnotherProject(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	if err := fix.app.db.Model(&requirementgorm.Requirement{}).
		Where("id = ?", fix.requirementID).
		Update("version_task_new_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	rec := fix.move(t, fix.actor, fix.crossUnit.ID)
	requireStatus(t, rec, http.StatusOK)
	var moved requirementgorm.Requirement
	if err := fix.app.db.First(&moved, fix.requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if moved.ProjectSetID != fix.targetProject.ID || moved.PlanUnitID != fix.crossUnit.ID {
		t.Fatalf("unscheduled move scope=%+v", moved)
	}
	var sourceAudit auditgorm.AuditLog
	if err := fix.app.db.Where(
		"workspace_id = ? AND project_id = ? AND resource = ? AND resource_id = ? AND action = ?",
		fix.workspace.ID, fix.sourceProject.ID, "requirement", fix.requirementID, "move",
	).First(&sourceAudit).Error; err != nil {
		t.Fatalf("unscheduled move audit missing: %v", err)
	}
	if !strings.Contains(sourceAudit.Detail, "需求池") ||
		!strings.Contains(sourceAudit.Detail, fix.crossUnit.Title) {
		t.Fatalf("unscheduled move audit detail=%q", sourceAudit.Detail)
	}
}

func TestMoveRequirementAllowsWorkspaceMembersAcrossProjectSets(t *testing.T) {
	productFix := setupRequirementMoveFixture(t)
	productMove := productFix.move(t, productFix.productOwner, productFix.targetUnit.ID)
	requireStatus(t, productMove, http.StatusOK)

	targetFix := setupRequirementMoveFixture(t)
	crossAllowed := targetFix.move(t, targetFix.productOwner, targetFix.crossUnit.ID)
	requireStatus(t, crossAllowed, http.StatusOK)
}

func TestMoveRequirementDoesNotCreateAutomaticPermissions(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	rec := fix.move(t, fix.actor, fix.crossUnit.ID)
	requireStatus(t, rec, http.StatusOK)

	var roles []projectgorm.ProjectMemberRole
	if err := fix.app.db.Where(
		"workspace_id = ? AND source_type = ? AND source_id = ?",
		fix.workspace.ID, "requirement_assignment", fix.requirementID,
	).Find(&roles).Error; err != nil {
		t.Fatal(err)
	}
	if len(roles) != 0 {
		t.Fatalf("automatic roles=%+v, want none", roles)
	}
	var automaticPermissionCount int64
	fix.app.db.Model(&projectgorm.ProjectPermission{}).Where(
		"workspace_id = ? AND user_id = ? AND access_level = ?",
		fix.workspace.ID, fix.productOwner.ID,
		"requirement_viewer",
	).Count(&automaticPermissionCount)
	if automaticPermissionCount != 0 {
		t.Fatalf("automatic permission count=%d, want none", automaticPermissionCount)
	}
}

func TestMoveRequirementDoesNotResolveTargetOutsideWorkspace(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	otherWorkspace := identitygorm.Workspace{ID: 2, Name: "隔离空间", Slug: "other", Enabled: true}
	otherTeam := identitygorm.Team{ID: 11, WorkspaceID: otherWorkspace.ID, Name: "其它团队"}
	otherProject := projectgorm.ProjectSet{
		ID: 22, WorkspaceID: otherWorkspace.ID, TeamID: otherTeam.ID,
		Name: "其它项目集", Code: "OTHER", Status: "active",
	}
	otherUnit := projectgorm.PlanUnit{
		ID: 33, WorkspaceID: otherWorkspace.ID, TeamID: otherTeam.ID,
		ProjectSetID: otherProject.ID, Title: "其它项目", Type: "version", Status: "planning",
	}
	for _, row := range []any{&otherWorkspace, &otherTeam, &otherProject, &otherUnit} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rec := fix.move(t, fix.actor, otherUnit.ID)
	requireStatus(t, rec, http.StatusNotFound)
	var requirement requirementgorm.Requirement
	if err := fix.app.db.First(&requirement, fix.requirementID).Error; err != nil {
		t.Fatal(err)
	}
	if requirement.WorkspaceID != fix.workspace.ID ||
		requirement.ProjectSetID != fix.sourceProject.ID ||
		requirement.PlanUnitID != fix.sourceUnit.ID {
		t.Fatalf("cross-workspace move changed requirement: %+v", requirement)
	}
}

func containsJSONText(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
