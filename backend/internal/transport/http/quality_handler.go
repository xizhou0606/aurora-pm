package httpapi

import (
	"github.com/gin-gonic/gin"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"gorm.io/gorm"
)

func (s *Server) createQualityBug(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body qualityBugBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	authActor, _ := currentActor(c)
	prepared, err := s.prepareQualityBugCreate(c, workspace, authActor, body)
	if err != nil {
		respondQualityBugCreateError(c, err)
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.createPreparedQualityBug(tx, c, authActor, &prepared)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	created(c, qualityBugDTO(prepared.Bug, prepared.NotificationContext))
}

func stringUpdate(updates map[string]any, key string) string {
	value, _ := updates[key].(string)
	return value
}

func (s *Server) getQualityBug(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	bug, _, found := s.qualityBugByID(c, id)
	if !found {
		return
	}
	workspace, _ := currentWorkspace(c)
	ok(c, qualityBugDTO(bug, s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withQualityBug(bug),
	)))
}

func (s *Server) deleteQualityBug(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	bug, project, found := s.qualityBugByID(c, id)
	if !found {
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND quality_bug_id = ?", bug.WorkspaceID, bug.ID).
			Delete(&qualitygorm.QualityBugComment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND quality_bug_id = ?", bug.WorkspaceID, bug.ID).
			Delete(&qualitygorm.QualityBugActivity{}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(tx, c, project.ID, "quality_bug", bug.ID, "deleted", bug.Title, bug); err != nil {
			return err
		}
		result := tx.Where("id = ? AND workspace_id = ?", bug.ID, bug.WorkspaceID).
			Delete(&qualitygorm.QualityBug{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) listQualityBugActivities(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	if _, _, found := s.qualityBugByID(c, id); !found {
		return
	}
	workspace, _ := currentWorkspace(c)
	limit, offset := pagination(c, 50)
	var total int64
	base := s.db.Model(&qualitygorm.QualityBugActivity{}).
		Where("workspace_id = ? AND quality_bug_id = ?", workspace.ID, id)
	if base.Count(&total).Error != nil {
		dbFail(c)
		return
	}
	var rows []qualitygorm.QualityBugActivity
	if base.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	refs := workspaceContextReferences{}
	for _, row := range rows {
		refs = refs.withUserID(row.UserID)
	}
	ctx := s.loadWorkspaceContext(workspace.ID, refs)
	result := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		userID := row.UserID
		result = append(result, gin.H{
			"id": row.ID, "quality_bug_id": row.QualityBugID, "user_id": row.UserID,
			"user_name": userDisplayName(ctx, &userID), "action": row.Action,
			"field_key": row.FieldKey, "old_value": row.OldValue,
			"new_value": row.NewValue, "summary": row.Summary, "created_at": row.CreatedAt,
		})
	}
	ok(c, gin.H{"items": result, "total": total})
}
