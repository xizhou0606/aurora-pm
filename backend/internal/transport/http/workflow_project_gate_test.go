package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowProjectGateActionFixture struct {
	app           *testApp
	token         string
	workspace     identitygorm.Workspace
	project       projectgorm.ProjectSet
	subjectName   string
	requirementID uint
	instanceID    uint
	body          map[string]any
}

func (fixture workflowProjectGateActionFixture) request(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	return fixture.app.workflowAction(t, fixture.workspace.ID, fixture.token, fixture.instanceID, fixture.body)
}

type workflowProjectGateState struct {
	projectLocked        bool
	subjectRead          bool
	queriesBeforeProject int
}

type workflowProjectGateProbe struct {
	mu          sync.Mutex
	projectID   uint
	subjectName string
	targets     map[string]struct{}
	states      map[string]*workflowProjectGateState
	writes      int
	violations  []string
}

func installWorkflowProjectGateProbe(
	t *testing.T,
	db *gorm.DB,
	projectID uint,
	subjectName string,
	targets ...string,
) *workflowProjectGateProbe {
	t.Helper()
	probe := &workflowProjectGateProbe{
		projectID: projectID, subjectName: subjectName,
		targets: map[string]struct{}{}, states: map[string]*workflowProjectGateState{},
	}
	for _, target := range targets {
		probe.targets[target] = struct{}{}
	}
	stateFor := func(tx *gorm.DB) *workflowProjectGateState {
		key := workflowProjectGateConnKey(tx)
		state := probe.states[key]
		if state == nil {
			state = &workflowProjectGateState{}
			probe.states[key] = state
		}
		return state
	}
	queryCallback := "test:workflow_project_gate_query"
	if err := db.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Error != nil || tx.Statement.Schema == nil {
			return
		}
		_, locked := tx.Statement.Clauses["FOR"]
		probe.mu.Lock()
		defer probe.mu.Unlock()
		state := stateFor(tx)
		switch tx.Statement.Schema.Name {
		case "ProjectSet":
			id, ok := workflowProjectGateLockedProjectID(tx.Statement.Dest)
			if locked && ok && id == probe.projectID {
				if state.queriesBeforeProject > 0 {
					probe.violations = append(probe.violations, fmt.Sprintf(
						"project lock followed %d transaction queries", state.queriesBeforeProject,
					))
					_ = tx.AddError(fmt.Errorf("project lock must be the first workflow transaction query"))
				}
				state.projectLocked = true
			}
		case probe.subjectName:
			if !state.projectLocked {
				state.queriesBeforeProject++
				return
			}
			if locked {
				probe.violations = append(probe.violations, "subject was locked before workflow mutation")
				_ = tx.AddError(fmt.Errorf("workflow project gate must not lock %s", probe.subjectName))
				return
			}
			state.subjectRead = true
		default:
			if !state.projectLocked {
				state.queriesBeforeProject++
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	verifyWrite := func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		if _, targeted := probe.targets[tx.Statement.Schema.Name]; !targeted {
			return
		}
		probe.mu.Lock()
		state := stateFor(tx)
		probe.writes++
		valid := state.projectLocked && state.subjectRead
		if !valid {
			probe.violations = append(probe.violations, fmt.Sprintf(
				"%s mutation occurred before project gate and subject reread", tx.Statement.Schema.Name,
			))
		}
		probe.mu.Unlock()
		if !valid {
			_ = tx.AddError(fmt.Errorf("workflow project gate missing before %s mutation", tx.Statement.Schema.Name))
		}
	}
	createCallback := "test:workflow_project_gate_create"
	updateCallback := "test:workflow_project_gate_update"
	deleteCallback := "test:workflow_project_gate_delete"
	if err := db.Callback().Create().Before("gorm:create").Register(createCallback, verifyWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register(updateCallback, verifyWrite); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register(deleteCallback, verifyWrite); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(queryCallback)
		_ = db.Callback().Create().Remove(createCallback)
		_ = db.Callback().Update().Remove(updateCallback)
		_ = db.Callback().Delete().Remove(deleteCallback)
	})
	return probe
}

func (probe *workflowProjectGateProbe) assert(t *testing.T) {
	t.Helper()
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.writes == 0 {
		t.Fatal("workflow project gate probe observed no target mutations")
	}
	if len(probe.violations) > 0 {
		t.Fatal(probe.violations)
	}
}

func TestWorkflowMutationTransactionsStartWithProjectGate(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T) workflowProjectGateActionFixture
	}{
		{name: "running rollback", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateRollback(t, false)
		}},
		{name: "completed rollback", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateRollback(t, true)
		}},
		{name: "plan unit running rollback", setup: setupWorkflowProjectGatePlanUnitRollback},
		{name: "legacy resume", setup: setupWorkflowProjectGateResume},
		{name: "current AI recheck", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateAIRecheck(t, false)
		}},
		{name: "historical AI recheck", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateAIRecheck(t, true)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := test.setup(t)
			subjectName := fixture.subjectName
			if subjectName == "" {
				subjectName = "Requirement"
			}
			probe := installWorkflowProjectGateProbe(
				t, fixture.app.db, fixture.project.ID, subjectName,
				"WorkflowToken", "WorkflowInstance", "WorkflowJob",
			)
			requireStatus(t, fixture.request(t), http.StatusOK)
			probe.assert(t)
		})
	}
}

