package httpapi

import (
	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

// deleteWorkflowRequirementRuntimeTx removes every workflow-, AI-, and
// material-scoped record owned by a requirement so the requirement can be
// deleted without leaving orphaned runtime rows. Audit mutation records and
// published outbox events are intentionally kept: they are facts about what
// happened, not runtime state.
func deleteWorkflowRequirementRuntimeTx(tx *gorm.DB, workspaceID, requirementID uint) error {
	var instanceIDs []uint
	if err := tx.Model(&workflowgorm.WorkflowInstance{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "requirement", requirementID).
		Pluck("id", &instanceIDs).Error; err != nil {
		return err
	}
	if len(instanceIDs) > 0 {
		// Jobs first: once the queue rows are gone the async workers can no
		// longer pick up work for instances that are about to disappear.
		for _, dependent := range []any{
			&workflowgorm.WorkflowJob{},
			&workflowgorm.WorkflowToken{},
			&workflowgorm.WorkflowApproval{},
			&workflowgorm.WorkflowNodeOpinion{},
			&workflowgorm.WorkflowNodeArtifact{},
			&workflowgorm.WorkflowNodeRun{},
		} {
			if err := tx.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
				Delete(dependent).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, instanceIDs).
			Delete(&workflowgorm.WorkflowInstance{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "requirement", requirementID).
		Delete(&aigorm.AIRun{}).Error; err != nil {
		return err
	}
	return tx.Where("workspace_id = ? AND requirement_id = ?", workspaceID, requirementID).
		Delete(&requirementgorm.RequirementMaterial{}).Error
}
