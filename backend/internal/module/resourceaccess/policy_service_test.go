package resourceaccess

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type resolverStub struct {
	resource documentlibrary.Resource
	ops      []documentlibrary.Operation
	err      error
}

func (stub *resolverStub) ResolveResource(context.Context, uint, documentlibrary.ResourceID) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	return stub.resource, documentlibrary.ContentBinding{}, stub.err
}
func (stub *resolverStub) ResolveContentResource(context.Context, uint, documentlibrary.TypeKey, string) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	return stub.resource, documentlibrary.ContentBinding{}, stub.err
}
func (stub *resolverStub) ResolveContentResources(context.Context, uint, documentlibrary.TypeKey, []string) (map[string]documentlibrary.Resource, error) {
	return map[string]documentlibrary.Resource{}, stub.err
}
func (stub *resolverStub) RequireTypeOperation(_ context.Context, _ uint, _ documentlibrary.ResourceID, operation documentlibrary.Operation) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	stub.ops = append(stub.ops, operation)
	return stub.resource, documentlibrary.ContentBinding{}, stub.err
}

type directoryStub struct {
	matches    map[identity.SubjectRef]bool
	resolveErr error
	matchErr   error
}

func (stub *directoryStub) SearchSubjects(context.Context, uint, identity.SubjectSearch) (identity.SubjectPage, error) {
	return identity.SubjectPage{}, nil
}
func (stub *directoryStub) ResolveSubjects(_ context.Context, _ uint, refs []identity.SubjectRef) ([]identity.SubjectCandidate, error) {
	if stub.resolveErr != nil {
		return nil, stub.resolveErr
	}
	result := make([]identity.SubjectCandidate, 0, len(refs))
	for _, ref := range refs {
		result = append(result, identity.SubjectCandidate{Kind: ref.Kind, ID: ref.ID, DisplayName: ref.ID})
	}
	return result, nil
}
func (stub *directoryStub) MatchSubjects(_ context.Context, request identity.SubjectMatchRequest) (identity.SubjectMatchResult, error) {
	if stub.matchErr != nil {
		return identity.SubjectMatchResult{}, stub.matchErr
	}
	result := identity.SubjectMatchResult{Matches: make([]identity.SubjectMatch, 0, len(request.Subjects))}
	for _, ref := range request.Subjects {
		result.Matches = append(result.Matches, identity.SubjectMatch{Subject: ref, Matched: stub.matches[ref]})
	}
	return result, nil
}

type capableDirectoryStub struct{ *directoryStub }

func (stub capableDirectoryStub) SubjectCapabilities() map[identity.SubjectKind]identity.SubjectKindCapability {
	return map[identity.SubjectKind]identity.SubjectKindCapability{
		identity.SubjectUser: {Available: true}, identity.SubjectProjectSet: {Available: true},
	}
}

type authorizerStub struct {
	allowed map[documentlibrary.Operation]bool
	ops     []documentlibrary.Operation
}

func (stub *authorizerStub) AuthorizeDocumentResourceOperation(_ context.Context, _ Principal, _ documentlibrary.Resource, operation documentlibrary.Operation) (bool, error) {
	stub.ops = append(stub.ops, operation)
	return stub.allowed[operation], nil
}

type policyStoreStub struct {
	policies   map[documentlibrary.ResourceID]Policy
	replaced   PolicyMutation
	linkLevels map[documentlibrary.ResourceID]Level
}

func (stub *policyStoreStub) GetPolicy(_ context.Context, _ uint, id documentlibrary.ResourceID) (Policy, error) {
	policy, ok := stub.policies[id]
	if !ok {
		return Policy{}, ErrNotFound
	}
	return policy, nil
}
func (stub *policyStoreStub) ListPolicies(_ context.Context, _ uint, ids []documentlibrary.ResourceID) (map[documentlibrary.ResourceID]Policy, error) {
	result := make(map[documentlibrary.ResourceID]Policy)
	for _, id := range ids {
		if policy, ok := stub.policies[id]; ok {
			result[id] = policy
		}
	}
	return result, nil
}
func (stub *policyStoreStub) ReplacePolicy(_ context.Context, mutation PolicyMutation) (Policy, error) {
	stub.replaced = mutation
	policy := stub.policies[mutation.ResourceID]
	policy.Mode, policy.WorkspaceLevel, policy.Rules = mutation.Mode, mutation.WorkspaceLevel, mutation.Rules
	policy.Revision++
	stub.policies[mutation.ResourceID] = policy
	return policy, nil
}
func (stub *policyStoreStub) ResolveLinkAccess(_ context.Context, _ Principal, _ []documentlibrary.ResourceID, _ string, now time.Time) (map[documentlibrary.ResourceID]LinkAccessDecision, error) {
	if stub.linkLevels == nil {
		return map[documentlibrary.ResourceID]LinkAccessDecision{}, nil
	}
	result := make(map[documentlibrary.ResourceID]LinkAccessDecision, len(stub.linkLevels))
	for id, level := range stub.linkLevels {
		result[id] = LinkAccessDecision{Level: level, ExpiresAt: now.Add(time.Minute)}
	}
	return result, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func TestGetPolicyAllowsViewerAndProjectsSubjectCapabilities(t *testing.T) {
	id := documentlibrary.ResourceID("dr_view")
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 99, TypeKey: documentlibrary.TypeRichText}
	level := LevelViewer
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModeWorkspace, WorkspaceLevel: &level, Rules: []Rule{}, Revision: 3},
	}}
	directory := capableDirectoryStub{&directoryStub{matches: map[identity.SubjectRef]bool{
		{Kind: identity.SubjectUser, ID: "7"}: true,
	}}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{
		documentlibrary.OperationViewAccess: true,
	}}
	service, err := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}

	view, err := service.GetPolicy(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id)
	if err != nil {
		t.Fatal(err)
	}
	if view.CanManageAccess || view.EffectiveLevel == nil || *view.EffectiveLevel != LevelViewer {
		t.Fatalf("unexpected projection: %+v", view)
	}
	if !view.SubjectCapabilities[identity.SubjectUser].Available || !view.SubjectCapabilities[identity.SubjectProjectSet].Available ||
		view.SubjectCapabilities[identity.SubjectOrganizationUnit].Available {
		t.Fatalf("capabilities must come from identity provider: %+v", view.SubjectCapabilities)
	}
}

