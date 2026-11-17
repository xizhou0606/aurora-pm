package httpapi

import (
	"errors"

	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

type workflowScheduleUnavailableError struct{}

func (*workflowScheduleUnavailableError) Error() string {
	return "workflow schedule data is unavailable"
}

func loadWorkflowInstanceScheduleForMutation(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	scope workflowScheduleScope,
) (workflowSubjectSchedule, error) {
	schedule, err := loadWorkflowInstanceSchedule(tx, instance, scope)
	var unavailable *workflowScheduleUnavailableError
	if err == nil || !errors.As(err, &unavailable) {
		return schedule, err
	}
	return workflowSubjectSchedule{}, &workflowScheduleRequestError{
		Code: apiErrorWorkflowScheduleConflict,
		Schedule: workflowSubjectSchedule{
			State: "conflict", Subject: scope.Subject,
			Constraints:            workflowScheduleBoundaryPointer(scope.Constraints),
			ActualDatesIndependent: true,
			Conflicts: []workflowScheduleConflictItem{{
				Code: scheduleConflictSubjectUnavailable, SubjectType: instance.SubjectType, SubjectID: instance.SubjectID,
			}},
		},
	}
}

func workflowScheduleConflictError(
	subject workflowScheduleBoundary,
	suggested *workflowScheduleBoundary,
	conflicts []workflowScheduleConflictItem,
) error {
	return workflowScheduleScopeConflictError(
		workflowScheduleScopeForBoundary(subject, true), suggested, conflicts,
	)
}

func workflowScheduleScopeConflictError(
	scope workflowScheduleScope,
	suggested *workflowScheduleBoundary,
	conflicts []workflowScheduleConflictItem,
) error {
	return &workflowScheduleRequestError{
		Code: apiErrorWorkflowScheduleConflict,
		Schedule: workflowSubjectSchedule{
			State: "conflict", Subject: scope.Subject, Suggested: suggested,
			Constraints:            workflowScheduleBoundaryPointer(scope.Constraints),
			ActualDatesIndependent: true, Conflicts: conflicts,
		},
	}
}

func workflowScheduleConfirmationError(schedule workflowSubjectSchedule) error {
	return &workflowScheduleRequestError{Code: apiErrorWorkflowScheduleConfirmation, Schedule: schedule}
}

func workflowScheduleAppendConstraintOrderConflict(
	schedule *workflowSubjectSchedule,
	scope workflowScheduleScope,
) {
	constraints := scope.Constraints
	if constraints.PlannedStartAt == nil || constraints.PlannedEndAt == nil ||
		!constraints.PlannedStartAt.After(*constraints.PlannedEndAt) {
		return
	}
	actual := scope.Subject
	if actual.PlannedStartAt != nil && actual.PlannedEndAt != nil &&
		actual.PlannedStartAt.After(*actual.PlannedEndAt) {
		return
	}
	conflict := workflowScheduleConflictItem{
		Code: scheduleConflictSubjectStartAfterEnd, Field: "planned_start_at",
		Boundary: constraints.PlannedEndAt, Value: constraints.PlannedStartAt,
		SubjectType: scope.StartConstraintType, SubjectID: scope.StartConstraintID,
	}
	if scope.StartConstraintType == "project_set" && scope.EndConstraintType == "project_set" &&
		scope.StartConstraintID == scope.EndConstraintID {
		conflict.Code = scheduleConflictProjectStartAfterEnd
	}
	schedule.Conflicts = append(schedule.Conflicts, conflict)
}

func workflowScheduleAssignmentConflicts(
	conflicts []workflowScheduleConflictItem,
) []workflowScheduleConflictItem {
	result := make([]workflowScheduleConflictItem, 0, len(conflicts))
	for _, conflict := range conflicts {
		switch conflict.Code {
		case scheduleConflictNodeBeforeSubject, scheduleConflictNodeAfterSubject,
			scheduleConflictSubjectStartAfterEnd, scheduleConflictProjectStartAfterEnd:
			result = append(result, conflict)
		}
	}
	return result
}
