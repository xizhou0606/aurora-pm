package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

const requirementsGuardPlanUnitDefinition = `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev-gate","type":"human_task","name":"开发与实施","config":{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":61,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"enable_action","empty_scope":"block","missing_binding":"block"}}},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev-gate"},{"id":"e2","source":"dev-gate","target":"end","kind":"complete"}]}`

func setupRequirementsGuardFixture(t *testing.T) (*testApp, uint, string) {
	t.Helper()
	app, userID, token := setupRequirementIntakeLockFixture(t)
	// Requirement 81 lives on plan unit 12 and runs the requirement workflow
	// with an active "dev" token.
	requirement := requirementgorm.Requirement{
		ID: 81, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: guardUintPtr(12), WorkflowInstanceID: guardUintPtr(72),
		Title: "联动需求", Status: "development", Version: 1,
	}
	now := time.Now().UTC()
	instance := workflowgorm.WorkflowInstance{
		ID: 72, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: 81, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}",
		StartedByUserID: userID, StartedAt: now, UpdatedAt: now,
	}
	tokenRow := workflowgorm.WorkflowToken{
		WorkspaceID: 1, InstanceID: 72, NodeID: "dev", Status: "active",
		AssigneeUserID: &userID, CreatedAt: now, UpdatedAt: now,
	}
	startRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: 72, NodeID: "start", NodeType: "start",
		Status: "completed", StartedAt: now, CompletedAt: &now,
	}
	for _, row := range []any{&requirement, &instance, &tokenRow, &startRun} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Plan unit 12 runs the project workflow with a guarded development node.
	seedPlanUnitWorkflowInstance(t, app, 71, 12, userID, "dev-gate", requirementsGuardPlanUnitDefinition)
	return app, userID, token
}

func guardUintPtr(value uint) *uint { return &value }

func nodeFromTimeline(t *testing.T, data any, nodeID string) map[string]any {
	t.Helper()
	nodes, ok := data.(map[string]any)["timeline_nodes"].([]any)
	if !ok {
		t.Fatal("timeline nodes missing")
	}
	for _, raw := range nodes {
		node := raw.(map[string]any)
		if node["id"] == nodeID {
			return node
		}
	}
	t.Fatalf("node %s missing from timeline", nodeID)
	return nil
}

func TestRequirementsGuardBlocksCompletionUntilEvidenceSatisfied(t *testing.T) {
	app, _, token := setupRequirementsGuardFixture(t)
	recData := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/71", token, 1, nil)
	if recData.Code != http.StatusOK {
		t.Fatalf("instance get failed: %d %s", recData.Code, recData.Body.String())
	}
	data := decodeData(t, recData)
	gate := nodeFromTimeline(t, data, "dev-gate")
	evaluation, ok := gate["requirements_guard"].(map[string]any)
	if !ok {
		t.Fatalf("guarded node must expose its evaluation, got %v", gate)
	}
	if evaluation["state"] != "unsatisfied" || evaluation["total_count"].(float64) != 1 {
		t.Fatalf("expected unsatisfied 0/1, got %v", evaluation)
	}
	actions, _ := gate["allowed_actions"].([]any)
	for _, action := range actions {
		if action == "complete" {
			t.Fatal("complete must be withdrawn while the guard is unsatisfied")
		}
	}
	// The action endpoint enforces the same rule inside its transaction.
	rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "dev-gate"})
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), "WORKFLOW_REQUIREMENTS_GUARD_BLOCKED") {
		t.Fatalf("expected guard conflict, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "requirements_guard") {
		t.Fatal("guard conflict must carry the evaluation payload")
	}

	// Completing the requirement's dev node satisfies the evidence; the
	// guarded node then completes normally (W04/W05 semantics).
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/72/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "dev"})
	requireStatus(t, rec, http.StatusOK)
	rec = app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/71/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "dev-gate"})
	requireStatus(t, rec, http.StatusOK)
}