func TestWorkflowMutationsRejectChangedOrDeletedSubjectAfterProjectGate(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T) workflowProjectGateActionFixture
		change string
	}{
		{name: "completed rollback moved", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateRollback(t, true)
		}, change: "move"},
		{name: "legacy resume deleted", setup: setupWorkflowProjectGateResume, change: "delete"},
		{name: "current AI recheck moved", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateAIRecheck(t, false)
		}, change: "move"},
		{name: "historical AI recheck deleted", setup: func(t *testing.T) workflowProjectGateActionFixture {
			return setupWorkflowProjectGateAIRecheck(t, true)
		}, change: "delete"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := test.setup(t)
			otherProject := projectgorm.ProjectSet{
				ID: 99, WorkspaceID: fixture.workspace.ID, TeamID: fixture.project.TeamID,
				Name: "Moved workflow subject", OwnerUserID: fixture.project.OwnerUserID,
			}
			if err := fixture.app.db.Create(&otherProject).Error; err != nil {
				t.Fatal(err)
			}
			triggered, workflowWrites := false, 0
			var mu sync.Mutex
			queryCallback := "test:change_workflow_subject_after_project_gate"
			if err := fixture.app.db.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
				if tx.Error != nil || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "ProjectSet" {
					return
				}
				if _, locked := tx.Statement.Clauses["FOR"]; !locked {
					return
				}
				id, ok := workflowProjectGateLockedProjectID(tx.Statement.Dest)
				mu.Lock()
				defer mu.Unlock()
				if triggered || !ok || id != fixture.project.ID {
					return
				}
				triggered = true
				currentTx := tx.Session(&gorm.Session{NewDB: true})
				var err error
				if test.change == "move" {
					err = currentTx.Model(&requirementgorm.Requirement{}).
						Where("id = ? AND workspace_id = ?", fixture.requirementID, fixture.workspace.ID).
						UpdateColumn("project_new_id", otherProject.ID).Error
				} else {
					err = currentTx.Where("id = ? AND workspace_id = ?", fixture.requirementID, fixture.workspace.ID).
						Delete(&requirementgorm.Requirement{}).Error
				}
				if err != nil {
					_ = tx.AddError(err)
				}
			}); err != nil {
				t.Fatal(err)
			}
			countWrite := func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				switch tx.Statement.Schema.Name {
				case "WorkflowToken", "WorkflowInstance", "WorkflowJob", "WorkflowNodeRun", "AIRun":
					mu.Lock()
					workflowWrites++
					mu.Unlock()
				}
			}
			createCallback := "test:count_changed_subject_workflow_create"
			updateCallback := "test:count_changed_subject_workflow_update"
			deleteCallback := "test:count_changed_subject_workflow_delete"
			if err := fixture.app.db.Callback().Create().Before("gorm:create").Register(createCallback, countWrite); err != nil {
				t.Fatal(err)
			}
			if err := fixture.app.db.Callback().Update().Before("gorm:update").Register(updateCallback, countWrite); err != nil {
				t.Fatal(err)
			}
			if err := fixture.app.db.Callback().Delete().Before("gorm:delete").Register(deleteCallback, countWrite); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = fixture.app.db.Callback().Query().Remove(queryCallback)
				_ = fixture.app.db.Callback().Create().Remove(createCallback)
				_ = fixture.app.db.Callback().Update().Remove(updateCallback)
				_ = fixture.app.db.Callback().Delete().Remove(deleteCallback)
			})

			response := fixture.request(t)
			requireStatus(t, response, http.StatusConflict)
			if !strings.Contains(response.Body.String(), "parent scope changed; refresh and retry") {
				t.Fatalf("scope conflict response=%s", response.Body.String())
			}
			mu.Lock()
			wasTriggered, writes := triggered, workflowWrites
			mu.Unlock()
			if !wasTriggered {
				t.Fatal("subject change callback did not run")
			}
			if writes != 0 {
				t.Fatalf("rejected workflow mutation performed %d workflow writes", writes)
			}
			var restored requirementgorm.Requirement
			if err := fixture.app.db.Where(
				"id = ? AND workspace_id = ?", fixture.requirementID, fixture.workspace.ID,
			).First(&restored).Error; err != nil {
				t.Fatal(err)
			}
			if restored.ProjectSetID != fixture.project.ID {
				t.Fatalf("subject change escaped rollback: project=%d", restored.ProjectSetID)
			}
		})
	}
}

