package httpapi

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type requirementMovePlan struct {
	sourceUnit    projectgorm.PlanUnit
	targetUnit    projectgorm.PlanUnit
	targetProject projectgorm.ProjectSet
}

func (plan *requirementMovePlan) auditDetail(title string) string {
	source := plan.sourceUnit.Title
	if plan.sourceUnit.ID == 0 {
		source = "需求池"
	}
	return fmt.Sprintf(
		"移动需求「%s」：%s → %s",
		title,
		source,
		plan.targetUnit.Title,
	)
}

func (plan *requirementMovePlan) targetAuditDetail(title string) string {
	return fmt.Sprintf(
		"迁入需求「%s」至%s / %s",
		title,
		plan.targetProject.Name,
		plan.targetUnit.Title,
	)
}

func (s *Server) buildRequirementMovePlan(
	c *gin.Context,
	requirement requirementgorm.Requirement,
	body requirementBody,
) (*requirementMovePlan, error) {
	if body.PlanUnitID == nil {
		return nil, nil
	}
	targetUnitID := *body.PlanUnitID
	if targetUnitID == 0 {
		return nil, errors.New("target project required")
	}
	if targetUnitID == requirement.PlanUnitID {
		return nil, nil
	}
	workspace, _ := currentWorkspace(c)
	plan := &requirementMovePlan{}
	if requirement.PlanUnitID > 0 {
		if err := s.db.Where("id = ? AND workspace_id = ?", requirement.PlanUnitID, workspace.ID).
			First(&plan.sourceUnit).Error; err != nil {
			return plan, err
		}
	}
	if err := s.db.Where("id = ? AND workspace_id = ?", targetUnitID, workspace.ID).
		First(&plan.targetUnit).Error; err != nil {
		return plan, err
	}
	if err := s.db.Where("id = ? AND workspace_id = ?", plan.targetUnit.ProjectSetID, workspace.ID).
		First(&plan.targetProject).Error; err != nil {
		return plan, err
	}
	return plan, nil
}

func applyRequirementMoveUpdates(updates map[string]any, plan *requirementMovePlan) {
	if plan == nil {
		return
	}
	updates["workspace_id"] = plan.targetProject.WorkspaceID
	updates["team_id"] = plan.targetUnit.TeamID
	updates["project_new_id"] = plan.targetProject.ID
	updates["version_task_new_id"] = plan.targetUnit.ID
}

func moveRequirementTasks(tx *gorm.DB, requirement requirementgorm.Requirement) error {
	if err := pmTaskQuery(tx).
		Model(&taskgorm.WorkTask{}).
		Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).
		Updates(map[string]any{
			"team_id":        requirement.TeamID,
			"project_set_id": requirement.ProjectSetID,
			"project_id":     requirement.PlanUnitID,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&qualitygorm.QualityBug{}).
		Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).
		Updates(map[string]any{
			"team_id":        requirement.TeamID,
			"project_set_id": requirement.ProjectSetID,
			"project_id":     requirement.PlanUnitID,
		}).Error
}
