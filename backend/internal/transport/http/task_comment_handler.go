package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

const (
	taskCommentBodyLimit        = 100_000
	taskCommentVisibleTextLimit = 20_000
)

type taskCommentBody struct {
	Body string `json:"body"`
}

func (s *Server) listTaskComments(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var comments []taskgorm.WorkTaskComment
	if s.db.Where("workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID).
		Order("id ASC").Find(&comments).Error != nil {
		dbFail(c)
		return
	}
	ok(c, s.taskCommentDTOs(c, comments))
}

func (s *Server) createTaskComment(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var body taskCommentBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		bad(c, "body required")
		return
	}
	if len(body.Body) > taskCommentBodyLimit || quality.RichTextRuneCount(body.Body) > taskCommentVisibleTextLimit {
		bad(c, "body is too long")
		return
	}
	comment := taskgorm.WorkTaskComment{
		WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID,
		UserID: currentFactUserID(c), Body: body.Body,
	}
	notificationContext := s.loadWorkspaceContext(
		task.WorkspaceID,
		workspaceContextReferences{}.withTask(task).withUserID(comment.UserID),
	)
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
		if err := tx.Create(&comment).Error; err != nil {
			return err
		}
		if err := tx.Create(&taskgorm.WorkTaskActivity{
			WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, UserID: comment.UserID,
			Action: "comment", Detail: truncateTaskComment(commentNotificationExcerpt(body.Body), 500),
		}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(
			tx, c, project.ID, "work_task_comment", comment.ID, "created", task.Title, comment,
		); err != nil {
			return err
		}
		return s.recordTaskCommentNotification(tx, c, task, comment, notificationContext)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	created(c, s.taskCommentDTO(c, comment))
}

func (s *Server) deleteTaskComment(c *gin.Context) {
	commentID, valid := paramID(c, "cid")
	if !valid {
		return
	}
	workspace, _ := currentWorkspace(c)
	var comment taskgorm.WorkTaskComment
	if s.db.Where("id = ? AND workspace_id = ?", commentID, workspace.ID).
		First(&comment).Error != nil {
		fail(c, http.StatusNotFound, "comment not found")
		return
	}
	if rawParent := strings.TrimSpace(c.Param("id")); rawParent != "" {
		parentID, err := strconv.ParseUint(rawParent, 10, 64)
		if err != nil || uint(parentID) != comment.WorkTaskID {
			fail(c, http.StatusNotFound, "comment not found")
			return
		}
	}
	task, project, allowed := s.taskByID(c, comment.WorkTaskID)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"workspace_id = ? AND work_task_id = ? AND user_id = ? AND action = ? AND created_at BETWEEN ? AND ?",
			comment.WorkspaceID, comment.WorkTaskID, comment.UserID, "comment",
			comment.CreatedAt.Add(-15*time.Second), comment.CreatedAt.Add(15*time.Second),
		).Where("detail = ?", truncateTaskComment(comment.Body, 500)).
			Delete(&taskgorm.WorkTaskActivity{}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(
			tx, c, project.ID, "work_task_comment", comment.ID, "deleted", task.Title, comment,
		); err != nil {
			return err
		}
		result := tx.Where("id = ? AND workspace_id = ? AND work_task_id = ?",
			comment.ID, comment.WorkspaceID, comment.WorkTaskID).Delete(&taskgorm.WorkTaskComment{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) taskCommentDTOs(c *gin.Context, rows []taskgorm.WorkTaskComment) []gin.H {
	workspace, _ := currentWorkspace(c)
	refs := workspaceContextReferences{}
	for _, row := range rows {
		refs = refs.withUserID(row.UserID)
	}
	ctx := s.loadWorkspaceContext(workspace.ID, refs)
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result = append(result, s.taskCommentDTOWithContext(c, row, ctx))
	}
	return result
}

func (s *Server) taskCommentDTO(c *gin.Context, row taskgorm.WorkTaskComment) gin.H {
	workspace, _ := currentWorkspace(c)
	ctx := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withUserID(row.UserID),
	)
	return s.taskCommentDTOWithContext(c, row, ctx)
}

func (s *Server) taskCommentDTOWithContext(
	_ *gin.Context, row taskgorm.WorkTaskComment, ctx workspaceContext,
) gin.H {
	result := gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "work_task_id": row.WorkTaskID,
		"user_id": row.UserID, "body": row.Body, "created_at": row.CreatedAt,
	}
	if user, ok := ctx.Users[row.UserID]; ok {
		result["username"] = user.Username
		result["display_name"] = user.DisplayName
	} else {
		result["display_name"] = retiredUserDisplayName(row.UserID)
	}
	return result
}

func truncateTaskComment(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
