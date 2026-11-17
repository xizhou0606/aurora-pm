package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockWorkflowInstanceTx(tx *gorm.DB, workspaceID, instanceID uint) error {
	var instance workflowgorm.WorkflowInstance
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ? AND workspace_id = ?", instanceID, workspaceID).
		First(&instance).Error
}

func lockAIRunTx(tx *gorm.DB, workspaceID, runID uint) error {
	var run aigorm.AIRun
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ? AND workspace_id = ?", runID, workspaceID).
		First(&run).Error
}
