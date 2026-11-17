package httpapi

import (
	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func lockRequirementScheduleInstancesTx(
	tx *gorm.DB,
	workspaceID, requirementID, targetPlanUnitID uint,
) ([]workflowgorm.WorkflowInstance, error) {
	targetRows, err := workflowInstancesForPlanUnitTx(tx, workspaceID, targetPlanUnitID, false)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(targetRows)+1)
	for _, row := range targetRows {
		ids = append(ids, row.ID)
	}
	ownIDs, err := workflowInstanceIDsForSubjectTx(tx, workspaceID, "requirement", requirementID)
	if err != nil {
		return nil, err
	}
	ids = append(ids, ownIDs...)
	return lockWorkflowInstanceIDsTx(tx, workspaceID, ids)
}

func (s *Server) validateRequirementTargetScheduleTx(
	tx *gorm.DB,
	c *gin.Context,
	requirement requirementgorm.Requirement,
	unit *projectgorm.PlanUnit,
	project projectgorm.ProjectSet,
	instances []workflowgorm.WorkflowInstance,
	sourceScope workflowScheduleScope,
	confirm bool,
) error {
	boundary := planUnitScheduleBoundary(*unit)
	scope := workflowScheduleScopeForPlanUnit(*unit, project)
	var confirmation *workflowSubjectSchedule
	allConflicts := make([]workflowScheduleConflictItem, 0)
	for _, instance := range instances {
		schedule, err := loadWorkflowInstanceScheduleForMutation(tx, instance, scope)
		if err != nil {
			return err
		}
		if instance.SubjectType != "requirement" || instance.SubjectID != requirement.ID {
			continue
		}
		sourceSchedule, err := loadWorkflowInstanceScheduleForMutation(tx, instance, sourceScope)
		if err != nil {
			return err
		}
		for _, conflict := range workflowScheduleNewAssignmentConflicts(
			sourceSchedule.Conflicts, schedule.Conflicts,
		) {
			conflict.SubjectType, conflict.SubjectID = instance.SubjectType, instance.SubjectID
			allConflicts = append(allConflicts, conflict)
		}
		if schedule.CanConfirm {
			copy := schedule
			confirmation = &copy
		}
	}
	if len(allConflicts) > 0 {
		return workflowScheduleScopeConflictError(scope, nil, allConflicts)
	}
	if confirmation == nil && confirm {
		return workflowScheduleConfirmationNotAvailableError(scope)
	}
	if confirmation == nil {
		return nil
	}
	if !confirm {
		return workflowScheduleConfirmationError(*confirmation)
	}
	proposed, changed := fillWorkflowScheduleBoundary(boundary, confirmation.Suggested)
	if !changed {
		return nil
	}
	conflicts := validatePlanUnitScheduleBoundary(proposed, projectSetScheduleBoundary(project))
	for index := range conflicts {
		conflicts[index].SubjectType, conflicts[index].SubjectID = "plan_unit", unit.ID
	}
	if len(conflicts) > 0 {
		proposedUnit := *unit
		proposedUnit.PlannedStartAt, proposedUnit.PlannedEndAt = proposed.PlannedStartAt, proposed.PlannedEndAt
		return workflowScheduleScopeConflictError(
			workflowScheduleScopeForPlanUnit(proposedUnit, project), confirmation.Suggested, conflicts,
		)
	}
	proposedUnit := *unit
	proposedUnit.PlannedStartAt, proposedUnit.PlannedEndAt = proposed.PlannedStartAt, proposed.PlannedEndAt
	if err := validateWorkflowInstancesAgainstPlanUnit(tx, instances, *unit, proposedUnit, project); err != nil {
		return err
	}
	updates := map[string]any{}
	if boundary.PlannedStartAt == nil && proposed.PlannedStartAt != nil {
		updates["planned_start_at"] = proposed.PlannedStartAt
		unit.PlannedStartAt = proposed.PlannedStartAt
	}
	if boundary.PlannedEndAt == nil && proposed.PlannedEndAt != nil {
		updates["planned_end_at"] = proposed.PlannedEndAt
		unit.PlannedEndAt = proposed.PlannedEndAt
	}
	updated := tx.Model(&projectgorm.PlanUnit{}).
		Where("id = ? AND workspace_id = ?", unit.ID, unit.WorkspaceID).
		Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errWorkflowNodePlansChanged
	}
	return s.recordMutation(tx, c, unit.ProjectSetID, "plan_unit", unit.ID, "schedule_inferred", unit.Title, proposed)
}

func workflowScheduleConfirmationNotAvailableError(scope workflowScheduleScope) error {
	return &workflowScheduleRequestError{
		Code: apiErrorWorkflowScheduleConflict,
		Schedule: workflowSubjectSchedule{
			State: "conflict", Subject: scope.Subject,
			Constraints:            workflowScheduleBoundaryPointer(scope.Constraints),
			ActualDatesIndependent: true,
			Conflicts:              []workflowScheduleConflictItem{{Code: scheduleConflictConfirmationNotAvailable}},
		},
	}
}

func (s *Server) respondRequirementScheduleStale(
	c *gin.Context,
	workspaceID, requirementID uint,
	field string,
) {
	scope, err := loadWorkflowScheduleScope(s.db, workflowgorm.WorkflowInstance{
		WorkspaceID: workspaceID, SubjectType: "requirement", SubjectID: requirementID,
	})
	if err != nil {
		dbFail(c)
		return
	}
	_ = respondWorkflowScheduleError(c, workflowScheduleStaleError(scope, []workflowScheduleConflictItem{{
		Code: apiErrorScheduleSubjectStale, SubjectType: "requirement", SubjectID: requirementID, Field: field,
	}}))
}

func workflowScheduleNewAssignmentConflicts(
	current, proposed []workflowScheduleConflictItem,
) []workflowScheduleConflictItem {
	existing := make(map[string]workflowScheduleConflictItem)
	for _, conflict := range workflowScheduleAssignmentConflicts(current) {
		existing[workflowScheduleBoundaryConflictKey(conflict)] = conflict
	}
	result := make([]workflowScheduleConflictItem, 0)
	for _, conflict := range workflowScheduleAssignmentConflicts(proposed) {
		previous, found := existing[workflowScheduleBoundaryConflictKey(conflict)]
		if !found || workflowScheduleBoundaryConflictTightened(previous, conflict) {
			result = append(result, conflict)
		}
	}
	return result
}

func workflowScheduleBoundaryConflictKey(conflict workflowScheduleConflictItem) string {
	return conflict.Code + "\x00" + conflict.NodeID + "\x00" + conflict.Field
}

func workflowScheduleBoundaryConflictTightened(
	current, proposed workflowScheduleConflictItem,
) bool {
	if current.Boundary == nil || proposed.Boundary == nil {
		return true
	}
	switch proposed.Code {
	case scheduleConflictNodeBeforeSubject:
		return proposed.Boundary.After(*current.Boundary)
	case scheduleConflictNodeAfterSubject:
		return proposed.Boundary.Before(*current.Boundary)
	default:
		return true
	}
}
