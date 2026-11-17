package bitable

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

const (
	testNativeResourceID  = "550e8400-e29b-41d4-a716-446655440000"
	testNativeResourceID2 = "550e8400-e29b-41d4-a716-446655440001"
	testUnifiedResourceID = "dr_bitable_authoritative"
)

type accessStore struct {
	items       []Resource
	lastGetID   string
	createCalls int
}

func (store *accessStore) Get(_ context.Context, _ uint, id string) (Resource, error) {
	store.lastGetID = id
	for _, item := range store.items {
		if item.ID == id {
			return item, nil
		}
	}
	return Resource{}, ErrNotFound
}
func (store *accessStore) List(context.Context, uint) ([]Resource, error) {
	return append([]Resource(nil), store.items...), nil
}
func (store *accessStore) Create(context.Context, Principal, string, string) (Resource, error) {
	store.createCalls++
	return Resource{}, nil
}

type accessGateStub struct {
	requiredIDs []documentlibrary.ResourceID
	operations  []documentlibrary.Operation
	bindings    map[string]ResourceBinding
	allowed     map[string]bool
	batchCalls  int
}

func (gate *accessGateStub) BatchAuthorizeBitableResources(_ context.Context, _ Principal, bindings []ResourceBinding, operations []documentlibrary.Operation) (map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, error) {
	gate.batchCalls++
	result := map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool{}
	for _, binding := range bindings {
		result[binding.ResourceID] = map[documentlibrary.Operation]bool{}
		for _, operation := range operations {
			result[binding.ResourceID][operation] = gate.allowed[string(binding.ResourceID)]
		}
	}
	return result, nil
}

func (gate *accessGateStub) RequireBitableResource(_ context.Context, _ Principal, id documentlibrary.ResourceID, operation documentlibrary.Operation) (ResourceBinding, error) {
	gate.requiredIDs = append(gate.requiredIDs, id)
	gate.operations = append(gate.operations, operation)
	binding, ok := gate.bindings[string(id)]
	if !ok || !gate.allowed[string(id)] {
		return ResourceBinding{}, ErrNotFound
	}
	return binding, nil
}
func (gate *accessGateStub) FilterBitableResources(_ context.Context, _ Principal, ids []string, operation documentlibrary.Operation) (map[string]ResourceBinding, error) {
	gate.operations = append(gate.operations, operation)
	result := map[string]ResourceBinding{}
	for _, nativeID := range ids {
		for unifiedID, binding := range gate.bindings {
			if binding.NativeResourceID == nativeID && gate.allowed[unifiedID] {
				result[nativeID] = binding
			}
		}
	}
	return result, nil
}
func (gate *accessGateStub) AuthorizeBitableResource(_ context.Context, _ Principal, binding ResourceBinding, operation documentlibrary.Operation) error {
	gate.operations = append(gate.operations, operation)
	if !gate.allowed[string(binding.ResourceID)] {
		return ErrNotFound
	}
	return nil
}
func (gate *accessGateStub) RequireBitableCreate(context.Context, Principal) error { return nil }

type creationCoordinatorStub struct {
	created    int
	duplicated int
	mutated    int
}

func (coordinator *creationCoordinatorStub) MutateBitableResource(_ context.Context, _ Principal, binding ResourceBinding, kind string, input ResourceMutation) (Resource, ResourceBinding, error) {
	coordinator.mutated++
	if kind == "owner" {
		binding.OwnerUserID = input.OwnerUserID
	}
	return Resource{ID: binding.NativeResourceID, WorkspaceID: 1}, binding, nil
}

func (coordinator *creationCoordinatorStub) CreatePrivateBitable(context.Context, Principal, string, string) (Resource, ResourceBinding, error) {
	coordinator.created++
	return Resource{ID: testNativeResourceID, WorkspaceID: 1}, ResourceBinding{
		ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1,
	}, nil
}
func (coordinator *creationCoordinatorStub) DuplicatePrivateBitable(context.Context, Principal, ResourceBinding, ResourceMutation) (Resource, ResourceBinding, error) {
	coordinator.duplicated++
	return Resource{ID: testNativeResourceID2, WorkspaceID: 1}, ResourceBinding{
		ResourceID: "dr_bitable_copy", NativeResourceID: testNativeResourceID2, OwnerUserID: 1,
	}, nil
}

