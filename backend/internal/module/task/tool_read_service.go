package task

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ToolReadStore interface {
	ListTasks(context.Context, uint, ListFilter) (ListResult, error)
	ListRelated(context.Context, uint, uint) ([]RelatedItem, error)
	ListActivities(context.Context, uint, uint, int, int) ([]ActivitySnapshot, int64, error)
	ListComments(context.Context, uint, uint) ([]CommentSnapshot, error)
	ListAttachments(context.Context, uint, uint) ([]AttachmentSnapshot, error)
	ListDueConflicts(context.Context, uint, uint, time.Time) ([]DueConflict, error)
	OverviewStats(context.Context, uint, ListFilter) (OverviewStats, error)
	DeliveryStats(context.Context, uint, ListFilter) (DeliveryStats, error)
	DueCompletionStats(context.Context, uint, ListFilter) (DueCompletionStats, error)
	AssigneeStatusStats(context.Context, uint, ListFilter) (AssigneeStatusStats, error)
}

type ToolReadService struct {
	store ToolReadStore
	now   func() time.Time
}

func NewToolReadService(store ToolReadStore) (*ToolReadService, error) {
	if store == nil {
		return nil, ErrReadQueryInvalid
	}
	return &ToolReadService{store: store, now: time.Now}, nil
}

func (service *ToolReadService) ListTasks(ctx context.Context, workspaceID uint, filter ListFilter) (ListResult, error) {
	if err := service.prepareFilter(workspaceID, &filter, true); err != nil {
		return ListResult{}, err
	}
	return service.store.ListTasks(ctx, workspaceID, filter)
}

func (service *ToolReadService) ListRelated(ctx context.Context, workspaceID, taskID uint) ([]RelatedItem, error) {
	if workspaceID == 0 || taskID == 0 {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListRelated(ctx, workspaceID, taskID)
}

func (service *ToolReadService) ListGitLinks(ctx context.Context, workspaceID, taskID uint) ([]any, error) {
	if _, err := service.ListRelated(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	// PM intentionally does not own source-control integrations.
	return []any{}, nil
}

func (service *ToolReadService) ListActivities(ctx context.Context, workspaceID, taskID uint, limit, offset int) ([]ActivitySnapshot, int64, error) {
	if workspaceID == 0 || taskID == 0 || offset < 0 {
		return nil, 0, ErrReadQueryInvalid
	}
	limit = normalizedLimit(limit, 40, 500)
	return service.store.ListActivities(ctx, workspaceID, taskID, limit, offset)
}

func (service *ToolReadService) ListComments(ctx context.Context, workspaceID, taskID uint) ([]CommentSnapshot, error) {
	if workspaceID == 0 || taskID == 0 {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListComments(ctx, workspaceID, taskID)
}

func (service *ToolReadService) ListAttachments(ctx context.Context, workspaceID, taskID uint) ([]AttachmentSnapshot, error) {
	if workspaceID == 0 || taskID == 0 {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListAttachments(ctx, workspaceID, taskID)
}

func (service *ToolReadService) ListDueConflicts(ctx context.Context, workspaceID, taskID uint, dueAt time.Time) ([]DueConflict, error) {
	if workspaceID == 0 || taskID == 0 || dueAt.IsZero() {
		return nil, ErrReadQueryInvalid
	}
	return service.store.ListDueConflicts(ctx, workspaceID, taskID, dueAt.UTC())
}

func (service *ToolReadService) OverviewStats(ctx context.Context, workspaceID uint, filter ListFilter) (OverviewStats, error) {
	if err := service.prepareFilter(workspaceID, &filter, false); err != nil {
		return OverviewStats{}, err
	}
	return service.store.OverviewStats(ctx, workspaceID, filter)
}

func (service *ToolReadService) DeliveryStats(ctx context.Context, workspaceID uint, filter ListFilter) (DeliveryStats, error) {
	if err := service.prepareFilter(workspaceID, &filter, false); err != nil {
		return DeliveryStats{}, err
	}
	return service.store.DeliveryStats(ctx, workspaceID, filter)
}

func (service *ToolReadService) DueCompletionStats(ctx context.Context, workspaceID uint, filter ListFilter) (DueCompletionStats, error) {
	if err := service.prepareFilter(workspaceID, &filter, false); err != nil {
		return DueCompletionStats{}, err
	}
	return service.store.DueCompletionStats(ctx, workspaceID, filter)
}

func (service *ToolReadService) AssigneeStatusStats(ctx context.Context, workspaceID uint, filter ListFilter) (AssigneeStatusStats, error) {
	if err := service.prepareFilter(workspaceID, &filter, false); err != nil {
		return AssigneeStatusStats{}, err
	}
	return service.store.AssigneeStatusStats(ctx, workspaceID, filter)
}

func (service *ToolReadService) prepareFilter(workspaceID uint, filter *ListFilter, paginate bool) error {
	if workspaceID == 0 || filter == nil || filter.Offset < 0 {
		return ErrReadQueryInvalid
	}
	filter.Query = strings.TrimSpace(filter.Query)
	filter.DueBucket = strings.ToLower(strings.TrimSpace(filter.DueBucket))
	filter.Sort = strings.ToLower(strings.TrimSpace(filter.Sort))
	filter.GroupBy = strings.ToLower(strings.TrimSpace(filter.GroupBy))
	filter.Hierarchy = strings.ToLower(strings.TrimSpace(filter.Hierarchy))
	filter.Statuses = normalizedValues(filter.Statuses)
	filter.Priorities = normalizedValues(filter.Priorities)
	filter.Kinds = normalizedValues(filter.Kinds)
	if filter.CreatedFrom != nil && filter.CreatedTo != nil && filter.CreatedFrom.After(*filter.CreatedTo) {
		return fmt.Errorf("%w: created_from cannot be after created_to", ErrReadQueryInvalid)
	}
	if !oneOf(filter.DueBucket, "", "overdue", "due_soon", "later", "unset") ||
		!oneOf(filter.Sort, "", "created_at_desc", "created_at_asc", "updated_at_desc", "updated_at_asc") ||
		!oneOf(filter.GroupBy, "", "requirement_id", "assignee_id", "priority") ||
		!oneOf(filter.Hierarchy, "", "filtered_tree") {
		return ErrReadQueryInvalid
	}
	if paginate {
		filter.Limit = normalizedLimit(filter.Limit, 50, 500)
	}
	filter.AsOf = service.now().UTC()
	return nil
}

func normalizedLimit(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func normalizedValues(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.ToLower(strings.TrimSpace(part))
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
