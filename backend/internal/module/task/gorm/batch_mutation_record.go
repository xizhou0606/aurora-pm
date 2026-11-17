package taskgorm

import "time"

// TaskBatchDeletionApply freezes a batch selector to exact task IDs before the
// canonical TaskDeletionService starts. The canonical deletion ledger remains
// authoritative for cleanup and completion; this record only makes task_ref
// replay possible after the selected rows have been deleted.
type TaskBatchDeletionApply struct {
	ID           uint      `gorm:"primaryKey"`
	WorkspaceID  uint      `gorm:"not null;index;uniqueIndex:ux_task_batch_delete_workspace_apply"`
	ApplyKey     string    `gorm:"size:160;not null;uniqueIndex:ux_task_batch_delete_workspace_apply"`
	ChangeSetID  string    `gorm:"size:160;not null;uniqueIndex"`
	InputDigest  string    `gorm:"size:64;not null"`
	SelectorJSON string    `gorm:"type:text;not null"`
	RootIDsJSON  string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (TaskBatchDeletionApply) TableName() string { return "task_batch_deletion_applies" }

// BatchMutationMigrationRecords belongs to schema version 0053. Database
// assembly owns the version pointer and calls this function from that step.
func BatchMutationMigrationRecords() []any { return []any{&TaskBatchDeletionApply{}} }