func TestSubjectCapabilitiesFailClosedWithoutProvider(t *testing.T) {
	capabilities := SubjectCapabilities(&directoryStub{})
	for _, kind := range []identity.SubjectKind{
		identity.SubjectUser, identity.SubjectRole, identity.SubjectUserGroup,
		identity.SubjectOrganizationUnit, identity.SubjectProjectSet,
	} {
		if capabilities[kind].Available {
			t.Fatalf("missing provider must make %s unavailable", kind)
		}
	}
}

func TestOwnerCannotBypassGlobalActions(t *testing.T) {
	id := documentlibrary.ResourceID("dr_owner")
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModePrivate, Rules: []Rule{}, Revision: 1},
	}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{}}
	service, _ := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})

	if _, err := service.GetPolicy(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner without global view_access must fail closed, got %v", err)
	}
	if _, err := service.ReplacePolicy(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id, ReplacePolicyInput{
		Mode: ModePrivate, ExpectedPolicyRevision: 1, IdempotencyKey: "request-1",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner without global manage_access must fail closed, got %v", err)
	}
}

func TestManageLinksDoesNotRequireViewAccessAction(t *testing.T) {
	id := documentlibrary.ResourceID("dr_manage_links")
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModePrivate, Rules: []Rule{}, Revision: 1},
	}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{
		documentlibrary.OperationManageLinks: true,
	}}
	service, _ := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})
	principal := Principal{WorkspaceID: 1, UserID: 7}

	if err := service.AuthorizeOperation(context.Background(), principal, id, documentlibrary.OperationManageLinks); err != nil {
		t.Fatalf("manage_links must not imply view_access: %v", err)
	}
	if err := service.AuthorizeOperation(context.Background(), principal, id, documentlibrary.OperationViewAccess); !errors.Is(err, ErrNotFound) {
		t.Fatalf("view_access must remain independently denied, got %v", err)
	}
}

func TestReplacePolicyRejectsUnavailableOrganizationUnit(t *testing.T) {
	id := documentlibrary.ResourceID("dr_org")
	resource := documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModePrivate, Rules: []Rule{}, Revision: 1},
	}}
	directory := &directoryStub{
		resolveErr: identity.ErrSubjectKindUnavailable,
		matches:    map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true},
	}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{documentlibrary.OperationManageAccess: true}}
	service, _ := NewPolicyService(&resolverStub{resource: resource}, directory, authorizer, nil, store, fixedClock{})

	_, err := service.ReplacePolicy(context.Background(), Principal{WorkspaceID: 1, UserID: 7}, id, ReplacePolicyInput{
		Mode: ModeRestricted, Rules: []Rule{{
			SubjectKind: identity.SubjectOrganizationUnit, SubjectID: "ou_1", Level: LevelViewer, IncludeDescendants: true,
		}}, ExpectedPolicyRevision: 1, IdempotencyKey: "request-org",
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("organization provider absence must fail closed, got %v", err)
	}
}

