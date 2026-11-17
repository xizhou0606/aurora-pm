package workflowgorm

import "gorm.io/gorm"

var initializationNodeTypes = []string{
	"start",
	"end",
	"parallel_split",
	"parallel_join",
}

func SubjectInstanceIDsTx(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) ([]uint, error) {
	var ids []uint
	err := tx.Model(&WorkflowInstance{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).
		Order("id").
		Pluck("id", &ids).Error
	return ids, err
}

// GateInstancesTx removes action tokens before inspecting or deleting an
// instance. A transaction rollback restores the tokens with every workflow row.
func GateInstancesTx(tx *gorm.DB, workspaceID uint, instanceIDs []uint) error {
	if len(instanceIDs) == 0 {
		return nil
	}
	return tx.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
		Delete(&WorkflowToken{}).Error
}

// InstancesHaveExecutionHistoryTx distinguishes creation-time workflow
// scaffolding from work that has actually been performed. Callers that also
// own AI subject lifecycle must check their AI execution history separately.
func InstancesHaveExecutionHistoryTx(tx *gorm.DB, workspaceID uint, instanceIDs []uint) (bool, error) {
	if len(instanceIDs) == 0 {
		return false, nil
	}

	checks := []*gorm.DB{
		tx.Model(&WorkflowNodeRun{}).
			Where("workspace_id = ? AND instance_id IN ? AND node_type NOT IN ?", workspaceID, instanceIDs, initializationNodeTypes),
		tx.Model(&WorkflowApproval{}).
			Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs),
		tx.Model(&WorkflowNodeOpinion{}).
			Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs),
		tx.Model(&WorkflowJob{}).
			Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs),
		tx.Model(&WorkflowNodeArtifact{}).
			Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs),
	}
	for _, query := range checks {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func DeleteInstanceRowsAfterGateTx(tx *gorm.DB, workspaceID uint, instanceIDs []uint) error {
	if len(instanceIDs) == 0 {
		return nil
	}
	if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, instanceIDs).
		Delete(&WorkflowInstance{}).Error; err != nil {
		return err
	}
	for _, row := range []any{
		// A legacy resume path can create a token while deletion waits on its
		// instance lock, so sweep tokens again after deleting instances.
		&WorkflowToken{},
		&WorkflowApproval{},
		&WorkflowNodeOpinion{},
		&WorkflowJob{},
		&WorkflowNodeArtifact{},
		&WorkflowNodeRun{},
	} {
		if err := tx.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
			Delete(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func DeleteInstanceRowsTx(tx *gorm.DB, workspaceID uint, instanceIDs []uint) error {
	if err := GateInstancesTx(tx, workspaceID, instanceIDs); err != nil {
		return err
	}
	return DeleteInstanceRowsAfterGateTx(tx, workspaceID, instanceIDs)
}
