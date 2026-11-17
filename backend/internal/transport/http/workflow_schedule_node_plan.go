package httpapi

import (
	"errors"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type workflowScheduleSubjectTarget struct {
	PlanUnitID  uint
	ProjectID   uint
	HasProject  bool
	HasPlanUnit bool
}

func resolveWorkflowScheduleSubjectTarget(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
) (workflowScheduleSubjectTarget, error) {
	planUnitID, projectID := instance.SubjectID, uint(0)
	if instance.SubjectType == "requirement" {
		var requirement requirementgorm.Requirement
		if err := tx.Select("id", "project_new_id", "version_task_new_id").
			Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
			First(&requirement).Error; err != nil {
			return workflowScheduleSubjectTarget{}, err
		}
		planUnitID = requirement.PlanUnitID
		projectID = requirement.ProjectSetID
	}
	if planUnitID == 0 {
		return workflowScheduleSubjectTarget{ProjectID: projectID, HasProject: projectID > 0}, nil
	}
	var unit projectgorm.PlanUnit
	if err := tx.Select("id", "project_new_id").
		Where("id = ? AND workspace_id = ?", planUnitID, instance.WorkspaceID).
		First(&unit).Error; err != nil {
		return workflowScheduleSubjectTarget{}, err
	}
	if projectID > 0 && projectID != unit.ProjectSetID {
		return workflowScheduleSubjectTarget{}, errors.New("workflow subject project changed")
	}
	return workflowScheduleSubjectTarget{
		PlanUnitID: planUnitID, ProjectID: unit.ProjectSetID, HasProject: true, HasPlanUnit: true,
	}, nil
}

func lockWorkflowScheduleProjectTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	target workflowScheduleSubjectTarget,
) (projectgorm.ProjectSet, error) {
	if !target.HasProject {
		return projectgorm.ProjectSet{}, nil
	}
	project, err := lockProjectSetTx(tx, instance.WorkspaceID, target.ProjectID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return projectgorm.ProjectSet{}, errWorkflowNodePlansChanged
	}
	return project, err
}

func lockWorkflowSchedulePlanUnitTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	target workflowScheduleSubjectTarget,
) (projectgorm.PlanUnit, bool, error) {
	if instance.SubjectType == "requirement" {
		var requirement requirementgorm.Requirement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", instance.SubjectID, instance.WorkspaceID).
			First(&requirement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return projectgorm.PlanUnit{}, false, errWorkflowNodePlansChanged
			}
			return projectgorm.PlanUnit{}, false, err
		}
		if requirement.PlanUnitID != target.PlanUnitID {
			return projectgorm.PlanUnit{}, false, errWorkflowNodePlansChanged
		}
		if requirement.ProjectSetID != target.ProjectID {
			return projectgorm.PlanUnit{}, false, errWorkflowNodePlansChanged
		}
	}
	if !target.HasPlanUnit {
		return projectgorm.PlanUnit{}, false, nil
	}
	if instance.SubjectType == "plan_unit" && instance.SubjectID != target.PlanUnitID {
		return projectgorm.PlanUnit{}, false, errWorkflowNodePlansChanged
	}
	units, err := lockProjectPlanUnitsTx(tx, instance.WorkspaceID, map[uint]uint{
		target.PlanUnitID: target.ProjectID,
	})
	if errors.Is(err, errProjectChildScopeChanged) {
		return projectgorm.PlanUnit{}, false, errWorkflowNodePlansChanged
	}
	if err != nil {
		return projectgorm.PlanUnit{}, false, err
	}
	return units[target.PlanUnitID], true, nil
}

