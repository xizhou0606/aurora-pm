package httpapi

import (
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

// deleteTaskTestingDependents is called by the shared task dependency cleanup
// so deletes behave consistently even when a database connection does not
// enforce foreign-key cascades.
func deleteTaskTestingDependents(tx *gorm.DB, workspaceID uint, taskIDs []uint) error {
	if len(taskIDs) == 0 {
		return nil
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.TaskTestExecutionRun{}).Error; err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.TaskTestExecution{}).Error; err != nil {
		return err
	}
	return tx.Where("workspace_id = ? AND work_task_id IN ?", workspaceID, taskIDs).
		Delete(&taskgorm.TaskTestDefinition{}).Error
}
