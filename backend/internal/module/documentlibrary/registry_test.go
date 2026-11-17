package documentlibrary

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryFailsClosedForUnknownAndUnsupportedTypes(t *testing.T) {
	registry := DefaultRegistry()
	if _, err := registry.Resolve("unknown"); !errors.Is(err, ErrTypeUnavailable) {
		t.Fatalf("Resolve unknown error = %v", err)
	}
	if err := registry.Require(TypeRichText, OperationCellLease); !errors.Is(err, ErrOperationUnsupported) {
		t.Fatalf("Require unsupported error = %v", err)
	}
}

func TestRegistryAcceptsThirdTypeWithoutChangingPolicySchema(t *testing.T) {
	registry := DefaultRegistry()
	err := registry.Register(TypeDescriptor{
		TypeKey:      "whiteboard",
		Capabilities: capabilities(OperationView, OperationEdit, OperationManageAccess),
	})
	if err != nil {
		t.Fatalf("Register third type: %v", err)
	}
	if err := registry.Require("whiteboard", OperationManageAccess); err != nil {
		t.Fatalf("Require manage access: %v", err)
	}
}

func TestServiceTypeGateDoesNotMasqueradeAsAuthorization(t *testing.T) {
	var _ interface {
		RequireTypeOperation(context.Context, uint, ResourceID, Operation) (Resource, ContentBinding, error)
	} = (*Service)(nil)
}

func TestRegistryDoesNotExposeMutableCapabilityState(t *testing.T) {
	input := TypeDescriptor{TypeKey: "whiteboard", Capabilities: capabilities(OperationView)}
	registry, err := NewRegistry(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Capabilities.Operations[OperationEdit] = true
	resolved, err := registry.Resolve("whiteboard")
	if err != nil {
		t.Fatal(err)
	}
	resolved.Capabilities.Operations[OperationEdit] = true
	if err := registry.Require("whiteboard", OperationEdit); !errors.Is(err, ErrOperationUnsupported) {
		t.Fatalf("mutable descriptor escaped registry: %v", err)
	}
}
