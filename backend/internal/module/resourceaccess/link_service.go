package resourceaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type LinkService struct {
	policies *PolicyService
	store    LinkStore
	limiter  LinkExchangeLimiter
}

func NewLinkService(policies *PolicyService, store LinkStore, limiter LinkExchangeLimiter) (*LinkService, error) {
	if policies == nil || store == nil || limiter == nil {
		return nil, ErrUnavailable
	}
	return &LinkService{policies: policies, store: store, limiter: limiter}, nil
}

func (service *LinkService) Create(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID, input CreateShareLinkInput,
) (CreatedShareLink, error) {
	resource, policy, err := service.requireLinkOperation(
		ctx, principal, resourceID, documentlibrary.OperationManageLinks,
	)
	if err != nil {
		return CreatedShareLink{}, err
	}
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !policy.Mode.AllowsLinks() || !input.Level.Valid() || !input.Duration.Valid() ||
		input.IdempotencyKey == "" || len(input.IdempotencyKey) > 160 {
		return CreatedShareLink{}, ErrInvalidInput
	}
	digest, err := linkInputDigest(input)
	if err != nil {
		return CreatedShareLink{}, err
	}
	secret, hash, err := newOpaqueSecret()
	if err != nil {
		return CreatedShareLink{}, ErrUnavailable
	}
	linkID, err := newOpaqueID("lnk_")
	if err != nil {
		return CreatedShareLink{}, ErrUnavailable
	}
	now := service.policies.clock.Now().UTC()
	link, err := service.store.CreateShareLink(ctx, LinkMutation{
		Principal: principal, ResourceID: resource.ID, LinkID: linkID,
		SecretHash: hash, Level: input.Level, ExpiresAt: GrantExpiry(now, input.Duration), Now: now,
		IdempotencyKey: input.IdempotencyKey, InputDigest: digest,
	})
	if err != nil {
		return CreatedShareLink{}, normalizeStoreError(err)
	}
	// Updating rules retains the original secret, which cannot be recovered from its hash.
	if link.ID != linkID {
		secret = ""
	}
	return CreatedShareLink{ShareLink: link, Secret: secret}, nil
}

func (service *LinkService) List(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID,
) ([]ShareLink, error) {
	resource, _, err := service.requireLinkOperation(ctx, principal, resourceID, documentlibrary.OperationViewLinks)
	if err != nil {
		return nil, err
	}
	links, err := service.store.ListShareLinks(ctx, principal.WorkspaceID, resource.ID)
	if err != nil {
		return nil, normalizeStoreError(err)
	}
	if links == nil {
		links = []ShareLink{}
	}
	return links, nil
}

func (service *LinkService) Revoke(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID, linkID string,
) error {
	resource, _, err := service.requireLinkOperation(ctx, principal, resourceID, documentlibrary.OperationManageLinks)
	if err != nil {
		return err
	}
	if strings.TrimSpace(linkID) == "" {
		return ErrInvalidInput
	}
	err = service.store.RevokeShareLink(ctx, LinkMutation{
		Principal: principal, ResourceID: resource.ID, LinkID: linkID, Now: service.policies.clock.Now().UTC(),
	})
	return normalizeStoreError(err)
}

func (service *LinkService) Rotate(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID, linkID string,
) (CreatedShareLink, error) {
	resource, policy, err := service.requireLinkOperation(
		ctx, principal, resourceID, documentlibrary.OperationManageLinks,
	)
	if err != nil {
		return CreatedShareLink{}, err
	}
	if !policy.Mode.AllowsLinks() || strings.TrimSpace(linkID) == "" {
		return CreatedShareLink{}, ErrInvalidInput
	}
	secret, hash, err := newOpaqueSecret()
	if err != nil {
		return CreatedShareLink{}, ErrUnavailable
	}
	link, err := service.store.RotateShareLink(ctx, LinkMutation{
		Principal: principal, ResourceID: resource.ID, LinkID: linkID,
		SecretHash: hash, Now: service.policies.clock.Now().UTC(),
	})
	if err != nil {
		return CreatedShareLink{}, normalizeStoreError(err)
	}
	return CreatedShareLink{ShareLink: link, Secret: secret}, nil
}

