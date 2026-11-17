package bitable

import (
	"context"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

type Service struct {
	store    ResourceStore
	creator  ResourceCreator
	policy   Authorizer
	provider DataProvider
	people   PeopleDirectory
	assets   AssetStorage
	folders  FolderDirectory
	access   ResourceAccessGate
	creation PrivateResourceCoordinator
}

func (s *Service) ConfigureUnifiedResourceAccess(gate ResourceAccessGate, creation PrivateResourceCoordinator) {
	s.access, s.creation = gate, creation
}

func NewService(store ResourceStore, creator ResourceCreator, policy Authorizer, provider DataProvider) (*Service, error) {
	if store == nil || policy == nil || provider == nil {
		return nil, ErrUnavailable
	}
	return &Service{store: store, creator: creator, policy: policy, provider: provider}, nil
}
func (s *Service) authorize(ctx context.Context, p Principal, a accesscatalog.ActionCode) error {
	if p.UserID == 0 || p.WorkspaceID == 0 {
		return ErrForbidden
	}
	return s.policy.RequireAction(ctx, p, a)
}
func (s *Service) Get(ctx context.Context, p Principal, id string) (Resource, error) {
	binding, e := s.requireResource(ctx, p, id, documentlibrary.OperationView)
	if e != nil {
		return Resource{}, e
	}
	if _, e := uuid.Parse(binding.NativeResourceID); e != nil {
		return Resource{}, ErrInvalidInput
	}
	b, e := s.store.Get(ctx, p.WorkspaceID, binding.NativeResourceID)
	if e != nil {
		return Resource{}, e
	}
	if b.WorkspaceID != p.WorkspaceID {
		return Resource{}, ErrForbidden
	}
	return s.decorateResource(ctx, p, b, binding)
}
func (s *Service) List(ctx context.Context, p Principal) ([]Resource, error) {
	if p.UserID == 0 || p.WorkspaceID == 0 {
		return nil, ErrForbidden
	}
	if s.access == nil {
		return nil, ErrUnavailable
	}
	items, e := s.store.List(ctx, p.WorkspaceID)
	if e != nil {
		return nil, e
	}
	if len(items) == 0 {
		return []Resource{}, nil
	}
	nativeIDs := make([]string, 0, len(items))
	for _, item := range items {
		nativeIDs = append(nativeIDs, item.ID)
	}
	bindings, e := s.access.FilterBitableResources(ctx, p, nativeIDs, documentlibrary.OperationView)
	if e != nil {
		return nil, e
	}
	authorizedBindings := make([]ResourceBinding, 0, len(bindings))
	for _, item := range items {
		if binding, allowed := bindings[item.ID]; allowed {
			authorizedBindings = append(authorizedBindings, binding)
		}
	}
	if len(authorizedBindings) == 0 {
		return []Resource{}, nil
	}
	decisions, e := s.access.BatchAuthorizeBitableResources(ctx, p, authorizedBindings, allBitableOperations())
	if e != nil {
		return nil, e
	}
	result := make([]Resource, 0, len(bindings))
	for i := range items {
		binding, allowed := bindings[items[i].ID]
		if !allowed {
			continue
		}
		items[i] = projectResource(items[i], binding)
		items[i].AllowedActions = allowedActionsFromDecisions(decisions[binding.ResourceID])
		if m, ok := s.store.(ResourceManager); ok {
			pref, err := m.ResourcePreference(ctx, p, binding.NativeResourceID)
			if err != nil {
				return nil, err
			}
			items[i].Pinned = pref.Pinned
			items[i].Favorited = pref.Favorited
			items[i].PreferenceRevision = pref.Revision
		}
		items[i], e = s.projectDirectoryLocation(ctx, p, items[i], binding)
		if e != nil {
			return nil, e
		}
		result = append(result, items[i])
	}
	return result, nil
}

func (s *Service) allowedActions(ctx context.Context, p Principal, binding ResourceBinding) []string {
	result := []string{}
	for _, candidate := range bitableActionOperations {
		if s.access.AuthorizeBitableResource(ctx, p, binding, candidate.operation) == nil {
			for _, action := range candidate.actions {
				result = append(result, string(action))
			}
		}
	}
	sort.Strings(result)
	return result
}
func (s *Service) Create(ctx context.Context, p Principal, name, key string) (Resource, error) {
	if s.access == nil || s.creation == nil {
		return Resource{}, ErrUnavailable
	}
	if e := s.access.RequireBitableCreate(ctx, p); e != nil {
		return Resource{}, e
	}
	id, e := uuid.Parse(key)
	name = strings.TrimSpace(name)
	if e != nil || id == uuid.Nil || id.String() != key || name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 200 {
		return Resource{}, ErrInvalidInput
	}
	resource, binding, e := s.creation.CreatePrivateBitable(ctx, p, name, key)
	if e != nil {
		return Resource{}, e
	}
	if e := validateResourceBinding(binding, resource.ID); e != nil {
		return Resource{}, e
	}
	return projectResource(resource, binding), nil
}
func (s *Service) scope(ctx context.Context, p Principal, ref, table string, operation documentlibrary.Operation) (Scope, ResourceBinding, error) {
	binding, e := s.requireResource(ctx, p, ref, operation)
	if e != nil {
		return Scope{}, ResourceBinding{}, e
	}
	if _, e := uuid.Parse(binding.NativeResourceID); e != nil || !validID(table) {
		return Scope{}, ResourceBinding{}, ErrInvalidInput
	}
	b, e := s.store.Get(ctx, p.WorkspaceID, binding.NativeResourceID)
	if e != nil {
		return Scope{}, ResourceBinding{}, e
	}
	if b.WorkspaceID != p.WorkspaceID {
		return Scope{}, ResourceBinding{}, ErrForbidden
	}
	for _, t := range b.Tables {
		if t.ID == table {
			return Scope{Principal: p, ResourceID: binding.NativeResourceID, DocumentResourceID: string(binding.ResourceID), PolicyRevision: binding.PolicyRevision, TableID: table}, binding, nil
		}
	}
	return Scope{}, ResourceBinding{}, ErrNotFound
}
func (s *Service) Bootstrap(ctx context.Context, p Principal, ref, table string) (Bootstrap, error) {
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return Bootstrap{}, e
	}
	v, e := s.provider.Bootstrap(ctx, scope)
	if e != nil {
		return Bootstrap{}, e
	}
	resource, e := s.Get(ctx, p, ref)
	if e != nil {
		return Bootstrap{}, e
	}
	v.Resource = resource
	v.TableID = table
	allowed := func(operation documentlibrary.Operation) bool {
		return s.access.AuthorizeBitableResource(ctx, p, binding, operation) == nil
	}
	write := allowed(documentlibrary.OperationEdit)
	structure := allowed(documentlibrary.OperationStructure)
	v.Capabilities = Capabilities{CanRead: true, CanCreateRecords: write, CanUpdateRecords: write, CanDeleteRecords: write, CanManageFields: structure, CanManageViews: structure, CanImport: write, CanExport: allowed(documentlibrary.OperationExport), CanUploadAttachments: allowed(documentlibrary.OperationAssetWrite), WritableFieldIDs: []string{}}
	v.Capabilities.CanComment = allowed(documentlibrary.OperationComment)
	if write {
		for _, f := range v.Fields {
			if !f.ReadOnly && !AuditField(f.Type) && (f.Type != FieldAttachment || v.Capabilities.CanUploadAttachments) {
				v.Capabilities.WritableFieldIDs = append(v.Capabilities.WritableFieldIDs, f.ID)
			}
		}
	}
	return v, nil
}
func (s *Service) Query(ctx context.Context, p Principal, ref, table string, q Query) (RecordPage, error) {
	if e := ValidateQuery(q); e != nil {
		return RecordPage{}, e
	}
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return RecordPage{}, e
	}
	v, e := s.provider.Query(ctx, scope, q)
	if e != nil {
		return RecordPage{}, e
	}
	if e = s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationView); e != nil {
		return RecordPage{}, e
	}
	return v, nil
}
func (s *Service) Search(ctx context.Context, p Principal, ref, table string, q Search) (SearchPage, error) {
	if e := ValidateSearch(q); e != nil {
		return SearchPage{}, e
	}
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return SearchPage{}, e
	}
	v, e := s.provider.Search(ctx, scope, q)
	if e != nil {
		return SearchPage{}, e
	}
	if e = s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationView); e != nil {
		return SearchPage{}, e
	}
	return v, nil
}
func (s *Service) Execute(ctx context.Context, p Principal, ref, table string, b CommandBatch, importing bool) (Operation, error) {
	if e := ValidateBatch(b); e != nil {
		return Operation{}, e
	}
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationEdit)
	if e != nil {
		return Operation{}, e
	}
	// The native transaction checks its receipt first, then locks the schema and
	// calls the shared field validators and people normalizer. A preflight schema
	// check here would incorrectly reject a valid replay after a schema change.
	b.Importing = importing
	return s.provider.Execute(ctx, scope, b)
}
func (s *Service) Operation(ctx context.Context, p Principal, ref, table, id string) (Operation, error) {
	if _, e := uuid.Parse(id); e != nil {
		return Operation{}, ErrInvalidInput
	}
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return Operation{}, e
	}
	result, e := s.provider.Operation(ctx, scope, id)
	if e != nil {
		return Operation{}, e
	}
	if len(result.RequiredActions) == 0 {
		return Operation{}, ErrForbidden
	}
	for _, action := range result.RequiredActions {
		operation, ok := operationForLegacyBitableAction(action)
		if !ok {
			return Operation{}, ErrForbidden
		}
		if e := s.access.AuthorizeBitableResource(ctx, p, binding, operation); e != nil {
			return Operation{}, e
		}
	}
	if e := s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationView); e != nil {
		return Operation{}, e
	}
	return result, nil
}
func (s *Service) Export(ctx context.Context, p Principal, ref, table string, q ExportQuery) (ExportSnapshot, error) {
	if e := ValidateQuery(q.Query); e != nil {
		return ExportSnapshot{}, e
	}
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationExport)
	if e != nil {
		return ExportSnapshot{}, e
	}
	v, e := s.provider.Export(ctx, scope, q)
	if e != nil {
		return ExportSnapshot{}, e
	}
	if e = s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationExport); e != nil {
		return ExportSnapshot{}, e
	}
	return v, nil
}
