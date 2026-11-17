package taskgorm

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

var todoUsernameMention = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)

func (repository *ReadRepository) ListPrincipalTodos(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	filter task.TodoFilter,
) (task.TodoPage, error) {
	query := repository.todoTasks(ctx, workspaceID, filter).
		Where("assignee_id = ?", principalID)
	return repository.todoPage(query, filter)
}

func (repository *ReadRepository) ListPrincipalMentions(
	ctx context.Context,
	workspaceID uint,
	principalID uint,
	username string,
	filter task.TodoFilter,
) (task.TodoPage, error) {
	query := repository.todoTasks(ctx, workspaceID, filter)
	var taskIDs []uint
	if err := query.Session(&gorm.Session{}).Select("id").Pluck("id", &taskIDs).Error; err != nil {
		return task.TodoPage{}, err
	}
	if len(taskIDs) == 0 {
		return task.TodoPage{Items: []task.TodoItem{}}, nil
	}
	var comments []WorkTaskComment
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND work_task_id IN ? AND created_at >= ?",
		workspaceID, taskIDs, filter.Now.AddDate(0, 0, -14),
	).Order("created_at DESC, id DESC").Find(&comments).Error; err != nil {
		return task.TodoPage{}, err
	}
	mentionAt := make(map[uint]time.Time)
	orderedIDs := make([]uint, 0)
	for _, comment := range comments {
		if _, exists := mentionAt[comment.WorkTaskID]; exists || !todoCommentMentions(comment.Body, principalID, username) {
			continue
		}
		mentionAt[comment.WorkTaskID] = comment.CreatedAt
		orderedIDs = append(orderedIDs, comment.WorkTaskID)
	}
	total := len(orderedIDs)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	pageIDs := orderedIDs[start:end]
	if len(pageIDs) == 0 {
		return task.TodoPage{Items: []task.TodoItem{}, Total: int64(total)}, nil
	}
	var rows []WorkTask
	if err := query.Session(&gorm.Session{}).Where("id IN ?", pageIDs).Find(&rows).Error; err != nil {
		return task.TodoPage{}, err
	}
	byID := make(map[uint]WorkTask, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	items := make([]task.TodoItem, 0, len(pageIDs))
	for _, id := range pageIDs {
		row, ok := byID[id]
		if !ok {
			continue
		}
		mentionedAt := mentionAt[id]
		items = append(items, task.TodoItem{ListItem: taskListItem(row), MentionAt: &mentionedAt})
	}
	return task.TodoPage{Items: items, Total: int64(total)}, nil
}

func (repository *ReadRepository) todoTasks(ctx context.Context, workspaceID uint, filter task.TodoFilter) *gorm.DB {
	query := repository.db.WithContext(ctx).Model(&WorkTask{}).
		Where("workspace_id = ? AND mutation_state = ? AND LOWER(TRIM(status)) NOT IN ?", workspaceID, workTaskStateActive, terminalStatuses())
	if filter.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("project_id = ?", filter.PlanUnitID)
	}
	if filter.RequirementID > 0 {
		query = query.Where("requirement_id = ?", filter.RequirementID)
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Query)); keyword != "" {
		pattern := "%" + escapeLike(keyword) + "%"
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(task_key) LIKE ? ESCAPE '\\'", pattern, pattern)
	}
	return query
}

func (repository *ReadRepository) todoPage(query *gorm.DB, filter task.TodoFilter) (task.TodoPage, error) {
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return task.TodoPage{}, err
	}
	var rows []WorkTask
	if err := query.Find(&rows).Error; err != nil {
		return task.TodoPage{}, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return todoTaskLess(rows[i], rows[j], filter.Now) })
	start := filter.Offset
	if start > len(rows) {
		start = len(rows)
	}
	end := start + filter.Limit
	if end > len(rows) {
		end = len(rows)
	}
	items := make([]task.TodoItem, 0, end-start)
	for _, row := range rows[start:end] {
		items = append(items, task.TodoItem{ListItem: taskListItem(row)})
	}
	return task.TodoPage{Items: items, Total: total}, nil
}

func todoTaskLess(left, right WorkTask, now time.Time) bool {
	leftPriority, rightPriority := todoTaskPriority(left, now), todoTaskPriority(right, now)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if (left.DueAt == nil) != (right.DueAt == nil) {
		return left.DueAt != nil
	}
	if left.DueAt != nil && !left.DueAt.Equal(*right.DueAt) {
		return left.DueAt.Before(*right.DueAt)
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	return left.ID > right.ID
}

func todoTaskPriority(row WorkTask, now time.Time) int {
	if row.DueAt != nil && row.DueAt.Before(now) {
		return 0
	}
	if status := strings.ToLower(strings.TrimSpace(row.Status)); status == "blocked" || status == "block" {
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(row.Priority)) {
	case "urgent":
		return 2
	case "high":
		return 3
	default:
		return 4
	}
}

func todoCommentMentions(body string, principalID uint, username string) bool {
	for _, id := range todoMentionIDs(body) {
		if id == principalID {
			return true
		}
	}
	for _, match := range todoUsernameMention.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 && strings.EqualFold(strings.TrimSpace(match[1]), username) {
			return true
		}
	}
	return false
}

func todoMentionIDs(body string) []uint {
	trimmed := strings.TrimPrefix(strings.TrimSpace(body), "\ufeff")
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return nil
	}
	var root any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return nil
	}
	seen := map[uint]struct{}{}
	result := []uint{}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if kind, _ := typed["type"].(string); kind == "mention" {
				if attrs, ok := typed["attrs"].(map[string]any); ok {
					for _, key := range []string{"id", "userId", "user_id"} {
						if id := todoMentionID(attrs[key]); id > 0 {
							if _, exists := seen[id]; !exists {
								seen[id] = struct{}{}
								result = append(result, id)
							}
							break
						}
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(root)
	return result
}

func todoMentionID(value any) uint {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case json.Number:
		raw = typed.String()
	case float64:
		if typed <= 0 || typed != float64(uint(typed)) {
			return 0
		}
		return uint(typed)
	default:
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0
	}
	return uint(id)
}

var _ task.TodoReadStore = (*ReadRepository)(nil)
