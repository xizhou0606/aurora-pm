package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

const (
	qualityCommentBodyLimit        = 100_000
	qualityCommentVisibleTextLimit = 5_000
)

type qualityCommentBody struct {
	Body string `json:"body"`
}

func (s *Server) listQualityBugComments(c *gin.Context) {
	bugID, valid := paramID(c, "id")
	if !valid {
		return
	}
	bug, _, allowed := s.qualityBugByID(c, bugID)
	if !allowed {
		return
	}
	var comments []qualitygorm.QualityBugComment
	if s.db.Where("workspace_id = ? AND quality_bug_id = ?", bug.WorkspaceID, bug.ID).
		Order("id ASC").Find(&comments).Error != nil {
		dbFail(c)
		return
	}
	ok(c, s.qualityBugCommentDTOs(c, comments))
}

func (s *Server) createQualityBugComment(c *gin.Context) {
	bugID, valid := paramID(c, "id")
	if !valid {
		return
	}
	bug, project, allowed := s.qualityBugByID(c, bugID)
	if !allowed {
		return
	}
	var body qualityCommentBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		bad(c, "body required")
		return
	}
	if len(body.Body) > qualityCommentBodyLimit || quality.RichTextRuneCount(body.Body) > qualityCommentVisibleTextLimit {
		bad(c, "body is too long")
		return
	}
	comment := qualitygorm.QualityBugComment{
		WorkspaceID: bug.WorkspaceID, QualityBugID: bug.ID,
		UserID: currentFactUserID(c), Body: body.Body,
	}
	notificationContext := s.loadWorkspaceContext(
		bug.WorkspaceID,
		workspaceContextReferences{}.withQualityBug(bug).withUserID(comment.UserID),
	)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, bug.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		lockedBug, err := lockProjectQualityBugTx(tx, bug.WorkspaceID, bug.ID, project.ID)
		if err != nil {
			return err
		}
		bug = lockedBug
		if err := tx.Create(&comment).Error; err != nil {
			return err
		}
		if err := tx.Create(&qualitygorm.QualityBugActivity{
			WorkspaceID: bug.WorkspaceID, QualityBugID: bug.ID, UserID: comment.UserID,
			Action: "comment", Summary: truncateTaskComment(commentNotificationExcerpt(body.Body), 500),
		}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(
			tx, c, project.ID, "quality_bug_comment", comment.ID, "created", bug.Title, comment,
		); err != nil {
			return err
		}
		return s.recordQualityCommentNotification(
			tx, c, bug, comment, notificationContext,
		)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	created(c, s.qualityBugCommentDTO(c, comment))
}

func (s *Server) deleteQualityBugComment(c *gin.Context) {
	commentID, valid := paramID(c, "cid")
	if !valid {
		return
	}
	workspace, _ := currentWorkspace(c)
	var comment qualitygorm.QualityBugComment
	if s.db.Where("id = ? AND workspace_id = ?", commentID, workspace.ID).
		First(&comment).Error != nil {
		fail(c, http.StatusNotFound, "comment not found")
		return
	}
	if rawParent := strings.TrimSpace(c.Param("id")); rawParent != "" {
		parentID, err := strconv.ParseUint(rawParent, 10, 64)
		if err != nil || uint(parentID) != comment.QualityBugID {
			fail(c, http.StatusNotFound, "comment not found")
			return
		}
	}
	bug, project, allowed := s.qualityBugByID(c, comment.QualityBugID)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"workspace_id = ? AND quality_bug_id = ? AND user_id = ? AND action = ? AND created_at BETWEEN ? AND ?",
			comment.WorkspaceID, comment.QualityBugID, comment.UserID, "comment",
			comment.CreatedAt.Add(-15*time.Second), comment.CreatedAt.Add(15*time.Second),
		).Where("summary = ?", truncateTaskComment(commentNotificationExcerpt(comment.Body), 500)).
			Delete(&qualitygorm.QualityBugActivity{}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(
			tx, c, project.ID, "quality_bug_comment", comment.ID, "deleted", bug.Title, comment,
		); err != nil {
			return err
		}
		result := tx.Where("id = ? AND workspace_id = ? AND quality_bug_id = ?",
			comment.ID, comment.WorkspaceID, comment.QualityBugID).Delete(&qualitygorm.QualityBugComment{})
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

func (s *Server) qualityBugCommentDTOs(c *gin.Context, rows []qualitygorm.QualityBugComment) []gin.H {
	workspace, _ := currentWorkspace(c)
	refs := workspaceContextReferences{}
	for _, row := range rows {
		refs = refs.withUserID(row.UserID)
	}
	ctx := s.loadWorkspaceContext(workspace.ID, refs)
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		result = append(result, qualityBugCommentDTOWithContext(row, ctx))
	}
	return result
}

func (s *Server) qualityBugCommentDTO(c *gin.Context, row qualitygorm.QualityBugComment) gin.H {
	workspace, _ := currentWorkspace(c)
	return qualityBugCommentDTOWithContext(row, s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withUserID(row.UserID),
	))
}

func qualityBugCommentDTOWithContext(row qualitygorm.QualityBugComment, ctx workspaceContext) gin.H {
	result := gin.H{
		"id": row.ID, "workspace_id": row.WorkspaceID, "quality_bug_id": row.QualityBugID,
		"user_id": row.UserID, "body": row.Body, "created_at": row.CreatedAt,
		"can_delete": true,
	}
	if user, ok := ctx.Users[row.UserID]; ok {
		result["username"] = user.Username
		result["display_name"] = user.DisplayName
	} else {
		result["display_name"] = retiredUserDisplayName(row.UserID)
	}
	return result
}
