package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"strings"
	"testing"
)

func TestPlanUnitCreationFixesPublishedWorkflowAtomically(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	templateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)

	created := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "交付项目", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, created, http.StatusCreated)
	data := decodeData(t, created).(map[string]any)
	unitID := uint(data["id"].(float64))
	if data["workflow_template_id"] != float64(templateID) || data["workflow_instance_id"] == nil {
		t.Fatalf("creation response omitted fixed workflow: %v", data)
	}
	var storedUnit projectgorm.PlanUnit
	if err := app.db.First(&storedUnit, unitID).Error; err != nil || storedUnit.Status != "development" {
		t.Fatalf("workflow did not synchronize initial plan status: status=%q err=%v", storedUnit.Status, err)
	}

	var instances int64
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "plan_unit", unitID,
	).Count(&instances).Error; err != nil || instances != 1 {
		t.Fatalf("workflow instance count=%d err=%v", instances, err)
	}

	secondBind := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances", token, workspace.ID, map[string]any{
		"template_id": templateID, "subject_type": "plan_unit", "subject_id": unitID,
	})
	requireStatus(t, secondBind, http.StatusConflict)
	requireStatus(t, app.request(t, http.MethodPatch, "/api/v1/platform/project-new/"+uintString(unitID), token, workspace.ID, map[string]any{
		"workflow_template_id": templateID,
	}), http.StatusBadRequest)

	invalid := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "不应创建", "type": "project", "workflow_template_id": 999999,
	})
	requireStatus(t, invalid, http.StatusConflict)
	var invalidUnits int64
	if err := app.db.Model(&projectgorm.PlanUnit{}).Where("workspace_id = ? AND title = ?", workspace.ID, "不应创建").Count(&invalidUnits).Error; err != nil || invalidUnits != 0 {
		t.Fatalf("invalid workflow left plan unit behind: count=%d err=%v", invalidUnits, err)
	}
}

func TestRequirementCreationFixesPublishedWorkflowAtomically(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	templateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)

	created := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "支付体验改造"})
	requirementID := uint(created["id"].(float64))
	instanceID := uint(created["workflow_instance_id"].(float64))
	if instanceID == 0 {
		t.Fatalf("creation response omitted fixed workflow: %v", created)
	}

	var instances int64
	if err := app.db.Model(&workflowgorm.WorkflowInstance{}).Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspace.ID, "requirement", requirementID,
	).Count(&instances).Error; err != nil || instances != 1 {
		t.Fatalf("workflow instance count=%d err=%v", instances, err)
	}

	pool := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "需求池入口", "workflow_template_id": templateID,
		"priority": "p1",
	})
	requireStatus(t, pool, http.StatusCreated)
	if decodeData(t, pool).(map[string]any)["workflow_instance_id"] == nil {
		t.Fatalf("requirement pool creation omitted workflow instance: %s", pool.Body.String())
	}

	missingSchedule := app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, map[string]any{
		"priority":       "p1",
		"project_new_id": project.ID, "title": "未排期入口", "workflow_template_id": templateID,
	})
	requireStatus(t, missingSchedule, http.StatusBadRequest)
	if !strings.Contains(missingSchedule.Body.String(), `"error":"version_task_new_id required"`) {
		t.Fatalf("missing plan unit error=%s", missingSchedule.Body.String())
	}
	var unscheduled int64
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("workspace_id = ? AND title = ?", workspace.ID, "未排期入口").Count(&unscheduled).Error; err != nil || unscheduled != 0 {
		t.Fatalf("unscheduled pool create persisted requirement count=%d err=%v", unscheduled, err)
	}

	// Existing requirements cannot add or replace their creation-time workflow.
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances", token, workspace.ID, map[string]any{
		"template_id": templateID, "subject_type": "requirement", "subject_id": requirementID,
	}), http.StatusConflict)
	requireStatus(t, app.request(t, http.MethodPatch, "/api/v1/platform/product-requirements-new/"+uintString(requirementID), token, workspace.ID, map[string]any{
		"workflow_template_id": templateID, "expected_version": 1,
	}), http.StatusBadRequest)

	invalid := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new", token, workspace.ID, map[string]any{
		"priority":            "p1",
		"version_task_new_id": unit.ID, "title": "不应创建的需求", "workflow_template_id": 999999,
	})
	requireStatus(t, invalid, http.StatusConflict)
	var invalidRequirements int64
	if err := app.db.Model(&requirementgorm.Requirement{}).Where("workspace_id = ? AND title = ?", workspace.ID, "不应创建的需求").Count(&invalidRequirements).Error; err != nil || invalidRequirements != 0 {
		t.Fatalf("invalid workflow left requirement behind: count=%d err=%v", invalidRequirements, err)
	}
}

