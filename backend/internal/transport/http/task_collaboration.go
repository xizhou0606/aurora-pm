package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) userWatchesTask(workspaceID, taskID, userID uint) bool {
	if workspaceID == 0 || taskID == 0 || userID == 0 {
		return false
	}
	var count int64
	if s.db.Model(&taskgorm.WorkTaskWatcher{}).Where(
		"workspace_id = ? AND work_task_id = ? AND user_id = ?", workspaceID, taskID, userID,
	).Count(&count).Error != nil {
		return false
	}
	return count > 0
}

func (s *Server) taskWatchSet(workspaceID, userID uint, taskIDs []uint) map[uint]bool {
	result := map[uint]bool{}
	if workspaceID == 0 || userID == 0 || len(taskIDs) == 0 {
		return result
	}
	var rows []taskgorm.WorkTaskWatcher
	if s.db.Select("work_task_id").Where(
		"workspace_id = ? AND user_id = ? AND work_task_id IN ?", workspaceID, userID, taskIDs,
	).Find(&rows).Error != nil {
		return result
	}
	for _, row := range rows {
		result[row.WorkTaskID] = true
	}
	return result
}

func (s *Server) putTaskWatch(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	var body struct {
		Watch *bool `json:"watch"`
	}
	if c.ShouldBindJSON(&body) != nil || body.Watch == nil {
		bad(c, "watch required")
		return
	}
	userID := currentFactUserID(c)
	action, summary := "unwatch", "已取消关注此工作项"
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, task.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		lockedTask, err := lockProjectWorkTaskTx(tx, task.WorkspaceID, task.ID, project.ID)
		if err != nil {
			return err
		}
		task = lockedTask
		if *body.Watch {
			action, summary = "watch", "已关注此工作项"
			row := taskgorm.WorkTaskWatcher{WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, UserID: userID}
			if err := tx.Where(
				"workspace_id = ? AND work_task_id = ? AND user_id = ?", task.WorkspaceID, task.ID, userID,
			).FirstOrCreate(&row).Error; err != nil {
				return err
			}
		} else if err := tx.Where(
			"workspace_id = ? AND work_task_id = ? AND user_id = ?", task.WorkspaceID, task.ID, userID,
		).Delete(&taskgorm.WorkTaskWatcher{}).Error; err != nil {
			return err
		}
		return tx.Create(&taskgorm.WorkTaskActivity{
			WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, UserID: userID,
			Action: action, Summary: summary,
		}).Error
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	ok(c, gin.H{"watched": *body.Watch, "work_task_id": task.ID})
}

func (s *Server) listTaskRelated(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	current, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	type related struct {
		Relation string `json:"relation"`
		Task     gin.H  `json:"task"`
	}
	type relatedTask struct {
		relation string
		task     taskgorm.WorkTask
	}
	pendingRows := make([]relatedTask, 0)
	appendTask := func(relation string, task taskgorm.WorkTask) {
		pendingRows = append(pendingRows, relatedTask{relation: relation, task: task})
	}
	base := pmTaskQuery(s.db).Where(
		"workspace_id = ? AND project_set_id = ? AND requirement_id = ?",
		current.WorkspaceID, current.ProjectSetID, current.RequirementID,
	)
	if current.ParentID != nil {
		var parent taskgorm.WorkTask
		if base.Session(&gorm.Session{}).Where("id = ?", *current.ParentID).First(&parent).Error == nil {
			appendTask("parent", parent)
		}
	}
	var children []taskgorm.WorkTask
	_ = base.Session(&gorm.Session{}).Where("parent_id = ?", current.ID).
		Order("id DESC").Limit(200).Find(&children).Error
	for _, child := range children {
		appendTask("child", child)
	}
	if current.ParentID != nil {
		var siblings []taskgorm.WorkTask
		_ = base.Session(&gorm.Session{}).Where("parent_id = ? AND id <> ?", *current.ParentID, current.ID).
			Order("id DESC").Limit(200).Find(&siblings).Error
		for _, sibling := range siblings {
			appendTask("sibling", sibling)
		}
	}
	taskIDs := make([]uint, 0, len(pendingRows))
	for _, item := range pendingRows {
		taskIDs = append(taskIDs, item.task.ID)
	}
	creators := s.taskCreatorIDs(current.WorkspaceID, taskIDs)
	refs := workspaceContextReferences{}
	for _, item := range pendingRows {
		refs = refs.withTask(item.task).withUserID(creators[item.task.ID])
	}
	context := s.loadWorkspaceContext(current.WorkspaceID, refs)
	rows := make([]related, 0, len(pendingRows))
	for _, item := range pendingRows {
		row := context.enrichTask(taskDTO(item.task), item.task)
		row = context.enrichTaskCreator(row, creators[item.task.ID])
		row["watched"] = s.userWatchesTask(item.task.WorkspaceID, item.task.ID, currentFactUserID(c))
		rows = append(rows, related{Relation: item.relation, Task: row})
	}
	ok(c, gin.H{"items": rows})
}

