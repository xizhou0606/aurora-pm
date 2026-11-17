package workflow

import (
	"context"
	"errors"
	"testing"
)

type auxReadStoreFake struct{ resolved AuxNodeContext }

func (fake auxReadStoreFake) ResolveAuxNode(context.Context, uint, uint, string, AuxPurpose) (AuxNodeContext, error) {
	return fake.resolved, nil
}

func (auxReadStoreFake) ListNodeOpinions(context.Context, uint, uint, string) ([]NodeOpinion, error) {
	return nil, nil
}

func TestAuxReadServiceValidatesInputAndNormalizesEmptyList(t *testing.T) {
	service, err := NewAuxReadService(auxReadStoreFake{resolved: AuxNodeContext{
		InstanceID: 2, WorkspaceID: 1, SubjectType: "requirement", SubjectID: 3,
		NodeID: "review", NodeType: "approval", NodeName: "预审",
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, items, err := service.ListOpinions(context.Background(), 1, 2, " review ")
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if _, err := service.ResolveNode(context.Background(), 0, 2, "review", AuxPurposeOpinion); !errors.Is(err, ErrAuxReadInvalid) {
		t.Fatalf("workspace err=%v", err)
	}
	if _, err := service.ResolveNode(context.Background(), 1, 2, "review", AuxPurpose("unknown")); !errors.Is(err, ErrAuxReadInvalid) {
		t.Fatalf("purpose err=%v", err)
	}
}
