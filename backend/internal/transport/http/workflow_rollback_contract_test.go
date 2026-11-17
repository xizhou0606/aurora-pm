package httpapi

import (
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"net/http"
	"testing"
)

func TestWorkflowTimelineMarksCompletedAIRecoveryTransitionRolledBack(t *testing.T) {
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "input", Type: "form"},
			{ID: "completeness", Type: "ai_check"},
			{ID: "review", Type: "approval"},
		},
		Edges: []workflowEdge{
			{ID: "input-completeness", Source: "input", Target: "completeness"},
			{ID: "completeness-review", Source: "completeness", Target: "review", Kind: "success"},
			{ID: "completeness-input", Source: "completeness", Target: "input", Kind: "failure", Loop: true},
		},
	}
	runs := []workflowgorm.WorkflowNodeRun{
		{NodeID: "input", NodeType: "form", Status: "completed"},
		{
			NodeID: "completeness", NodeType: "ai_check", Status: "completed",
			OutputJSON: encodeWorkflowRunTransition(0, "", "input"),
		},
	}

	statuses, _ := workflowTimelineState(definition, runs)
	if statuses["completeness"] != "rolled_back" {
		t.Fatalf("AI recovery transition status=%q, want rolled_back", statuses["completeness"])
	}
}

func TestWorkflowTimelineRollbackKeepsCompletedParallelSibling(t *testing.T) {
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "dev", Type: "human_task", Name: "研发实现"},
			{ID: "test-prep", Type: "human_task", Name: "测试用例编写"},
			{ID: "join", Type: "parallel_join", Name: "研发与测试准备完成"},
			{ID: "qa", Type: "human_task", Name: "QA测试执行"},
			{ID: "acceptance", Type: "approval", Name: "产品验收"},
			{ID: "end", Type: "end", Name: "结束"},
		},
		Edges: []workflowEdge{
			{ID: "dev-join", Source: "dev", Target: "join"},
			{ID: "test-join", Source: "test-prep", Target: "join"},
			{ID: "join-qa", Source: "join", Target: "qa"},
			{ID: "qa-acceptance", Source: "qa", Target: "acceptance"},
			{ID: "acceptance-end", Source: "acceptance", Target: "end", Kind: "approve"},
			{ID: "acceptance-reject", Source: "acceptance", Target: "dev", Kind: "reject", Loop: true},
		},
	}
	runs := []workflowgorm.WorkflowNodeRun{
		{NodeID: "dev", NodeType: "human_task", Status: "completed"},
		{NodeID: "test-prep", NodeType: "human_task", Status: "completed"},
		{NodeID: "join", NodeType: "parallel_join", Status: "completed"},
		{NodeID: "qa", NodeType: "human_task", Status: "completed"},
		{NodeID: "acceptance", NodeType: "approval", Status: "rolled_back", OutputJSON: encodeWorkflowRunRollback(1, "测试准备需要补充", "test-prep")},
		{NodeID: "test-prep", NodeType: "human_task", Status: "completed"},
		{NodeID: "join", NodeType: "parallel_join", Status: "completed"},
	}

	statuses, _ := workflowTimelineState(definition, runs)
	if statuses["dev"] != "completed" {
		t.Fatalf("completed sibling branch status=%q, want completed", statuses["dev"])
	}
	if statuses["qa"] != "pending" || statuses["acceptance"] != "rolled_back" {
		t.Fatalf("downstream rollback statuses=%v", statuses)
	}
}

