package httpapi

import (
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskmodule "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func deleteWorkTaskDependents(tx *gorm.DB, workspaceID uint, taskIDs []uint) error {
	if len(taskIDs) == 0 {
		return nil
	}
	guard, err := taskmodule.NewCascadeDeletionGuardService(taskgorm.NewCascadeDeletionGuardRepository(tx))
	if err != nil {
		return err
	}
	if err := guard.ValidateCascadeDeletion(tx.Statement.Context, taskmodule.CascadeDeletionScope{WorkspaceID: workspaceID, TaskIDs: taskIDs}); err != nil {
		return err
	}
	var executionIDs []uint
	if err := tx.Model(&taskgorm.TaskTestExecution{}).Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Order("id").Pluck("id", &executionIDs).Error; err != nil {
		return err
	}
	if err := deleteTaskTestingDependents(tx, workspaceID, taskIDs); err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.WorkTaskComment{}).Error; err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.WorkTaskActivity{}).Error; err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.WorkTaskWatcher{}).Error; err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.WorkTaskAttachment{}).Error; err != nil {
		return err
	}
	// SourceExecutionID is canonically a TaskTestExecution ID. Older native HTTP
	// data used the owning task ID instead, so the legacy deletion path clears
	// both representations while the owning task-domain saga remains strict.
	executionReferenceIDs := append(executionIDs, taskIDs...)
	if len(executionReferenceIDs) > 0 {
		if err := tx.Model(&qualitygorm.QualityBug{}).Where(
			"workspace_id = ? AND source_execution_id IN ?", workspaceID, executionReferenceIDs,
		).Update("source_execution_id", nil).Error; err != nil {
			return err
		}
	}
	return tx.Model(&qualitygorm.QualityBug{}).Where(
		"workspace_id = ? AND source_test_id IN ?", workspaceID, taskIDs,
	).Update("source_test_id", nil).Error
}
