package resourceaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type linkStoreStub struct {
	links          []ShareLink
	createMutation LinkMutation
	foundWorkspace uint
	foundLink      ShareLink
	contextCreated bool
	expectedHash   string
}

func (stub *linkStoreStub) CreateShareLink(_ context.Context, mutation LinkMutation) (ShareLink, error) {
	stub.createMutation = mutation
	link := ShareLink{
		ID: mutation.LinkID, ResourceID: mutation.ResourceID, Level: mutation.Level, Version: 1,
		ExpiresAt: mutation.ExpiresAt, CreatedBy: mutation.Principal.UserID, CreatedAt: mutation.Now, UpdatedAt: mutation.Now,
	}
	stub.links = append(stub.links, link)
	return link, nil
}
func (stub *linkStoreStub) ListShareLinks(context.Context, uint, documentlibrary.ResourceID) ([]ShareLink, error) {
	return stub.links, nil
}
func (stub *linkStoreStub) RevokeShareLink(context.Context, LinkMutation) error { return nil }
func (stub *linkStoreStub) RotateShareLink(_ context.Context, mutation LinkMutation) (ShareLink, error) {
	return ShareLink{ID: mutation.LinkID, ResourceID: mutation.ResourceID, Level: LevelViewer, Version: 2}, nil
}
func (stub *linkStoreStub) FindShareLinkBySecretHash(context.Context, string) (uint, ShareLink, error) {
	return stub.foundWorkspace, stub.foundLink, nil
}
func (stub *linkStoreStub) CreateLinkAccessContext(_ context.Context, _ Principal, _ ShareLink, expectedHash, _ string, _, _ time.Time) error {
	stub.contextCreated, stub.expectedHash = true, expectedHash
	return nil
}

type limiterStub struct{ allowed bool }

func (stub limiterStub) AllowDocumentShareLinkExchange(context.Context, Principal) (bool, error) {
	return stub.allowed, nil
}

func TestLinkOperationsUseFineGrainedGlobalActionsAndCreateReceiptInput(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	id := documentlibrary.ResourceID("dr_links")
	resolver := &resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "7"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{
		documentlibrary.OperationViewLinks: true, documentlibrary.OperationManageLinks: true,
	}}
	policyStore := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModeLink, Rules: []Rule{}, Revision: 1},
	}}
	policies, _ := NewPolicyService(resolver, directory, authorizer, nil, policyStore, fixedClock{now: now})
	links := &linkStoreStub{}
	service, _ := NewLinkService(policies, links, limiterStub{allowed: true})
	principal := Principal{WorkspaceID: 1, UserID: 7}

	if _, err := service.List(context.Background(), principal, id); err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), principal, id, CreateShareLinkInput{
		Level: LevelEditor, Duration: LinkDuration7Days, IdempotencyKey: "create-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Secret == "" || links.createMutation.SecretHash == "" || links.createMutation.IdempotencyKey != "create-key" ||
		links.createMutation.InputDigest == "" || links.createMutation.ExpiresAt == nil ||
		!links.createMutation.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("incomplete create mutation: %+v", links.createMutation)
	}
	if links.createMutation.SecretHash == created.Secret {
		t.Fatal("store received plaintext secret")
	}
	if len(authorizer.ops) < 2 || authorizer.ops[0] != documentlibrary.OperationViewLinks ||
		authorizer.ops[1] != documentlibrary.OperationManageLinks {
		t.Fatalf("wrong link operations: %+v", authorizer.ops)
	}
}

func TestExchangeRequiresExchangeAndContentViewActions(t *testing.T) {
	now := time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)
	id := documentlibrary.ResourceID("dr_exchange")
	secret := "public-share-secret"
	resolver := &resolverStub{resource: documentlibrary.Resource{ID: id, WorkspaceID: 1, OwnerUserID: 7, TypeKey: documentlibrary.TypeRichText}}
	directory := &directoryStub{matches: map[identity.SubjectRef]bool{{Kind: identity.SubjectUser, ID: "9"}: true}}
	authorizer := &authorizerStub{allowed: map[documentlibrary.Operation]bool{
		documentlibrary.OperationExchangeLink: true, documentlibrary.OperationView: true,
	}}
	policyStore := &policyStoreStub{policies: map[documentlibrary.ResourceID]Policy{
		id: {WorkspaceID: 1, ResourceID: id, Mode: ModeLink, Rules: []Rule{}, Revision: 1},
	}}
	policies, _ := NewPolicyService(resolver, directory, authorizer, nil, policyStore, fixedClock{now: now})
	links := &linkStoreStub{
		foundWorkspace: 1,
		foundLink:      ShareLink{ID: "lnk_exchange", ResourceID: id, Level: LevelViewer, Version: 4, ExpiresAt: linkTimePointer(now.Add(time.Hour))},
	}
	service, _ := NewLinkService(policies, links, limiterStub{allowed: true})

	result, err := service.Exchange(context.Background(), Principal{WorkspaceID: 1, UserID: 9}, secret)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessSecret == "" || !links.contextCreated || !result.ExpiresAt.Equal(*links.foundLink.ExpiresAt) {
		t.Fatalf("unexpected exchange result: %+v", result)
	}
	wantHash := sha256.Sum256([]byte(secret))
	if links.expectedHash != hex.EncodeToString(wantHash[:]) {
		t.Fatal("context creation was not bound to current link hash")
	}
	if len(authorizer.ops) != 2 || authorizer.ops[0] != documentlibrary.OperationExchangeLink ||
		authorizer.ops[1] != documentlibrary.OperationView {
		t.Fatalf("exchange did not intersect both global actions: %+v", authorizer.ops)
	}

	authorizer.allowed[documentlibrary.OperationView] = false
	links.contextCreated = false
	if _, err := service.Exchange(context.Background(), Principal{WorkspaceID: 1, UserID: 9}, secret); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("missing content view action must fail without disclosure, got %v", err)
	}
	if links.contextCreated {
		t.Fatal("denied exchange must not create access context")
	}
}

func linkTimePointer(value time.Time) *time.Time { return &value }
