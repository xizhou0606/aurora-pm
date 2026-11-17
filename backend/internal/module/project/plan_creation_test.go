package project

import (
	"context"
	"errors"
	"testing"
)

type planCreationTestStore struct {
	saga        PlanCreationSaga
	found       bool
	completions int
}

func (store *planCreationTestStore) FindPlanCreation(context.Context, string) (PlanCreationSaga, bool, error) {
	return store.saga, store.found, nil
}
func (store *planCreationTestStore) BeginPlanCreation(_ context.Context, command PlanMutation) (PlanCreationSaga, error) {
	store.saga = PlanCreationSaga{WorkspaceID: command.WorkspaceID, ProjectSetID: command.ProjectSetID, PlanUnitID: 12,
		WorkflowTemplateID: command.WorkflowTemplateID, ApplyKey: command.ApplyKey, InputDigest: command.InputDigest,
		Status: "in_progress", Result: PlanMutationResult{ID: 12, WorkspaceID: command.WorkspaceID, TeamID: 8,
			ProjectSetID: command.ProjectSetID, Title: command.Title, Type: command.Type, Status: "planning"}}
	store.found = true
	return store.saga, nil
}
func (store *planCreationTestStore) RecordPlanCreationWorkflow(_ context.Context, _ string, id uint, status string) (PlanCreationSaga, error) {
	store.saga.WorkflowInstanceID, store.saga.SubjectStatus, store.saga.NextStep = id, status, 1
	return store.saga, nil
}
func (store *planCreationTestStore) CompletePlanCreation(context.Context, string) (PlanMutationResult, error) {
	store.completions++
	store.saga.Status, store.saga.NextStep = "succeeded", 2
	store.saga.Result.Status = store.saga.SubjectStatus
	return store.saga.Result, nil
}
func (store *planCreationTestStore) RejectPlanCreation(context.Context, string) error {
	store.saga.Status = "rejected"
	return nil
}

type planCreationWorkflowStub struct {
	calls int
	fail  bool
}

type planCreationReferencesStub struct{}

func (planCreationReferencesStub) ProjectReferencesValid(context.Context, uint, uint, []uint) (bool, error) {
	return true, nil
}

func (stub *planCreationWorkflowStub) StartPlanUnitWorkflow(context.Context, PlanWorkflowStartCommand) (PlanWorkflowStartResult, error) {
	stub.calls++
	if stub.fail {
		stub.fail = false
		return PlanWorkflowStartResult{}, errors.New("temporary workflow store error")
	}
	return PlanWorkflowStartResult{InstanceID: 20, SubjectStatus: "grooming"}, nil
}

func TestPlanCreationResumesWorkflowStartAfterTransientFailure(t *testing.T) {
	store, workflow := &planCreationTestStore{}, &planCreationWorkflowStub{fail: true}
	service, err := NewPlanCreationService(store, planCreationReferencesStub{}, workflow)
	if err != nil {
		t.Fatal(err)
	}
	command := PlanMutation{Operation: PlanCreate, WorkspaceID: 7, ActorUserID: 9, ProjectSetID: 11,
		WorkflowTemplateID: 10, Title: "V1", Type: "version", ApplyKey: "plan-create-0001",
		InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := service.ApplyPlanUnitCreation(context.Background(), command); !errors.Is(err, ErrMutationRetryable) {
		t.Fatalf("first err=%v", err)
	}
	result, err := service.ApplyPlanUnitCreation(context.Background(), command)
	if err != nil || result.ID != 12 || result.Status != "grooming" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if workflow.calls != 2 || store.completions != 1 {
		t.Fatalf("workflow=%d completions=%d", workflow.calls, store.completions)
	}
}
