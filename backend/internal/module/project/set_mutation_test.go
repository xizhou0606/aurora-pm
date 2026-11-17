package project

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type setMutationStoreCapture struct {
	command SetMutation
	result  SetMutationResult
}

func (store *setMutationStoreCapture) ApplyProjectSet(_ context.Context, command SetMutation) (SetMutationResult, error) {
	store.command = command
	return store.result, nil
}

type projectReferenceCapture struct {
	workspaceID uint
	teamID      uint
	userIDs     []uint
	valid       bool
	err         error
}

func (capture *projectReferenceCapture) ProjectReferencesValid(
	_ context.Context,
	workspaceID uint,
	teamID uint,
	userIDs []uint,
) (bool, error) {
	capture.workspaceID, capture.teamID = workspaceID, teamID
	capture.userIDs = append([]uint(nil), userIDs...)
	return capture.valid, capture.err
}

func TestSetMutationServiceDefaultsOwnerAndUsesReferencePort(t *testing.T) {
	store := &setMutationStoreCapture{result: SetMutationResult{ID: 9}}
	references := &projectReferenceCapture{valid: true}
	service, err := NewSetMutationService(store, references)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ApplyProjectSet(context.Background(), SetMutation{
		Operation: SetCreate, WorkspaceID: 7, ActorUserID: 3, ApplyKey: "changeset_12345678",
		InputDigest: strings.Repeat("a", 64), TeamID: 8, TeamIDSet: true, Name: "  Project  ", NameSet: true,
	})
	if err != nil || result.ID != 9 || store.command.Name != "Project" || store.command.OwnerUserID == nil ||
		*store.command.OwnerUserID != 3 || references.workspaceID != 7 || references.teamID != 8 || len(references.userIDs) != 1 {
		t.Fatalf("result=%+v command=%+v references=%+v err=%v", result, store.command, references, err)
	}
}

func TestSetMutationServiceFailsClosedForInvalidReferences(t *testing.T) {
	store := &setMutationStoreCapture{}
	references := &projectReferenceCapture{valid: false}
	service, _ := NewSetMutationService(store, references)
	_, err := service.ApplyProjectSet(context.Background(), SetMutation{
		Operation: SetTransferOwner, WorkspaceID: 7, ActorUserID: 3, ApplyKey: "changeset_12345678",
		InputDigest: strings.Repeat("a", 64), ID: 9, NewOwnerUserID: 5,
	})
	if !errors.Is(err, ErrProjectReferenceInvalid) || store.command.Operation != "" {
		t.Fatalf("command=%+v err=%v", store.command, err)
	}
}