func (s *Server) listTaskActivities(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	query := s.db.Where("workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID)
	var total int64
	if query.Model(&taskgorm.WorkTaskActivity{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 40)
	var activities []taskgorm.WorkTaskActivity
	if query.Order("id DESC").Limit(limit).Offset(offset).Find(&activities).Error != nil {
		dbFail(c)
		return
	}
	refs := workspaceContextReferences{}
	for _, activity := range activities {
		refs = refs.withUserID(activity.UserID)
	}
	context := s.loadWorkspaceContext(task.WorkspaceID, refs)
	items := make([]gin.H, 0, len(activities))
	for _, activity := range activities {
		field, oldValue, newValue, summary := activity.FieldKey, activity.OldValue, activity.NewValue, activity.Summary
		if field == "" && strings.HasPrefix(strings.TrimSpace(activity.Detail), "{") {
			var legacy map[string]string
			if json.Unmarshal([]byte(activity.Detail), &legacy) == nil {
				field, oldValue, newValue, summary = legacy["field_key"], legacy["old_value"], legacy["new_value"], legacy["summary"]
			}
		}
		userID := activity.UserID
		name := userDisplayName(context, &userID)
		items = append(items, gin.H{
			"id": activity.ID, "workspace_id": activity.WorkspaceID, "work_task_id": activity.WorkTaskID,
			"user_id": activity.UserID, "action": activity.Action, "detail": activity.Detail,
			"field_key": field, "old_value": oldValue, "new_value": newValue, "summary": summary,
			"username": name, "display_name": name, "created_at": activity.CreatedAt,
		})
	}
	ok(c, gin.H{"items": items, "total": total})
}

// PM deliberately does not own source-control integrations. Returning an
// empty compatibility collection keeps the migrated activity module intact
// without importing CI/CD domain code or data.
func (s *Server) listTaskGitLinks(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	if _, _, allowed := s.taskByID(c, id); !allowed {
		return
	}
	ok(c, []any{})
}

func taskActivityValue(value any) string {
	if value == nil {
		return ""
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflected.IsNil() {
			return ""
		}
	}
	switch typed := value.(type) {
	case fmt.Stringer:
		return typed.String()
	case *uint:
		if typed == nil {
			return ""
		}
		return fmt.Sprint(*typed)
	case *int:
		if typed == nil {
			return ""
		}
		return fmt.Sprint(*typed)
	case *float64:
		if typed == nil {
			return ""
		}
		return fmt.Sprint(*typed)
	case *string:
		if typed == nil {
			return ""
		}
		return *typed
	default:
		return fmt.Sprint(value)
	}
}

func taskFieldLabel(field string) string {
	labels := map[string]string{
		"title": "标题", "description": "描述", "status": "状态", "priority": "优先级",
		"assignee_id": "经办人", "reporter_user_id": "报告人", "requirement_id": "需求", "parent_id": "父项",
		"estimated_minutes": "预估工时", "starts_at": "开始时间", "due_at": "截止日期",
	}
	if label := labels[field]; label != "" {
		return label
	}
	return field
}

func failTaskConflict(c *gin.Context, message string) {
	fail(c, http.StatusConflict, message)
}
