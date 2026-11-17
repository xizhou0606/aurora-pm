package httpapi

import (
	"net/http"
	"testing"
	"time"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const deliveryProgressPlanUnitDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"dev-gate","type":"requirement_gate","name":"等待开发完成","config":{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":61,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"auto_complete","empty_scope":"block","missing_binding":"block"}}},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"dev-gate"},
{"id":"e2","source":"dev-gate","target":"end"}],
"delivery_progress":{"schema_version":1,"milestones":[{"key":"development_completed","name":"开发完成","weight":100,"is_acceptance":false,"bindings":[{"requirement_template_version_id":61,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}]}]}}`

func seedDeliveryProgressPlanUnit(t *testing.T, app *testApp, userID, unitID, requirementID, instanceID uint) {
	t.Helper()
	seedPlanUnitWorkflowInstance(t, app, 71, unitID, userID, "dev-gate", deliveryProgressPlanUnitDefinition)
	now := time.Now().UTC()
	requirement := requirementgorm.Requirement{
		ID: requirementID, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: &unitID, WorkflowInstanceID: uintPtr(instanceID),
		Title: "联动需求", Status: "development", Version: 1,
	}
	requirementInstance := workflowgorm.WorkflowInstance{
		ID: instanceID, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: requirementID, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}",
		StartedByUserID: userID, StartedAt: now, UpdatedAt: now,
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&requirementInstance).Error; err != nil {
		t.Fatal(err)
	}
}

func completeRequirementDevRun(t *testing.T, app *testApp, instanceID uint) {
	t.Helper()
	now := time.Now().UTC()
	if err := app.db.Exec(
		"INSERT INTO workflow_node_runs (workspace_id, instance_id, node_id, node_type, status, output_json, started_at, completed_at) VALUES (1, ?, 'dev', 'human_task', 'completed', '{}', ?, ?)",
		instanceID, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
}

func assertDeliveryScopeRevision(t *testing.T, app *testApp, unitID uint, expected int64) {
	t.Helper()
	var unit projectgorm.PlanUnit
	if err := app.db.First(&unit, unitID).Error; err != nil {
		t.Fatal(err)
	}
	if unit.DeliveryScopeRevision != expected {
		t.Fatalf("unit %d revision = %d, want %d", unitID, unit.DeliveryScopeRevision, expected)
	}
}

func TestPlanUnitDeliveryProgressDTO(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedDeliveryProgressPlanUnit(t, app, userID, 12, 90, 72)
	data := decodeData(t, app.request(t, http.MethodGet, "/api/v1/platform/project-new/12", token, 1, nil)).(map[string]any)
	if data["delivery_progress_configured"] != true {
		t.Fatalf("delivery progress must be configured, got %v", data["delivery_progress_configured"])
	}
	if data["delivery_progress"] != 0.0 {
		t.Fatalf("no completed evidence yet, got %v", data["delivery_progress"])
	}
	if data["delivery_scope_count"] != float64(1) || data["delivery_scope_revision"] != float64(0) {
		t.Fatalf("scope facts missing: %v %v", data["delivery_scope_count"], data["delivery_scope_revision"])
	}
	if _, ok := data["delivery_milestones"].([]any); !ok {
		t.Fatalf("milestones summary missing: %v", data["delivery_milestones"])
	}

	// 证据满足后进度立即反映（批量预取路径）。
	completeRequirementDevRun(t, app, 72)
	data = decodeData(t, app.request(t, http.MethodGet, "/api/v1/platform/project-new/12", token, 1, nil)).(map[string]any)
	if data["delivery_progress"] != 100.0 {
		t.Fatalf("satisfied evidence must yield 100, got %v", data["delivery_progress"])
	}
}

func TestDeliveryScopeRevisionBumpsOnCreateAndMove(t *testing.T) {
	app, _, token := setupRequirementIntakeLockFixture(t)
	seedDeliveryProgressPlanUnit(t, app, uint(1), 12, 90, 72)
	// 直接把联动需求排到版本 13，作为移动源。
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", 90).
		Update("version_task_new_id", 13).Error; err != nil {
		t.Fatal(err)
	}
	// 创建需求：目标版本 revision +1。
	rec := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, 1, map[string]any{
		"priority":            "p1",
		"version_task_new_id": 13, "title": "新需求", "workflow_template_id": 51,
	})
	requireStatus(t, rec, http.StatusCreated)
	assertDeliveryScopeRevision(t, app, 13, 1)
	assertDeliveryScopeRevision(t, app, 12, 0)
	// 移入版本 12：两端 revision 均递增。
	rec = app.request(t, http.MethodPost, "/api/v1/platform/requirements/batch-move", token, 1, map[string]any{
		"version_task_new_id": 12,
		"items":               []map[string]any{{"requirement_id": 90, "expected_version": 1}},
	})
	requireStatus(t, rec, http.StatusOK)
	assertDeliveryScopeRevision(t, app, 13, 2)
	assertDeliveryScopeRevision(t, app, 12, 1)
}

const gateAfterKickoffDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"kickoff","type":"human_task","name":"启动"},
{"id":"dev-gate","type":"requirement_gate","name":"等待开发完成","config":{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":61,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"auto_complete","empty_scope":"block","missing_binding":"block"}}},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"kickoff"},
{"id":"e2","source":"kickoff","target":"dev-gate","kind":"complete"},
{"id":"e3","source":"dev-gate","target":"end"}]}`

func TestRequirementGateAutoAdvancesOnEntry(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	// 实例停在启动节点，需求证据已满足。
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "kickoff", gateAfterKickoffDefinition)
	now := time.Now().UTC()
	requirement := requirementgorm.Requirement{
		ID: 90, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: uintPtr(12), WorkflowInstanceID: uintPtr(72),
		Title: "联动需求", Status: "development", Version: 1,
	}
	requirementInstance := workflowgorm.WorkflowInstance{
		ID: 72, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: 90, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}", StartedByUserID: userID, StartedAt: now,
	}
	devRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: 72, NodeID: "dev", NodeType: "human_task",
		Status: "completed", StartedAt: now, CompletedAt: &now,
	}
	for _, row := range []any{&requirement, &requirementInstance, &devRun} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 完成启动节点：gate 在同一事务内评估并自动完成，实例直接结束。
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "kickoff"})
	requireStatus(t, rec, http.StatusOK)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "completed" {
		t.Fatalf("satisfied gate must auto-advance on entry, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	var gateRun int64
	app.db.Model(&workflowgorm.WorkflowNodeRun{}).
		Where("instance_id = ? AND node_id = ? AND output_json = ?", 71, "dev-gate", `{"system_actor":true}`).
		Count(&gateRun)
	if gateRun != 1 {
		t.Fatalf("gate pass run missing, got %d", gateRun)
	}
}
