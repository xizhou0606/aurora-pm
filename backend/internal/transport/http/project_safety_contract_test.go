package httpapi

import (
	"net/http"
	"testing"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestProjectSetDeletionImpactMatchesCascadeAndIsWorkspaceScoped(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "project-delete-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	task := taskgorm.WorkTask{
		ID: 701, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-701", Title: "Cascade task",
		Status: "draft", Priority: "normal",
	}
	bug := qualitygorm.QualityBug{
		ID: 801, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, BugKey: "BUG-801", Title: "Cascade bug",
		Status: "triage", Priority: "medium", SeverityKey: "p2",
	}
	otherProject := projectgorm.ProjectSet{ID: 14, WorkspaceID: 1, TeamID: 10, Name: "Other", Code: "OTHER"}
	otherPlan := projectgorm.PlanUnit{ID: 15, WorkspaceID: 1, TeamID: 10, ProjectSetID: 14, Title: "Other plan"}
	otherRequirement := requirementgorm.Requirement{
		ID: 16, WorkspaceID: 1, TeamID: 10, ProjectSetID: 14, PlanUnitID: 15, Title: "Other requirement",
	}
	externalBug := qualitygorm.QualityBug{
		ID: 802, WorkspaceID: 1, TeamID: 10, ProjectSetID: 14, ProjectID: 15,
		RequirementID: 16, BugKey: "BUG-802", Title: "External reference",
		Status: "triage", Priority: "medium", SeverityKey: "p2", SourceExecutionID: &task.ID,
	}
	label := projectgorm.DeliveryNodeLabel{
		ID: 901, WorkspaceID: 1, ProjectSetID: 11, PlanUnitID: 12, ClientKey: "release", Name: "Release",
	}
	node := projectgorm.DeliveryNode{
		ID: 902, WorkspaceID: 1, PlanUnitID: 12, LabelID: &label.ID, LabelName: label.Name,
	}
	permission := projectgorm.ProjectPermission{
		ID: 903, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: admin.ID, AccessLevel: "viewer",
	}
	for _, row := range []any{
		&otherProject, &otherPlan, &otherRequirement, &task, &bug, &externalBug, &label, &node,
		&taskgorm.WorkTaskComment{ID: 711, WorkspaceID: 1, WorkTaskID: task.ID, UserID: admin.ID, Body: "comment"},
		&taskgorm.WorkTaskAttachment{ID: 712, WorkspaceID: 1, WorkTaskID: task.ID, UploaderUserID: admin.ID,
			Filename: "a.txt", Size: 1, ObjectKey: "a.txt", StorageBackend: "s3"},
		&taskgorm.WorkTaskActivity{ID: 713, WorkspaceID: 1, WorkTaskID: task.ID, UserID: admin.ID, Action: "created"},
		&taskgorm.WorkTaskWatcher{ID: 714, WorkspaceID: 1, WorkTaskID: task.ID, UserID: admin.ID},
		&taskgorm.TaskTestDefinition{ID: 721, WorkspaceID: 1, WorkTaskID: task.ID, Version: 1,
			CasesJSON: "[]", CreatedByUserID: admin.ID, UpdatedByUserID: admin.ID},
		&qualitygorm.QualityBugActivity{ID: 811, WorkspaceID: 1, QualityBugID: bug.ID, UserID: admin.ID, Action: "created"},
		&projectgorm.ProjectNews{ID: 904, WorkspaceID: 1, ProjectSetID: 11, PlanUnitID: 12,
			UserID: admin.ID, Title: "News"},
		&taskgorm.KeySequence{ID: 905, WorkspaceID: 1, ProjectSetID: 11, Prefix: "A", NextValue: 2},
		&permission,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	execution := taskgorm.TaskTestExecution{
		ID: 722, WorkspaceID: 1, WorkTaskID: task.ID, TaskTestDefinitionID: 721,
		DefinitionVersion: 1, Environment: "test", ExecutorUserID: admin.ID, Status: "pending",
	}
	if err := app.db.Create(&execution).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&taskgorm.TaskTestExecutionRun{
		ID: 723, WorkspaceID: 1, WorkTaskID: task.ID, TaskTestExecutionID: execution.ID,
		Version: 1, Sequence: 1, CaseKey: "case-1", Title: "Case 1", Status: "pending", EvidenceJSON: "[]",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&projectgorm.ProjectMemberRole{
		ID: 906, WorkspaceID: 1, ProjectPermissionID: permission.ID, RoleKey: "viewer", ScopeType: "project",
	}).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/project-sets/11/deletion-impact", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	counts := data["counts"].(map[string]any)
	for _, key := range []string{
		"plan_units", "requirements", "tasks", "quality_bugs", "delivery_node_labels", "delivery_nodes",
		"task_comments", "task_attachments", "task_activities", "task_watchers",
		"task_test_definitions", "task_test_executions", "task_test_execution_runs",
		"quality_bug_activities", "project_news", "key_sequences",
		"legacy_project_permissions", "legacy_project_member_roles", "quality_source_references",
	} {
		if counts[key] != float64(1) {
			t.Fatalf("count[%s]=%v impact=%s", key, counts[key], response.Body.String())
		}
	}
	if data["cascade_required"] != true || data["legacy_authorization_cleanup_only"] != true {
		t.Fatalf("project deletion contract=%s", response.Body.String())
	}
	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/project-sets/11/deletion-impact", token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)

	deleted := app.request(t, http.MethodDelete, "/api/v1/platform/project-sets/11", token, 1, nil)
	requireStatus(t, deleted, http.StatusConflict)
	for name, target := range map[string]any{
		"project": &projectgorm.ProjectSet{}, "task": &taskgorm.WorkTask{},
		"attachment": &taskgorm.WorkTaskAttachment{}, "definition": &taskgorm.TaskTestDefinition{},
		"execution": &taskgorm.TaskTestExecution{}, "run": &taskgorm.TaskTestExecutionRun{},
	} {
		var count int64
		app.db.Model(target).Where("workspace_id = ?", 1).Count(&count)
		if count == 0 {
			t.Fatalf("%s was mutated despite attachment conflict", name)
		}
	}
	var retained qualitygorm.QualityBug
	if err := app.db.First(&retained, externalBug.ID).Error; err != nil || retained.SourceExecutionID == nil {
		t.Fatalf("external quality reference changed despite conflict: %+v err=%v", retained, err)
	}

	if err := app.db.Where("workspace_id = ? AND id = ?", 1, 712).
		Delete(&taskgorm.WorkTaskAttachment{}).Error; err != nil {
		t.Fatal(err)
	}
	deleted = app.request(t, http.MethodDelete, "/api/v1/platform/project-sets/11", token, 1, nil)
	requireStatus(t, deleted, http.StatusOK)
	for name, target := range map[string]any{
		"definitions": &taskgorm.TaskTestDefinition{}, "executions": &taskgorm.TaskTestExecution{},
		"runs": &taskgorm.TaskTestExecutionRun{},
	} {
		var count int64
		app.db.Model(target).Where("workspace_id = ? AND work_task_id = ?", 1, task.ID).Count(&count)
		if count != 0 {
			t.Fatalf("%s remained after project cascade: %d", name, count)
		}
	}
	if err := app.db.First(&retained, externalBug.ID).Error; err != nil || retained.SourceExecutionID != nil {
		t.Fatalf("external quality source reference was not cleared: %+v err=%v", retained, err)
	}
}

func TestPlanAndDeliveryLabelDeletionImpactContracts(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "plan-delete-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	blocked := app.request(t, http.MethodGet,
		"/api/v1/platform/project-new/12/deletion-impact", token, 1, nil)
	requireStatus(t, blocked, http.StatusOK)
	blockedData := decodeData(t, blocked).(map[string]any)
	if blockedData["blocked"] != true || blockedData["blocked_reason"] != "plan unit has requirements" {
		t.Fatalf("blocked plan impact=%s", blocked.Body.String())
	}

	unit := projectgorm.PlanUnit{ID: 14, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "Empty plan"}
	label := projectgorm.DeliveryNodeLabel{
		ID: 15, WorkspaceID: 1, ProjectSetID: 11, PlanUnitID: unit.ID, ClientKey: "release", Name: "Release",
	}
	node := projectgorm.DeliveryNode{ID: 16, WorkspaceID: 1, PlanUnitID: unit.ID, LabelID: &label.ID, LabelName: label.Name}
	for _, row := range []any{&unit, &label, &node} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	impact := app.request(t, http.MethodGet,
		"/api/v1/platform/project-new/14/deletion-impact", token, 1, nil)
	requireStatus(t, impact, http.StatusOK)
	impactData := decodeData(t, impact).(map[string]any)
	if impactData["blocked"] != false || impactData["affected_count"] != float64(3) ||
		impactData["cascade_required"] != true {
		t.Fatalf("empty plan impact=%s", impact.Body.String())
	}

	labelImpact := app.request(t, http.MethodGet,
		"/api/v1/platform/project-delivery-node-labels/15/deletion-impact", token, 1, nil)
	requireStatus(t, labelImpact, http.StatusOK)
	labelData := decodeData(t, labelImpact).(map[string]any)
	if labelData["soft_delete"] != true || labelData["affected_count"] != float64(1) ||
		len(labelData["linked_delivery_node_ids"].([]any)) != 1 {
		t.Fatalf("label impact=%s", labelImpact.Body.String())
	}
	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/project-delivery-node-labels/15/deletion-impact", token, 2, nil)
	requireStatus(t, foreign, http.StatusNotFound)

	softDeleted := app.request(t, http.MethodDelete,
		"/api/v1/platform/project-delivery-node-labels/15", token, 1, nil)
	requireStatus(t, softDeleted, http.StatusOK)
	var retainedNode projectgorm.DeliveryNode
	if err := app.db.First(&retainedNode, node.ID).Error; err != nil || retainedNode.LabelID == nil {
		t.Fatalf("soft delete changed linked node: %+v err=%v", retainedNode, err)
	}
}