func TestUnifiedResourceIDResolvesNativePayloadWithoutMasquerading(t *testing.T) {
	store := &accessStore{items: []Resource{{
		ID: testNativeResourceID, WorkspaceID: 1, OwnerUserID: 1, Tables: []Table{{ID: "table-1"}},
	}}}
	gate := &accessGateStub{
		bindings: map[string]ResourceBinding{testUnifiedResourceID: {
			ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1,
		}},
		allowed: map[string]bool{testUnifiedResourceID: true},
	}
	service, _ := NewService(store, store, &testPolicy{}, &testProvider{})
	service.ConfigureUnifiedResourceAccess(gate, nil)

	resource, err := service.Get(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, testUnifiedResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if store.lastGetID != testNativeResourceID || resource.ID != testUnifiedResourceID ||
		resource.ResourceID != testUnifiedResourceID || resource.NativeResourceID != testNativeResourceID {
		t.Fatalf("unified/native projection mismatch: %+v native lookup %s", resource, store.lastGetID)
	}
	if resource.ID == testNativeResourceID || resource.ResourceID == testNativeResourceID {
		t.Fatal("native bitable UUID masqueraded as unified resource ID")
	}
}

func TestListFiltersWithOneBatchBeforeProjection(t *testing.T) {
	store := &accessStore{items: []Resource{
		{ID: testNativeResourceID, WorkspaceID: 1}, {ID: testNativeResourceID2, WorkspaceID: 1},
	}}
	gate := &accessGateStub{
		bindings: map[string]ResourceBinding{
			testUnifiedResourceID: {ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1},
			"dr_hidden":           {ResourceID: "dr_hidden", NativeResourceID: testNativeResourceID2, OwnerUserID: 2},
		},
		allowed: map[string]bool{testUnifiedResourceID: true},
	}
	service, _ := NewService(store, store, &testPolicy{}, &testProvider{})
	service.ConfigureUnifiedResourceAccess(gate, nil)
	items, err := service.List(context.Background(), Principal{UserID: 1, WorkspaceID: 1})
	if err != nil || len(items) != 1 || items[0].ID != testUnifiedResourceID {
		t.Fatalf("list leaked or lost resources: %+v %v", items, err)
	}
	if gate.batchCalls != 1 {
		t.Fatalf("list capabilities must use one batch authorization, got %d", gate.batchCalls)
	}
}

func TestListWithNoAuthorizedResourcesSkipsCapabilityBatch(t *testing.T) {
	store := &accessStore{items: []Resource{{ID: testNativeResourceID, WorkspaceID: 1}}}
	gate := &accessGateStub{
		bindings: map[string]ResourceBinding{testUnifiedResourceID: {
			ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1,
		}},
		allowed: map[string]bool{},
	}
	service, _ := NewService(store, store, &testPolicy{}, &testProvider{})
	service.ConfigureUnifiedResourceAccess(gate, nil)
	items, err := service.List(context.Background(), Principal{UserID: 1, WorkspaceID: 1})
	if err != nil || len(items) != 0 {
		t.Fatalf("fully filtered list = %#v, %v", items, err)
	}
	if gate.batchCalls != 0 {
		t.Fatalf("empty authorized set called capability batch %d times", gate.batchCalls)
	}
}

func TestCreateAndDuplicateFailWithoutAtomicPrivateCoordinator(t *testing.T) {
	store := &accessStore{items: []Resource{{ID: testNativeResourceID, WorkspaceID: 1}}}
	gate := &accessGateStub{
		bindings: map[string]ResourceBinding{testUnifiedResourceID: {
			ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1,
		}}, allowed: map[string]bool{testUnifiedResourceID: true},
	}
	service, _ := NewService(store, store, &testPolicy{}, &testProvider{})
	service.ConfigureUnifiedResourceAccess(gate, nil)
	principal := Principal{UserID: 1, WorkspaceID: 1}
	if _, err := service.Create(context.Background(), principal, "Table", testNativeResourceID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("create must fail closed without atomic coordinator: %v", err)
	}
	if _, err := service.MutateResource(context.Background(), principal, testUnifiedResourceID, "duplicate", ResourceMutation{
		OperationID: testNativeResourceID2, ExpectedRevision: 1,
	}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("duplicate must fail closed without atomic coordinator: %v", err)
	}
	if store.createCalls != 0 {
		t.Fatal("legacy native creator was called before private policy transaction")
	}
}

func TestAtomicCoordinatorOwnsCreateAndDuplicate(t *testing.T) {
	store := &accessStore{items: []Resource{{ID: testNativeResourceID, WorkspaceID: 1}}}
	gate := &accessGateStub{
		bindings: map[string]ResourceBinding{testUnifiedResourceID: {
			ResourceID: testUnifiedResourceID, NativeResourceID: testNativeResourceID, OwnerUserID: 1,
		}}, allowed: map[string]bool{testUnifiedResourceID: true},
	}
	coordinator := &creationCoordinatorStub{}
	service, _ := NewService(store, store, &testPolicy{}, &testProvider{})
	service.ConfigureUnifiedResourceAccess(gate, coordinator)
	principal := Principal{UserID: 1, WorkspaceID: 1}
	created, err := service.Create(context.Background(), principal, "Table", testNativeResourceID)
	if err != nil || created.ID != testUnifiedResourceID || coordinator.created != 1 {
		t.Fatalf("coordinated create failed: %+v %v", created, err)
	}
	copy, err := service.MutateResource(context.Background(), principal, testUnifiedResourceID, "duplicate", ResourceMutation{
		OperationID: testNativeResourceID2, ExpectedRevision: 1,
	})
	if err != nil || copy.ID != "dr_bitable_copy" || coordinator.duplicated != 1 {
		t.Fatalf("coordinated duplicate failed: %+v %v", copy, err)
	}
}

func TestPublicBitableOperationsMapToUnifiedPolicyOperations(t *testing.T) {
	want := map[string]documentlibrary.Operation{
		"name": documentlibrary.OperationStructure, "location": documentlibrary.OperationView,
		"owner": documentlibrary.OperationManageAccess, "delete": documentlibrary.OperationDelete,
		"duplicate": documentlibrary.OperationCopy, "shortcut": documentlibrary.OperationCopy,
		"pin": documentlibrary.OperationView, "favorite": documentlibrary.OperationView,
	}
	for kind, operation := range want {
		if resourceOperations[kind] != operation {
			t.Errorf("resource operation %s = %s, want %s", kind, resourceOperations[kind], operation)
		}
	}
	legacyActions := map[documentlibrary.Operation][]accesscatalog.ActionCode{
		documentlibrary.OperationView:       {accesscatalog.ActionBitablesResourcesView},
		documentlibrary.OperationEdit:       {accesscatalog.ActionBitablesRecordsWrite},
		documentlibrary.OperationStructure:  {accesscatalog.ActionBitablesFieldsManage, accesscatalog.ActionBitablesViewsManage},
		documentlibrary.OperationComment:    {accesscatalog.ActionBitablesCommentsCreate},
		documentlibrary.OperationAssetRead:  {accesscatalog.ActionBitablesAssetsDownload},
		documentlibrary.OperationAssetWrite: {accesscatalog.ActionBitablesAssetsUpload},
		documentlibrary.OperationExport:     {accesscatalog.ActionBitablesTransferExport},
	}
	for operation, actions := range legacyActions {
		for _, action := range actions {
			got, ok := operationForLegacyBitableAction(action)
			if !ok || got != operation {
				t.Errorf("legacy action %s = %s/%v, want %s", action, got, ok, operation)
			}
		}
	}
}