func TestCreationRequiresPositivePublishedWorkflowOfMatchingSubjectType(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	planTemplateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)
	requirementTemplateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	unpublishedPlanTemplateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)
	unpublishedRequirementTemplateID := createPublishedRequirementWorkflow(t, app, token, workspace.ID)
	for _, templateID := range []uint{unpublishedPlanTemplateID, unpublishedRequirementTemplateID} {
		if err := app.db.Model(&workflowgorm.WorkflowTemplate{}).Where("id = ?", templateID).
			Update("published_version", 0).Error; err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name        string
		path        string
		body        map[string]any
		wantStatus  int
		subjectType string
	}{
		{name: "plan unit missing template", path: "/api/v1/platform/project-new", body: map[string]any{
			"project_new_id": project.ID, "title": "plan-missing", "type": "version",
		}, wantStatus: http.StatusBadRequest, subjectType: "plan_unit"},
		{name: "plan unit zero template", path: "/api/v1/platform/project-new", body: map[string]any{
			"project_new_id": project.ID, "title": "plan-zero", "type": "version", "workflow_template_id": 0,
		}, wantStatus: http.StatusBadRequest, subjectType: "plan_unit"},
		{name: "plan unit wrong subject template", path: "/api/v1/platform/project-new", body: map[string]any{
			"project_new_id": project.ID, "title": "plan-wrong-subject", "type": "version", "workflow_template_id": requirementTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "plan_unit"},
		{name: "plan unit unpublished template", path: "/api/v1/platform/project-new", body: map[string]any{
			"project_new_id": project.ID, "title": "plan-unpublished", "type": "version", "workflow_template_id": unpublishedPlanTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "plan_unit"},
		{name: "legacy requirement missing template", path: "/api/v1/platform/product-requirements-new", body: map[string]any{
			"priority":            "p1",
			"version_task_new_id": unit.ID, "title": "legacy-requirement-missing",
		}, wantStatus: http.StatusBadRequest, subjectType: "requirement"},
		{name: "legacy requirement zero template", path: "/api/v1/platform/product-requirements-new", body: map[string]any{
			"priority":            "p1",
			"version_task_new_id": unit.ID, "title": "legacy-requirement-zero", "workflow_template_id": 0,
		}, wantStatus: http.StatusBadRequest, subjectType: "requirement"},
		{name: "legacy requirement wrong subject template", path: "/api/v1/platform/product-requirements-new", body: map[string]any{
			"priority":            "p1",
			"version_task_new_id": unit.ID, "title": "legacy-requirement-wrong-subject", "workflow_template_id": planTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "requirement"},
		{name: "legacy requirement unpublished template", path: "/api/v1/platform/product-requirements-new", body: map[string]any{
			"priority":            "p1",
			"version_task_new_id": unit.ID, "title": "legacy-requirement-unpublished", "workflow_template_id": unpublishedRequirementTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "requirement"},
		{name: "requirement pool missing template", path: "/api/v1/platform/requirements", body: map[string]any{
			"priority":       "p1",
			"project_new_id": project.ID, "title": "pool-requirement-missing",
		}, wantStatus: http.StatusBadRequest, subjectType: "requirement"},
		{name: "requirement pool zero template", path: "/api/v1/platform/requirements", body: map[string]any{
			"priority":       "p1",
			"project_new_id": project.ID, "title": "pool-requirement-zero", "workflow_template_id": 0,
		}, wantStatus: http.StatusBadRequest, subjectType: "requirement"},
		{name: "requirement pool wrong subject template", path: "/api/v1/platform/requirements", body: map[string]any{
			"priority":       "p1",
			"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "pool-requirement-wrong-subject", "workflow_template_id": planTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "requirement"},
		{name: "requirement pool unpublished template", path: "/api/v1/platform/requirements", body: map[string]any{
			"priority":       "p1",
			"project_new_id": project.ID, "version_task_new_id": unit.ID, "title": "pool-requirement-unpublished", "workflow_template_id": unpublishedRequirementTemplateID,
		}, wantStatus: http.StatusConflict, subjectType: "requirement"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := app.request(t, http.MethodPost, test.path, token, workspace.ID, test.body)
			requireStatus(t, rec, test.wantStatus)
			if test.wantStatus == http.StatusBadRequest && !strings.Contains(rec.Body.String(), `"error":"workflow_template_id required"`) {
				t.Fatalf("missing required-template error: %s", rec.Body.String())
			}
			var count int64
			title := test.body["title"]
			if test.subjectType == "plan_unit" {
				app.db.Model(&projectgorm.PlanUnit{}).Where("workspace_id = ? AND title = ?", workspace.ID, title).Count(&count)
			} else {
				app.db.Model(&requirementgorm.Requirement{}).Where("workspace_id = ? AND title = ?", workspace.ID, title).Count(&count)
			}
			if count != 0 {
				t.Fatalf("rejected creation persisted subject %q", title)
			}
		})
	}
}

func TestWorkflowNodePlansAreEditableAfterCreationAndWorkspaceScoped(t *testing.T) {
	app, token, workspace, project, _ := newAIWorkflowFixture(t)
	templateID := createPublishedPlanUnitWorkflow(t, app, token, workspace.ID)
	created := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
		"project_new_id": project.ID, "title": "待排期项目", "type": "project", "workflow_template_id": templateID,
	})
	requireStatus(t, created, http.StatusCreated)
	instanceID := uint(decodeData(t, created).(map[string]any)["workflow_instance_id"].(float64))
	assignWorkflowStarterToDefinitionNodes(t, app, token, workspace.ID, instanceID)

	invalid := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", token, workspace.ID, map[string]any{
		"nodes": []map[string]any{{"node_id": "delivery", "planned_start_at": "2026-08-20", "expected_planned_start_at": nil, "planned_end_at": "2026-08-10", "expected_planned_end_at": nil}},
	})
	requireStatus(t, invalid, http.StatusConflict)
	assertScheduleEnvelopeConflict(t, invalid, "NODE_START_AFTER_END")

	updated := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/node-plans", token, workspace.ID, map[string]any{
		"confirm_subject_schedule": true,
		"nodes":                    []map[string]any{{"node_id": "delivery", "planned_start_at": "2026-08-10", "expected_planned_start_at": nil, "planned_end_at": "2026-08-20", "expected_planned_end_at": nil}},
	})
	requireStatus(t, updated, http.StatusOK)

	detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
	requireStatus(t, detail, http.StatusOK)
	nodes := decodeData(t, detail).(map[string]any)["timeline_nodes"].([]any)
	found := false
	for _, raw := range nodes {
		node := raw.(map[string]any)
		if node["id"] == "delivery" {
			found = strings.HasPrefix(node["planned_start_at"].(string), "2026-08-10") && strings.HasPrefix(node["planned_end_at"].(string), "2026-08-20")
		}
	}
	if !found {
		t.Fatalf("node plan not rendered in workflow timeline: %v", nodes)
	}

	completed := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/actions", token, workspace.ID, map[string]any{
		"action": "complete", "node_id": "delivery",
	})
	requireStatus(t, completed, http.StatusOK)
	completedNodes := decodeData(t, completed).(map[string]any)["timeline_nodes"].([]any)
	actualCompletionFound := false
	for _, raw := range completedNodes {
		node := raw.(map[string]any)
		if node["id"] == "delivery" {
			actualCompletionFound = node["status"] == "completed" && strings.TrimSpace(node["actual_completed_at"].(string)) != ""
		}
	}
	if !actualCompletionFound {
		t.Fatalf("completed node omitted actual completion time: %v", completedNodes)
	}

	other := workflowgorm.WorkflowInstance{WorkspaceID: 2, TemplateID: templateID, SubjectType: "plan_unit", SubjectID: 999, Status: "running", CurrentNodeIDs: "[]", NodePlansJSON: "{}"}
	if err := app.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	isolated := app.request(t, http.MethodPatch, "/api/v1/platform/pm/workflows/instances/"+uintString(other.ID)+"/node-plans", token, workspace.ID, map[string]any{"nodes": []any{}})
	requireStatus(t, isolated, http.StatusNotFound)
}

func createPublishedPlanUnitWorkflow(t *testing.T, app *testApp, token string, workspaceID uint) uint {
	t.Helper()
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "delivery", "type": "human_task", "name": "交付执行"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "delivery"},
			{"id": "e2", "source": "delivery", "target": "end"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspaceID, map[string]any{
		"name": "通用交付流程", "subject_type": "plan_unit", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspaceID, nil), http.StatusOK)
	return templateID
}

func createPublishedRequirementWorkflow(t *testing.T, app *testApp, token string, workspaceID uint) uint {
	t.Helper()
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "review", "type": "approval", "name": "需求评审", "config": map[string]any{"critical_gate": true, "subject_status": "grooming"}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "review"},
			{"id": "e2", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspaceID, map[string]any{
		"name": "通用需求评审流", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspaceID, nil), http.StatusOK)
	return templateID
}
