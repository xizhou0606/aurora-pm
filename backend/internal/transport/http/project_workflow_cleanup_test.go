package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func TestDeleteProjectSetCascadesWorkflowAndAISubjectRecords(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "待随项目删除的需求", Priority: "normal", Status: "backlog",
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	requirementRows := seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "requirement", requirement.ID)
	planUnitRows := seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "plan_unit", unit.ID)
	if err := app.db.Model(&requirement).Update("workflow_instance_id", requirementRows.instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&taskgorm.KeySequence{
		WorkspaceID: workspace.ID, ProjectSetID: project.ID, Prefix: "AI", NextValue: 3,
	}).Error; err != nil {
		t.Fatal(err)
	}

	unrelatedProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: unit.TeamID, Name: "保留项目",
	}
	unrelatedUnit := projectgorm.PlanUnit{
		ID: 22, WorkspaceID: workspace.ID, TeamID: unit.TeamID,
		ProjectSetID: unrelatedProject.ID, Title: "保留版本", Type: "version",
	}
	for _, row := range []any{&unrelatedProject, &unrelatedUnit} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	unrelatedRows := seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "plan_unit", unrelatedUnit.ID)
	foreignRows := seedProjectWorkflowCascadeRows(t, app.db, 2, "requirement", requirement.ID)
	if err := app.db.Create(&taskgorm.KeySequence{
		WorkspaceID: workspace.ID, ProjectSetID: unrelatedProject.ID, Prefix: "KEEP", NextValue: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(
		t, http.MethodDelete,
		"/api/v1/platform/project-sets/"+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, response, http.StatusOK)

	targetInstances := []uint{requirementRows.instanceID, planUnitRows.instanceID}
	targetAIRuns := []uint{requirementRows.aiRunID, planUnitRows.aiRunID}
	for _, check := range []struct {
		name  string
		row   any
		where string
		args  []any
	}{
		{"workflow approvals", &workflowgorm.WorkflowApproval{}, "instance_id IN ?", []any{targetInstances}},
		{"workflow jobs", &workflowgorm.WorkflowJob{}, "instance_id IN ?", []any{targetInstances}},
		{"workflow artifacts", &workflowgorm.WorkflowNodeArtifact{}, "instance_id IN ?", []any{targetInstances}},
		{"workflow runs", &workflowgorm.WorkflowNodeRun{}, "instance_id IN ?", []any{targetInstances}},
		{"workflow tokens", &workflowgorm.WorkflowToken{}, "instance_id IN ?", []any{targetInstances}},
		{"workflow instances", &workflowgorm.WorkflowInstance{}, "id IN ?", []any{targetInstances}},
		{"AI applications", &aigorm.AIArtifactApplication{}, "run_id IN ?", []any{targetAIRuns}},
		{"AI feedback", &aigorm.AIFeedback{}, "run_id IN ?", []any{targetAIRuns}},
		{"AI artifacts", &aigorm.AIArtifact{}, "run_id IN ?", []any{targetAIRuns}},
		{"AI sources", &aigorm.AIRunSource{}, "run_id IN ?", []any{targetAIRuns}},
		{"AI runs", &aigorm.AIRun{}, "id IN ?", []any{targetAIRuns}},
		{"key sequences", &taskgorm.KeySequence{}, "project_set_id = ?", []any{project.ID}},
		{"requirements", &requirementgorm.Requirement{}, "id = ?", []any{requirement.ID}},
		{"plan units", &projectgorm.PlanUnit{}, "id = ?", []any{unit.ID}},
		{"project set", &projectgorm.ProjectSet{}, "id = ?", []any{project.ID}},
	} {
		requireModelCount(t, app.db, check.name, check.row, check.where, 0, check.args...)
	}

	requireWorkflowCascadeRows(t, app.db, unrelatedRows)
	requireWorkflowCascadeRows(t, app.db, foreignRows)
	requireModelCount(t, app.db, "unrelated project", &projectgorm.ProjectSet{}, "id = ?", 1, unrelatedProject.ID)
	requireModelCount(t, app.db, "unrelated plan unit", &projectgorm.PlanUnit{}, "id = ?", 1, unrelatedUnit.ID)
	requireModelCount(t, app.db, "unrelated key sequence", &taskgorm.KeySequence{}, "project_set_id = ?", 1, unrelatedProject.ID)
}

func TestDeleteProjectSetRollsBackWorkflowCleanupOnFailure(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "事务回滚需求", Priority: "normal", Status: "backlog",
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	rows := seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "requirement", requirement.ID)

	callbackName := "test:fail_project_workflow_run_delete"
	if err := app.db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WorkflowNodeRun" {
			tx.AddError(errors.New("forced workflow cleanup failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Delete().Remove(callbackName) })

	response := app.request(
		t, http.MethodDelete,
		"/api/v1/platform/project-sets/"+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, response, http.StatusInternalServerError)

	requireModelCount(t, app.db, "project after rollback", &projectgorm.ProjectSet{}, "id = ?", 1, project.ID)
	requireModelCount(t, app.db, "requirement after rollback", &requirementgorm.Requirement{}, "id = ?", 1, requirement.ID)
	requireWorkflowCascadeRows(t, app.db, rows)
}

func TestDeleteProjectSetSweepsDependentsWrittenWhileParentsAreDeleted(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "并发删除需求", Priority: "normal", Status: "backlog",
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	rows := seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "requirement", requirement.ID)

	callbackName := "test:write_project_dependents_during_parent_delete"
	if err := app.db.Callback().Delete().After("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		lateWrite := tx.Session(&gorm.Session{NewDB: true})
		switch tx.Statement.Schema.Name {
		case "WorkflowInstance":
			if err := lateWrite.Create(&workflowgorm.WorkflowToken{
				WorkspaceID: workspace.ID, InstanceID: rows.instanceID, NodeID: "late-token", Status: "active",
			}).Error; err != nil {
				tx.AddError(err)
				return
			}
			tx.AddError(lateWrite.Create(&workflowgorm.WorkflowNodeRun{
				WorkspaceID: workspace.ID, InstanceID: rows.instanceID, NodeID: "late-run",
				NodeType: "human_task", Status: "completed", StartedAt: time.Now().UTC(),
			}).Error)
		case "AIRun":
			tx.AddError(lateWrite.Create(&aigorm.AIArtifact{
				WorkspaceID: workspace.ID, RunID: rows.aiRunID, Kind: "structured_result",
				Title: "late artifact", ContentJSON: `{}`,
			}).Error)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Delete().Remove(callbackName) })

	response := app.request(
		t, http.MethodDelete,
		"/api/v1/platform/project-sets/"+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, response, http.StatusOK)
	requireModelCount(t, app.db, "late workflow token", &workflowgorm.WorkflowToken{}, "instance_id = ? AND node_id = ?", 0, rows.instanceID, "late-token")
	requireModelCount(t, app.db, "late workflow run", &workflowgorm.WorkflowNodeRun{}, "instance_id = ?", 0, rows.instanceID)
	requireModelCount(t, app.db, "late AI artifact", &aigorm.AIArtifact{}, "run_id = ?", 0, rows.aiRunID)
}

func TestDeleteProjectSetUsesCanonicalWorkflowAndAILockOrder(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "锁顺序需求", Priority: "normal", Status: "backlog",
	}
	if err := app.db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	seedProjectWorkflowCascadeRows(t, app.db, workspace.ID, "requirement", requirement.ID)
	if err := app.db.Create(&taskgorm.KeySequence{
		WorkspaceID: workspace.ID, ProjectSetID: project.ID, Prefix: "LOCK", NextValue: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}

	events := make([]string, 0, 8)
	queryCallback := "test:record_project_delete_lock_queries"
	if err := app.db.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		if _, locked := tx.Statement.Clauses["FOR"]; !locked {
			return
		}
		switch tx.Statement.Schema.Name {
		case "Requirement":
			events = append(events, "lock-requirement")
		case "PlanUnit":
			events = append(events, "lock-plan-unit")
		}
	}); err != nil {
		t.Fatal(err)
	}
	deleteCallback := "test:record_project_delete_order"
	if err := app.db.Callback().Delete().Before("gorm:delete").Register(deleteCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		switch tx.Statement.Schema.Name {
		case "WorkflowInstance":
			events = append(events, "delete-workflow-instance")
		case "KeySequence":
			events = append(events, "delete-key-sequence")
		case "AIRun":
			events = append(events, "delete-ai-run")
		case "Requirement":
			events = append(events, "delete-requirement")
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = app.db.Callback().Query().Remove(queryCallback)
		_ = app.db.Callback().Delete().Remove(deleteCallback)
	})

	response := app.request(
		t, http.MethodDelete,
		"/api/v1/platform/project-sets/"+uintString(project.ID),
		token, workspace.ID, nil,
	)
	requireStatus(t, response, http.StatusOK)
	for _, pair := range [][2]string{
		{"delete-workflow-instance", "lock-requirement"},
		{"lock-requirement", "lock-plan-unit"},
		{"lock-plan-unit", "delete-key-sequence"},
		{"delete-key-sequence", "delete-ai-run"},
		{"delete-ai-run", "delete-requirement"},
	} {
		requireEventBefore(t, events, pair[0], pair[1])
	}
}

