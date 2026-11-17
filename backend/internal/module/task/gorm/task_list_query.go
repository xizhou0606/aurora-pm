package taskgorm

import (
	"context"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

const taskRootExpression = "CASE WHEN parent_id IS NULL OR parent_id = 0 THEN id ELSE parent_id END"

var _ task.ToolReadStore = (*ReadRepository)(nil)

func (repository *ReadRepository) ListTasks(ctx context.Context, workspaceID uint, filter task.ListFilter) (task.ListResult, error) {
	query := repository.filteredTasks(ctx, workspaceID, filter)
	if filter.Hierarchy == "filtered_tree" {
		return repository.listFilteredTree(ctx, workspaceID, query, filter)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Model(&WorkTask{}).Count(&total).Error; err != nil {
		return task.ListResult{}, err
	}
	var rows []WorkTask
	if err := query.Order(taskListOrder(filter)).Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return task.ListResult{}, err
	}
	return task.ListResult{Items: taskListItems(rows), Total: total}, nil
}

func (repository *ReadRepository) listFilteredTree(
	ctx context.Context, workspaceID uint, matched *gorm.DB, filter task.ListFilter,
) (task.ListResult, error) {
	var matchedTotal int64
	if err := matched.Session(&gorm.Session{}).Model(&WorkTask{}).Count(&matchedTotal).Error; err != nil {
		return task.ListResult{}, err
	}
	rootSet := matched.Session(&gorm.Session{}).Model(&WorkTask{}).
		Select(taskRootExpression + " AS root_id").Group(taskRootExpression)
	var rootTotal int64
	if err := repository.db.WithContext(ctx).Table("(?) AS matched_roots", rootSet).Count(&rootTotal).Error; err != nil {
		return task.ListResult{}, err
	}
	type rootRow struct{ RootID uint }
	var roots []rootRow
	if err := matched.Session(&gorm.Session{}).Model(&WorkTask{}).
		Select(taskRootExpression + " AS root_id").Group(taskRootExpression).
		Order(taskTreeOrder(filter)).Limit(filter.Limit).Offset(filter.Offset).Scan(&roots).Error; err != nil {
		return task.ListResult{}, err
	}
	result := task.ListResult{Items: []task.ListItem{}, Total: rootTotal, MatchTotal: matchedTotal, RootTotal: rootTotal, PageUnit: "root_task", MatchPolicy: "filtered_tree"}
	if len(roots) == 0 {
		return result, nil
	}
	rootIDs := make([]uint, 0, len(roots))
	for _, row := range roots {
		rootIDs = append(rootIDs, row.RootID)
	}
	var matches, rootRows []WorkTask
	if err := matched.Session(&gorm.Session{}).Where(taskRootExpression+" IN ?", rootIDs).
		Order(taskListOrder(filter)).Find(&matches).Error; err != nil {
		return task.ListResult{}, err
	}
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id IN ? AND mutation_state = ?", workspaceID, rootIDs, workTaskStateActive).Find(&rootRows).Error; err != nil {
		return task.ListResult{}, err
	}
	matchedIDs := make(map[uint]struct{}, len(matches))
	byRoot := make(map[uint][]WorkTask, len(rootIDs))
	for _, row := range matches {
		matchedIDs[row.ID] = struct{}{}
		rootID := row.ID
		if row.ParentID != nil && *row.ParentID != 0 {
			rootID = *row.ParentID
		}
		byRoot[rootID] = append(byRoot[rootID], row)
	}
	rootByID := make(map[uint]WorkTask, len(rootRows))
	for _, row := range rootRows {
		rootByID[row.ID] = row
	}
	seen := map[uint]struct{}{}
	for _, rootID := range rootIDs {
		rows := append([]WorkTask(nil), rootByID[rootID])
		rows = append(rows, byRoot[rootID]...)
		for _, row := range rows {
			if row.ID == 0 {
				continue
			}
			if _, exists := seen[row.ID]; exists {
				continue
			}
			seen[row.ID] = struct{}{}
			item := taskListItem(row)
			_, item.FilterMatch = matchedIDs[row.ID]
			item.ContextOnly = !item.FilterMatch
			result.Items = append(result.Items, item)
		}
	}
	return result, nil
}

