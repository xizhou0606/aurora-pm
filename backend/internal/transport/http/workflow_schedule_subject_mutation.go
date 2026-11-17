package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func planUnitScheduleTouched(body planUnitBody) bool {
	return body.PlannedStartAt.Set || body.PlannedEndAt.Set
}

func projectSetScheduleTouched(body projectBody) bool {
	return body.PlannedStartAt.Set || body.PlannedEndAt.Set
}

func validateProjectSetScheduleBoundary(boundary workflowScheduleBoundary) []workflowScheduleConflictItem {
	if boundary.PlannedStartAt == nil || boundary.PlannedEndAt == nil ||
		!boundary.PlannedStartAt.After(*boundary.PlannedEndAt) {
		return nil
	}
	return []workflowScheduleConflictItem{{
		Code:  scheduleConflictProjectStartAfterEnd,
		Field: "planned_start_at", Boundary: boundary.PlannedEndAt, Value: boundary.PlannedStartAt,
	}}
}

func validateProjectSetPlanUnitsTx(tx *gorm.DB, current, proposed projectgorm.ProjectSet) error {
	currentBoundary := projectSetScheduleBoundary(current)
	proposedBoundary := projectSetScheduleBoundary(proposed)
	startTightened, endTightened := workflowScheduleTightenedSides(currentBoundary, proposedBoundary)
	conflicts := validateProjectSetScheduleBoundary(proposedBoundary)
	if !workflowScheduleBoundaryChanged(currentBoundary, proposedBoundary) {
		conflicts = nil
	}
	for index := range conflicts {
		conflicts[index].SubjectType, conflicts[index].SubjectID = "project_set", proposed.ID
	}
	units, requirements, instances, err := lockProjectScheduleSubjectsTx(tx, proposed)
	if err != nil {
		return err
	}
	for _, unit := range units {
		items := validatePlanUnitScheduleBoundary(planUnitScheduleBoundary(unit), proposedBoundary)
		for _, item := range items {
			if item.Code == scheduleConflictPlanUnitStartAfterEnd ||
				item.Code == scheduleConflictPlanUnitBeforeProject && !startTightened ||
				item.Code == scheduleConflictPlanUnitAfterProject && !endTightened {
				continue
			}
			item.SubjectType, item.SubjectID = "plan_unit", unit.ID
			conflicts = append(conflicts, item)
		}
	}
	unitByID := make(map[uint]projectgorm.PlanUnit, len(units))
	for _, unit := range units {
		unitByID[unit.ID] = unit
	}
	for _, instance := range instances {
		scope := workflowScheduleScopeForProject(proposed)
		if instance.SubjectType == "plan_unit" {
			unit, exists := unitByID[instance.SubjectID]
			if !exists {
				return errProjectChildScopeChanged
			}
			scope = workflowScheduleScopeForPlanUnit(unit, proposed)
		} else if requirement, exists := requirements[instance.SubjectID]; exists && requirement.PlanUnitID > 0 {
			unit, exists := unitByID[requirement.PlanUnitID]
			if !exists {
				return errProjectChildScopeChanged
			}
			scope = workflowScheduleScopeForPlanUnit(unit, proposed)
		}
		schedule, err := loadWorkflowInstanceScheduleForMutation(tx, instance, scope)
		if err != nil {
			return err
		}
		for _, item := range workflowScheduleBoundaryConflicts(schedule.Conflicts) {
			if item.SubjectType != "project_set" || item.SubjectID != proposed.ID ||
				item.Code == scheduleConflictNodeBeforeSubject && !startTightened ||
				item.Code == scheduleConflictNodeAfterSubject && !endTightened {
				continue
			}
			item.SubjectType, item.SubjectID = instance.SubjectType, instance.SubjectID
			conflicts = append(conflicts, item)
		}
	}
	if len(conflicts) > 0 {
		return workflowScheduleConflictError(proposedBoundary, nil, conflicts)
	}
	return nil
}

func workflowScheduleBoundaryChanged(left, right workflowScheduleBoundary) bool {
	return !sameWorkflowNodePlanTime(left.PlannedStartAt, right.PlannedStartAt) ||
		!sameWorkflowNodePlanTime(left.PlannedEndAt, right.PlannedEndAt)
}

func workflowScheduleTightenedSides(
	current, proposed workflowScheduleBoundary,
) (bool, bool) {
	startTightened := proposed.PlannedStartAt != nil &&
		(current.PlannedStartAt == nil || proposed.PlannedStartAt.After(*current.PlannedStartAt))
	endTightened := proposed.PlannedEndAt != nil &&
		(current.PlannedEndAt == nil || proposed.PlannedEndAt.Before(*current.PlannedEndAt))
	return startTightened, endTightened
}

func lockProjectScheduleSubjectsTx(
	tx *gorm.DB,
	project projectgorm.ProjectSet,
) ([]projectgorm.PlanUnit, map[uint]requirementgorm.Requirement, []workflowgorm.WorkflowInstance, error) {
	var unitSnapshots []projectgorm.PlanUnit
	if err := tx.Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
		Order("id ASC").Find(&unitSnapshots).Error; err != nil {
		return nil, nil, nil, err
	}
	var requirementRows []requirementgorm.Requirement
	if err := tx.Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
		Order("id ASC").Find(&requirementRows).Error; err != nil {
		return nil, nil, nil, err
	}
	unitIDs, requirementIDs := make([]uint, 0, len(unitSnapshots)), make([]uint, 0, len(requirementRows))
	expectedProjects := make(map[uint]uint, len(unitSnapshots))
	for _, unit := range unitSnapshots {
		unitIDs = append(unitIDs, unit.ID)
		expectedProjects[unit.ID] = project.ID
	}
	requirements := make(map[uint]requirementgorm.Requirement, len(requirementRows))
	for _, requirement := range requirementRows {
		requirementIDs = append(requirementIDs, requirement.ID)
		requirements[requirement.ID] = requirement
	}
	instanceIDs, err := workflowInstanceIDsForSubjectSetsTx(tx, project.WorkspaceID, unitIDs, requirementIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	instances, err := lockWorkflowInstanceIDsTx(tx, project.WorkspaceID, instanceIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	lockedUnits, err := lockProjectPlanUnitsTx(tx, project.WorkspaceID, expectedProjects)
	if err != nil {
		return nil, nil, nil, err
	}
	units := make([]projectgorm.PlanUnit, 0, len(unitSnapshots))
	for _, snapshot := range unitSnapshots {
		units = append(units, lockedUnits[snapshot.ID])
	}
	return units, requirements, instances, nil
}

func workflowInstanceIDsForSubjectSetsTx(
	tx *gorm.DB,
	workspaceID uint,
	planUnitIDs, requirementIDs []uint,
) ([]uint, error) {
	ids := make([]uint, 0)
	for _, subject := range []struct {
		typeName string
		ids      []uint
	}{{"plan_unit", planUnitIDs}, {"requirement", requirementIDs}} {
		if len(subject.ids) == 0 {
			continue
		}
		var found []uint
		if err := tx.Model(&workflowgorm.WorkflowInstance{}).
			Where("workspace_id = ? AND subject_type = ? AND subject_id IN ?", workspaceID, subject.typeName, subject.ids).
			Order("id ASC").Pluck("id", &found).Error; err != nil {
			return nil, err
		}
		ids = append(ids, found...)
	}
	return ids, nil
}