func TestCreateAIRunLocksCurrentSubjectBeforeInsert(t *testing.T) {
	tests := []struct {
		name          string
		subjectType   string
		subjectSchema string
		capability    string
	}{
		{name: "requirement", subjectType: "requirement", subjectSchema: "Requirement", capability: "requirement.completeness"},
		{name: "plan unit", subjectType: "plan_unit", subjectSchema: "PlanUnit", capability: "delivery.risk_monitor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			subjectID := unit.ID
			if test.subjectType == "requirement" {
				requirement := requirementgorm.Requirement{
					ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
					PlanUnitID: unit.ID, Title: "AI Run 锁顺序需求", Priority: "normal", Status: "backlog", Version: 1,
				}
				if err := app.db.Create(&requirement).Error; err != nil {
					t.Fatal(err)
				}
				subjectID = requirement.ID
			}

			events := make([]string, 0, 4)
			projectLockConn, subjectLockConn, createConn := "", "", ""
			queryCallback := "test:record_ai_run_project_and_subject_locks"
			if err := app.db.Callback().Query().After("gorm:query").Register(queryCallback, func(tx *gorm.DB) {
				if tx.Statement.Schema == nil {
					return
				}
				_, locked := tx.Statement.Clauses["FOR"]
				switch tx.Statement.Schema.Name {
				case "ProjectSet":
					if locked {
						events = append(events, "lock-project")
						projectLockConn = fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
					}
				case test.subjectSchema:
					if locked {
						events = append(events, "lock-subject")
						subjectLockConn = fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
					} else {
						events = append(events, "read-subject")
					}
				}
			}); err != nil {
				t.Fatal(err)
			}
			createCallback := "test:record_ai_run_insert"
			if err := app.db.Callback().Create().Before("gorm:create").Register(createCallback, func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AIRun" {
					events = append(events, "create-ai-run")
					createConn = fmt.Sprintf("%T:%p", tx.Statement.ConnPool, tx.Statement.ConnPool)
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = app.db.Callback().Query().Remove(queryCallback)
				_ = app.db.Callback().Create().Remove(createCallback)
			})

			response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
				"capability": test.capability, "subject_type": test.subjectType, "subject_id": subjectID,
			})
			requireStatus(t, response, http.StatusCreated)
			want := []string{"read-subject", "lock-project", "lock-subject", "create-ai-run"}
			if len(events) != len(want) {
				t.Fatalf("AI run lock events=%v, want %v", events, want)
			}
			for index := range want {
				if events[index] != want[index] {
					t.Fatalf("AI run lock events=%v, want %v", events, want)
				}
			}
			if projectLockConn == "" || projectLockConn != subjectLockConn || projectLockConn != createConn {
				t.Fatalf(
					"AI run transaction connections project=%q subject=%q create=%q, want one transaction",
					projectLockConn, subjectLockConn, createConn,
				)
			}
		})
	}
}

