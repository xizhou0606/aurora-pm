package httpapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func applyWorkspaceTaskContractFilters(c *gin.Context, query *gorm.DB) (*gorm.DB, error) {
	if raw, exists := c.GetQuery("task_key"); exists {
		taskKey := strings.ToLower(strings.TrimSpace(raw))
		if taskKey == "" {
			return nil, fmt.Errorf("task_key is invalid")
		}
		query = query.Where("LOWER(TRIM(work_tasks.task_key)) = ?", taskKey)
	}
	if raw, exists := c.GetQuery("title_exact"); exists {
		title := strings.ToLower(strings.TrimSpace(raw))
		if title == "" {
			return nil, fmt.Errorf("title_exact is invalid")
		}
		query = query.Where("LOWER(TRIM(work_tasks.title)) = ?", title)
	}
	if _, exists := c.Request.URL.Query()["ids"]; exists {
		ids, err := parseUintListQuery(c, "ids")
		if err != nil {
			return nil, err
		}
		query = query.Where("work_tasks.id IN ?", ids)
	}
	if raw, exists := c.GetQuery("created_by_user_id"); exists {
		creatorID, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil || creatorID == 0 {
			return nil, fmt.Errorf("created_by_user_id is invalid")
		}
		query = query.Where(`EXISTS (
			SELECT 1 FROM work_task_activities creator
			WHERE creator.workspace_id = work_tasks.workspace_id
			 AND creator.work_task_id = work_tasks.id
			 AND LOWER(TRIM(creator.action)) IN ('created', 'cloned')
			 AND creator.user_id = ?
		)`, uint(creatorID))
	}
	createdFrom, fromSet, _, err := taskCreatedBoundary(c, "created_from")
	if err != nil {
		return nil, err
	}
	createdTo, toSet, toDateOnly, err := taskCreatedBoundary(c, "created_to")
	if err != nil {
		return nil, err
	}
	if fromSet {
		query = query.Where("work_tasks.created_at >= ?", createdFrom)
	}
	if toSet {
		if toDateOnly {
			query = query.Where("work_tasks.created_at < ?", createdTo.AddDate(0, 0, 1))
		} else {
			query = query.Where("work_tasks.created_at <= ?", createdTo)
		}
	}
	if fromSet && toSet {
		upper := createdTo
		if toDateOnly {
			upper = upper.AddDate(0, 0, 1)
			if !createdFrom.Before(upper) {
				return nil, fmt.Errorf("created_from cannot be after created_to")
			}
		} else if createdFrom.After(upper) {
			return nil, fmt.Errorf("created_from cannot be after created_to")
		}
	}
	return query, nil
}

func taskCreatedBoundary(c *gin.Context, name string) (time.Time, bool, bool, error) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return time.Time{}, false, false, nil
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false, false, fmt.Errorf("%s is invalid", name)
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), true, false, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, biLocation(c)); err == nil {
		return parsed.UTC(), true, true, nil
	}
	return time.Time{}, false, false, fmt.Errorf("%s is invalid", name)
}

func (s *Server) taskCreatorIDs(workspaceID uint, taskIDs []uint) map[uint]uint {
	creators, _ := s.loadTaskCreatorIDs(workspaceID, taskIDs)
	return creators
}

func (s *Server) loadTaskCreatorIDs(workspaceID uint, taskIDs []uint) (map[uint]uint, error) {
	creators := make(map[uint]uint, len(taskIDs))
	if workspaceID == 0 || len(taskIDs) == 0 {
		return creators, nil
	}
	var rows []taskgorm.WorkTaskActivity
	if err := s.db.Where(
		"workspace_id = ? AND work_task_id IN ? AND LOWER(TRIM(action)) IN ?",
		workspaceID, taskIDs, []string{"created", "cloned"},
	).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return creators, err
	}
	for _, row := range rows {
		if creators[row.WorkTaskID] == 0 {
			creators[row.WorkTaskID] = row.UserID
		}
	}
	return creators, nil
}
