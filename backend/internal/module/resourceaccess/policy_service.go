package resourceaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type PolicyService struct {
	resolver          documentlibrary.Resolver
	subjects          identity.SubjectDirectory
	authorizer        GlobalOperationAuthorizer
	legacy            LegacyPolicyEvaluator
	store             PolicyStore
	clock             Clock
	requirementSource RequirementRuleSource
	approvedRequests  ApprovedRequestSource
}

func (service *PolicyService) ConfigureApprovedRequests(source ApprovedRequestSource) {
	service.approvedRequests = source
}

func NewPolicyService(
	resolver documentlibrary.Resolver,
	subjects identity.SubjectDirectory,
	authorizer GlobalOperationAuthorizer,
	legacy LegacyPolicyEvaluator,
	store PolicyStore,
	clock Clock,
) (*PolicyService, error) {
	if resolver == nil || subjects == nil || authorizer == nil || store == nil || clock == nil {
		return nil, ErrUnavailable
	}
	return &PolicyService{
		resolver: resolver, subjects: subjects, authorizer: authorizer, legacy: legacy, store: store, clock: clock,
	}, nil
}

func (service *PolicyService) GetPolicy(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID,
) (PolicyView, error) {
	if !validPrincipal(principal) || strings.TrimSpace(string(resourceID)) == "" {
		return PolicyView{}, ErrInvalidInput
	}
	evaluation, err := service.Evaluate(ctx, principal, EvaluationRequest{
		ResourceID: resourceID, Operation: documentlibrary.OperationViewAccess,
	})
	if err != nil {
		return PolicyView{}, err
	}
	if !evaluation.Allowed {
		return PolicyView{}, ErrNotFound
	}
	resource, _, err := service.resolver.RequireTypeOperation(
		ctx, principal.WorkspaceID, resourceID, documentlibrary.OperationViewAccess,
	)
	if err != nil {
		return PolicyView{}, ErrNotFound
	}
	policy, err := service.store.GetPolicy(ctx, principal.WorkspaceID, resource.ID)
	if err != nil {
		return PolicyView{}, normalizeStoreError(err)
	}
	canManage := false
	if resource.OwnerUserID == principal.UserID {
		if _, _, typeErr := service.resolver.RequireTypeOperation(
			ctx, principal.WorkspaceID, resourceID, documentlibrary.OperationManageAccess,
		); typeErr == nil {
			allowed, authErr := service.authorizer.AuthorizeDocumentResourceOperation(
				ctx, principal, resource, documentlibrary.OperationManageAccess,
			)
			if authErr != nil {
				return PolicyView{}, ErrUnavailable
			}
			canManage = allowed
		}
	}
	return PolicyView{
		Policy: policy, CanManageAccess: canManage, EffectiveLevel: evaluation.EffectiveLevel,
		EffectiveSources: evaluation.Sources, SubjectCapabilities: SubjectCapabilities(service.subjects),
	}, nil
}

// AuthorizeOperation exposes the same point decision used by BatchEvaluate to
// HTTP object-policy markers and resource-type adapters. A denied decision is
// intentionally collapsed to not-found so callers cannot probe resources.
func (service *PolicyService) AuthorizeOperation(
	ctx context.Context,
	principal Principal,
	resourceID documentlibrary.ResourceID,
	operation documentlibrary.Operation,
) error {
	evaluation, err := service.Evaluate(ctx, principal, EvaluationRequest{ResourceID: resourceID, Operation: operation})
	if err != nil {
		return err
	}
	if !evaluation.Allowed {
		return ErrNotFound
	}
	return nil
}

func (service *PolicyService) ReplacePolicy(
	ctx context.Context,
	principal Principal,
	resourceID documentlibrary.ResourceID,
	input ReplacePolicyInput,
) (PolicyView, error) {
	resource, err := service.requireManagement(ctx, principal, resourceID)
	if err != nil {
		return PolicyView{}, err
	}
	normalized, err := normalizePolicyInput(input)
	if err != nil {
		return PolicyView{}, err
	}
	if len(normalized.Rules) > 0 {
		refs := make([]identity.SubjectRef, 0, len(normalized.Rules))
		for _, rule := range normalized.Rules {
			refs = append(refs, rule.Subject())
		}
		if _, err := service.subjects.ResolveSubjects(ctx, principal.WorkspaceID, refs); err != nil {
			if errors.Is(err, identity.ErrSubjectKindUnavailable) {
				return PolicyView{}, ErrUnavailable
			}
			return PolicyView{}, ErrInvalidInput
		}
	}
	digest, err := policyInputDigest(normalized)
	if err != nil {
		return PolicyView{}, err
	}
	policy, err := service.store.ReplacePolicy(ctx, PolicyMutation{
		Principal: principal, ResourceID: resource.ID, Mode: normalized.Mode,
		WorkspaceLevel: normalized.WorkspaceLevel, Rules: normalized.Rules,
		ExpectedRevision: normalized.ExpectedPolicyRevision,
		IdempotencyKey:   normalized.IdempotencyKey, InputDigest: digest,
		Now: service.clock.Now().UTC(),
	})
	if err != nil {
		return PolicyView{}, normalizeStoreError(err)
	}
	level := LevelEditor
	return PolicyView{
		Policy: policy, CanManageAccess: true, EffectiveLevel: &level,
		EffectiveSources: []string{"owner"}, SubjectCapabilities: SubjectCapabilities(service.subjects),
	}, nil
}

