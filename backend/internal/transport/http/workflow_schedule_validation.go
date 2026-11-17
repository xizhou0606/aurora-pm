package httpapi

import (
	"errors"
	"sort"
	"time"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func workflowScheduleDisplayTopology(
	tx *gorm.DB,
	definition workflowDefinition,
	instance workflowgorm.WorkflowInstance,
	runs []workflowgorm.WorkflowNodeRun,
) (map[string]bool, []string, error) {
	selected, conditionEdges, err := workflowSelectedTopologyForInstance(tx, definition, instance, runs)
	if err != nil {
		return nil, nil, err
	}
	nodeIDs, edgeIDs := workflowDisplayTopology(
		definition, runs, conditionEdges, currentWorkflowNodes(instance.CurrentNodeIDs),
	)
	display := make(map[string]bool, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if selected[nodeID] {
			display[nodeID] = true
		}
	}
	return display, edgeIDs, nil
}

func planUnitScheduleBoundary(unit projectgorm.PlanUnit) workflowScheduleBoundary {
	return workflowScheduleBoundary{PlannedStartAt: unit.PlannedStartAt, PlannedEndAt: unit.PlannedEndAt}
}

func projectSetScheduleBoundary(project projectgorm.ProjectSet) workflowScheduleBoundary {
	return workflowScheduleBoundary{PlannedStartAt: project.PlannedStartAt, PlannedEndAt: project.PlannedEndAt}
}

func fillWorkflowScheduleBoundary(
	current workflowScheduleBoundary,
	suggested *workflowScheduleBoundary,
) (workflowScheduleBoundary, bool) {
	if suggested == nil {
		return current, false
	}
	result := current
	changed := false
	if result.PlannedStartAt == nil && suggested.PlannedStartAt != nil {
		value := *suggested.PlannedStartAt
		result.PlannedStartAt = &value
		changed = true
	}
	if result.PlannedEndAt == nil && suggested.PlannedEndAt != nil {
		value := *suggested.PlannedEndAt
		result.PlannedEndAt = &value
		changed = true
	}
	return result, changed
}

func validatePlanUnitScheduleBoundary(
	unit workflowScheduleBoundary,
	project workflowScheduleBoundary,
) []workflowScheduleConflictItem {
	conflicts := make([]workflowScheduleConflictItem, 0)
	if unit.PlannedStartAt != nil && unit.PlannedEndAt != nil && unit.PlannedStartAt.After(*unit.PlannedEndAt) {
		conflicts = append(conflicts, workflowScheduleConflictItem{
			Code: scheduleConflictPlanUnitStartAfterEnd, Field: "planned_start_at",
			Boundary: unit.PlannedEndAt, Value: unit.PlannedStartAt,
		})
	}
	for _, value := range []struct {
		field string
		date  *time.Time
	}{{"planned_start_at", unit.PlannedStartAt}, {"planned_end_at", unit.PlannedEndAt}} {
		if value.date == nil {
			continue
		}
		if project.PlannedStartAt != nil && value.date.Before(*project.PlannedStartAt) {
			conflicts = append(conflicts, workflowScheduleConflictItem{
				Code: scheduleConflictPlanUnitBeforeProject, Field: value.field,
				Boundary: project.PlannedStartAt, Value: value.date,
			})
		}
		if project.PlannedEndAt != nil && value.date.After(*project.PlannedEndAt) {
			conflicts = append(conflicts, workflowScheduleConflictItem{
				Code: scheduleConflictPlanUnitAfterProject, Field: value.field,
				Boundary: project.PlannedEndAt, Value: value.date,
			})
		}
	}
	return conflicts
}

func loadWorkflowInstanceSchedule(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	scope workflowScheduleScope,
) (workflowSubjectSchedule, error) {
	var version workflowgorm.WorkflowTemplateVersion
	if err := tx.Select("definition").
		Where("id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID).
		First(&version).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return workflowSubjectSchedule{}, &workflowScheduleUnavailableError{}
	} else if err != nil {
		return workflowSubjectSchedule{}, err
	}
	definition, issues := compileWorkflowDefinition(version.Definition)
	if len(issues) > 0 {
		return workflowSubjectSchedule{}, &workflowScheduleUnavailableError{}
	}
	var runs []workflowgorm.WorkflowNodeRun
	if err := tx.Where("workspace_id = ? AND instance_id = ?", instance.WorkspaceID, instance.ID).
		Order("id ASC").Find(&runs).Error; err != nil {
		return workflowSubjectSchedule{}, err
	}
	plans, err := decodeWorkflowNodePlans(instance)
	if err != nil {
		return workflowSubjectSchedule{}, &workflowScheduleUnavailableError{}
	}
	selected, edges, err := workflowScheduleDisplayTopology(tx, definition, instance, runs)
	if err != nil {
		var selectionErr *workflowTopologySelectionError
		if errors.As(err, &selectionErr) {
			return workflowSubjectSchedule{}, &workflowScheduleUnavailableError{}
		}
		return workflowSubjectSchedule{}, err
	}
	return workflowScheduleForParts(definition, selected, edges, plans, scope), nil
}

func workflowInstancesForPlanUnitTx(
	tx *gorm.DB,
	workspaceID, planUnitID uint,
	lock bool,
) ([]workflowgorm.WorkflowInstance, error) {
	var requirementIDs []uint
	if err := tx.Model(&requirementgorm.Requirement{}).
		Where("workspace_id = ? AND version_task_new_id = ?", workspaceID, planUnitID).
		Order("id").Pluck("id", &requirementIDs).Error; err != nil {
		return nil, err
	}
	query := tx.Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "plan_unit", planUnitID)
	if len(requirementIDs) > 0 {
		query = tx.Where(
			"workspace_id = ? AND ((subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?))",
			workspaceID, "plan_unit", planUnitID, "requirement", requirementIDs,
		)
	}
	var rows []workflowgorm.WorkflowInstance
	if err := query.Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if !lock {
		return rows, nil
	}
	locked := make([]workflowgorm.WorkflowInstance, 0, len(rows))
	for _, row := range rows {
		var current workflowgorm.WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", row.ID, workspaceID).First(&current).Error; err != nil {
			return nil, err
		}
		locked = append(locked, current)
	}
	return locked, nil
}

