package task

import (
	"context"
	"errors"
	"testing"
)

type testingReadFake struct {
	runFilter TestRunFilter
}

func (*testingReadFake) GetTestDefinition(context.Context, uint, uint) (TestDefinitionSnapshot, error) {
	return TestDefinitionSnapshot{}, nil
}
func (*testingReadFake) ListTestExecutions(context.Context, uint, uint, int, int) (TestExecutionPage, error) {
	return TestExecutionPage{}, nil
}
func (*testingReadFake) GetTestExecution(context.Context, uint, uint, uint) (TestExecutionSnapshot, error) {
	return TestExecutionSnapshot{}, nil
}
func (fake *testingReadFake) ListTestExecutionRuns(_ context.Context, _, _, _ uint, filter TestRunFilter) (TestExecutionRunPage, error) {
	fake.runFilter = filter
	return TestExecutionRunPage{}, nil
}

func TestTestingReadServiceNormalizesAndCapsRunFilter(t *testing.T) {
	fake := &testingReadFake{}
	service, err := NewTestingReadService(fake)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListExecutionRuns(context.Background(), 1, 2, 3, TestRunFilter{Status: " PASSED ", Limit: 1000}); err != nil {
		t.Fatal(err)
	}
	if fake.runFilter.Status != "passed" || fake.runFilter.Limit != 500 {
		t.Fatalf("filter=%+v", fake.runFilter)
	}
	if _, err := service.ListExecutionRuns(context.Background(), 1, 2, 3, TestRunFilter{Status: "unknown"}); !errors.Is(err, ErrTestingReadInvalid) {
		t.Fatalf("invalid status err=%v", err)
	}
}
