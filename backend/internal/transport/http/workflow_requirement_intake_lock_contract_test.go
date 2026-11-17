package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const requirementIntakeLockDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"intake","type":"human_task","name":"需求录入","config":{"requirement_intake_lock":true}},
{"id":"review","type":"human_task","name":"人工复核"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"intake"},
{"id":"e2","source":"intake","target":"review","kind":"complete"},
{"id":"e3","source":"review","target":"end","kind":"complete"}]}`

const plainPlanUnitDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"task","type":"human_task","name":"普通任务"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"task"},
{"id":"e2","source":"task","target":"end","kind":"complete"}]}`

const intakeRequirementWorkflowDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"dev","type":"human_task","name":"开发"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"dev"},
{"id":"e2","source":"dev","target":"end","kind":"complete"}]}`

func setupRequirementIntakeLockFixture(t *testing.T) (*testApp, uint, string) {
	t.Helper()
	app := newTestApp(t, nil)
	user := app.createUser(t, 1, "intake-owner", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "AI PM", OwnerUserID: &user.ID}
	unit := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "1.2", Type: "version"}
	sibling := projectgorm.PlanUnit{ID: 13, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "1.3", Type: "version"}
	for _, row := range []any{&workspace, &team, &project, &unit, &sibling} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedWorkflowTemplate(t, app, 51, 61, "requirement", intakeRequirementWorkflowDefinition)
	return app, user.ID, app.token(t, user)
}

func seedWorkflowTemplate(t *testing.T, app *testApp, templateID, versionID uint, subjectType, definition string) {
	t.Helper()
	now := time.Now().UTC()
	template := workflowgorm.WorkflowTemplate{
		ID: templateID, WorkspaceID: 1, Name: "seed-template", SubjectType: subjectType,
		DraftDefinition: definition, CreatedByUserID: 1, UpdatedByUserID: 1,
		PublishedVersion: 1,
	}
	version := workflowgorm.WorkflowTemplateVersion{
		ID: versionID, WorkspaceID: 1, TemplateID: templateID, Version: 1,
		Definition: definition, DefinitionHash: "seed-hash",
		PublishedByUser: 1, PublishedAt: now,
	}
	if err := app.db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
}

func seedPlanUnitWorkflowInstance(t *testing.T, app *testApp, instanceID, planUnitID, assigneeID uint, nodeID, definition string) {
	t.Helper()
	now := time.Now().UTC()
	template := workflowgorm.WorkflowTemplate{
		ID: 52, WorkspaceID: 1, Name: "project-flow", SubjectType: "plan_unit",
		DraftDefinition: definition, CreatedByUserID: 1, UpdatedByUserID: 1, PublishedVersion: 1,
	}
	version := workflowgorm.WorkflowTemplateVersion{
		ID: 62, WorkspaceID: 1, TemplateID: 52, Version: 1,
		Definition: definition, DefinitionHash: "hash", PublishedByUser: 1, PublishedAt: now,
	}
	if err := app.db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	instance := workflowgorm.WorkflowInstance{
		ID: instanceID, WorkspaceID: 1, TemplateID: 52, TemplateVersionID: 62,
		SubjectType: "plan_unit", SubjectID: planUnitID, Status: "running",
		CurrentNodeIDs: `["` + nodeID + `"]`, NodePlansJSON: "{}",
		StartedByUserID: assigneeID, StartedAt: now, UpdatedAt: now,
	}
	token := workflowgorm.WorkflowToken{
		WorkspaceID: 1, InstanceID: instanceID, NodeID: nodeID, Status: "active",
		AssigneeUserID: &assigneeID, CreatedAt: now, UpdatedAt: now,
	}
	startRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: instanceID, NodeID: "start", NodeType: "start",
		Status: "completed", StartedAt: now, CompletedAt: &now,
	}
	for _, row := range []any{&instance, &token, &startRun} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func lockPlanUnitDirectly(t *testing.T, app *testApp, unitID, userID uint) time.Time {
	t.Helper()
	lockedAt := time.Now().UTC().Truncate(time.Second)
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("id = ?", unitID).
		Updates(map[string]any{"requirements_locked_at": lockedAt, "requirements_locked_by_user_id": userID}).Error; err != nil {
		t.Fatal(err)
	}
	return lockedAt
}

func (app *testApp) planUnit(t *testing.T, id uint) projectgorm.PlanUnit {
	t.Helper()
	var unit projectgorm.PlanUnit
	if err := app.db.First(&unit, id).Error; err != nil {
		t.Fatal(err)
	}
	return unit
}

// seedEnteredRequirement 在项目 12 的范围内放一条已录入需求；完成需求录入
// 节点（空范围防呆）要求范围非空。
func seedEnteredRequirement(t *testing.T, app *testApp) {
	t.Helper()
	unitID := uint(12)
	requirement := requirementgorm.Requirement{
		ID: 80, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: &unitID, Title: "已录入需求", Status: "backlog", Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
}

func TestRequirementIntakeCompletionRequiresLockConfirmation(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "intake"})
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "REQUIREMENT_LOCK_CONFIRMATION_REQUIRED") {
		t.Fatalf("expected confirmation code, got %s", rec.Body.String())
	}
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt != nil {
		t.Fatal("plan unit must not be locked before confirmation")
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || instance.CurrentNodeIDs != `["intake"]` {
		t.Fatalf("node must stay active, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	var activeToken workflowgorm.WorkflowToken
	if err := app.db.Where("instance_id = ? AND node_id = ?", 71, "intake").First(&activeToken).Error; err != nil {
		t.Fatal(err)
	}
	if activeToken.Status != "active" {
		t.Fatalf("token must stay active, got %s", activeToken.Status)
	}
}

func TestRequirementIntakeCompletionLocksPlanUnitAtomically(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedEnteredRequirement(t, app)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake", "comment": "录入完成",
			"confirm_requirement_lock": true,
			"next_node_assignees":      []map[string]any{{"node_id": "review", "assignee_user_id": userID}},
		})
	requireStatus(t, rec, http.StatusOK)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt == nil {
		t.Fatal("plan unit must be locked after confirmation")
	}
	if unit.RequirementsLockedByUserID == nil || *unit.RequirementsLockedByUserID != userID {
		t.Fatalf("locker mismatch: %v", unit.RequirementsLockedByUserID)
	}
	lockedAt := *unit.RequirementsLockedAt
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || instance.CurrentNodeIDs != `["review"]` {
		t.Fatalf("flow must advance to review, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}

	// Roll back to the intake node, then complete it again: the original lock
	// record must survive untouched (L16/L17).
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "rollback", "node_id": "review", "target_node_id": "intake", "comment": "回退补录"})
	requireStatus(t, rec, http.StatusOK)
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake", "comment": "再次完成",
			"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": userID}},
		})
	requireStatus(t, rec, http.StatusOK)
	unit = app.planUnit(t, 12)
	if unit.RequirementsLockedByUserID == nil || *unit.RequirementsLockedByUserID != userID {
		t.Fatalf("locker must stay the first user: %v", unit.RequirementsLockedByUserID)
	}
	if !unit.RequirementsLockedAt.Equal(lockedAt) {
		t.Fatalf("lock time must not be rewritten: %v vs %v", lockedAt, *unit.RequirementsLockedAt)
	}
}

// 纯锁版端点：不经工作流节点直接锁版，幂等且共用空范围防呆口径。
func TestPlanUnitRequirementsLockEndpoint(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedEnteredRequirement(t, app)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/project-new/12/requirements-lock", token, 1, nil)
	requireStatus(t, rec, http.StatusOK)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt == nil || unit.RequirementsLockedByUserID == nil || *unit.RequirementsLockedByUserID != userID {
		t.Fatalf("lock facts missing: %+v", unit)
	}
	lockedAt := *unit.RequirementsLockedAt
	rec = app.request(t, http.MethodPost, "/api/v1/platform/project-new/12/requirements-lock", token, 1, nil)
	requireStatus(t, rec, http.StatusOK)
	unit = app.planUnit(t, 12)
	if !unit.RequirementsLockedAt.Equal(lockedAt) {
		t.Fatal("idempotent lock must keep the first lock time")
	}
	// 空范围防呆：没有需求的单元同样拦截。
	rec = app.request(t, http.MethodPost, "/api/v1/platform/project-new/13/requirements-lock", token, 1, nil)
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "REQUIREMENT_SCOPE_EMPTY") {
		t.Fatalf("expected empty-scope conflict, got %s", rec.Body.String())
	}
	// 跨工作区不知道该单元的存在。
	rec = app.request(t, http.MethodPost, "/api/v1/platform/project-new/12/requirements-lock", token, 2, nil)
	requireStatus(t, rec, http.StatusNotFound)
}

// 列表行的锁版快捷操作事实：流程停在需求录入节点且未锁版时返回待录入
// 标记与节点 ID；锁版后不再标记。
func TestPlanUnitListExposesIntakePendingLockTarget(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	rows := decodeData(t, app.request(t, http.MethodGet,
		"/api/v1/platform/project-new?project_new_id=11", token, 1, nil)).([]any)
	var row map[string]any
	for _, raw := range rows {
		item := raw.(map[string]any)
		if item["id"] == float64(12) {
			row = item
			break
		}
	}
	if row == nil {
		t.Fatal("plan unit 12 missing from list")
	}
	if row["workflow_intake_pending"] != true || row["workflow_intake_node_id"] != "intake" {
		t.Fatalf("intake pending facts missing, got %v / %v", row["workflow_intake_pending"], row["workflow_intake_node_id"])
	}
	lockPlanUnitDirectly(t, app, 12, userID)
	rows = decodeData(t, app.request(t, http.MethodGet,
		"/api/v1/platform/project-new?project_new_id=11", token, 1, nil)).([]any)
	for _, raw := range rows {
		item := raw.(map[string]any)
		if item["id"] == float64(12) && item["workflow_intake_pending"] == true {
			t.Fatalf("locked unit must not be intake pending, got %v", item)
		}
	}
}

// 空范围防呆：范围里没有需求时，需求录入不能完成也不能锁版（防止空项目
// 锁版后阶段门禁死锁）。
func TestRequirementIntakeCompletionBlockedOnEmptyScope(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake", "confirm_requirement_lock": true,
			"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": userID}},
		})
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "REQUIREMENT_SCOPE_EMPTY") {
		t.Fatalf("expected empty-scope conflict, got %s", rec.Body.String())
	}
	if unit := app.planUnit(t, 12); unit.RequirementsLockedAt != nil {
		t.Fatal("empty scope must not lock the plan unit")
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.CurrentNodeIDs != `["intake"]` {
		t.Fatalf("node must stay active, got %s", instance.CurrentNodeIDs)
	}
}

func TestRequirementIntakeLockConfirmationFlagIsRestricted(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "task", plainPlanUnitDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "task", "confirm_requirement_lock": true})
	requireStatus(t, rec, http.StatusBadRequest)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt != nil {
		t.Fatal("unrelated node must not lock the plan unit")
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.CurrentNodeIDs != `["task"]` {
		t.Fatalf("unrelated node must not complete with the flag, got %s", instance.CurrentNodeIDs)
	}
}

// 锁版语义（用户确认）：完成需求录入只结束当前节点；未指定下一节点负责人
// 时默认就是操作者本人，复合提交不因未选人阻塞。
func TestRequirementIntakeLockDefaultsNextAssigneeToOperator(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedEnteredRequirement(t, app)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake", "confirm_requirement_lock": true,
		})
	requireStatus(t, rec, http.StatusOK)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt == nil {
		t.Fatal("plan unit must be locked after confirmation")
	}
	var reviewToken workflowgorm.WorkflowToken
	if err := app.db.Where("instance_id = ? AND node_id = ?", 71, "review").First(&reviewToken).Error; err != nil {
		t.Fatal(err)
	}
	if reviewToken.AssigneeUserID == nil || *reviewToken.AssigneeUserID != userID {
		t.Fatalf("next node must default to the operator, got %v", reviewToken.AssigneeUserID)
	}
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.CurrentNodeIDs != `["review"]` {
		t.Fatalf("flow must advance to review, got %s", instance.CurrentNodeIDs)
	}
}

func TestIntakeNodeDTOExposesLockConfirmationFact(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", requirementIntakeLockDefinition)
	data := decodeData(t, app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/71", token, 1, nil))
	nodes, ok := data.(map[string]any)["timeline_nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatal("timeline nodes missing")
	}
	found := false
	for _, raw := range nodes {
		node := raw.(map[string]any)
		if node["id"] == "intake" {
			found = true
			if node["requires_requirement_lock_confirmation"] != true {
				t.Fatalf("intake node must require lock confirmation, got %v", node["requires_requirement_lock_confirmation"])
			}
		}
	}
	if !found {
		t.Fatal("intake node missing from timeline")
	}
}

func TestLockedPlanUnitBlocksRequirementCreationOnAllHTTPPaths(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	lockPlanUnitDirectly(t, app, 12, userID)
	createBody := map[string]any{
		"version_task_new_id": 12, "title": "新需求", "workflow_template_id": 51,
		"priority": "p1",
	}
	rec := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, 1, createBody)
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "PROJECT_REQUIREMENTS_LOCKED") {
		t.Fatalf("project-scoped create must be blocked, got %s", rec.Body.String())
	}
	rec = app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, 1, map[string]any{
		"priority":       "p1",
		"project_new_id": 11, "version_task_new_id": 12, "title": "新需求", "workflow_template_id": 51,
	})
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "PROJECT_REQUIREMENTS_LOCKED") {
		t.Fatalf("pool create must be blocked, got %s", rec.Body.String())
	}
	// Same project set, unlocked sibling stays creatable (L13).
	rec = app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, 1, map[string]any{
		"priority":            "p1",
		"version_task_new_id": 13, "title": "旁支需求", "workflow_template_id": 51,
	})
	requireStatus(t, rec, http.StatusCreated)
	// Other workspaces learn nothing about the locked unit (L19).
	rec = app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, 2, map[string]any{
		"priority":            "p1",
		"version_task_new_id": 12, "title": "越权需求", "workflow_template_id": 51,
	})
	requireStatus(t, rec, http.StatusNotFound)
}

func TestLockedPlanUnitStillAllowsMove(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	lockPlanUnitDirectly(t, app, 12, userID)
	sourceUnitID := uint(13)
	requirement := requirementgorm.Requirement{
		ID: 81, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: &sourceUnitID, Title: "已有需求", Status: "development", Version: 1,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	// Moving the existing requirement into the locked unit stays allowed (L10).
	rec := app.request(t, http.MethodPost, "/api/v1/platform/requirements/batch-move", token, 1, map[string]any{
		"version_task_new_id": 12,
		"items":               []map[string]any{{"requirement_id": 81, "expected_version": 1}},
	})
	requireStatus(t, rec, http.StatusOK)
	var moved requirementgorm.Requirement
	if err := app.db.First(&moved, 81).Error; err != nil {
		t.Fatal(err)
	}
	if moved.ScheduledPlanUnitID == nil || *moved.ScheduledPlanUnitID != 12 {
		t.Fatalf("requirement must move into the locked unit, got %v", moved.ScheduledPlanUnitID)
	}
}

const nameMatchedIntakeDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"intake","type":"human_task","name":"需求录入"},
{"id":"review","type":"human_task","name":"产品评审"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"intake"},
{"id":"e2","source":"intake","target":"review","kind":"complete"},
{"id":"e3","source":"review","target":"end","kind":"complete"}]}`

const explicitOptOutIntakeDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"intake","type":"human_task","name":"需求录入","config":{"requirement_intake_lock":false}},
{"id":"review","type":"human_task","name":"产品评审"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"intake"},
{"id":"e2","source":"intake","target":"review","kind":"complete"},
{"id":"e3","source":"review","target":"end","kind":"complete"}]}`

// L15：未显式配置时，名称含「需求录入」的项目节点按历史兼容默认要求锁版确认。
func TestNameMatchedIntakeNodeRequiresLockConfirmation(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedEnteredRequirement(t, app)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", nameMatchedIntakeDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "intake"})
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "REQUIREMENT_LOCK_CONFIRMATION_REQUIRED") {
		t.Fatalf("name-matched intake node must require confirmation, got %s", rec.Body.String())
	}
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake", "confirm_requirement_lock": true,
			"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": userID}},
		})
	requireStatus(t, rec, http.StatusOK)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt == nil {
		t.Fatal("confirmed completion must lock the plan unit")
	}
}

// L15：显式关闭（requirement_intake_lock=false）时不再按名称兜底触发。
func TestExplicitLockOptOutSkipsConfirmation(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "intake", explicitOptOutIntakeDefinition)
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{
			"action": "complete", "node_id": "intake",
			"next_node_assignees": []map[string]any{{"node_id": "review", "assignee_user_id": userID}},
		})
	requireStatus(t, rec, http.StatusOK)
	unit := app.planUnit(t, 12)
	if unit.RequirementsLockedAt != nil {
		t.Fatal("explicit opt-out must not lock the plan unit")
	}
}
