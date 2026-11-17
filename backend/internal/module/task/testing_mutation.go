package task

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrTestingMutationInvalid  = errors.New("task testing mutation is invalid")
	ErrTestingMutationNotFound = errors.New("task testing mutation target not found")
	ErrTestingMutationConflict = errors.New("task testing mutation conflicts with current state")
)

type TestingMutationOperation string

const (
	TestingDefinitionUpdate   TestingMutationOperation = "testing.definition.update"
	TestingExecutionCreate    TestingMutationOperation = "testing.execution.create"
	TestingExecutionRunUpdate TestingMutationOperation = "testing.execution_run.update"
)

type TestingDefinitionValues struct {
	Version             uint
	Purpose             string
	Preconditions       string
	EnvironmentGuidance string
	Cases               []TestCase
}

type TestingExecutionValues struct {
	DefinitionVersion uint
	Environment       string
	BuildVersion      string
	ExecutorUserID    uint
	Summary           string
	CaseKeys          []string
}

type TestingRunValues struct {
	RunID         uint
	Version       uint
	Status        string
	ActualResult  string
	FailureReason string
	ExecutedAt    *time.Time
	Fields        map[string]bool
}

type MutationTaskScope struct {
	TeamID        uint
	ProjectSetID  uint
	PlanUnitID    uint
	RequirementID uint
}

type TestingMutation struct {
	Operation   TestingMutationOperation
	WorkspaceID uint
	PrincipalID uint
	ApplyKey    string
	ChangeSetID string
	InputDigest string
	TaskID      uint
	TaskScope   MutationTaskScope
	Definition  TestingDefinitionValues
	Execution   TestingExecutionValues
	Run         TestingRunValues
}

type TestingMutationResult struct {
	Definition *TestDefinitionSnapshot    `json:"definition,omitempty"`
	Execution  *TestExecutionSnapshot     `json:"execution,omitempty"`
	Runs       []TestExecutionRunSnapshot `json:"runs,omitempty"`
	Run        *TestExecutionRunSnapshot  `json:"run,omitempty"`
}

type TestingMutationStore interface {
	ApplyTestingMutation(context.Context, TestingMutation) (TestingMutationResult, error)
}

type TestingMutationService struct{ store TestingMutationStore }

func NewTestingMutationService(store TestingMutationStore) (*TestingMutationService, error) {
	if store == nil {
		return nil, ErrTestingMutationInvalid
	}
	return &TestingMutationService{store: store}, nil
}

func (service *TestingMutationService) ApplyTestingMutation(ctx context.Context, command TestingMutation) (TestingMutationResult, error) {
	if service == nil || service.store == nil || command.WorkspaceID == 0 || command.PrincipalID == 0 || command.TaskID == 0 ||
		command.TaskScope.TeamID == 0 || command.TaskScope.ProjectSetID == 0 || command.TaskScope.PlanUnitID == 0 || command.TaskScope.RequirementID == 0 ||
		strings.TrimSpace(command.ApplyKey) == "" || strings.TrimSpace(command.ChangeSetID) == "" || len(command.InputDigest) != 64 {
		return TestingMutationResult{}, ErrTestingMutationInvalid
	}
	var err error
	switch command.Operation {
	case TestingDefinitionUpdate:
		command.Definition, err = normalizeTestingDefinition(command.Definition)
	case TestingExecutionCreate:
		command.Execution, err = normalizeTestingExecution(command.Execution, command.PrincipalID)
	case TestingExecutionRunUpdate:
		command.Run, err = normalizeTestingRun(command.Run)
	default:
		err = ErrTestingMutationInvalid
	}
	if err != nil {
		return TestingMutationResult{}, err
	}
	return service.store.ApplyTestingMutation(ctx, command)
}

