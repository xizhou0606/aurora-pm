package project

import (
	"context"
	"errors"
	"testing"
)

type deletionTestStore struct {
	saga  DeletionSaga
	found bool
}

func (store *deletionTestStore) FindProjectDeletion(context.Context, string) (DeletionSaga, bool, error) {
	return store.saga, store.found, nil
}
func (store *deletionTestStore) BeginProjectDeletion(_ context.Context, command SetMutation, impact DeletionImpact) (DeletionSaga, error) {
	store.saga = DeletionSaga{WorkspaceID: command.WorkspaceID, ProjectSetID: command.ID, ApplyKey: command.ApplyKey,
		InputDigest: command.InputDigest, PlanUnitIDs: impact.Local.PlanUnitIDs, RequirementIDs: impact.RequirementIDs,
		NextStep: DeletionPreflight, Status: "in_progress"}
	store.found = true
	return store.saga, nil
}
func (store *deletionTestStore) RefreshProjectDeletionImpact(_ context.Context, _ string, impact DeletionImpact) (DeletionSaga, error) {
	store.saga.PlanUnitIDs, store.saga.RequirementIDs = impact.Local.PlanUnitIDs, impact.RequirementIDs
	return store.saga, nil
}
func (store *deletionTestStore) AdvanceProjectDeletion(_ context.Context, _ string, step DeletionStep) (DeletionSaga, error) {
	if store.saga.NextStep != step {
		return DeletionSaga{}, ErrMutationConflict
	}
	store.saga.NextStep++
	return store.saga, nil
}
func (store *deletionTestStore) CompleteProjectDeletion(context.Context, string) (SetMutationResult, error) {
	if store.saga.NextStep != DeletionProject {
		return SetMutationResult{}, ErrMutationConflict
	}
	store.saga.NextStep, store.saga.Status = DeletionDone, "succeeded"
	store.saga.Result = SetMutationResult{ID: store.saga.ProjectSetID, Deleted: true}
	return store.saga.Result, nil
}

type deletionTestImpact struct{}

func (deletionTestImpact) InspectProjectDeletionImpact(context.Context, uint, uint) (DeletionImpact, error) {
	return DeletionImpact{Local: LocalProjectImpact{PlanUnitIDs: []uint{12}}, RequirementIDs: []uint{13}}, nil
}

type retryPreflightImpact struct{ calls int }

func (impact *retryPreflightImpact) InspectProjectDeletionImpact(context.Context, uint, uint) (DeletionImpact, error) {
	impact.calls++
	if impact.calls == 2 {
		return DeletionImpact{}, errors.New("temporary snapshot error")
	}
	return DeletionImpact{Local: LocalProjectImpact{PlanUnitIDs: []uint{12}}, RequirementIDs: []uint{13, 14}}, nil
}

type deletionTestSteps struct {
	calls           map[DeletionStep]int
	effects         map[DeletionStep]int
	failAfterEffect DeletionStep
	failed          bool
	businessErr     error
}

func (steps *deletionTestSteps) RunProjectDeletionStep(_ context.Context, step DeletionStep, _ DeletionSaga) error {
	steps.calls[step]++
	if steps.businessErr != nil {
		return steps.businessErr
	}
	if steps.effects[step] == 0 {
		steps.effects[step]++
	}
	if step == steps.failAfterEffect && !steps.failed {
		steps.failed = true
		return errors.New("temporary database failure")
	}
	return nil
}

func TestDeletionServiceResumesAfterCrashWithoutDuplicatingSideEffect(t *testing.T) {
	store := &deletionTestStore{}
	steps := &deletionTestSteps{calls: map[DeletionStep]int{}, effects: map[DeletionStep]int{}, failAfterEffect: DeletionAI}
	service, err := NewDeletionService(store, deletionTestImpact{}, steps)
	if err != nil {
		t.Fatal(err)
	}
	command := SetMutation{Operation: SetDelete, WorkspaceID: 7, ID: 11, ApplyKey: "delete-project-0001",
		InputDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := service.ApplyProjectSetDeletion(context.Background(), command); !errors.Is(err, ErrMutationRetryable) {
		t.Fatalf("first deletion err=%v", err)
	}
	if store.saga.NextStep != DeletionAI {
		t.Fatalf("next step=%d", store.saga.NextStep)
	}
	result, err := service.ApplyProjectSetDeletion(context.Background(), command)
	if err != nil || !result.Deleted || result.ID != 11 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if steps.calls[DeletionWorkflow] != 1 || steps.calls[DeletionAI] != 2 {
		t.Fatalf("calls=%v", steps.calls)
	}
	for step := DeletionWorkflow; step < DeletionProject; step++ {
		if steps.effects[step] != 1 {
			t.Fatalf("step %d effects=%d", step, steps.effects[step])
		}
	}
}

func TestDeletionServiceKeepsBusinessErrorNonRetryable(t *testing.T) {
	steps := &deletionTestSteps{calls: map[DeletionStep]int{}, effects: map[DeletionStep]int{}, businessErr: ErrMutationConflict}
	service, _ := NewDeletionService(&deletionTestStore{}, deletionTestImpact{}, steps)
	_, err := service.ApplyProjectSetDeletion(context.Background(), SetMutation{Operation: SetDelete, WorkspaceID: 7, ID: 11,
		ApplyKey: "delete-project-0002", InputDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	if !errors.Is(err, ErrMutationConflict) || errors.Is(err, ErrMutationRetryable) {
		t.Fatalf("err=%v", err)
	}
}

func TestDeletionServiceRetriesPostGatePreflightBeforeAnySideEffect(t *testing.T) {
	store := &deletionTestStore{}
	impact := &retryPreflightImpact{}
	steps := &deletionTestSteps{calls: map[DeletionStep]int{}, effects: map[DeletionStep]int{}}
	service, _ := NewDeletionService(store, impact, steps)
	command := SetMutation{Operation: SetDelete, WorkspaceID: 7, ID: 11, ApplyKey: "delete-project-preflight",
		InputDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
	if _, err := service.ApplyProjectSetDeletion(context.Background(), command); !errors.Is(err, ErrMutationRetryable) {
		t.Fatalf("err=%v", err)
	}
	if store.saga.NextStep != DeletionPreflight || len(steps.effects) != 0 {
		t.Fatalf("saga=%+v effects=%v", store.saga, steps.effects)
	}
	result, err := service.ApplyProjectSetDeletion(context.Background(), command)
	if err != nil || !result.Deleted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(store.saga.RequirementIDs) != 2 {
		t.Fatalf("snapshot=%+v", store.saga.RequirementIDs)
	}
}
