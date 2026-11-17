package project

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mutationStoreCapture struct {
	command DeliveryLabelMutation
	result  DeliveryLabelMutationResult
}

func (store *mutationStoreCapture) ApplyDeliveryLabel(_ context.Context, command DeliveryLabelMutation) (DeliveryLabelMutationResult, error) {
	store.command = command
	return store.result, nil
}

func TestMutationServiceValidatesAndNormalizesDeliveryLabels(t *testing.T) {
	store := &mutationStoreCapture{result: DeliveryLabelMutationResult{ID: 9}}
	service, err := NewMutationService(store)
	if err != nil {
		t.Fatal(err)
	}
	command := DeliveryLabelMutation{
		Operation: DeliveryLabelCreate, WorkspaceID: 7, ApplyKey: "changeset_12345678",
		InputDigest: strings.Repeat("a", 64), ID: 8, Name: "  Release  ", SortOrder: 2,
	}
	result, err := service.ApplyDeliveryLabel(context.Background(), command)
	if err != nil || result.ID != 9 || store.command.Name != "Release" {
		t.Fatalf("result=%+v command=%+v err=%v", result, store.command, err)
	}
	for _, invalid := range []DeliveryLabelMutation{
		{},
		{Operation: DeliveryLabelCreate, WorkspaceID: 7, ApplyKey: "changeset_12345678", InputDigest: strings.Repeat("a", 64), ID: 8},
		{Operation: DeliveryLabelDelete, WorkspaceID: 7, ApplyKey: "changeset_12345678", InputDigest: "short", ID: 8},
	} {
		if _, err := service.ApplyDeliveryLabel(context.Background(), invalid); !errors.Is(err, ErrMutationInvalid) {
			t.Fatalf("invalid=%+v err=%v", invalid, err)
		}
	}
}