func TestRequirementsGuardSeesNewlyMovedInRequirements(t *testing.T) {
	app, userID, token := setupRequirementsGuardFixture(t)
	// Requirement 81 completes its dev node.
	if rec := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/72/actions",
		token, 1, map[string]any{"action": "complete", "node_id": "dev"}); rec.Code != http.StatusOK {
		t.Fatalf("requirement dev completion failed: %s", rec.Body.String())
	}
	// A second requirement scheduled to the unit but with its own workflow
	// still pending joins the denominator after the move.
	secondUnitID := uint(13)
	second := requirementgorm.Requirement{
		ID: 82, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		ScheduledPlanUnitID: &secondUnitID, WorkflowInstanceID: guardUintPtr(73),
		Title: "移入需求", Status: "development", Version: 1,
	}
	now := time.Now().UTC()
	secondInstance := workflowgorm.WorkflowInstance{
		ID: 73, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: 82, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}",
		StartedByUserID: userID, StartedAt: now, UpdatedAt: now,
	}
	secondToken := workflowgorm.WorkflowToken{
		WorkspaceID: 1, InstanceID: 73, NodeID: "dev", Status: "active",
		AssigneeUserID: &userID, CreatedAt: now, UpdatedAt: now,
	}
	secondStart := workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: 73, NodeID: "start", NodeType: "start",
		Status: "completed", StartedAt: now, CompletedAt: &now,
	}
	for _, row := range []any{&second, &secondInstance, &secondToken, &secondStart} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", 82).
		Update("version_task_new_id", 12).Error; err != nil {
		t.Fatal(err)
	}
	data := decodeData(t, app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/71", token, 1, nil))
	evaluation := nodeFromTimeline(t, data, "dev-gate")["requirements_guard"].(map[string]any)
	if evaluation["total_count"].(float64) != 2 || evaluation["state"] != "unsatisfied" {
		t.Fatalf("moved-in requirement must join the denominator, got %v", evaluation)
	}
}

func TestRequirementsGuardBindingValidation(t *testing.T) {
	app, _, _ := setupRequirementsGuardFixture(t)
	valid, _ := compileWorkflowDefinition(requirementsGuardPlanUnitDefinition)
	if issues := (&Server{db: app.db}).validateWorkflowRequirementsGuardBindings(1, valid); len(issues) != 0 {
		t.Fatalf("valid bindings must pass, got %v", issues)
	}
	unknownVersion, _ := compileWorkflowDefinition(strings.Replace(
		requirementsGuardPlanUnitDefinition, `"requirement_template_version_id":61`, `"requirement_template_version_id":999`, 1))
	if issues := (&Server{db: app.db}).validateWorkflowRequirementsGuardBindings(1, unknownVersion); len(issues) == 0 {
		t.Fatal("unknown binding version must fail")
	}
	unknownNode, _ := compileWorkflowDefinition(strings.Replace(
		requirementsGuardPlanUnitDefinition, `"node_id":"dev"`, `"node_id":"missing"`, 1))
	if issues := (&Server{db: app.db}).validateWorkflowRequirementsGuardBindings(1, unknownNode); len(issues) == 0 {
		t.Fatal("unknown target node must fail")
	}
	wrongAction, _ := compileWorkflowDefinition(strings.Replace(
		requirementsGuardPlanUnitDefinition, `"success_action":"complete"`, `"success_action":"approve"`, 1))
	if issues := (&Server{db: app.db}).validateWorkflowRequirementsGuardBindings(1, wrongAction); len(issues) == 0 {
		t.Fatal("mismatched success action must fail")
	}
	// requirement_intake_lock is rejected on non human-task/form nodes and on
	// requirement templates.
	_, issues := compileWorkflowDefinition(`{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"gate","type":"approval","name":"审批","config":{"requirement_intake_lock":true}},
{"id":"end","type":"end","name":"结束"}],
"edges":[{"id":"e1","source":"start","target":"gate"},{"id":"e2","source":"gate","target":"end","kind":"approve"}]}`)
	if len(issues) == 0 {
		t.Fatal("requirement_intake_lock on approval node must fail structural validation")
	}
}