func TestCreateAIRunRejectsSubjectProjectChangeBeforeInsert(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := requirementgorm.Requirement{
		ID: 13, WorkspaceID: workspace.ID, TeamID: unit.TeamID, ProjectSetID: project.ID,
		PlanUnitID: unit.ID, Title: "迁移中的 AI Run 需求", Priority: "normal", Status: "backlog", Version: 1,
	}
	otherProject := projectgorm.ProjectSet{
		ID: 21, WorkspaceID: workspace.ID, TeamID: unit.TeamID, Name: "AI Run 迁移目标",
	}
	for _, row := range []any{&requirement, &otherProject} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	projectChanged, subjectLocked := false, false
	callbackName := "test:move_ai_run_subject_after_project_lock"
	if err := app.db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		_, locked := tx.Statement.Clauses["FOR"]
		if tx.Statement.Schema.Name == "Requirement" && locked {
			subjectLocked = true
			return
		}
		if projectChanged || tx.Statement.Schema.Name != "ProjectSet" || !locked {
			return
		}
		projectChanged = true
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).
			Model(&requirementgorm.Requirement{}).
			Where("id = ? AND workspace_id = ?", requirement.ID, workspace.ID).
			Update("project_new_id", otherProject.ID).Error)
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.db.Callback().Query().Remove(callbackName) })

	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{
		"capability": "requirement.completeness", "subject_type": "requirement", "subject_id": requirement.ID,
	})
	requireStatus(t, response, http.StatusConflict)
	if !projectChanged || !subjectLocked {
		t.Fatalf("projectChanged=%v subjectLocked=%v, want both true", projectChanged, subjectLocked)
	}
	requireModelCount(t, app.db, "AI run after subject move conflict", &aigorm.AIRun{}, "subject_type = ? AND subject_id = ?", 0, "requirement", requirement.ID)
}