func TestObjectOperationMatrix(t *testing.T) {
	viewer, editor := LevelViewer, LevelEditor
	tests := []struct {
		operation documentlibrary.Operation
		level     *Level
		owner     bool
		want      bool
	}{
		{documentlibrary.OperationView, &viewer, false, true},
		{documentlibrary.OperationViewAccess, &viewer, false, true},
		{documentlibrary.OperationExport, &viewer, false, true},
		{documentlibrary.OperationAssetRead, &viewer, false, true},
		{documentlibrary.OperationCopy, &viewer, false, true},
		{documentlibrary.OperationCollaborate, &viewer, false, true},
		{documentlibrary.OperationEdit, &viewer, false, false},
		{documentlibrary.OperationEdit, &editor, false, true},
		{documentlibrary.OperationAssetWrite, &editor, false, true},
		{documentlibrary.OperationComment, &editor, false, true},
		{documentlibrary.OperationStructure, &editor, false, true},
		{documentlibrary.OperationCellLease, &editor, false, true},
		{documentlibrary.OperationManageAccess, &editor, false, false},
		{documentlibrary.OperationViewLinks, &editor, false, false},
		{documentlibrary.OperationManageLinks, &editor, true, true},
		{documentlibrary.OperationMove, &editor, true, true},
		{documentlibrary.OperationDelete, &editor, true, true},
		{documentlibrary.OperationExchangeLink, &editor, true, false},
	}
	for _, test := range tests {
		if got := objectLevelAllows(test.level, test.owner, test.operation); got != test.want {
			t.Errorf("operation %s level %v owner %v: got %v want %v", test.operation, *test.level, test.owner, got, test.want)
		}
	}
}

func TestBatchEvaluateFourModesAndHighestDynamicSubjectLevel(t *testing.T) {
	ids := []documentlibrary.ResourceID{"dr_private", "dr_restricted", "dr_workspace", "dr_link"}
	resource := documentlibrary.Resource{WorkspaceID: 1, OwnerUserID: 99, TypeKey: documentlibrary.TypeRichText}
	multiResolver := &multiResolverStub{base: resource}
	viewer, editor := LevelViewer, LevelEditor
	role := identity.SubjectRef{Kind: identity.SubjectRole, ID: "2"}
	group := identity.SubjectRef{Kind: identity.SubjectUserGroup, ID: "3"}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{
		{Kind: identity.SubjectUser, ID: "7"}: true, role: true, group: true,
	}}
	store := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		ids[0]: {WorkspaceID: 1, ResourceID: ids[0], Mode: ModePrivate, Rules: []Rule{}, Revision: 1},
		ids[1]: {WorkspaceID: 1, ResourceID: ids[1], Mode: ModeRestricted, Rules: []Rule{
			{SubjectKind: role.Kind, SubjectID: role.ID, Level: viewer},
			{SubjectKind: group.Kind, SubjectID: group.ID, Level: editor},
		}, Revision: 2},
		ids[2]: {WorkspaceID: 1, ResourceID: ids[2], Mode: ModeWorkspace, WorkspaceLevel: &viewer, Rules: []Rule{}, Revision: 3},
		ids[3]: {WorkspaceID: 1, ResourceID: ids[3], Mode: ModeLink, Rules: []Rule{}, Revision: 4},
	}, linkLevels: map[documentlibrary.ResourceID]Level{ids[3]: LevelViewer}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{
		documentlibrary.OperationView: true, documentlibrary.OperationEdit: true,
	}}
	service, _ := NewPolicyService(multiResolver, directory, authorizer, nil, store, fixedClock{})
	requests := []EvaluationRequest{
		{ResourceID: ids[0], Operation: documentlibrary.OperationView},
		{ResourceID: ids[1], Operation: documentlibrary.OperationEdit},
		{ResourceID: ids[2], Operation: documentlibrary.OperationEdit},
		{ResourceID: ids[3], Operation: documentlibrary.OperationView},
	}
	results, err := service.BatchEvaluate(
		context.Background(), Principal{WorkspaceID: 1, UserID: 7, LinkAccessSecret: "context"}, requests,
	)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Allowed || !results[1].Allowed || results[2].Allowed || !results[3].Allowed {
		t.Fatalf("unexpected four-mode decisions: %+v", results)
	}
	if results[1].EffectiveLevel == nil || *results[1].EffectiveLevel != LevelEditor || len(results[1].Sources) != 2 {
		t.Fatalf("restricted mode must use highest dynamic source: %+v", results[1])
	}
	if results[3].ExpiresAt == nil {
		t.Fatalf("link expiry missing from evaluation: %+v", results[3])
	}
}

type multiResolverStub struct{ base documentlibrary.Resource }

func (stub *multiResolverStub) ResolveResource(_ context.Context, workspaceID uint, id documentlibrary.ResourceID) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	return stub.resource(workspaceID, id), documentlibrary.ContentBinding{}, nil
}
func (stub *multiResolverStub) ResolveContentResource(context.Context, uint, documentlibrary.TypeKey, string) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, nil
}
func (stub *multiResolverStub) ResolveContentResources(context.Context, uint, documentlibrary.TypeKey, []string) (map[string]documentlibrary.Resource, error) {
	return map[string]documentlibrary.Resource{}, nil
}
func (stub *multiResolverStub) RequireTypeOperation(_ context.Context, workspaceID uint, id documentlibrary.ResourceID, _ documentlibrary.Operation) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	return stub.resource(workspaceID, id), documentlibrary.ContentBinding{}, nil
}
func (stub *multiResolverStub) resource(workspaceID uint, id documentlibrary.ResourceID) documentlibrary.Resource {
	resource := stub.base
	resource.ID, resource.WorkspaceID = id, workspaceID
	return resource
}
