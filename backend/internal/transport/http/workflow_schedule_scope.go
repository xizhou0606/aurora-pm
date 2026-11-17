package httpapi

import (
	"errors"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowScheduleScope struct {
	Subject             workflowScheduleBoundary
	Constraints         workflowScheduleBoundary
	HasPlanUnit         bool
	StartConstraintType string
	StartConstraintID   uint
	EndConstraintType   string
	EndConstraintID     uint
}

func workflowScheduleScopeForPlanUnit(unit projectgorm.PlanUnit, project projectgorm.ProjectSet) workflowScheduleScope {
	scope := workflowScheduleScope{
		Subject:     planUnitScheduleBoundary(unit),
		HasPlanUnit: true,
	}
	if unit.PlannedStartAt != nil {
		scope.Constraints.PlannedStartAt = unit.PlannedStartAt
		scope.StartConstraintType, scope.StartConstraintID = "plan_unit", unit.ID
	} else if project.PlannedStartAt != nil {
		scope.Constraints.PlannedStartAt = project.PlannedStartAt
		scope.StartConstraintType, scope.StartConstraintID = "project_set", project.ID
	}
	if unit.PlannedEndAt != nil {
		scope.Constraints.PlannedEndAt = unit.PlannedEndAt
		scope.EndConstraintType, scope.EndConstraintID = "plan_unit", unit.ID
	} else if project.PlannedEndAt != nil {
		scope.Constraints.PlannedEndAt = project.PlannedEndAt
		scope.EndConstraintType, scope.EndConstraintID = "project_set", project.ID
	}
	return scope
}

func workflowScheduleScopeForProject(project projectgorm.ProjectSet) workflowScheduleScope {
	boundary := projectSetScheduleBoundary(project)
	return workflowScheduleScope{
		Constraints:         boundary,
		StartConstraintType: "project_set", StartConstraintID: project.ID,
		EndConstraintType: "project_set", EndConstraintID: project.ID,
	}
}

func workflowScheduleScopeForBoundary(boundary workflowScheduleBoundary, hasPlanUnit bool) workflowScheduleScope {
	return workflowScheduleScope{Subject: boundary, Constraints: boundary, HasPlanUnit: hasPlanUnit}
}

func workflowScheduleBoundaryPointer(boundary workflowScheduleBoundary) *workflowScheduleBoundary {
	if boundary.PlannedStartAt == nil && boundary.PlannedEndAt == nil {
		return nil
	}
	return &boundary
}

func loadWorkflowScheduleScope(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
) (workflowScheduleScope, error) {
	var unit projectgorm.PlanUnit
	var projectID uint
	switch instance.SubjectType {
	case "plan_unit":
		if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
			First(&unit).Error; err != nil {
			return workflowScheduleScope{}, err
		}
		projectID = unit.ProjectSetID
	case "requirement":
		var requirement requirementgorm.Requirement
		if err := tx.Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
			First(&requirement).Error; err != nil {
			return workflowScheduleScope{}, err
		}
		projectID = requirement.ProjectSetID
		if requirement.PlanUnitID > 0 {
			if err := tx.Where("id = ? AND workspace_id = ?", requirement.PlanUnitID, instance.WorkspaceID).
				First(&unit).Error; err != nil {
				return workflowScheduleScope{}, err
			}
			if unit.ProjectSetID != projectID {
				return workflowScheduleScope{}, errors.New("workflow subject project changed")
			}
		}
	default:
		return workflowScheduleScope{}, errors.New("unsupported workflow subject")
	}
	var project projectgorm.ProjectSet
	if err := tx.Where("id = ? AND workspace_id = ?", projectID, instance.WorkspaceID).
		First(&project).Error; err != nil {
		return workflowScheduleScope{}, err
	}
	if unit.ID == 0 {
		return workflowScheduleScopeForProject(project), nil
	}
	return workflowScheduleScopeForPlanUnit(unit, project), nil
}
