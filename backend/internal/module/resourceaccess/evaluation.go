package resourceaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func (service *PolicyService) Evaluate(
	ctx context.Context, principal Principal, request EvaluationRequest,
) (Evaluation, error) {
	results, err := service.BatchEvaluate(ctx, principal, []EvaluationRequest{request})
	if err != nil {
		return Evaluation{}, err
	}
	return results[0], nil
}

// BatchEvaluate is the single policy algorithm used by both point and batch
// gates. Subject membership is resolved once for the full resource batch.
func (service *PolicyService) BatchEvaluate(
	ctx context.Context, principal Principal, requests []EvaluationRequest,
) ([]Evaluation, error) {
	if !validPrincipal(principal) || len(requests) == 0 {
		return nil, ErrInvalidInput
	}
	resources := make(map[documentlibrary.ResourceID]documentlibrary.Resource, len(requests))
	ids := make([]documentlibrary.ResourceID, 0, len(requests))
	seenIDs := make(map[documentlibrary.ResourceID]struct{}, len(requests))
	for _, request := range requests {
		if strings.TrimSpace(string(request.ResourceID)) == "" || request.Operation == "" {
			return nil, ErrInvalidInput
		}
		resource, _, err := service.resolver.RequireTypeOperation(
			ctx, principal.WorkspaceID, request.ResourceID, request.Operation,
		)
		if err != nil {
			return nil, ErrNotFound
		}
		resources[request.ResourceID] = resource
		if _, exists := seenIDs[request.ResourceID]; !exists {
			seenIDs[request.ResourceID] = struct{}{}
			ids = append(ids, request.ResourceID)
		}
	}
	policies, err := service.store.ListPolicies(ctx, principal.WorkspaceID, ids)
	if err != nil {
		return nil, normalizeStoreError(err)
	}
	if len(policies) != len(ids) {
		return nil, ErrNotFound
	}
	if err := service.applyRequirementSources(ctx, principal.WorkspaceID, policies); err != nil {
		return nil, err
	}
	active, matches, err := service.subjectMatches(ctx, principal, policies)
	if err != nil {
		return nil, err
	}
	linkLevels, err := service.linkLevels(ctx, principal, requests)
	if err != nil {
		return nil, err
	}
	results := make([]Evaluation, 0, len(requests))
	approved := map[documentlibrary.ResourceID][]RequestGrant{}
	if service.approvedRequests != nil {
		approved, err = service.approvedRequests.ApprovedRequestResources(ctx, principal, ids)
		if err != nil {
			return nil, normalizeStoreError(err)
		}
	}
	for _, request := range requests {
		resource, policy := resources[request.ResourceID], policies[request.ResourceID]
		globalAllowed, err := service.authorizer.AuthorizeDocumentResourceOperation(ctx, principal, resource, request.Operation)
		if err != nil {
			return nil, ErrUnavailable
		}
		result := Evaluation{ResourceID: resource.ID, Operation: request.Operation, PolicyRevision: policy.Revision}
		if !active || !globalAllowed {
			results = append(results, result)
			continue
		}
		if policy.Mode == ModeLegacy {
			if ownerOnlyOperation(request.Operation) {
				if resource.OwnerUserID == principal.UserID {
					level := LevelEditor
					result.Allowed, result.EffectiveLevel = true, &level
					result.Source, result.Sources = "owner", []string{"owner"}
				}
				results = append(results, result)
				continue
			}
			if service.legacy == nil {
				return nil, ErrUnavailable
			}
			allowed, err := service.legacy.EvaluateLegacyDocumentResource(ctx, principal, resource, request.Operation)
			if err != nil {
				return nil, ErrUnavailable
			}
			result.Allowed, result.Source = allowed, "legacy"
			if allowed {
				result.Sources = []string{"legacy"}
			}
			applyRequestGrants(&result, approved[resource.ID], service.clock.Now().UTC())
			results = append(results, result)
			continue
		}
		linkAccess := linkLevels[resource.ID]
		level, sources := effectiveObjectLevel(principal.UserID, resource, policy, matches, linkAccess.Level)
		result.EffectiveLevel, result.Sources = level, sources
		if len(sources) > 0 {
			result.Source = sources[0]
		}
		result.Allowed = objectLevelAllows(level, containsSource(sources, "owner"), request.Operation)
		baseLevel, baseSources := effectiveObjectLevel(principal.UserID, resource, policy, matches, "")
		if result.Allowed && !objectLevelAllows(baseLevel, containsSource(baseSources, "owner"), request.Operation) && containsSource(sources, "link") && !linkAccess.ExpiresAt.IsZero() {
			expiresAt := linkAccess.ExpiresAt
			result.ExpiresAt = &expiresAt
		}
		applyRequestGrants(&result, approved[resource.ID], service.clock.Now().UTC())
		results = append(results, result)
	}
	return results, nil
}

// Each independent source is a union: only a source granting this operation may
// extend its lifetime. A permanent viewer must never prolong a timed editor.
func applyRequestGrants(result *Evaluation, grants []RequestGrant, now time.Time) {
	if ownerOnlyOperation(result.Operation) {
		return
	}
	for _, grant := range grants {
		if !grant.Active(now) {
			continue
		}
		if result.EffectiveLevel == nil || maxLevel(*result.EffectiveLevel, grant.Level) != *result.EffectiveLevel {
			level := grant.Level
			result.EffectiveLevel = &level
		}
		if !objectLevelAllows(&grant.Level, false, result.Operation) {
			continue
		}
		if !containsSource(result.Sources, "approved_request") {
			result.Sources = append(result.Sources, "approved_request")
		}
		if result.Source == "" || !result.Allowed {
			result.Source = "approved_request"
		}
		if !result.Allowed {
			result.ExpiresAt = grant.ExpiresAt
		} else if result.ExpiresAt != nil && (grant.ExpiresAt == nil || grant.ExpiresAt.After(*result.ExpiresAt)) {
			result.ExpiresAt = grant.ExpiresAt
		}
		result.Allowed = true
	}
}

