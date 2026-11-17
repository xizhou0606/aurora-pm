package project

import (
	"context"
	"strings"
	"testing"
)

type planMutationStoreFake struct{ command PlanMutation }

func (fake *planMutationStoreFake) ApplyPlanUnit(_ context.Context, command PlanMutation) (PlanMutationResult, error) {
	fake.command = command
	return PlanMutationResult{ID: command.ID}, nil
}

type planReferencesFake struct{ valid bool }

func (fake planReferencesFake) ProjectReferencesValid(context.Context, uint, uint, []uint) (bool, error) {
	return fake.valid, nil
}

func TestPlanMutationUpdateValidatesReferencesAndDelegatesScheduleAtomically(t *testing.T) {
	store := &planMutationStoreFake{}
	service, _ := NewPlanMutationService(store, planReferencesFake{valid: true})
	owner := uint(9)
	result, err := service.ApplyPlanUnit(context.Background(), PlanMutation{Operation: PlanUpdate, WorkspaceID: 7, ActorUserID: 3,
		ApplyKey: "plan-update-0001", InputDigest: strings.Repeat("a", 64), ID: 11, Title: " Renamed ", TitleSet: true, OwnerUserID: &owner, OwnerSet: true})
	if err != nil || result.ID != 11 || store.command.Title != "Renamed" {
		t.Fatalf("result=%+v command=%+v err=%v", result, store.command, err)
	}
	blocked := store.command
	blocked.ApplyKey = "plan-update-0002"
	blocked.PlannedStartSet = true
	if _, err := service.ApplyPlanUnit(context.Background(), blocked); err != nil || !store.command.PlannedStartSet {
		t.Fatalf("schedule err=%v", err)
	}
}
