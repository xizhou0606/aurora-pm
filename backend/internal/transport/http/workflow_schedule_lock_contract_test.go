package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestWorkflowScheduleTransactionsUseReadCommitted(t *testing.T) {
	if options := workflowScheduleTransactionOptions(); options.Isolation != sql.LevelReadCommitted {
		t.Fatalf("schedule isolation=%v, want READ COMMITTED", options.Isolation)
	}
}

func TestWorkflowNodePlanScheduleLockOrderAndCurrentRunRead(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "节点排期锁序", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "节点排期锁序"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	nextOwner := addWorkflowAssigneeUser(t, app, workspace.ID, unit.TeamID, 2, "lock-owner")

	events := make([]string, 0, 8)
	txConnection := ""
	instanceUpdated := false
	queryCallback := "test:schedule_node_plan_query_order"
	if err := app.db.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		connection := scheduleTestConnection(tx)
		_, locked := tx.Statement.Clauses["FOR"]
		switch tx.Statement.Schema.Name {
		case "ProjectSet":
			if locked {
				events = append(events, "project")
				txConnection = connection
			}
		case "Requirement":
			if locked && connection == txConnection {
				events = append(events, "requirement")
			}
		case "PlanUnit":
			if locked && connection == txConnection {
				events = append(events, "plan_unit")
			}
		case "WorkflowNodeRun":
			if instanceUpdated && connection == txConnection {
				if locked {
					tx.AddError(fmt.Errorf("workflow node runs must use a plain current read"))
				}
				events = append(events, "runs")
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	updateCallback := "test:schedule_node_plan_update_order"
	if err := app.db.Callback().Update().Before("gorm:update").Register(updateCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || scheduleTestConnection(tx) != txConnection {
			return
		}
		switch tx.Statement.Schema.Name {
		case "WorkflowToken":
			events = append(events, "token")
		case "WorkflowInstance":
			events = append(events, "instance")
			instanceUpdated = true
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = app.db.Callback().Query().Remove(queryCallback)
		_ = app.db.Callback().Update().Remove(updateCallback)
	})

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil,
			"planned_end_at": "2026-08-20", "expected_planned_end_at": nil, "assignee_user_id": nextOwner.ID,
		}}})
	requireStatus(t, response, http.StatusOK)
	for _, pair := range [][2]string{
		{"project", "token"}, {"token", "instance"}, {"instance", "requirement"},
		{"requirement", "plan_unit"}, {"plan_unit", "runs"},
	} {
		assertScheduleEventBefore(t, events, pair[0], pair[1])
	}
}

func TestPlanUnitScheduleLocksInstancesBeforePlanUnitAndReadsRunsPlain(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "版本排期锁序", scheduleSingleDefinition())
	createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "版本排期锁序"})
	events, txConnection := make([]string, 0, 5), ""
	callback := "test:schedule_plan_unit_query_order"
	if err := app.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		connection := scheduleTestConnection(tx)
		_, locked := tx.Statement.Clauses["FOR"]
		switch tx.Statement.Schema.Name {
		case "ProjectSet":
			if locked {
				txConnection = connection
				events = append(events, "project")
			}
		case "WorkflowInstance":
			if locked && connection == txConnection {
				events = append(events, "instance")
			}
		case "PlanUnit":
			if locked && connection == txConnection {
				events = append(events, "plan_unit")
			}
		case "WorkflowNodeRun":
			if connection == txConnection {
				if locked {
					tx.AddError(fmt.Errorf("workflow node runs must not be locked after the instance"))
				}
				events = append(events, "runs")
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })

	response := app.request(t, http.MethodPatch,
		"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
		map[string]any{"planned_end_at": "2026-08-30", "expected_planned_end_at": "2026-08-31"})
	requireStatus(t, response, http.StatusOK)
	for _, pair := range [][2]string{{"project", "instance"}, {"instance", "plan_unit"}, {"plan_unit", "runs"}} {
		assertScheduleEventBefore(t, events, pair[0], pair[1])
	}
}

func TestSameValueSchedulePayloadSkipsWorkflowLockFanout(t *testing.T) {
	for _, subject := range []string{"project", "plan_unit"} {
		t.Run(subject, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
			templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "同值排期不扩锁", scheduleSingleDefinition())
			createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "同值排期不扩锁"})
			projectLocked := false
			unexpectedLocks := make([]string, 0)
			callback := "test:same_value_schedule_" + subject
			if err := app.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				if _, locked := tx.Statement.Clauses["FOR"]; !locked {
					return
				}
				if tx.Statement.Schema.Name == "ProjectSet" {
					projectLocked = true
					return
				}
				if tx.Statement.Schema.Name == "WorkflowInstance" ||
					subject == "project" && tx.Statement.Schema.Name == "PlanUnit" {
					unexpectedLocks = append(unexpectedLocks, tx.Statement.Schema.Name)
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })
			path := "/api/v1/platform/project-sets/" + uintString(project.ID)
			body := map[string]any{
				"name": "同值项目改名", "planned_start_at": "2026-08-01", "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-31", "expected_planned_end_at": "2026-08-31",
			}
			if subject == "plan_unit" {
				path = "/api/v1/platform/project-new/" + uintString(unit.ID)
				body = map[string]any{
					"title": "同值版本改名", "planned_start_at": "2026-08-01", "expected_planned_start_at": "2026-08-01", "planned_end_at": "2026-08-31", "expected_planned_end_at": "2026-08-31",
				}
			}
			requireStatus(t, app.request(t, http.MethodPatch, path, token, workspace.ID, body), http.StatusOK)
			if !projectLocked || len(unexpectedLocks) > 0 {
				t.Fatalf("same-value schedule locks project=%t fanout=%v", projectLocked, unexpectedLocks)
			}
		})
	}
}

