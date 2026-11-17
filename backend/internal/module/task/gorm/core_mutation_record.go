package taskgorm

import "time"

const (
	workTaskStateActive   = "active"
	workTaskStateDeleting = "deleting"
	deletionObjectPending = "pending"
	deletionObjectRemoved = "removed"
)

type TaskDeletionSaga struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;index:idx_task_deletion_saga_scope"`
	ApplyKey    string `gorm:"size:128;not null;uniqueIndex:ux_task_deletion_saga_apply"`
	TaskIDsJSON string `gorm:"type:text;not null"`
	Status      string `gorm:"size:24;not null;index"`
	CompletedAt *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (TaskDeletionSaga) TableName() string { return "task_deletion_sagas" }

type TaskDeletionObject struct {
	ID           uint      `gorm:"primaryKey"`
	SagaID       uint      `gorm:"not null;uniqueIndex:ux_task_deletion_object,priority:1;index"`
	AttachmentID uint      `gorm:"not null;uniqueIndex:ux_task_deletion_object,priority:2"`
	ObjectKey    string    `gorm:"size:1024;not null"`
	Status       string    `gorm:"size:24;not null;index"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (TaskDeletionObject) TableName() string { return "task_deletion_objects" }

func CoreMutationMigrationRecords() []any {
	return []any{&WorkTask{}, &TaskDeletionSaga{}, &TaskDeletionObject{}}
}
