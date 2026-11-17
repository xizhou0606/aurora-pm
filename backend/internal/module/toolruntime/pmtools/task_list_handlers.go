package pmtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	TaskList                = "task.list"
	TaskStatsOverview       = "task.stats.overview"
	TaskStatsDelivery       = "task.stats.delivery"
	TaskStatsDueCompletion  = "task.stats.due_completion"
	TaskStatsAssigneeStatus = "task.stats.assignee_status"
	TaskCommentList         = "task.comment.list"
	TaskActivityList        = "task.activity.list"
	TaskAttachmentList      = "task.attachment.list"
	TaskGitLinkList         = "task.git_link.list"
	TaskRelatedList         = "task.related.list"
	TaskDueConflictList     = "task.due_conflict.list"
)

var taskReadHandlerNames = []string{
	TaskList,
	TaskStatsOverview,
	TaskStatsDelivery,
	TaskStatsDueCompletion,
	TaskStatsAssigneeStatus,
	TaskCommentList,
	TaskActivityList,
	TaskAttachmentList,
	TaskGitLinkList,
	TaskRelatedList,
	TaskDueConflictList,
}

type TaskReadQueries interface {
	ListTasks(context.Context, uint, task.ListFilter) (task.ListResult, error)
	ListRelated(context.Context, uint, uint) ([]task.RelatedItem, error)
	ListGitLinks(context.Context, uint, uint) ([]any, error)
	ListActivities(context.Context, uint, uint, int, int) ([]task.ActivitySnapshot, int64, error)
	ListComments(context.Context, uint, uint) ([]task.CommentSnapshot, error)
	ListAttachments(context.Context, uint, uint) ([]task.AttachmentSnapshot, error)
	ListDueConflicts(context.Context, uint, uint, time.Time) ([]task.DueConflict, error)
	OverviewStats(context.Context, uint, task.ListFilter) (task.OverviewStats, error)
	DeliveryStats(context.Context, uint, task.ListFilter) (task.DeliveryStats, error)
	DueCompletionStats(context.Context, uint, task.ListFilter) (task.DueCompletionStats, error)
	AssigneeStatusStats(context.Context, uint, task.ListFilter) (task.AssigneeStatusStats, error)
}

var _ TaskReadQueries = (*task.ToolReadService)(nil)

// TaskReadHandlerNames is the independent activation contract for task reads.
// The composition root may merge these names only with handlers built by the
// matching factory below.
func TaskReadHandlerNames() []string { return append([]string(nil), taskReadHandlerNames...) }

func NewTaskReadHandlers(queries TaskReadQueries) (map[string]toolruntime.ToolHandler, error) {
	if queries == nil {
		return nil, ErrReadHandlersInvalid
	}
	handlers := map[string]toolruntime.ToolHandler{
		TaskList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			filter, err := taskFilterInput(call.Input)
			if err != nil {
				return nil, err
			}
			return queries.ListTasks(ctx, call.Authorization.WorkspaceID, filter)
		}),
		TaskRelatedList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			items, err := queries.ListRelated(ctx, call.Authorization.WorkspaceID, input.ID)
			return taskListEnvelope(items), err
		}),
		TaskGitLinkList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			items, err := queries.ListGitLinks(ctx, call.Authorization.WorkspaceID, input.ID)
			return taskListEnvelope(items), err
		}),
		TaskActivityList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input taskActivityListInput
			if err := decodeInput(call.Input, &input); err != nil || input.ID == 0 || input.Offset < 0 {
				return nil, invalidTaskReadInput("id and valid pagination are required")
			}
			items, total, err := queries.ListActivities(ctx, call.Authorization.WorkspaceID, input.ID, input.Limit, input.Offset)
			return taskPageEnvelope[task.ActivitySnapshot]{Items: nonNil(items), Total: total}, err
		}),
		TaskCommentList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			items, err := queries.ListComments(ctx, call.Authorization.WorkspaceID, input.ID)
			return taskListEnvelope(items), err
		}),
		TaskAttachmentList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := inputID(call.Input)
			if err != nil {
				return nil, err
			}
			items, err := queries.ListAttachments(ctx, call.Authorization.WorkspaceID, input.ID)
			return taskListEnvelope(items), err
		}),
		TaskDueConflictList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			var input taskDueConflictInput
			if err := decodeInput(call.Input, &input); err != nil || input.ID == 0 {
				return nil, invalidTaskReadInput("id and due_at are required")
			}
			dueAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.DueAt))
			if err != nil {
				return nil, invalidTaskReadInput("due_at must be RFC3339")
			}
			items, err := queries.ListDueConflicts(ctx, call.Authorization.WorkspaceID, input.ID, dueAt)
			return taskListEnvelope(items), err
		}),
		TaskStatsOverview: taskStatsHandler(func(ctx context.Context, workspaceID uint, filter task.ListFilter) (any, error) {
			return queries.OverviewStats(ctx, workspaceID, filter)
		}),
		TaskStatsDelivery: taskStatsHandler(func(ctx context.Context, workspaceID uint, filter task.ListFilter) (any, error) {
			return queries.DeliveryStats(ctx, workspaceID, filter)
		}),
		TaskStatsDueCompletion: taskStatsHandler(func(ctx context.Context, workspaceID uint, filter task.ListFilter) (any, error) {
			return queries.DueCompletionStats(ctx, workspaceID, filter)
		}),
		TaskStatsAssigneeStatus: taskStatsHandler(func(ctx context.Context, workspaceID uint, filter task.ListFilter) (any, error) {
			return queries.AssigneeStatusStats(ctx, workspaceID, filter)
		}),
	}
	return handlers, nil
}