func (s *Server) applyWorkflowNodePlanScheduleTx(
	tx *gorm.DB,
	c *gin.Context,
	instance workflowgorm.WorkflowInstance,
	definition workflowDefinition,
	plans map[string]workflowNodePlan,
	updatedNodeIDs map[string]bool,
	plannedChanged bool,
	confirm bool,
	target workflowScheduleSubjectTarget,
	project projectgorm.ProjectSet,
) error {
	unit, hasPlanUnit, err := lockWorkflowSchedulePlanUnitTx(tx, instance, target)
	if err != nil {
		return err
	}
	var runs []workflowgorm.WorkflowNodeRun
	if err := tx.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).
		Order("id ASC").Find(&runs).Error; err != nil {
		return err
	}
	selected, edges, err := workflowScheduleDisplayTopology(tx, definition, instance, runs)
	if err != nil {
		return err
	}
	for nodeID := range updatedNodeIDs {
		if !selected[nodeID] {
			return errWorkflowNodePlansChanged
		}
	}
	scope := workflowScheduleScopeForProject(project)
	if hasPlanUnit {
		scope = workflowScheduleScopeForPlanUnit(unit, project)
	}
	schedule := workflowScheduleForParts(definition, selected, edges, plans, scope)
	if !plannedChanged && !confirm {
		return nil
	}
	fatal := make([]workflowScheduleConflictItem, 0, len(schedule.Conflicts))
	for _, conflict := range schedule.Conflicts {
		if conflict.Code != scheduleConflictSubjectPlanUnitRequired {
			fatal = append(fatal, conflict)
		}
	}
	if len(fatal) > 0 {
		schedule.State, schedule.Conflicts, schedule.CanConfirm = "conflict", fatal, false
		return &workflowScheduleRequestError{Code: apiErrorWorkflowScheduleConflict, Schedule: schedule}
	}
	if confirm && !hasPlanUnit {
		schedule.State, schedule.CanConfirm = "unscheduled", false
		return &workflowScheduleRequestError{Code: apiErrorWorkflowScheduleConflict, Schedule: schedule}
	}
	if !schedule.CanConfirm {
		if confirm {
			schedule.State = "conflict"
			schedule.Conflicts = append(schedule.Conflicts, workflowScheduleConflictItem{Code: scheduleConflictConfirmationNotAvailable})
			return &workflowScheduleRequestError{Code: apiErrorWorkflowScheduleConflict, Schedule: schedule}
		}
		return nil
	}
	if !confirm {
		if plannedChanged {
			return workflowScheduleConfirmationError(schedule)
		}
		return nil
	}

	boundary := scope.Subject
	proposed, changed := fillWorkflowScheduleBoundary(boundary, schedule.Suggested)
	if !changed {
		return nil
	}
	projectConflicts := validatePlanUnitScheduleBoundary(proposed, projectSetScheduleBoundary(project))
	for index := range projectConflicts {
		projectConflicts[index].SubjectType = "plan_unit"
		projectConflicts[index].SubjectID = unit.ID
	}
	if len(projectConflicts) > 0 {
		proposedUnit := unit
		proposedUnit.PlannedStartAt, proposedUnit.PlannedEndAt = proposed.PlannedStartAt, proposed.PlannedEndAt
		return workflowScheduleScopeConflictError(
			workflowScheduleScopeForPlanUnit(proposedUnit, project), schedule.Suggested, projectConflicts,
		)
	}
	instances, err := workflowInstancesForPlanUnitTx(tx, instance.WorkspaceID, unit.ID, false)
	if err != nil {
		return err
	}
	proposedUnit := unit
	proposedUnit.PlannedStartAt, proposedUnit.PlannedEndAt = proposed.PlannedStartAt, proposed.PlannedEndAt
	if err := validateWorkflowInstancesAgainstPlanUnit(tx, instances, unit, proposedUnit, project); err != nil {
		return err
	}
	updates := map[string]any{}
	if boundary.PlannedStartAt == nil && proposed.PlannedStartAt != nil {
		updates["planned_start_at"] = proposed.PlannedStartAt
	}
	if boundary.PlannedEndAt == nil && proposed.PlannedEndAt != nil {
		updates["planned_end_at"] = proposed.PlannedEndAt
	}
	updated := tx.Model(&projectgorm.PlanUnit{}).
		Where("id = ? AND workspace_id = ?", unit.ID, instance.WorkspaceID).
		Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errWorkflowNodePlansChanged
	}
	return s.recordMutation(tx, c, unit.ProjectSetID, "plan_unit", unit.ID, "schedule_inferred", unit.Title, proposed)
}