func validateWorkflowInstancesAgainstPlanUnit(
	tx *gorm.DB,
	instances []workflowgorm.WorkflowInstance,
	currentUnit, proposedUnit projectgorm.PlanUnit,
	project projectgorm.ProjectSet,
) error {
	currentScope := workflowScheduleScopeForPlanUnit(currentUnit, project)
	proposedScope := workflowScheduleScopeForPlanUnit(proposedUnit, project)
	startTightened, endTightened := workflowScheduleTightenedSides(currentScope.Constraints, proposedScope.Constraints)
	if !startTightened && !endTightened {
		return nil
	}
	allConflicts := make([]workflowScheduleConflictItem, 0)
	for _, instance := range instances {
		schedule, err := loadWorkflowInstanceScheduleForMutation(tx, instance, proposedScope)
		if err != nil {
			return err
		}
		for _, conflict := range workflowScheduleBoundaryConflicts(schedule.Conflicts) {
			if conflict.Code == scheduleConflictNodeBeforeSubject && !startTightened ||
				conflict.Code == scheduleConflictNodeAfterSubject && !endTightened {
				continue
			}
			conflict.SubjectType = instance.SubjectType
			conflict.SubjectID = instance.SubjectID
			allConflicts = append(allConflicts, conflict)
		}
	}
	if len(allConflicts) > 0 {
		return workflowScheduleScopeConflictError(proposedScope, nil, allConflicts)
	}
	return nil
}

func validatePlanUnitScheduleMutation(
	current, proposed projectgorm.PlanUnit,
	project projectgorm.ProjectSet,
) []workflowScheduleConflictItem {
	currentBoundary, proposedBoundary := planUnitScheduleBoundary(current), planUnitScheduleBoundary(proposed)
	if !workflowScheduleBoundaryChanged(currentBoundary, proposedBoundary) {
		return nil
	}
	startChanged := !sameWorkflowNodePlanTime(current.PlannedStartAt, proposed.PlannedStartAt)
	endChanged := !sameWorkflowNodePlanTime(current.PlannedEndAt, proposed.PlannedEndAt)
	result := make([]workflowScheduleConflictItem, 0)
	if proposedBoundary.PlannedStartAt != nil || proposedBoundary.PlannedEndAt != nil {
		projectConflicts := validateProjectSetScheduleBoundary(projectSetScheduleBoundary(project))
		for index := range projectConflicts {
			projectConflicts[index].SubjectType, projectConflicts[index].SubjectID = "project_set", project.ID
		}
		result = append(result, projectConflicts...)
	}
	for _, conflict := range validatePlanUnitScheduleBoundary(proposedBoundary, projectSetScheduleBoundary(project)) {
		if conflict.Code == scheduleConflictPlanUnitStartAfterEnd ||
			conflict.Field == "planned_start_at" && startChanged ||
			conflict.Field == "planned_end_at" && endChanged {
			result = append(result, conflict)
		}
	}
	return result
}

func workflowScheduleBoundaryConflicts(conflicts []workflowScheduleConflictItem) []workflowScheduleConflictItem {
	result := make([]workflowScheduleConflictItem, 0, len(conflicts))
	for _, conflict := range conflicts {
		if conflict.Code == scheduleConflictNodeBeforeSubject || conflict.Code == scheduleConflictNodeAfterSubject {
			result = append(result, conflict)
		}
	}
	return result
}

func workflowInstanceIDsForSubjectTx(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) ([]uint, error) {
	var ids []uint
	err := tx.Model(&workflowgorm.WorkflowInstance{}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, subjectType, subjectID).
		Order("id").Pluck("id", &ids).Error
	return ids, err
}

func lockWorkflowInstanceIDsTx(tx *gorm.DB, workspaceID uint, ids []uint) ([]workflowgorm.WorkflowInstance, error) {
	unique := make(map[uint]bool, len(ids))
	ordered := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !unique[id] {
			unique[id] = true
			ordered = append(ordered, id)
		}
	}
	ids = ordered
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	rows := make([]workflowgorm.WorkflowInstance, 0, len(ids))
	for _, id := range ids {
		var row workflowgorm.WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", id, workspaceID).First(&row).Error; err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