func (service *PolicyService) requireManagement(
	ctx context.Context, principal Principal, resourceID documentlibrary.ResourceID,
) (documentlibrary.Resource, error) {
	return service.requireOwnerOperation(ctx, principal, resourceID, documentlibrary.OperationManageAccess)
}

func (service *PolicyService) requireOwnerOperation(
	ctx context.Context,
	principal Principal,
	resourceID documentlibrary.ResourceID,
	operation documentlibrary.Operation,
) (documentlibrary.Resource, error) {
	if !validPrincipal(principal) || strings.TrimSpace(string(resourceID)) == "" {
		return documentlibrary.Resource{}, ErrInvalidInput
	}
	if err := service.AuthorizeOperation(ctx, principal, resourceID, operation); err != nil {
		return documentlibrary.Resource{}, err
	}
	resource, _, err := service.resolver.RequireTypeOperation(
		ctx, principal.WorkspaceID, resourceID, operation,
	)
	if err != nil {
		return documentlibrary.Resource{}, ErrNotFound
	}
	if resource.OwnerUserID != principal.UserID {
		return documentlibrary.Resource{}, ErrNotFound
	}
	return resource, nil
}

func normalizePolicyInput(input ReplacePolicyInput) (ReplacePolicyInput, error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ExpectedPolicyRevision == 0 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 160 ||
		!input.Mode.ValidStored() || input.Mode == ModeLegacy {
		return ReplacePolicyInput{}, ErrInvalidInput
	}
	if input.Mode == ModeWorkspace {
		if input.WorkspaceLevel == nil || !input.WorkspaceLevel.Valid() || len(input.Rules) != 0 {
			return ReplacePolicyInput{}, ErrInvalidInput
		}
	} else if input.WorkspaceLevel != nil {
		return ReplacePolicyInput{}, ErrInvalidInput
	}
	// Restricted policies may have only derived requirement sources. An empty
	// direct rule list grants nobody by itself; do not persist derived rules here.
	if input.Mode != ModeRestricted && len(input.Rules) != 0 {
		return ReplacePolicyInput{}, ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(input.Rules))
	for _, rule := range input.Rules {
		if !rule.SubjectKind.Valid() || strings.TrimSpace(rule.SubjectID) == "" || !rule.Level.Valid() ||
			(rule.IncludeDescendants && rule.SubjectKind != identity.SubjectOrganizationUnit) {
			return ReplacePolicyInput{}, ErrInvalidInput
		}
		key := string(rule.SubjectKind) + "\x00" + rule.SubjectID
		if _, exists := seen[key]; exists {
			return ReplacePolicyInput{}, ErrInvalidInput
		}
		seen[key] = struct{}{}
	}
	return input, nil
}

func policyInputDigest(input ReplacePolicyInput) (string, error) {
	encoded, err := json.Marshal(struct {
		Mode           Mode
		WorkspaceLevel *Level
		Rules          []Rule
		Expected       uint64
	}{input.Mode, input.WorkspaceLevel, input.Rules, input.ExpectedPolicyRevision})
	if err != nil {
		return "", ErrInvalidInput
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validPrincipal(principal Principal) bool {
	return principal.WorkspaceID > 0 && principal.UserID > 0
}

func SubjectCapabilities(directory identity.SubjectDirectory) map[identity.SubjectKind]identity.SubjectKindCapability {
	result := map[identity.SubjectKind]identity.SubjectKindCapability{
		identity.SubjectUser: {}, identity.SubjectRole: {}, identity.SubjectUserGroup: {},
		identity.SubjectOrganizationUnit: {}, identity.SubjectProjectSet: {},
	}
	provider, ok := directory.(identity.SubjectCapabilityProvider)
	if !ok {
		return result
	}
	for kind, capability := range provider.SubjectCapabilities() {
		if kind.Valid() {
			result[kind] = capability
		}
	}
	return result
}

func normalizeStoreError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrConflict), errors.Is(err, ErrIdempotencyConflict),
		errors.Is(err, ErrInvalidInput), errors.Is(err, ErrUnavailable), errors.Is(err, ErrLinkInvalid):
		return err
	default:
		return ErrUnavailable
	}
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