func workflowProjectGateConnKey(tx *gorm.DB) string {
	return fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
}

func workflowProjectGateLockedProjectID(destination any) (uint, bool) {
	project, ok := destination.(*projectgorm.ProjectSet)
	if !ok || project == nil || project.ID == 0 {
		return 0, false
	}
	return project.ID, true
}

func setupWorkflowProjectGatePlanUnitRollback(t *testing.T) workflowProjectGateActionFixture {
	t.Helper()
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "Start"},
			{"id": "input", "type": "form", "name": "Input"},
			{"id": "review", "type": "approval", "name": "Review", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "End"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "Plan unit project gate rollback", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, createdTemplate, http.StatusCreated)
	templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish",
		token, workspace.ID, nil,
	), http.StatusOK)
	createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "Plan unit project gate rollback",
		"type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, createdUnit, http.StatusCreated)
	unitData := decodeData(t, createdUnit).(map[string]any)
	instanceID := uint(unitData["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	return workflowProjectGateActionFixture{
		app: app, token: token, workspace: workspace, project: project, subjectName: "PlanUnit",
		instanceID: instanceID,
		body: map[string]any{
			"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "Recheck",
		},
	}
}

func setupWorkflowProjectGateRollback(t *testing.T, completed bool) workflowProjectGateActionFixture {
	t.Helper()
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "Start"},
			{"id": "input", "type": "form", "name": "Input"},
			{"id": "review", "type": "approval", "name": "Review", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "End"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "Project gate rollback", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "Project gate rollback"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	if completed {
		requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
			"action": "approve", "node_id": "review",
		}), http.StatusOK)
	}
	return workflowProjectGateActionFixture{
		app: app, token: token, workspace: workspace, project: project,
		requirementID: uint(requirement["id"].(float64)), instanceID: instanceID,
		body: map[string]any{
			"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "Recheck",
		},
	}
}

func setupWorkflowProjectGateResume(t *testing.T) workflowProjectGateActionFixture {
	t.Helper()
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "Start"},
			{"id": "input", "type": "form", "name": "Input"},
			{"id": "review", "type": "approval", "name": "Review", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "End"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "review"},
			{"id": "e3", "source": "review", "target": "end", "kind": "approve"},
			{"id": "e4", "source": "review", "target": "input", "kind": "reject", "loop": true},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "Project gate resume", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "Project gate resume"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	now := time.Now().UTC()
	if err := app.db.Create(&workflowgorm.WorkflowNodeRun{
		WorkspaceID: workspace.ID, InstanceID: instanceID, NodeID: "review", NodeType: "approval",
		Status: "rejected", OutputJSON: encodeWorkflowRunActor(1), StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowToken{}).
		Where("workspace_id = ? AND instance_id = ? AND node_id = ? AND status = ?", workspace.ID, instanceID, "review", "active").
		Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).
		Where("workspace_id = ? AND id = ?", workspace.ID, instanceID).
		Updates(map[string]any{"status": "rejected", "current_node_ids": "[]", "completed_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	return workflowProjectGateActionFixture{
		app: app, token: token, workspace: workspace, project: project,
		requirementID: uint(requirement["id"].(float64)), instanceID: instanceID,
		body: map[string]any{"action": "resume"},
	}
}

func setupWorkflowProjectGateAIRecheck(t *testing.T, historical bool) workflowProjectGateActionFixture {
	t.Helper()
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "Start"},
			{"id": "input", "type": "form", "name": "Input"},
			{"id": "ai", "type": "ai_check", "name": "AI check", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "Review", "config": map[string]any{"critical_gate": true}},
			{"id": "end", "type": "end", "name": "End"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "input"},
			{"id": "e2", "source": "input", "target": "ai"},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e5", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	templateID := createPublishedWorkflowWithDefinition(t, app, token, workspace.ID, "Project gate AI recheck", definition)
	requirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "Project gate AI recheck"})
	instanceID := uint(requirement["workflow_instance_id"].(float64))
	requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "input",
	}), http.StatusOK)
	var aiRun aigorm.AIRun
	if err := app.db.Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", uint(requirement["id"].(float64)),
	).Order("id DESC").First(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	action := "retry_ai"
	if historical {
		output, _ := json.Marshal(workflowCompletenessOutput(true, aiRun.SubjectVersion, 90))
		if err := app.db.Model(&aiRun).Updates(map[string]any{"status": "completed", "output_json": string(output)}).Error; err != nil {
			t.Fatal(err)
		}
		requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
			"action": "complete", "node_id": "ai",
		}), http.StatusOK)
		action = "recheck_ai"
	} else if err := app.db.Model(&aiRun).Updates(map[string]any{
		"status": "failed", "error_message": "provider failure",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return workflowProjectGateActionFixture{
		app: app, token: token, workspace: workspace, project: project,
		requirementID: uint(requirement["id"].(float64)), instanceID: instanceID,
		body: map[string]any{"action": action, "node_id": "ai"},
	}
}
