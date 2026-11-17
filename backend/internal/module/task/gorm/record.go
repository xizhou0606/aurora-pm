package taskgorm

import (
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

const (
	WorkTaskKindTask    = task.WorkTaskKindTask
	WorkTaskKindSubtask = task.WorkTaskKindSubtask
)

func NormalizePMWorkTaskKind(value string) string {
	return task.NormalizePMWorkTaskKind(value)
}

func IsPMWorkTaskKind(value string) bool {
	return task.IsPMWorkTaskKind(value)
}

func PMWorkTaskKinds() []string {
	return task.PMWorkTaskKinds()
}

type WorkTask struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	WorkspaceID      uint       `gorm:"not null;index;uniqueIndex:ux_task_key_scope" json:"workspace_id"`
	TeamID           uint       `gorm:"not null;index" json:"team_id"`
	ProjectSetID     uint       `gorm:"not null;index;uniqueIndex:ux_task_key_scope" json:"project_set_id"`
	ProjectID        uint       `gorm:"not null;index" json:"project_id"`
	RequirementID    uint       `gorm:"not null;index" json:"requirement_id"`
	TaskClientID     *uint      `gorm:"index" json:"task_client_id,omitempty"`
	Kind             string     `gorm:"size:30;not null;default:task" json:"kind"`
	TaskKey          string     `gorm:"size:60;not null;index;uniqueIndex:ux_task_key_scope" json:"task_key,omitempty"`
	Title            string     `gorm:"size:220;not null" json:"title"`
	Description      string     `gorm:"type:text" json:"description,omitempty"`
	Status           string     `gorm:"size:30;not null;default:draft" json:"status"`
	Priority         string     `gorm:"size:30;not null;default:normal" json:"priority"`
	AssigneeID       *uint      `gorm:"index" json:"assignee_id,omitempty"`
	ReporterUserID   *uint      `gorm:"index" json:"reporter_user_id,omitempty"`
	ParentID         *uint      `gorm:"index" json:"parent_id,omitempty"`
	EstimatedMinutes int        `json:"estimated_minutes,omitempty"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	DueAt            *time.Time `json:"due_at,omitempty"`
	CompletedAt      *time.Time `gorm:"index" json:"completed_at,omitempty"`
	MutationState    string     `gorm:"size:24;not null;default:active;index" json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type WorkTaskActivity struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index" json:"workspace_id"`
	WorkTaskID  uint      `gorm:"not null;index" json:"work_task_id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Action      string    `gorm:"size:80;not null" json:"action"`
	Detail      string    `gorm:"type:text" json:"detail,omitempty"`
	FieldKey    string    `gorm:"size:64" json:"field_key,omitempty"`
	OldValue    string    `gorm:"type:text" json:"old_value,omitempty"`
	NewValue    string    `gorm:"type:text" json:"new_value,omitempty"`
	Summary     string    `gorm:"type:text" json:"summary,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// WorkTaskComment is a PM-owned task comment. UserID stores the shared
// platform fact id (rather than a PM-local identity id), while WorkspaceID
// keeps every lookup and mutation tenant scoped.
type WorkTaskComment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;index:idx_task_comment_scope,priority:1" json:"workspace_id"`
	WorkTaskID  uint      `gorm:"not null;index:idx_task_comment_scope,priority:2;index" json:"work_task_id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Body        string    `gorm:"type:text;not null" json:"body"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

// WorkTaskAttachment stores task attachment metadata. The object itself lives
// in the configured PM-owned S3-compatible bucket; WorkspaceID is duplicated
// deliberately so every attachment lookup can fail closed on tenant scope.
type WorkTaskAttachment struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID    uint      `gorm:"not null;index:idx_task_attachment_scope,priority:1" json:"workspace_id"`
	WorkTaskID     uint      `gorm:"not null;index:idx_task_attachment_scope,priority:2;index" json:"work_task_id"`
	UploaderUserID uint      `gorm:"not null;index" json:"uploader_user_id"`
	Filename       string    `gorm:"size:256;not null" json:"filename"`
	ContentType    string    `gorm:"size:128" json:"content_type"`
	Size           int64     `gorm:"not null" json:"size"`
	ObjectKey      string    `gorm:"size:1024;not null" json:"-"`
	StorageBackend string    `gorm:"size:16;not null;default:s3" json:"storage_backend"`
	MutationState  string    `gorm:"size:24;not null;default:active;index" json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

// WorkTaskWatcher records the authenticated platform user facts that follow a
// task. The workspace column prevents cross-tenant joins even if IDs collide.
type WorkTaskWatcher struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_task_watcher_scope,priority:1;index" json:"workspace_id"`
	WorkTaskID  uint      `gorm:"not null;uniqueIndex:ux_task_watcher_scope,priority:2;index" json:"work_task_id"`
	UserID      uint      `gorm:"not null;uniqueIndex:ux_task_watcher_scope,priority:3;index" json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type KeySequence struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;uniqueIndex:ux_sequence_scope" json:"workspace_id"`
	ProjectSetID uint      `gorm:"not null;uniqueIndex:ux_sequence_scope" json:"project_set_id"`
	Prefix       string    `gorm:"size:20;not null;uniqueIndex:ux_sequence_scope" json:"prefix"`
	NextValue    uint      `gorm:"not null;default:1" json:"next_value"`
	UpdatedAt    time.Time `json:"updated_at"`
}
