package httpapi

import (
	"net/http"
	"testing"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

func TestDeleteRequirementCascadesWorkflowRuntimeAIRunsAndMaterials(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	var requirement requirementgorm.Requirement
	if err := fix.app.db.First(&requirement, fix.requirementID).Error; err != nil {
		t.Fatal(err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := fix.app.db.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?",
		fix.workspace.ID, "requirement", fix.requirementID).First(&instance).Error; err != nil {
		t.Fatalf("requirement creation should auto-start a workflow instance: %v", err)
	}
	actorID := fix.actor.ID
	nodeRun := workflowgorm.WorkflowNodeRun{
		WorkspaceID: fix.workspace.ID, InstanceID: instance.ID,
		NodeID: "review", NodeType: "approval", Status: "completed",
	}
	if err := fix.app.db.Create(&nodeRun).Error; err != nil {
		t.Fatal(err)
	}
	seeded := []any{
		&workflowgorm.WorkflowToken{
			WorkspaceID: fix.workspace.ID, InstanceID: instance.ID,
			NodeID: "review", Status: "done", AssigneeUserID: &actorID,
		},
		&workflowgorm.WorkflowApproval{
			WorkspaceID: fix.workspace.ID, InstanceID: instance.ID, NodeRunID: nodeRun.ID,
			Status: "approved", ApproverUserID: &actorID,
		},
		&workflowgorm.WorkflowNodeOpinion{
			WorkspaceID: fix.workspace.ID, InstanceID: instance.ID,
			NodeID: "review", AuthorUserID: actorID, Body: "同意",
		},
		&workflowgorm.WorkflowJob{
			WorkspaceID: fix.workspace.ID, InstanceID: instance.ID, NodeRunID: nodeRun.ID,
			Kind: "ai_node", Status: "pending", PayloadJSON: "{}",
			IdempotencyKey: "del-cascade-job", AvailableAt: nodeRun.StartedAt,
		},
		&workflowgorm.WorkflowNodeArtifact{
			WorkspaceID: fix.workspace.ID, InstanceID: instance.ID,
			NodeID: "review", Kind: "design_image", Name: "design.png",
			ContentType: "image/png", Size: 12, ObjectKey: "del-cascade/design.png",
			URL: "https://cos.example.com/del-cascade/design.png", UploaderUserID: actorID,
		},
		&aigorm.AIRun{
			WorkspaceID: fix.workspace.ID, Capability: "requirement_breakdown",
			SubjectType: "requirement", SubjectID: fix.requirementID, SubjectVersion: requirement.Version,
			Status: "succeeded", Model: "gpt-test", InputJSON: "{}", RequestedByUser: actorID,
		},
		&requirementgorm.RequirementMaterial{
			WorkspaceID: fix.workspace.ID, RequirementID: fix.requirementID,
			Kind: "prd", Source: "upload", Name: "prd.md", ContentType: "text/markdown",
			Size: 8, ObjectKey: "del-cascade/prd.md", URL: "https://cos.example.com/del-cascade/prd.md",
			UploaderUserID: actorID,
		},
	}
	for _, row := range seeded {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	rec := fix.app.request(t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/"+uintString(fix.requirementID),
		fix.app.token(t, fix.actor), fix.workspace.ID, nil)
	requireStatus(t, rec, http.StatusOK)

	var requirementCount, instanceCount, tokenCount, nodeRunCount int64
	var approvalCount, opinionCount, jobCount, artifactCount int64
	var aiRunCount, materialCount int64
	count := func(model any, query string, args ...any) int64 {
		t.Helper()
		var total int64
		if err := fix.app.db.Unscoped().Model(model).Where(query, args...).Count(&total).Error; err != nil {
			t.Fatal(err)
		}
		return total
	}
	requirementCount = count(&requirementgorm.Requirement{}, "id = ?", fix.requirementID)
	instanceCount = count(&workflowgorm.WorkflowInstance{}, "workspace_id = ? AND subject_type = ? AND subject_id = ?",
		fix.workspace.ID, "requirement", fix.requirementID)
	tokenCount = count(&workflowgorm.WorkflowToken{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	nodeRunCount = count(&workflowgorm.WorkflowNodeRun{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	approvalCount = count(&workflowgorm.WorkflowApproval{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	opinionCount = count(&workflowgorm.WorkflowNodeOpinion{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	jobCount = count(&workflowgorm.WorkflowJob{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	artifactCount = count(&workflowgorm.WorkflowNodeArtifact{}, "workspace_id = ? AND instance_id = ?", fix.workspace.ID, instance.ID)
	aiRunCount = count(&aigorm.AIRun{}, "workspace_id = ? AND subject_type = ? AND subject_id = ?",
		fix.workspace.ID, "requirement", fix.requirementID)
	materialCount = count(&requirementgorm.RequirementMaterial{}, "workspace_id = ? AND requirement_id = ?",
		fix.workspace.ID, fix.requirementID)
	if requirementCount != 0 || instanceCount != 0 || tokenCount != 0 || nodeRunCount != 0 ||
		approvalCount != 0 || opinionCount != 0 || jobCount != 0 || artifactCount != 0 ||
		aiRunCount != 0 || materialCount != 0 {
		t.Fatalf("cascade deletion left rows: requirement=%d instance=%d token=%d node_run=%d approval=%d opinion=%d job=%d artifact=%d ai_run=%d material=%d",
			requirementCount, instanceCount, tokenCount, nodeRunCount, approvalCount, opinionCount, jobCount, artifactCount, aiRunCount, materialCount)
	}
}