type projectWorkflowCascadeRows struct {
	instanceID uint
	aiRunID    uint
}

func seedProjectWorkflowCascadeRows(
	t *testing.T,
	db *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) projectWorkflowCascadeRows {
	t.Helper()
	now := time.Now().UTC()
	instance := workflowgorm.WorkflowInstance{
		WorkspaceID: workspaceID, TemplateID: 1, TemplateVersionID: 1,
		SubjectType: subjectType, SubjectID: subjectID, Status: "running",
		CurrentNodeIDs: `["review"]`, NodePlansJSON: "{}", StartedByUserID: 1, StartedAt: now,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	nodeRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: workspaceID, InstanceID: instance.ID, NodeID: "review",
		NodeType: "approval", Status: "waiting", StartedAt: now,
	}
	if err := db.Create(&nodeRun).Error; err != nil {
		t.Fatal(err)
	}
	aiRun := aigorm.AIRun{
		WorkspaceID: workspaceID, Capability: "requirement.completeness",
		SubjectType: subjectType, SubjectID: subjectID, SubjectVersion: 1,
		Status: "queued", Model: "test", InputJSON: `{}`, RequestedByUser: 1,
	}
	if err := db.Create(&aiRun).Error; err != nil {
		t.Fatal(err)
	}
	aiArtifact := aigorm.AIArtifact{
		WorkspaceID: workspaceID, RunID: aiRun.ID, Kind: "analysis",
		Title: "分析结果", ContentJSON: `{}`,
	}
	if err := db.Create(&aiArtifact).Error; err != nil {
		t.Fatal(err)
	}
	aiRunID := aiRun.ID
	for _, row := range []any{
		&workflowgorm.WorkflowToken{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeID: "review", Status: "active"},
		&workflowgorm.WorkflowApproval{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeRunID: nodeRun.ID, Status: "pending", CreatedAt: now},
		&workflowgorm.WorkflowJob{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeRunID: nodeRun.ID, Kind: "ai_run", Status: "pending", PayloadJSON: fmt.Sprintf(`{"ai_run_id":%d}`, aiRun.ID), IdempotencyKey: fmt.Sprintf("cascade:%d", instance.ID), AvailableAt: now},
		&workflowgorm.WorkflowNodeArtifact{WorkspaceID: workspaceID, InstanceID: instance.ID, NodeID: "review", Kind: "uploaded", Name: "cases.csv", ContentType: "text/csv", Size: 5, ObjectKey: fmt.Sprintf("cascade/%d/%d.csv", workspaceID, instance.ID), URL: "https://objects.example.test/cases.csv", AIRunID: &aiRunID, UploaderUserID: 1},
		&aigorm.AIRunSource{WorkspaceID: workspaceID, RunID: aiRun.ID, SourceType: "feishu_link", SourceURL: "https://example.test/doc"},
		&aigorm.AIArtifactApplication{WorkspaceID: workspaceID, RunID: aiRun.ID, ArtifactID: aiArtifact.ID, SubjectVersion: 1, ChangesJSON: `{}`, AppliedByUser: 1},
		&aigorm.AIFeedback{WorkspaceID: workspaceID, RunID: aiRun.ID, UserID: 1, Rating: "up"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return projectWorkflowCascadeRows{instanceID: instance.ID, aiRunID: aiRun.ID}
}

func requireWorkflowCascadeRows(t *testing.T, db *gorm.DB, rows projectWorkflowCascadeRows) {
	t.Helper()
	for _, check := range []struct {
		name  string
		row   any
		where string
		id    uint
	}{
		{"workflow approval", &workflowgorm.WorkflowApproval{}, "instance_id = ?", rows.instanceID},
		{"workflow job", &workflowgorm.WorkflowJob{}, "instance_id = ?", rows.instanceID},
		{"workflow artifact", &workflowgorm.WorkflowNodeArtifact{}, "instance_id = ?", rows.instanceID},
		{"workflow run", &workflowgorm.WorkflowNodeRun{}, "instance_id = ?", rows.instanceID},
		{"workflow token", &workflowgorm.WorkflowToken{}, "instance_id = ?", rows.instanceID},
		{"workflow instance", &workflowgorm.WorkflowInstance{}, "id = ?", rows.instanceID},
		{"AI application", &aigorm.AIArtifactApplication{}, "run_id = ?", rows.aiRunID},
		{"AI feedback", &aigorm.AIFeedback{}, "run_id = ?", rows.aiRunID},
		{"AI artifact", &aigorm.AIArtifact{}, "run_id = ?", rows.aiRunID},
		{"AI source", &aigorm.AIRunSource{}, "run_id = ?", rows.aiRunID},
		{"AI run", &aigorm.AIRun{}, "id = ?", rows.aiRunID},
	} {
		requireModelCount(t, db, check.name, check.row, check.where, 1, check.id)
	}
}

func requireModelCount(
	t *testing.T,
	db *gorm.DB,
	name string,
	row any,
	where string,
	want int64,
	args ...any,
) {
	t.Helper()
	var count int64
	if err := db.Model(row).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", name, err)
	}
	if count != want {
		t.Fatalf("%s count=%d, want %d", name, count, want)
	}
}

func requireEventBefore(t *testing.T, events []string, first, second string) {
	t.Helper()
	firstIndex, secondIndex := -1, -1
	for index, event := range events {
		if event == first && firstIndex < 0 {
			firstIndex = index
		}
		if event == second && secondIndex < 0 {
			secondIndex = index
		}
	}
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("delete events=%v, want %q before %q", events, first, second)
	}
}