func TestConcurrentPlanUnitNarrowingWinsBeforeNodePlanLinearization(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "并发收窄线性化", scheduleSingleDefinition())
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "并发收窄线性化"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(2)
	triggered := false
	var narrowingStatus int
	auditsAfterNarrowing, eventsAfterNarrowing := int64(-1), int64(-1)
	callback := "test:narrow_plan_before_node_schedule_project_lock"
	if err := app.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if triggered || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "ProjectSet" {
			return
		}
		if _, locked := tx.Statement.Clauses["FOR"]; !locked {
			return
		}
		triggered = true
		narrowed := app.request(t, http.MethodPatch,
			"/api/v1/platform/project-new/"+uintString(unit.ID), token, workspace.ID,
			map[string]any{"planned_end_at": "2026-08-15", "expected_planned_end_at": "2026-08-31"})
		narrowingStatus = narrowed.Code
		auditsAfterNarrowing, eventsAfterNarrowing = allScheduleMutationCounts(t, app)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callback) })

	nodePlan := app.request(t, http.MethodPatch,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans",
		token, workspace.ID, map[string]any{"nodes": []map[string]any{{
			"node_id": "delivery", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil,
		}}})
	if !triggered || narrowingStatus != http.StatusOK {
		t.Fatalf("narrowing linearization triggered=%t status=%d", triggered, narrowingStatus)
	}
	requireStatus(t, nodePlan, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, nodePlan, "NODE_AFTER_SUBJECT_END")
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unit.ID).Error; err != nil {
		t.Fatal(err)
	}
	assertScheduleDate(t, storedUnit.PlannedEndAt, "2026-08-15")
	afterAudits, afterEvents := allScheduleMutationCounts(t, app)
	if afterAudits != auditsAfterNarrowing || afterEvents != eventsAfterNarrowing {
		t.Fatalf("rejected node plan leaked side effects audits=%d/%d events=%d/%d", afterAudits, auditsAfterNarrowing, afterEvents, eventsAfterNarrowing)
	}
}

func TestWorkflowNodePlanReloadsRunHistoryAfterInstanceCAS(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	setScheduleBoundaries(t, app, &project, &unit, "2026-08-01", "2026-08-31")
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "choice", "type": "condition", "name": "选择"},
			{"id": "left", "type": "human_task", "name": "左侧"},
			{"id": "right", "type": "human_task", "name": "右侧"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "choice"},
			{"id": "e2", "source": "choice", "target": "left", "kind": "yes"},
			{"id": "e3", "source": "choice", "target": "right", "kind": "no"},
			{"id": "e4", "source": "left", "target": "end"},
			{"id": "e5", "source": "right", "target": "end"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "事务内重读路径", definition)
	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "事务内重读路径"})
	instanceID := uint(created["workflow_instance_id"].(float64))
	instancePath := "/api/v1/platform/pm/workflows/instances/" + uintString(instanceID)
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{"action": "condition_yes", "node_id": "choice"}), http.StatusOK)
	var before workflowgorm.WorkflowInstance
	if err := app.db.First(&before, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	var conditionRun workflowgorm.WorkflowNodeRun
	if err := app.db.Where("workspace_id = ? AND instance_id = ? AND node_id = ?", workspace.ID, instanceID, "choice").
		Order("id DESC").First(&conditionRun).Error; err != nil {
		t.Fatal(err)
	}
	originalOutput := conditionRun.OutputJSON
	triggered := false
	callback := "test:change_condition_history_before_node_plan_cas"
	if err := app.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if triggered || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "WorkflowInstance" {
			return
		}
		triggered = true
		err := tx.Session(&gorm.Session{NewDB: true}).Model(&workflowgorm.WorkflowNodeRun{}).
			Where("id = ? AND workspace_id = ?", conditionRun.ID, workspace.ID).
			UpdateColumn("output_json", encodeWorkflowRunDecision(1, "", false, "", "right")).Error
		if err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Update().Remove(callback) })

	response := app.request(t, http.MethodPatch, instancePath+"/node-plans", token, workspace.ID,
		map[string]any{"nodes": []map[string]any{{"node_id": "left", "planned_end_at": "2026-08-20", "expected_planned_end_at": nil}}})
	requireStatus(t, response, http.StatusConflict)
	if !triggered {
		t.Fatal("condition history callback did not run")
	}
	assertInstancePlansJSON(t, app, instanceID, before.NodePlansJSON)
	if err := app.db.First(&conditionRun, conditionRun.ID).Error; err != nil {
		t.Fatal(err)
	}
	if conditionRun.OutputJSON != originalOutput {
		t.Fatalf("rejected stale topology update leaked run mutation: %s", conditionRun.OutputJSON)
	}
}

func scheduleTestConnection(tx *gorm.DB) string {
	return fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
}

func assertScheduleEventBefore(t *testing.T, events []string, left, right string) {
	t.Helper()
	leftIndex, rightIndex := -1, -1
	for index, event := range events {
		if event == left && leftIndex < 0 {
			leftIndex = index
		}
		if event == right && rightIndex < 0 {
			rightIndex = index
		}
	}
	if leftIndex < 0 || rightIndex < 0 || leftIndex >= rightIndex {
		t.Fatalf("schedule lock events=%v, want %s before %s", events, left, right)
	}
}
