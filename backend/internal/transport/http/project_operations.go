package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskmodule "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) projectFromParam(c *gin.Context) (projectgorm.ProjectSet, bool) {
	id, okID := paramID(c, "id")
	if !okID {
		return projectgorm.ProjectSet{}, false
	}
	workspace, _ := currentWorkspace(c)
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return projectgorm.ProjectSet{}, false
	}
	return project, true
}

func (s *Server) deleteProjectSet(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		var taskIDs, planUnitIDs, permissionIDs []uint
		if err := deleteProjectWorkflowDependents(tx, project.WorkspaceID, project.ID); err != nil {
			return err
		}
		if err := tx.Model(&taskgorm.WorkTask{}).
			Where("workspace_id = ? AND project_set_id = ?", project.WorkspaceID, project.ID).
			Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&projectgorm.PlanUnit{}).
			Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
			Pluck("id", &planUnitIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&projectgorm.ProjectPermission{}).
			Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
			Pluck("id", &permissionIDs).Error; err != nil {
			return err
		}
		if len(taskIDs) > 0 {
			if err := deleteWorkTaskDependents(tx, project.WorkspaceID, taskIDs); err != nil {
				return err
			}
		}
		if err := deleteQualityBugsForProjectSet(tx, project.WorkspaceID, project.ID); err != nil {
			return err
		}
		if len(planUnitIDs) > 0 {
			if err := tx.Where("workspace_id = ? AND plan_unit_id IN ?",
				project.WorkspaceID, planUnitIDs).Delete(&projectgorm.DeliveryNode{}).Error; err != nil {
				return err
			}
		}
		if len(permissionIDs) > 0 {
			if err := tx.Where("workspace_id = ? AND project_permission_id IN ?",
				project.WorkspaceID, permissionIDs).Delete(&projectgorm.ProjectMemberRole{}).Error; err != nil {
				return err
			}
		}
		var requirementIDs []uint
		if err := tx.Model(&requirementgorm.Requirement{}).Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
			Pluck("id", &requirementIDs).Error; err != nil {
			return err
		}
		for _, requirementID := range requirementIDs {
			if err := requirementcover.MarkDeletedTx(tx, project.WorkspaceID, requirementID); err != nil {
				return err
			}
		}
		for _, row := range []struct {
			model any
			where string
		}{
			{&taskgorm.WorkTask{}, "project_set_id = ?"},
			{&requirementgorm.Requirement{}, "project_new_id = ?"},
			{&projectgorm.DeliveryNodeLabel{}, "project_set_id = ?"},
			{&projectgorm.PlanUnit{}, "project_new_id = ?"},
			{&projectgorm.ProjectPermission{}, "project_new_id = ?"},
			{&projectgorm.ProjectNews{}, "project_set_id = ?"},
		} {
			if err := tx.Where("workspace_id = ?", project.WorkspaceID).
				Delete(row.model, row.where, project.ID).Error; err != nil {
				return err
			}
		}
		if err := s.recordMutation(tx, c, project.ID, "project_set", project.ID, "deleted", project.Name, project); err != nil {
			return err
		}
		return tx.Delete(&project).Error
	})
	if errors.Is(err, taskmodule.ErrCascadeDeletionConflict) {
		fail(c, http.StatusConflict, "remove task attachments before deleting the project")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (s *Server) transferProjectCreator(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	// Creator transfer is an administrator-only operation.
	actor, _ := currentActor(c)
	actorID := factUserID(actor.User)
	isAdmin, err := s.projectAccess.IsAccessAdministrator(c.Request.Context(), project.WorkspaceID, actorID)
	if err != nil || !isAdmin {
		fail(c, http.StatusForbidden, "only access administrators may transfer the project set creator")
		return
	}
	var body struct {
		NewOwnerUserID           uint   `json:"new_owner_user_id"`
		KeepPreviousOwnerAsAdmin bool   `json:"keep_previous_owner_as_sub_admin"`
		TransferReason           string `json:"transfer_reason"`
	}
	if c.ShouldBindJSON(&body) != nil || body.NewOwnerUserID == 0 {
		bad(c, "new_owner_user_id must be positive")
		return
	}
	if err := s.validateActivePMUserReferences(c, []uint{body.NewOwnerUserID}); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	previous := project.CreatedByUserID
	err = s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if err := tx.Model(&project).Update("created_by_user_id", body.NewOwnerUserID).Error; err != nil {
			return err
		}
		payload := gin.H{"previous_creator_user_id": previous, "new_creator_user_id": body.NewOwnerUserID,
			"reason": strings.TrimSpace(body.TransferReason)}
		return s.recordMutation(tx, c, project.ID, "project_set", project.ID, "creator_transferred", project.Name, payload)
	})
	if err != nil {
		dbFail(c)
		return
	}
	project.CreatedByUserID = &body.NewOwnerUserID
	ok(c, s.projectDTO(c, project))
}

func (s *Server) listAuditLogs(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	limit, offset := pagination(c, 30)
	query := s.db.Where("workspace_id = ? AND project_id = ?", project.WorkspaceID, project.ID)
	var total int64
	query.Model(&auditgorm.AuditLog{}).Count(&total)
	var rows []auditgorm.AuditLog
	if query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
}

func (s *Server) listProjectNews(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	query := s.db.Where(
		"workspace_id = ? AND project_set_id = ?",
		project.WorkspaceID,
		project.ID,
	).Order("created_at DESC, id DESC")
	if _, bounded := c.GetQuery("limit"); bounded {
		limit, _ := pagination(c, 30)
		query = query.Limit(limit)
	}
	var rows []projectgorm.ProjectNews
	if query.Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	ok(c, rows)
}

func (s *Server) createProjectNews(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Title) == "" {
		bad(c, "title required")
		return
	}
	a, _ := currentActor(c)
	row := projectgorm.ProjectNews{
		WorkspaceID: project.WorkspaceID, ProjectSetID: project.ID, UserID: factUserID(a.User),
		Title: strings.TrimSpace(body.Title), Content: strings.TrimSpace(body.Content),
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "project_news", row.ID, "created", row.Title, row)
	})
	if err != nil {
		dbFail(c)
		return
	}
	created(c, row)
}