func normalizeTestingDefinition(values TestingDefinitionValues) (TestingDefinitionValues, error) {
	values.Purpose = strings.TrimSpace(values.Purpose)
	values.Preconditions = strings.TrimSpace(values.Preconditions)
	values.EnvironmentGuidance = strings.TrimSpace(values.EnvironmentGuidance)
	if utf8.RuneCountInString(values.Purpose) > 20_000 || utf8.RuneCountInString(values.Preconditions) > 20_000 ||
		utf8.RuneCountInString(values.EnvironmentGuidance) > 20_000 || len(values.Cases) == 0 || len(values.Cases) > 200 {
		return values, ErrTestingMutationInvalid
	}
	seen := make(map[string]struct{}, len(values.Cases))
	for index := range values.Cases {
		item := &values.Cases[index]
		item.Key = strings.TrimSpace(item.Key)
		item.Title = strings.TrimSpace(item.Title)
		item.ExpectedResult = strings.TrimSpace(item.ExpectedResult)
		if item.Key == "" || utf8.RuneCountInString(item.Key) > 100 || item.Title == "" ||
			utf8.RuneCountInString(item.Title) > 240 || len(item.Steps) == 0 || len(item.Steps) > 100 ||
			utf8.RuneCountInString(item.ExpectedResult) > 20_000 {
			return values, ErrTestingMutationInvalid
		}
		if _, duplicate := seen[item.Key]; duplicate {
			return values, ErrTestingMutationInvalid
		}
		seen[item.Key] = struct{}{}
		for stepIndex := range item.Steps {
			step := &item.Steps[stepIndex]
			step.Action = strings.TrimSpace(step.Action)
			step.ExpectedResult = strings.TrimSpace(step.ExpectedResult)
			if step.Action == "" || utf8.RuneCountInString(step.Action) > 20_000 || utf8.RuneCountInString(step.ExpectedResult) > 20_000 {
				return values, ErrTestingMutationInvalid
			}
		}
	}
	return values, nil
}

func normalizeTestingExecution(values TestingExecutionValues, principalID uint) (TestingExecutionValues, error) {
	values.Environment = strings.TrimSpace(values.Environment)
	values.BuildVersion = strings.TrimSpace(values.BuildVersion)
	values.Summary = strings.TrimSpace(values.Summary)
	if values.ExecutorUserID == 0 {
		values.ExecutorUserID = principalID
	}
	if values.DefinitionVersion == 0 || values.Environment == "" || utf8.RuneCountInString(values.Environment) > 160 ||
		utf8.RuneCountInString(values.BuildVersion) > 160 || utf8.RuneCountInString(values.Summary) > 4_000 || values.ExecutorUserID == 0 {
		return values, ErrTestingMutationInvalid
	}
	seen := make(map[string]struct{}, len(values.CaseKeys))
	for index := range values.CaseKeys {
		values.CaseKeys[index] = strings.TrimSpace(values.CaseKeys[index])
		if values.CaseKeys[index] == "" || utf8.RuneCountInString(values.CaseKeys[index]) > 100 {
			return values, ErrTestingMutationInvalid
		}
		if _, duplicate := seen[values.CaseKeys[index]]; duplicate {
			return values, ErrTestingMutationInvalid
		}
		seen[values.CaseKeys[index]] = struct{}{}
	}
	return values, nil
}

func normalizeTestingRun(values TestingRunValues) (TestingRunValues, error) {
	if values.RunID == 0 || values.Version == 0 || len(values.Fields) == 0 {
		return values, ErrTestingMutationInvalid
	}
	values.Status = strings.ToLower(strings.TrimSpace(values.Status))
	values.ActualResult = strings.TrimSpace(values.ActualResult)
	values.FailureReason = strings.TrimSpace(values.FailureReason)
	if values.Fields["status"] && !validTestRunStatus(values.Status) ||
		utf8.RuneCountInString(values.ActualResult) > 20_000 || utf8.RuneCountInString(values.FailureReason) > 20_000 {
		return values, ErrTestingMutationInvalid
	}
	if values.ExecutedAt != nil {
		utc := values.ExecutedAt.UTC()
		values.ExecutedAt = &utc
	}
	return values, nil
}

func IsTestingMutationBusinessError(err error) bool {
	return errors.Is(err, ErrTestingMutationInvalid) || errors.Is(err, ErrTestingMutationNotFound) || errors.Is(err, ErrTestingMutationConflict)
}
