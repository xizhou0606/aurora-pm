package taskgorm

import "time"

const (
	taskMutationPending     = "pending"
	taskMutationSucceeded   = "succeeded"
	attachmentStateActive   = "active"
	attachmentStateDeleting = "deleting"
)

type TaskToolMutationApply struct {
	ID          uint      `gorm:"primaryKey"`
	WorkspaceID uint      `gorm:"not null;index:idx_task_tool_apply_workspace"`
	ApplyKey    string    `gorm:"size:128;not null;uniqueIndex:ux_task_tool_apply_key"`
	ChangeSetID string    `gorm:"size:128;not null;uniqueIndex:ux_task_tool_change_set"`
	Operation   string    `gorm:"size:80;not null"`
	InputDigest string    `gorm:"size:64;not null"`
	Status      string    `gorm:"size:24;not null;index"`
	ResultJSON  string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (TaskToolMutationApply) TableName() string { return "task_tool_mutation_applies" }

type AttachmentDeletionSaga struct {
	ID           uint   `gorm:"primaryKey"`
	WorkspaceID  uint   `gorm:"not null;index:idx_task_attachment_delete_scope,priority:1"`
	WorkTaskID   uint   `gorm:"not null;index:idx_task_attachment_delete_scope,priority:2"`
	AttachmentID uint   `gorm:"not null;uniqueIndex:ux_task_attachment_delete_attachment"`
	ApplyKey     string `gorm:"size:128;not null;uniqueIndex:ux_task_attachment_delete_apply"`
	ObjectKey    string `gorm:"size:1024;not null"`
	Status       string `gorm:"size:24;not null;index"`
	CompletedAt  *time.Time
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (AttachmentDeletionSaga) TableName() string { return "task_attachment_deletion_sagas" }

func CollaborationMutationMigrationRecords() []any {
	return []any{&TaskToolMutationApply{}, &AttachmentDeletionSaga{}, &WorkTaskAttachment{}}
}
