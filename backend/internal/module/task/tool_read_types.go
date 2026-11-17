package task

import "time"

type ListFilter struct {
	IDs             []uint
	AssigneeID      uint
	CreatedByUserID uint
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
	ParentID        uint
	Query           string
	Statuses        []string
	Priorities      []string
	Kinds           []string
	DueBucket       string
	Unfinished      bool
	Sort            string
	GroupBy         string
	Hierarchy       string
	ProjectSetID    uint
	PlanUnitID      uint
	RequirementID   uint
	TaskClientID    uint
	Limit           int
	Offset          int
	AsOf            time.Time
}

type ListItem struct {
	ID               uint       `json:"id"`
	WorkspaceID      uint       `json:"workspace_id"`
	TeamID           uint       `json:"team_id"`
	ProjectSetID     uint       `json:"project_set_id"`
	PlanUnitID       uint       `json:"project_new_id"`
	RequirementID    uint       `json:"requirement_id"`
	TaskClientID     *uint      `json:"task_client_id,omitempty"`
	Kind             string     `json:"kind"`
	TaskKey          string     `json:"task_key"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	AssigneeID       *uint      `json:"assignee_id,omitempty"`
	ReporterUserID   *uint      `json:"reporter_user_id,omitempty"`
	ParentID         *uint      `json:"parent_id,omitempty"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	DueAt            *time.Time `json:"due_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	FilterMatch      bool       `json:"filter_match,omitempty"`
	ContextOnly      bool       `json:"context_only,omitempty"`
}

type ListResult struct {
	Items       []ListItem `json:"items"`
	Total       int64      `json:"total"`
	MatchTotal  int64      `json:"match_total,omitempty"`
	RootTotal   int64      `json:"root_total,omitempty"`
	PageUnit    string     `json:"page_unit,omitempty"`
	MatchPolicy string     `json:"match_policy,omitempty"`
}

type RelatedItem struct {
	Relation string   `json:"relation"`
	Task     ListItem `json:"task"`
}

type ActivitySnapshot struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	WorkTaskID  uint      `json:"work_task_id"`
	UserID      uint      `json:"user_id"`
	Action      string    `json:"action"`
	Detail      string    `json:"detail,omitempty"`
	FieldKey    string    `json:"field_key,omitempty"`
	OldValue    string    `json:"old_value,omitempty"`
	NewValue    string    `json:"new_value,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CommentSnapshot struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	WorkTaskID  uint      `json:"work_task_id"`
	UserID      uint      `json:"user_id"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type AttachmentSnapshot struct {
	ID             uint      `json:"id"`
	WorkspaceID    uint      `json:"workspace_id"`
	WorkTaskID     uint      `json:"work_task_id"`
	UploaderUserID uint      `json:"uploader_user_id"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type,omitempty"`
	Size           int64     `json:"size"`
	StorageBackend string    `json:"storage_backend"`
	CreatedAt      time.Time `json:"created_at"`
}

type DueConflict struct {
	ID      uint      `json:"id"`
	TaskKey string    `json:"task_key"`
	Title   string    `json:"title"`
	DueAt   time.Time `json:"due_at"`
}

type DeliveryStats struct {
	Total          int64            `json:"total"`
	Completed      int64            `json:"completed"`
	Open           int64            `json:"open"`
	Overdue        int64            `json:"overdue"`
	CompletionRate float64          `json:"completion_rate"`
	ByStatus       map[string]int64 `json:"by_status"`
	ByPriority     map[string]int64 `json:"by_priority"`
}

type OverviewStats struct {
	Total           int64            `json:"total"`
	Completed       int64            `json:"completed"`
	Incomplete      int64            `json:"incomplete"`
	Overdue         int64            `json:"overdue"`
	DueSoon         int64            `json:"due_soon"`
	DueSoonDays     int              `json:"due_soon_days"`
	ByKind          map[string]int64 `json:"by_kind"`
	ByKindCompleted map[string]int64 `json:"by_kind_completed"`
	ByStatus        map[string]int64 `json:"by_status"`
}

type DueCompletionStats struct {
	Total               int64     `json:"total"`
	Overdue             int64     `json:"overdue"`
	DueToday            int64     `json:"due_today"`
	DueSoon             int64     `json:"due_soon"`
	Later               int64     `json:"later"`
	OpenWithoutDue      int64     `json:"open_without_due"`
	CompletedOnTime     int64     `json:"completed_on_time"`
	CompletedLate       int64     `json:"completed_late"`
	CompletedWithoutDue int64     `json:"completed_without_due"`
	AsOf                time.Time `json:"as_of"`
	Timezone            string    `json:"timezone"`
}

type AssigneeStatusStats struct {
	Items          []AssigneeStatusItem `json:"items"`
	TotalAssignees int                  `json:"total_assignees"`
}

type AssigneeStatusItem struct {
	AssigneeID *uint            `json:"assignee_id"`
	ByStatus   map[string]int64 `json:"by_status"`
	Total      int64            `json:"total"`
}