func (service *LinkService) Exchange(
	ctx context.Context, principal Principal, secret string,
) (ExchangeResult, error) {
	if !validPrincipal(principal) || strings.TrimSpace(secret) == "" {
		return ExchangeResult{}, ErrLinkInvalid
	}
	allowed, err := service.limiter.AllowDocumentShareLinkExchange(ctx, principal)
	if err != nil {
		return ExchangeResult{}, ErrUnavailable
	}
	if !allowed {
		return ExchangeResult{}, ErrRateLimited
	}
	self := identity.SubjectRef{Kind: identity.SubjectUser, ID: uintString(principal.UserID)}
	membership, err := service.policies.subjects.MatchSubjects(ctx, identity.SubjectMatchRequest{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, Subjects: []identity.SubjectRef{self},
	})
	if err != nil || len(membership.Matches) != 1 || !membership.Matches[0].Matched {
		return ExchangeResult{}, ErrLinkInvalid
	}
	digest := sha256.Sum256([]byte(secret))
	workspaceID, link, err := service.store.FindShareLinkBySecretHash(ctx, hex.EncodeToString(digest[:]))
	now := service.policies.clock.Now().UTC()
	if err != nil || workspaceID != principal.WorkspaceID || !validLiveLink(link, now) {
		return ExchangeResult{}, ErrLinkInvalid
	}
	resource, _, err := service.policies.resolver.RequireTypeOperation(
		ctx, principal.WorkspaceID, link.ResourceID, documentlibrary.OperationExchangeLink,
	)
	if err != nil {
		return ExchangeResult{}, ErrLinkInvalid
	}
	if _, _, err := service.policies.resolver.RequireTypeOperation(
		ctx, principal.WorkspaceID, link.ResourceID, documentlibrary.OperationView,
	); err != nil {
		return ExchangeResult{}, ErrLinkInvalid
	}
	policy, err := service.policies.store.GetPolicy(ctx, principal.WorkspaceID, link.ResourceID)
	if err != nil || !policy.Mode.AllowsLinks() {
		return ExchangeResult{}, ErrLinkInvalid
	}
	exchangeGlobal, err := service.policies.authorizer.AuthorizeDocumentResourceOperation(
		ctx, principal, resource, documentlibrary.OperationExchangeLink,
	)
	if err != nil || !exchangeGlobal {
		return ExchangeResult{}, ErrLinkInvalid
	}
	viewGlobal, err := service.policies.authorizer.AuthorizeDocumentResourceOperation(
		ctx, principal, resource, documentlibrary.OperationView,
	)
	if err != nil || !viewGlobal {
		return ExchangeResult{}, ErrLinkInvalid
	}
	accessSecret, accessHash, err := newOpaqueSecret()
	if err != nil {
		return ExchangeResult{}, ErrUnavailable
	}
	expiresAt := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	if link.ExpiresAt != nil {
		expiresAt = *link.ExpiresAt
	}
	if err := service.store.CreateLinkAccessContext(
		ctx, principal, link, hex.EncodeToString(digest[:]), accessHash, now, expiresAt,
	); err != nil {
		return ExchangeResult{}, normalizeStoreError(err)
	}
	return ExchangeResult{ResourceID: link.ResourceID, AccessSecret: accessSecret, Level: link.Level, ExpiresAt: expiresAt}, nil
}

func GrantExpiry(now time.Time, duration LinkDuration) *time.Time {
	var value time.Time
	switch duration {
	case LinkDuration24Hours:
		value = now.Add(24 * time.Hour)
	case LinkDuration7Days:
		value = now.Add(7 * 24 * time.Hour)
	default:
		return nil
	}
	return &value
}

func validLiveLink(link ShareLink, now time.Time) bool {
	return link.ID != "" && link.Level.Valid() && link.RevokedAt == nil &&
		(link.ExpiresAt == nil || link.ExpiresAt.After(now))
}

func (service *LinkService) requireLinkOperation(
	ctx context.Context,
	principal Principal,
	resourceID documentlibrary.ResourceID,
	operation documentlibrary.Operation,
) (documentlibrary.Resource, Policy, error) {
	resource, err := service.policies.requireOwnerOperation(ctx, principal, resourceID, operation)
	if err != nil {
		return documentlibrary.Resource{}, Policy{}, err
	}
	policy, err := service.policies.store.GetPolicy(ctx, principal.WorkspaceID, resource.ID)
	if err != nil {
		return documentlibrary.Resource{}, Policy{}, normalizeStoreError(err)
	}
	return resource, policy, nil
}

func linkInputDigest(input CreateShareLinkInput) (string, error) {
	encoded, err := json.Marshal(struct {
		Level    Level
		Duration LinkDuration
	}{Level: input.Level, Duration: input.Duration})
	if err != nil {
		return "", ErrInvalidInput
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
