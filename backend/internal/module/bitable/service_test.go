package bitable

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

type testStore struct{ workspace uint }

func (s testStore) Get(context.Context, uint, string) (Resource, error) {
	return Resource{ID: "550e8400-e29b-41d4-a716-446655440000", WorkspaceID: s.workspace, Tables: []Table{{ID: "table-1"}}}, nil
}
func (s testStore) List(context.Context, uint) ([]Resource, error) { return []Resource{}, nil }

type testPolicy struct {
	deny   bool
	writes bool
}

type testResourceAccess struct{ policy *testPolicy }

func (gate testResourceAccess) RequireBitableResource(_ context.Context, _ Principal, id documentlibrary.ResourceID, operation documentlibrary.Operation) (ResourceBinding, error) {
	binding := ResourceBinding{ResourceID: id, NativeResourceID: string(id), OwnerUserID: 1}
	return binding, gate.AuthorizeBitableResource(context.Background(), Principal{}, binding, operation)
}
func (gate testResourceAccess) FilterBitableResources(_ context.Context, _ Principal, ids []string, operation documentlibrary.Operation) (map[string]ResourceBinding, error) {
	result := map[string]ResourceBinding{}
	for _, id := range ids {
		binding, err := gate.RequireBitableResource(context.Background(), Principal{}, documentlibrary.ResourceID(id), operation)
		if err == nil {
			result[id] = binding
		}
	}
	return result, nil
}
func (gate testResourceAccess) AuthorizeBitableResource(_ context.Context, _ Principal, _ ResourceBinding, operation documentlibrary.Operation) error {
	if gate.policy.deny || operation != documentlibrary.OperationView && !gate.policy.writes {
		return ErrForbidden
	}
	return nil
}
func (gate testResourceAccess) BatchAuthorizeBitableResources(_ context.Context, _ Principal, bindings []ResourceBinding, operations []documentlibrary.Operation) (map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, error) {
	result := map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool{}
	for _, binding := range bindings {
		result[binding.ResourceID] = map[documentlibrary.Operation]bool{}
		for _, operation := range operations {
			result[binding.ResourceID][operation] = gate.AuthorizeBitableResource(context.Background(), Principal{}, binding, operation) == nil
		}
	}
	return result, nil
}
func (gate testResourceAccess) RequireBitableCreate(context.Context, Principal) error { return nil }

func configureTestAccess(service *Service, policy *testPolicy) {
	service.ConfigureUnifiedResourceAccess(testResourceAccess{policy: policy}, nil)
}

func (p *testPolicy) RequireAction(_ context.Context, _ Principal, a accesscatalog.ActionCode) error {
	if p.deny || a != accesscatalog.ActionBitablesResourcesView && !p.writes {
		return ErrForbidden
	}
	return nil
}

type testProvider struct {
	calls     int
	schema    Bootstrap
	afterRead func()
	operation Operation
}