func (service *PolicyService) subjectMatches(
	ctx context.Context, principal Principal, policies map[documentlibrary.ResourceID]Policy,
) (bool, map[identity.SubjectRef]bool, error) {
	self := identity.SubjectRef{Kind: identity.SubjectUser, ID: uintString(principal.UserID)}
	refs := []identity.SubjectRef{self}
	seen := map[identity.SubjectRef]struct{}{self: {}}
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			ref := rule.Subject()
			if _, exists := seen[ref]; !exists {
				seen[ref] = struct{}{}
				refs = append(refs, ref)
			}
		}
	}
	result, err := service.subjects.MatchSubjects(ctx, identity.SubjectMatchRequest{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, Subjects: refs,
	})
	if err != nil {
		if errors.Is(err, identity.ErrSubjectKindUnavailable) {
			return false, nil, ErrUnavailable
		}
		return false, nil, ErrUnavailable
	}
	matches := make(map[identity.SubjectRef]bool, len(result.Matches))
	for _, match := range result.Matches {
		matches[match.Subject] = match.Matched
	}
	return matches[self], matches, nil
}

func (service *PolicyService) linkLevels(
	ctx context.Context, principal Principal, requests []EvaluationRequest,
) (map[documentlibrary.ResourceID]LinkAccessDecision, error) {
	levels := make(map[documentlibrary.ResourceID]LinkAccessDecision)
	bySecret := make(map[string][]documentlibrary.ResourceID)
	for _, request := range requests {
		secret := request.LinkAccessSecret
		if secret == "" {
			secret = principal.LinkAccessSecret
		}
		// Account grants remain visible even while another share entry supplies
		// its scoped browser credential (e.g. the document directory request).
		bySecret[""] = append(bySecret[""], request.ResourceID)
		if secret != "" {
			bySecret[secret] = append(bySecret[secret], request.ResourceID)
		}
	}
	for secret, ids := range bySecret {
		hash := sha256.Sum256([]byte(secret))
		tokenHash := ""
		if secret != "" {
			tokenHash = hex.EncodeToString(hash[:])
		}
		resolved, err := service.store.ResolveLinkAccess(
			ctx, principal, ids, tokenHash, service.clock.Now().UTC(),
		)
		if err != nil {
			return nil, normalizeStoreError(err)
		}
		for id, decision := range resolved {
			current := levels[id]
			if !current.Level.Valid() || maxLevel(current.Level, decision.Level) != current.Level || decision.Level == current.Level && decision.ExpiresAt.After(current.ExpiresAt) {
				levels[id] = decision
			}
		}
	}
	return levels, nil
}

func effectiveObjectLevel(
	userID uint,
	resource documentlibrary.Resource,
	policy Policy,
	matches map[identity.SubjectRef]bool,
	linkLevel Level,
) (*Level, []string) {
	if resource.OwnerUserID == userID {
		value := LevelEditor
		return &value, []string{"owner"}
	}
	var effective Level
	sources := make([]string, 0, len(policy.Rules)+1)
	switch policy.Mode {
	case ModeRestricted:
		for _, rule := range policy.Rules {
			if matches[rule.Subject()] {
				effective = maxLevel(effective, rule.Level)
				sources = append(sources, "rule:"+string(rule.SubjectKind)+":"+rule.SubjectID)
			}
		}
	case ModeWorkspace:
		if policy.WorkspaceLevel != nil && policy.WorkspaceLevel.Valid() {
			effective = *policy.WorkspaceLevel
			sources = append(sources, "workspace")
		}
	}
	if policy.Mode.AllowsLinks() && linkLevel.Valid() && (!effective.Valid() || maxLevel(effective, linkLevel) != effective) {
		effective = maxLevel(effective, linkLevel)
		sources = append(sources, "link")
	}
	if effective.Valid() {
		return &effective, sources
	}
	return nil, []string{}
}

func objectLevelAllows(level *Level, owner bool, operation documentlibrary.Operation) bool {
	if level == nil {
		return false
	}
	switch operation {
	case documentlibrary.OperationManageAccess, documentlibrary.OperationViewLinks,
		documentlibrary.OperationManageLinks, documentlibrary.OperationMove, documentlibrary.OperationDelete:
		return owner
	case documentlibrary.OperationEdit, documentlibrary.OperationAssetWrite, documentlibrary.OperationComment,
		documentlibrary.OperationStructure, documentlibrary.OperationCellLease:
		return *level == LevelEditor
	case documentlibrary.OperationView, documentlibrary.OperationViewAccess, documentlibrary.OperationExport,
		documentlibrary.OperationAssetRead,
		documentlibrary.OperationCollaborate, documentlibrary.OperationCopy:
		return level.Valid()
	default:
		return false
	}
}

func ownerOnlyOperation(operation documentlibrary.Operation) bool {
	switch operation {
	case documentlibrary.OperationManageAccess, documentlibrary.OperationViewLinks,
		documentlibrary.OperationManageLinks, documentlibrary.OperationMove, documentlibrary.OperationDelete:
		return true
	default:
		return false
	}
}

func containsSource(sources []string, expected string) bool {
	for _, source := range sources {
		if source == expected {
			return true
		}
	}
	return false
}

func maxLevel(left, right Level) Level {
	if left == LevelEditor || right == LevelEditor {
		return LevelEditor
	}
	if left == LevelViewer || right == LevelViewer {
		return LevelViewer
	}
	return ""
}