func (repository *ReadRepository) filteredTasks(ctx context.Context, workspaceID uint, filter task.ListFilter) *gorm.DB {
	query := repository.db.WithContext(ctx).Model(&WorkTask{}).Where("workspace_id = ? AND mutation_state = ?", workspaceID, workTaskStateActive)
	if len(filter.IDs) > 0 {
		query = query.Where("id IN ?", filter.IDs)
	}
	if filter.ProjectSetID > 0 {
		query = query.Where("project_set_id = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("project_id = ?", filter.PlanUnitID)
	}
	if filter.RequirementID > 0 {
		query = query.Where("requirement_id = ?", filter.RequirementID)
	}
	if filter.TaskClientID > 0 {
		query = query.Where("task_client_id = ?", filter.TaskClientID)
	}
	if filter.AssigneeID > 0 {
		query = query.Where("assignee_id = ?", filter.AssigneeID)
	}
	if filter.ParentID > 0 {
		query = query.Where("parent_id = ?", filter.ParentID)
	}
	if len(filter.Statuses) > 0 {
		query = query.Where("LOWER(TRIM(status)) IN ?", filter.Statuses)
	}
	if len(filter.Priorities) > 0 {
		query = query.Where("LOWER(TRIM(priority)) IN ?", filter.Priorities)
	}
	if len(filter.Kinds) > 0 {
		query = query.Where("LOWER(TRIM(kind)) IN ?", filter.Kinds)
	}
	if filter.Query != "" {
		like := "%" + escapeLike(strings.ToLower(filter.Query)) + "%"
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(task_key) LIKE ? ESCAPE '\\' OR LOWER(description) LIKE ? ESCAPE '\\'", like, like, like)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if filter.CreatedByUserID > 0 {
		query = query.Where(`EXISTS (
			SELECT 1 FROM work_task_activities creator
			WHERE creator.workspace_id = work_tasks.workspace_id
			 AND creator.work_task_id = work_tasks.id
			 AND LOWER(TRIM(creator.action)) IN ('created', 'cloned')
			 AND creator.user_id = ?)`, filter.CreatedByUserID)
	}
	query = applyDueFilter(query, filter)
	if filter.Unfinished {
		query = query.Where("LOWER(TRIM(status)) NOT IN ?", terminalStatuses())
	}
	return query
}

func applyDueFilter(query *gorm.DB, filter task.ListFilter) *gorm.DB {
	now := filter.AsOf.UTC()
	switch filter.DueBucket {
	case "overdue":
		return query.Where("LOWER(TRIM(status)) NOT IN ?", terminalStatuses()).Where("due_at IS NOT NULL AND due_at < ?", dayStartUTC(now))
	case "due_soon":
		return query.Where("LOWER(TRIM(status)) NOT IN ?", terminalStatuses()).Where("due_at IS NOT NULL AND due_at >= ? AND due_at < ?", now, now.AddDate(0, 0, 7))
	case "later":
		return query.Where("due_at IS NOT NULL AND due_at >= ?", now.AddDate(0, 0, 7))
	case "unset":
		return query.Where("due_at IS NULL")
	default:
		return query
	}
}

func taskListOrder(filter task.ListFilter) string {
	order := map[string]string{
		"created_at_desc": "created_at DESC, id DESC", "created_at_asc": "created_at ASC, id ASC",
		"updated_at_desc": "updated_at DESC, id DESC", "updated_at_asc": "updated_at ASC, id ASC",
	}[filter.Sort]
	if order == "" {
		order = "id DESC"
	}
	if filter.GroupBy != "" {
		order = filter.GroupBy + " ASC, " + order
	}
	return order
}

func taskTreeOrder(filter task.ListFilter) string {
	switch filter.Sort {
	case "created_at_asc":
		return "MIN(created_at) ASC, MIN(id) ASC"
	case "updated_at_asc":
		return "MIN(updated_at) ASC, MIN(id) ASC"
	case "updated_at_desc":
		return "MAX(updated_at) DESC, MAX(id) DESC"
	default:
		return "MAX(created_at) DESC, MAX(id) DESC"
	}
}

func taskListItems(rows []WorkTask) []task.ListItem {
	result := make([]task.ListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, taskListItem(row))
	}
	return result
}

func taskListItem(row WorkTask) task.ListItem {
	return task.ListItem{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		PlanUnitID: row.ProjectID, RequirementID: row.RequirementID, TaskClientID: row.TaskClientID, Kind: row.Kind, TaskKey: row.TaskKey,
		Title: row.Title, Description: row.Description, Status: row.Status, Priority: row.Priority,
		AssigneeID: row.AssigneeID, ReporterUserID: row.ReporterUserID, ParentID: row.ParentID,
		EstimatedMinutes: row.EstimatedMinutes, StartsAt: row.StartsAt, DueAt: row.DueAt,
		CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func dayStartUTC(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func terminalStatuses() []string {
	return []string{"completed", "done", "released", "archived", "closed", "cancelled", "canceled"}
}
