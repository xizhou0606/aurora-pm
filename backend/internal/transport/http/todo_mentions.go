package httpapi

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

const (
	workbenchMentionDays = 14
)

func (s *Server) fetchWorkbenchTodoMentions(
	c *gin.Context,
	scope workspaceScope,
	limit int,
	offset int,
) ([]gin.H, int, error) {
	workspace, _ := currentWorkspace(c)
	a, _ := currentActor(c)
	query := applyWorkspaceScope(
		pmTaskQuery(s.db).Where("workspace_id = ?", workspace.ID), scope,
	)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspace.ID).
		Where("LOWER(TRIM(status)) NOT IN ?", terminalTaskStatuses())
	// Project data access is enforced in SQL on work_tasks.project_set_id so
	// a task in a project the user cannot access never reaches this handler —
	// not even as a mention carrier. Filtering in Go after fetching would leak
	// task titles, comment bodies and comment times of inaccessible projects.
	s.applyProjectDataFilter(c, query, "work_tasks", "project_set_id")
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := escapedLikePattern(search)
		query = query.Where(
			"LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(task_key) LIKE ? ESCAPE '\\'", like, like,
		)
	}

	var taskIDs []uint
	if err := query.Session(&gorm.Session{}).Model(&taskgorm.WorkTask{}).Select("work_tasks.id").
		Pluck("work_tasks.id", &taskIDs).Error; err != nil {
		return nil, 0, err
	}
	if len(taskIDs) == 0 {
		return []gin.H{}, 0, nil
	}
	var comments []taskgorm.WorkTaskComment
	if err := s.db.Where(
		"workspace_id = ? AND work_task_id IN ? AND created_at >= ?",
		workspace.ID, taskIDs, time.Now().UTC().AddDate(0, 0, -workbenchMentionDays),
	).Order("created_at DESC, id DESC").Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	mentionAt := map[uint]time.Time{}
	orderedIDs := make([]uint, 0)
	for _, comment := range comments {
		if _, exists := mentionAt[comment.WorkTaskID]; exists {
			continue
		}
		if !taskCommentMentionsUser(comment.Body, factUserID(a.User), a.User.Username) {
			continue
		}
		mentionAt[comment.WorkTaskID] = comment.CreatedAt
		orderedIDs = append(orderedIDs, comment.WorkTaskID)
	}
	total := len(orderedIDs)
	if limit <= 0 || offset >= total {
		return []gin.H{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	pageIDs := orderedIDs[offset:end]
	var tasks []taskgorm.WorkTask
	if err := query.Session(&gorm.Session{}).Where("work_tasks.id IN ?", pageIDs).
		Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	byID := make(map[uint]taskgorm.WorkTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	ctx := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTasks(tasks),
	)
	rows := make([]gin.H, 0, len(pageIDs))
	for _, taskID := range pageIDs {
		task, exists := byID[taskID]
		if !exists {
			continue
		}
		if _, exists := ctx.ProjectSets[task.ProjectSetID]; !exists {
			continue
		}
		row := ctx.enrichTask(taskDTO(task), task)
		row["mention_at"] = mentionAt[task.ID]
		rows = append(rows, row)
	}
	return rows, total, nil
}