func taskStatsHandler(invoke func(context.Context, uint, task.ListFilter) (any, error)) toolruntime.ToolHandler {
	return handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
		filter, err := taskFilterInput(call.Input)
		if err != nil {
			return nil, err
		}
		return invoke(ctx, call.Authorization.WorkspaceID, filter)
	})
}

type taskListInput struct {
	IDs             []uint `json:"ids"`
	AssigneeID      uint   `json:"assignee_id"`
	TaskClientID    uint   `json:"task_client_id"`
	CreatedByUserID uint   `json:"created_by_user_id"`
	CreatedFrom     string `json:"created_from"`
	CreatedTo       string `json:"created_to"`
	ParentID        uint   `json:"parent_id"`
	Query           string `json:"q"`
	Status          string `json:"status"`
	Priority        string `json:"priority"`
	Kind            string `json:"kind"`
	DueBucket       string `json:"due_bucket"`
	Unfinished      bool   `json:"unfinished"`
	Sort            string `json:"sort"`
	GroupBy         string `json:"group_by"`
	Hierarchy       string `json:"hierarchy"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
	ProjectSetID    uint   `json:"project_set_id"`
	PlanUnitID      uint   `json:"project_new_id"`
	RequirementID   uint   `json:"requirement_id"`
}

type taskActivityListInput struct {
	ID     uint `json:"id"`
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
}

type taskDueConflictInput struct {
	ID    uint   `json:"id"`
	DueAt string `json:"due_at"`
}

type taskPageEnvelope[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

func taskListEnvelope[T any](items []T) taskPageEnvelope[T] {
	items = nonNil(items)
	return taskPageEnvelope[T]{Items: items, Total: int64(len(items))}
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func taskFilterInput(raw json.RawMessage) (task.ListFilter, error) {
	var input taskListInput
	if err := decodeInput(raw, &input); err != nil || input.Offset < 0 {
		return task.ListFilter{}, invalidTaskReadInput("invalid task filter")
	}
	createdFrom, err := optionalRFC3339(input.CreatedFrom)
	if err != nil {
		return task.ListFilter{}, invalidTaskReadInput("created_from must be RFC3339")
	}
	createdTo, err := optionalRFC3339(input.CreatedTo)
	if err != nil {
		return task.ListFilter{}, invalidTaskReadInput("created_to must be RFC3339")
	}
	return task.ListFilter{
		IDs: input.IDs, AssigneeID: input.AssigneeID, CreatedByUserID: input.CreatedByUserID,
		CreatedFrom: createdFrom, CreatedTo: createdTo, ParentID: input.ParentID, Query: input.Query,
		Statuses: []string{input.Status}, Priorities: []string{input.Priority}, Kinds: []string{input.Kind},
		DueBucket: input.DueBucket, Unfinished: input.Unfinished, Sort: input.Sort, GroupBy: input.GroupBy,
		Hierarchy: input.Hierarchy, Limit: input.Limit, Offset: input.Offset, ProjectSetID: input.ProjectSetID,
		PlanUnitID: input.PlanUnitID, RequirementID: input.RequirementID, TaskClientID: input.TaskClientID,
	}, nil
}

func optionalRFC3339(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func invalidTaskReadInput(message string) error {
	return fmt.Errorf("%w: %s", ErrReadHandlersInvalid, message)
}