func TestProjectAndPlanOwnerMutationsValidateActivePMUsers(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "owner-validation-admin", "super_admin", "")
	app.createUser(t, 47, "owner-validation-user", "user", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	// Create always forces the authenticated user as owner. owner_user_id
	// in the body is ignored.
	create := app.request(t, http.MethodPost, "/api/v1/platform/project-sets", token, 1,
		map[string]any{"team_id": 10, "name": "Create forces actor"})
	requireStatus(t, create, http.StatusCreated)
	if ownerID := decodeData(t, create).(map[string]any)["owner_user_id"]; ownerID != float64(1) {
		t.Fatalf("create owner must be actor: %v", ownerID)
	}
	if creatorID := decodeData(t, create).(map[string]any)["created_by_user_id"]; creatorID != float64(1) {
		t.Fatalf("create creator must be actor: %v", creatorID)
	}

	// PATCH and transfer still validate the owner_user_id.
	invalidPatch := app.request(t, http.MethodPatch, "/api/v1/platform/project-sets/11", token, 1,
		map[string]any{"owner_user_id": 999})
	requireStatus(t, invalidPatch, http.StatusBadRequest)
	invalidTransfer := app.request(t, http.MethodPost,
		"/api/v1/platform/project-sets/11/transfer-owner-new", token, 1,
		map[string]any{"new_owner_user_id": 999})
	requireStatus(t, invalidTransfer, http.StatusBadRequest)
	invalidPlan := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, 1,
		map[string]any{"project_new_id": 11, "title": "Invalid plan owner", "owner_user_id": 999})
	requireStatus(t, invalidPlan, http.StatusBadRequest)
	invalidDeliveryOwner := app.request(t, http.MethodPatch, "/api/v1/platform/project-new/12", token, 1,
		map[string]any{"delivery_nodes": []map[string]any{{"label_name": "Release", "owner_user_id": 999}}})
	requireStatus(t, invalidDeliveryOwner, http.StatusBadRequest)
}

func TestIssueKindsUsesWorkflowScope(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "issue-kind-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-kinds?team_id=10&project_id=12", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	items := decodeData(t, response).([]any)
	if len(items) != 2 || items[0].(map[string]any)["kind_key"] != "task" ||
		items[1].(map[string]any)["kind_key"] != "subtask" {
		t.Fatalf("issue kinds=%s", response.Body.String())
	}
	foreign := app.request(t, http.MethodGet,
		"/api/v1/platform/issue-kinds?team_id=20&project_id=22", token, 1, nil)
	requireStatus(t, foreign, http.StatusNotFound)
}
