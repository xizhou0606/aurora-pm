package resourceaccess

import (
	"context"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

// ToolMutationStore reuses resource-access receipts and binds the existing
// domain services to the transaction that persists the receipt.
type ToolMutationStore interface {
	ApplyToolMutation(context.Context, Principal, documentlibrary.ResourceID, string, string, func(PolicyStore, RequestStore, LinkStore) (json.RawMessage, error)) (json.RawMessage, error)
}

type ToolMutationService struct {
	Policies *PolicyService
	Requests *RequestService
	Links    *LinkService
}

func (s *ToolMutationService) Apply(ctx context.Context, p Principal, id documentlibrary.ResourceID, key, digest string, apply func(*ToolMutationService) (json.RawMessage, error)) (json.RawMessage, error) {
	if s == nil || s.Policies == nil || s.Requests == nil || s.Links == nil {
		return nil, ErrUnavailable
	}
	if !validPrincipal(p) || id == "" || key == "" || len(key) > 160 || len(digest) != 64 || apply == nil {
		return nil, ErrInvalidInput
	}
	store, ok := s.Policies.store.(ToolMutationStore)
	if !ok {
		return nil, ErrUnavailable
	}
	return store.ApplyToolMutation(ctx, p, id, key, digest, func(policies PolicyStore, requests RequestStore, links LinkStore) (json.RawMessage, error) {
		policyService, requestService, linkService := *s.Policies, *s.Requests, *s.Links
		policyService.store = policies
		requestService.policies, requestService.store = &policyService, requests
		linkService.policies, linkService.store = &policyService, links
		return apply(&ToolMutationService{Policies: &policyService, Requests: &requestService, Links: &linkService})
	})
}
