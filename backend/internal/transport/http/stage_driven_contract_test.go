package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const stageDrivenPlanUnitDefinition = `{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"kickoff","type":"human_task","name":"启动"},
{"id":"dev-node","type":"human_task","name":"开发与实施"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"kickoff"},
{"id":"e2","source":"kickoff","target":"dev-node","kind":"complete"},
{"id":"e3","source":"dev-node","target":"end","kind":"complete"}]}`

func seedStageDrivenFixture(t *testing.T, app *testApp, userID uint, requirementStatus string) {
	t.Helper()
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "kickoff", stageDrivenPlanUnitDefinition)
	now := time.Now().UTC()
	requirement := requirementgorm.Requirement{
		ID: 90, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: uintPtr(12), WorkflowInstanceID: uintPtr(72),
		Title: "联动需求", Status: requirementStatus, Version: 1,
	}
	requirementInstance := workflowgorm.WorkflowInstance{
		ID: 72, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: 90, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}", StartedByUserID: userID, StartedAt: now,
	}
	for _, row := range []any{&requirement, &requirementInstance} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestStageDrivenNodeWaitsForRequirementStage(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedStageDrivenFixture(t, app, userID, "development")
	// 完成启动节点：开发与实施按名称等待所有需求进入验收中（当前研发中 → waiting）。
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "kickoff"})
	requireStatus(t, rec, http.StatusOK)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "running" || instance.CurrentNodeIDs != `["dev-node"]` {
		t.Fatalf("stage node must wait, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	data := decodeData(t, app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/71", token, 1, nil))
	node := nodeFromTimeline(t, data, "dev-node")
	if node["status"] != "waiting" {
		t.Fatalf("stage node must render waiting, got %v", node["status"])
	}
	evaluation, ok := node["requirements_guard"].(map[string]any)
	if !ok || evaluation["state"] != "unsatisfied" || evaluation["total_count"] != float64(1) {
		t.Fatalf("stage evaluation missing, got %v", node["requirements_guard"])
	}
	// 需求是唯一的驱动者：等待中的阶段节点不接受人工跳过。
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "dev-node"})
	requireStatus(t, rec, http.StatusConflict)
}

func TestStageDrivenNodeAutoAdvancesWhenStageReached(t *testing.T) {
	app, userID, token := setupRequirementIntakeLockFixture(t)
	seedStageDrivenFixture(t, app, userID, "acceptance")
	// 完成启动节点：所有需求已进入验收中 → 开发与实施自动完成，实例结束。
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "kickoff"})
	requireStatus(t, rec, http.StatusOK)
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, 71).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "completed" {
		t.Fatalf("satisfied stage node must auto-advance, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	var autoRuns int64
	app.db.Model(&workflowgorm.WorkflowNodeRun{}).
		Where("instance_id = ? AND node_id = ? AND output_json = ?", 71, "dev-node", `{"system_actor":true}`).
		Count(&autoRuns)
	if autoRuns != 1 {
		t.Fatalf("system-actor run missing, got %d", autoRuns)
	}
}

func TestStageDrivenConfigValidation(t *testing.T) {
	definition, issues := compileWorkflowDefinition(`{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"dev","type":"human_task","name":"开发","config":{"requirement_stage":"shipping"}},
{"id":"end","type":"end","name":"结束"}],
"edges":[{"id":"e1","source":"start","target":"dev"},{"id":"e2","source":"dev","target":"end","kind":"complete"}]}`)
	issues = append(issues, validateWorkflowCapabilities(definition, "plan_unit")...)
	if len(issues) == 0 || !strings.Contains(strings.Join(issues, "；"), "需求阶段无效") {
		t.Fatalf("invalid requirement_stage must fail validation, got %v", issues)
	}
}
