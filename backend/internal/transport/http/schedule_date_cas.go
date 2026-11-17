package httpapi

import (
	"fmt"
	"strings"
	"time"
)

type scheduleDateCASInput struct {
	Field       string
	ActualSet   bool
	ExpectedRaw optionalValue[string]
}

type scheduleDateExpectation struct {
	Field    string
	Expected *time.Time
}

func prepareScheduleDateExpectations(inputs ...scheduleDateCASInput) ([]scheduleDateExpectation, error) {
	expectations := make([]scheduleDateExpectation, 0, len(inputs))
	for _, input := range inputs {
		expectedField := "expected_" + input.Field
		switch {
		case input.ActualSet && !input.ExpectedRaw.Set:
			return nil, fmt.Errorf("%s required when %s is provided", expectedField, input.Field)
		case !input.ActualSet && input.ExpectedRaw.Set:
			return nil, fmt.Errorf("%s requires %s", expectedField, input.Field)
		case !input.ActualSet:
			continue
		}
		if input.ExpectedRaw.Value != nil && strings.TrimSpace(*input.ExpectedRaw.Value) == "" {
			return nil, fmt.Errorf("%s must be a date or null", expectedField)
		}
		expected, err := parseOptionalTime(input.ExpectedRaw.Value)
		if err != nil {
			return nil, fmt.Errorf("%s is invalid", expectedField)
		}
		expectations = append(expectations, scheduleDateExpectation{Field: input.Field, Expected: expected})
	}
	return expectations, nil
}

func scheduleDateStaleConflicts(
	expectations []scheduleDateExpectation,
	current map[string]*time.Time,
	subjectType string,
	subjectID uint,
	nodeID string,
) []workflowScheduleConflictItem {
	conflicts := make([]workflowScheduleConflictItem, 0)
	for _, expectation := range expectations {
		actual := current[expectation.Field]
		if sameWorkflowNodePlanTime(actual, expectation.Expected) {
			continue
		}
		conflicts = append(conflicts, workflowScheduleConflictItem{
			Code: apiErrorScheduleSubjectStale, SubjectType: subjectType, SubjectID: subjectID,
			NodeID: nodeID, Field: expectation.Field, Boundary: actual, Value: expectation.Expected,
		})
	}
	return conflicts
}

func workflowScheduleStaleError(
	scope workflowScheduleScope,
	conflicts []workflowScheduleConflictItem,
) error {
	return &workflowScheduleRequestError{
		Code: apiErrorScheduleSubjectStale,
		Schedule: workflowSubjectSchedule{
			State: "conflict", Subject: scope.Subject,
			Constraints:            workflowScheduleBoundaryPointer(scope.Constraints),
			ActualDatesIndependent: true,
			Conflicts:              conflicts,
		},
	}
}
