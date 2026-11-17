package project

import (
	"context"
	"errors"
	"testing"
)

type planDeletionTestStore struct {
	saga        PlanDeletionSaga
	found       bool
	rejects     int
	completions int
}

func (store *planDeletionTestStore) FindPlanDeletion(context.Context, string) (PlanDeletionSaga, bool, error) {
	return store.saga, store.found, nil
}

func (store *planDeletionTestStore) BeginPlanDeletion(_ context.Context, command PlanMutation) (PlanDeletionSaga, error) {
	store.saga = PlanDeletionSaga{WorkspaceID: command.WorkspaceID, ProjectSetID: 11, PlanUnitID: command.ID,
		ApplyKey: command.ApplyKey, InputDigest: command.InputDigest, Status: "in_progress"}
	store.found = true
	return store.saga, nil
}

func (store *planDeletionTestStore) RejectPlanDeletion(context.Context, string) error {
	store.rejects++
	store.saga.Status = "rejected"
	return nil
}

func (store *planDeletionTestStore) AdvancePlanDeletion(_ context.Context, _ string, step int) (PlanDeletionSaga, error) {
	if store.saga.Status != "in_progress" || store.saga.NextStep != step {
		return PlanDeletionSaga{}, ErrMutationConflict
	}
	store.saga.NextStep++
	return store.saga, nil
}

func (store *planDeletionTestStore) CompletePlanDeletion(context.Context, string) (PlanMutationResult, error) {
	if store.saga.Status != "in_progress" || store.saga.NextStep != 2 {
		return PlanMutationResult{}, ErrMutationConflict
	}
	store.completions++
	store.saga.Status = "succeeded"
	store.saga.Result = PlanMutationResult{ID: store.saga.PlanUnitID, Deleted: true}
	return store.saga.Result, nil
}

type planDeletionTestImpact struct {
	impact PlanDeletionImpact
	err    error
	calls  int
}

func (value *planDeletionTestImpact) InspectPlanUnitDeletion(context.Context, uint, uint) (PlanDeletionImpact, error) {
	value.calls++
	return value.impact, value.err
}

type planDeletionTestWorkflow struct {
	calls   int
	effects int
	fail    bool
}

func (workflow *planDeletionTestWorkflow) DeletePlanUnitScaffolding(context.Context, uint, uint) error {
	workflow.calls++
	if workflow.effects == 0 {
		workflow.effects++
	}
	if workflow.fail {
		workflow.fail = false
		return errors.New("temporary workflow failure")
	}
	return nil
}

func planDeleteCommand() PlanMutation {
	return PlanMutation{Operation: PlanDelete, WorkspaceID: 7, ActorUserID: 9, ID: 12,
		ApplyKey: "plan-delete-0001", InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
}

func TestPlanDeletionRejectsImpactBeforeWorkflowSideEffect(t *testing.T) {
	store := &planDeletionTestStore{}
	impact := &planDeletionTestImpact{impact: PlanDeletionImpact{RequirementCount: 1}}
	workflow := &planDeletionTestWorkflow{}
	service, err := NewPlanDeletionService(store, impact, workflow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyPlanUnitDeletion(context.Background(), planDeleteCommand()); !errors.Is(err, ErrMutationConflict) {
		t.Fatalf("err=%v", err)
	}
	if store.rejects != 1 || workflow.calls != 0 || store.completions != 0 {
		t.Fatalf("rejects=%d workflow=%d completions=%d", store.rejects, workflow.calls, store.completions)
	}
}

func TestPlanDeletionResumesAfterTransientWorkflowFailureWithoutDuplicatingEffect(t *testing.T) {
	store := &planDeletionTestStore{}
	impact := &planDeletionTestImpact{}
	workflow := &planDeletionTestWorkflow{fail: true}
	service, _ := NewPlanDeletionService(store, impact, workflow)
	command := planDeleteCommand()
	if _, err := service.ApplyPlanUnitDeletion(context.Background(), command); !errors.Is(err, ErrMutationRetryable) {
		t.Fatalf("first err=%v", err)
	}
	if store.saga.NextStep != 1 || workflow.effects != 1 {
		t.Fatalf("saga=%+v effects=%d", store.saga, workflow.effects)
	}
	result, err := service.ApplyPlanUnitDeletion(context.Background(), command)
	if err != nil || !result.Deleted || result.ID != command.ID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if impact.calls != 1 || workflow.calls != 2 || workflow.effects != 1 || store.completions != 1 {
		t.Fatalf("impact=%d workflow=%d effects=%d completions=%d", impact.calls, workflow.calls, workflow.effects, store.completions)
	}
}

func TestPlanDeletionRejectsReplayWithDifferentBinding(t *testing.T) {
	command := planDeleteCommand()
	store := &planDeletionTestStore{found: true, saga: PlanDeletionSaga{WorkspaceID: command.WorkspaceID + 1,
		PlanUnitID: command.ID, ApplyKey: command.ApplyKey, InputDigest: command.InputDigest, Status: "in_progress"}}
	service, _ := NewPlanDeletionService(store, &planDeletionTestImpact{}, &planDeletionTestWorkflow{})
	if _, err := service.ApplyPlanUnitDeletion(context.Background(), command); !errors.Is(err, ErrMutationConflict) {
		t.Fatalf("err=%v", err)
	}
}