func TestProjectAndRequirementWorkflowNodesCanRollbackToCompletedNode(t *testing.T) {
	for _, subjectType := range []string{"requirement", "plan_unit"} {
		t.Run(subjectType, func(t *testing.T) {
			app, token, workspace, project, unit := newAIWorkflowFixture(t)
			definition := map[string]any{
				"nodes": []map[string]any{
					{"id": "start", "type": "start", "name": "开始"},
					{"id": "input", "type": "form", "name": "信息准备"},
					{"id": "delivery", "type": "human_task", "name": "执行处理"},
					{"id": "review", "type": "human_task", "name": "最终确认"},
					{"id": "end", "type": "end", "name": "结束"},
				},
				"edges": []map[string]any{
					{"id": "e1", "source": "start", "target": "input"},
					{"id": "e2", "source": "input", "target": "delivery"},
					{"id": "e3", "source": "delivery", "target": "review"},
					{"id": "e4", "source": "review", "target": "end"},
				},
			}
			createdTemplate := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
				"name": "可回退流程", "subject_type": subjectType, "definition": definition,
			})
			requireStatus(t, createdTemplate, http.StatusCreated)
			templateID := uint(decodeData(t, createdTemplate).(map[string]any)["id"].(float64))
			requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)

			instanceID := uint(0)
			if subjectType == "requirement" {
				createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "回退需求"})
				instanceID = uint(createdRequirement["workflow_instance_id"].(float64))
			} else {
				createdUnit := app.request(t, http.MethodPost, "/api/v1/platform/project-new", token, workspace.ID, map[string]any{
					"project_new_id": project.ID, "title": "回退项目", "type": "project", "workflow_template_id": templateID,
				})
				requireStatus(t, createdUnit, http.StatusCreated)
				instanceID = uint(decodeData(t, createdUnit).(map[string]any)["workflow_instance_id"].(float64))
				assignWorkflowStarterToDefinitionNodes(t, app, token, workspace.ID, instanceID)
			}

			for _, nodeID := range []string{"input", "delivery"} {
				requireStatus(t, app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
					"action": "complete", "node_id": nodeID,
				}), http.StatusOK)
			}
			detail := app.request(t, http.MethodGet, "/api/v1/platform/pm/workflows/instances/"+uintString(instanceID), token, workspace.ID, nil)
			requireStatus(t, detail, http.StatusOK)
			targets := decodeData(t, detail).(map[string]any)["rollback_targets"].([]any)
			if len(targets) != 2 {
				t.Fatalf("rollback targets=%v", targets)
			}

			rolledBack := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
				"action": "rollback", "node_id": "review", "target_node_id": "input", "comment": "材料需要补充",
			})
			requireStatus(t, rolledBack, http.StatusOK)
			data := decodeData(t, rolledBack).(map[string]any)
			current := data["current_nodes"].([]any)
			if data["status"] != "running" || len(current) != 1 || current[0].(map[string]any)["id"] != "input" || data["progress"] != float64(0) {
				t.Fatalf("rollback result=%v", data)
			}
			history := data["transition_history"].([]any)
			last := history[len(history)-1].(map[string]any)
			if last["action"] != "rolled_back" || last["target_node_id"] != "input" || last["comment"] != "材料需要补充" || last["operator_name"] != "owner" {
				t.Fatalf("rollback history=%v", last)
			}
			if remaining := data["rollback_targets"].([]any); len(remaining) != 0 {
				t.Fatalf("rollback should reset downstream completion: %v", remaining)
			}
		})
	}
}

func TestCompletedRequirementWorkflowNodeAcceptsLateDesignMaterial(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "design", "type": "human_task", "name": "UI 设计", "config": map[string]any{"require_design_image": true}},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{{"id": "e1", "source": "start", "target": "design"}, {"id": "e2", "source": "design", "target": "end"}},
	}
	created := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{
		"name": "设计交付流程", "subject_type": "requirement", "definition": definition,
	})
	requireStatus(t, created, http.StatusCreated)
	templateID := uint(decodeData(t, created).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	createdRequirement := createRequirementWithWorkflow(t, app, token, workspace.ID, unit.ID, templateID, map[string]any{"title": "补交设计稿"})
	requirementID := uint(createdRequirement["id"].(float64))
	instanceID := uint(createdRequirement["workflow_instance_id"].(float64))
	completed := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "complete", "node_id": "design", "design_url": "https://design.example.test/initial.png",
	})
	requireStatus(t, completed, http.StatusOK)
	initialActualCompletedAt := workflowTimelineNode(t, decodeData(t, completed).(map[string]any), "design")["actual_completed_at"]

	supplemented := app.workflowAction(t, workspace.ID, token, instanceID, map[string]any{
		"action": "supplement_design", "node_id": "design", "design_url": "https://design.example.test/final.png",
	})
	requireStatus(t, supplemented, http.StatusOK)
	if data := decodeData(t, supplemented).(map[string]any); data["status"] != "completed" {
		t.Fatalf("supplementing material changed workflow state: %v", data)
	} else {
		designNode := workflowTimelineNode(t, data, "design")
		if designNode["status"] != "completed" || designNode["actual_completed_at"] != initialActualCompletedAt {
			t.Fatalf("supplementing material changed node execution state: %v", designNode)
		}
		history := data["transition_history"].([]any)
		last := history[len(history)-1].(map[string]any)
		materials, materialsOK := last["materials"].([]any)
		if last["action"] != "material_supplemented" || last["node_id"] != "design" || !materialsOK || len(materials) != 1 || materials[0].(map[string]any)["url"] != "https://design.example.test/final.png" {
			t.Fatalf("supplemented material history=%v", last)
		}
	}
	var updated requirementgorm.Requirement
	if err := app.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).First(&updated).Error; err != nil || updated.DesignURL != "https://design.example.test/final.png" {
		t.Fatalf("late design material=%q err=%v", updated.DesignURL, err)
	}
}