func (p *testProvider) Bootstrap(context.Context, Scope) (Bootstrap, error) {
	p.calls++
	return p.schema, nil
}
func (p *testProvider) Query(context.Context, Scope, Query) (RecordPage, error) {
	p.calls++
	if p.afterRead != nil {
		p.afterRead()
	}
	return RecordPage{Records: []Record{{ID: "row"}}}, nil
}
func (p *testProvider) Search(context.Context, Scope, Search) (SearchPage, error) {
	p.calls++
	return SearchPage{}, nil
}
func (p *testProvider) Execute(context.Context, Scope, CommandBatch) (Operation, error) {
	p.calls++
	return Operation{State: "committed"}, nil
}
func (p *testProvider) Operation(context.Context, Scope, string) (Operation, error) {
	p.calls++
	return p.operation, nil
}
func TestOperationRequiresOriginalWritePermission(t *testing.T) {
	provider := &testProvider{operation: Operation{State: "committed", Records: []Record{{ID: "secret-row"}}, RequiredActions: []accesscatalog.ActionCode{accesscatalog.ActionBitablesRecordsWrite}}}
	policy := &testPolicy{}
	service, _ := NewService(testStore{1}, nil, policy, provider)
	configureTestAccess(service, policy)
	result, err := service.Operation(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1", "550e8400-e29b-41d4-a716-446655440001")
	if !errors.Is(err, ErrForbidden) || len(result.Records) != 0 {
		t.Fatal("receipt exposed after write revocation", err)
	}
}
func (p *testProvider) Export(context.Context, Scope, ExportQuery) (ExportSnapshot, error) {
	p.calls++
	return ExportSnapshot{}, nil
}

func TestWorkspaceAndActionDenialNeverCallsProvider(t *testing.T) {
	for _, tc := range []struct {
		name      string
		workspace uint
		deny      bool
	}{{"workspace", 2, false}, {"permission", 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := &testProvider{}
			policy := &testPolicy{deny: tc.deny}
			s, _ := NewService(testStore{tc.workspace}, nil, policy, p)
			configureTestAccess(s, policy)
			_, e := s.Query(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1", Query{ViewID: "view", Limit: 1})
			if !errors.Is(e, ErrForbidden) || p.calls != 0 {
				t.Fatal("provider accessed on denied scope", e, p.calls)
			}
		})
	}
}
func TestReadRechecksRevocationBeforeReturn(t *testing.T) {
	policy := &testPolicy{}
	provider := &testProvider{afterRead: func() { policy.deny = true }}
	s, _ := NewService(testStore{1}, nil, policy, provider)
	configureTestAccess(s, policy)
	v, e := s.Query(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1", Query{ViewID: "view", Limit: 1})
	if !errors.Is(e, ErrForbidden) || len(v.Records) > 0 {
		t.Fatal("revoked data released", e)
	}
}
func TestReadonlyBootstrapHasNoWritableCapabilities(t *testing.T) {
	policy := &testPolicy{}
	s, _ := NewService(testStore{1}, nil, policy, &testProvider{schema: Bootstrap{Fields: []Field{{ID: "field", Type: FieldText}}}})
	configureTestAccess(s, policy)
	v, e := s.Bootstrap(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1")
	if e != nil || !v.Capabilities.CanRead || v.Capabilities.CanCreateRecords || len(v.Capabilities.WritableFieldIDs) != 0 {
		t.Fatal("readonly capability escaped", e)
	}
}

func TestWritableBootstrapIncludesUploadableAttachmentField(t *testing.T) {
	policy := &testPolicy{writes: true}
	s, _ := NewService(testStore{1}, nil, policy, &testProvider{schema: Bootstrap{Fields: []Field{{ID: "files", Type: FieldAttachment}, {ID: "readonly-files", Type: FieldAttachment, ReadOnly: true}, {ID: "created", Type: FieldCreatedBy}}}})
	configureTestAccess(s, policy)
	v, err := s.Bootstrap(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1")
	if err != nil || !v.Capabilities.CanUploadAttachments || len(v.Capabilities.WritableFieldIDs) != 1 || v.Capabilities.WritableFieldIDs[0] != "files" {
		t.Fatalf("uploadable attachment capability missing: %+v, %v", v.Capabilities, err)
	}
}

func TestFieldValueGuards(t *testing.T) {
	for _, f := range []Field{{Type: FieldAttachment}, {Type: FieldCreatedBy}, {Type: FieldLastModifiedTime}, {Type: FieldText, ReadOnly: true}} {
		if !errors.Is(ValidateValue(f, nil), ErrForbidden) {
			t.Fatal("immutable field cleared")
		}
	}
	if ValidateValue(Field{Type: FieldDate}, "yesterday") == nil {
		t.Fatal("invalid date accepted")
	}
	if ValidateValue(Field{Type: FieldUser}, []UserRef{{ID: 1}, {ID: 2}}) == nil {
		t.Fatal("single user accepted multiple")
	}
	if ValidateValue(Field{Type: FieldSingleSelect, Options: FieldOptions{Choices: []Choice{{ID: "choice", Name: "label"}}}}, "label") == nil {
		t.Fatal("label accepted as stable choice ID")
	}
}
