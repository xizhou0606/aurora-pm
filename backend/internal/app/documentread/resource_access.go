package documentread

import (
	"context"
	"strconv"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
)

type documentResourcePolicyEvaluator interface {
	Evaluate(context.Context, resourceaccess.Principal, resourceaccess.EvaluationRequest) (resourceaccess.Evaluation, error)
	BatchEvaluate(context.Context, resourceaccess.Principal, []resourceaccess.EvaluationRequest) ([]resourceaccess.Evaluation, error)
}

type ResourceAccessResolver struct {
	Resources documentlibrary.Resolver
	Policies  documentResourcePolicyEvaluator
}

func (resolver ResourceAccessResolver) ResolveDocumentAccess(
	ctx context.Context,
	principal document.Principal,
	documentID uint,
) (document.ResourceAccess, error) {
	resolved, err := resolver.ResolveDocumentsAccess(ctx, principal, []uint{documentID})
	if err != nil {
		return document.ResourceAccess{}, err
	}
	value, ok := resolved[documentID]
	if !ok {
		return document.ResourceAccess{}, document.ErrForbidden
	}
	return value, nil
}

func (resolver ResourceAccessResolver) ResolveDocumentsAccess(
	ctx context.Context,
	principal document.Principal,
	documentIDs []uint,
) (map[uint]document.ResourceAccess, error) {
	if resolver.Resources == nil || resolver.Policies == nil || principal.UserID == 0 ||
		principal.WorkspaceID == 0 || len(documentIDs) == 0 {
		return nil, document.ErrForbidden
	}
	payloadRefs := make([]string, 0, len(documentIDs))
	byPayload := make(map[string]uint, len(documentIDs))
	for _, documentID := range documentIDs {
		if documentID == 0 {
			return nil, document.ErrForbidden
		}
		payloadRef := strconv.FormatUint(uint64(documentID), 10)
		payloadRefs = append(payloadRefs, payloadRef)
		byPayload[payloadRef] = documentID
	}
	resources, err := resolver.Resources.ResolveContentResources(
		ctx, principal.WorkspaceID, documentlibrary.TypeRichText, payloadRefs,
	)
	if err != nil || len(resources) != len(byPayload) {
		return nil, document.ErrForbidden
	}
	requests := make([]resourceaccess.EvaluationRequest, 0, len(resources)*3)
	ordered := make([]documentlibrary.Resource, 0, len(payloadRefs))
	for _, payloadRef := range payloadRefs {
		resource, ok := resources[payloadRef]
		if !ok {
			return nil, document.ErrForbidden
		}
		ordered = append(ordered, resource)
		requests = append(requests,
			resourceaccess.EvaluationRequest{ResourceID: resource.ID, Operation: documentlibrary.OperationView},
			resourceaccess.EvaluationRequest{ResourceID: resource.ID, Operation: documentlibrary.OperationComment},
			resourceaccess.EvaluationRequest{ResourceID: resource.ID, Operation: documentlibrary.OperationEdit},
		)
	}
	actorFactUserID := principal.ParticipantUserID
	if actorFactUserID == 0 {
		actorFactUserID = principal.UserID
	}
	evaluations, err := resolver.Policies.BatchEvaluate(
		ctx,
		resourceaccess.Principal{
			WorkspaceID: principal.WorkspaceID, UserID: principal.UserID,
			ActorFactUserID: actorFactUserID, LinkAccessSecret: principal.LinkAccessSecret,
		},
		requests,
	)
	if err != nil || len(evaluations) != len(requests) {
		return nil, document.ErrForbidden
	}
	result := make(map[uint]document.ResourceAccess, len(ordered))
	for index, resource := range ordered {
		documentID := byPayload[payloadRefs[index]]
		result[documentID] = document.ResourceAccess{
			ResourceID: string(resource.ID),
			CanView:    evaluations[index*3].Allowed,
			CanComment: evaluations[index*3+1].Allowed,
			CanEdit:    evaluations[index*3+2].Allowed,
			CanManage:  resource.OwnerUserID == principal.UserID,
		}
	}
	return result, nil
}

type legacyDocumentAccess interface {
	EvaluateLegacyResourceAccess(context.Context, document.Principal, uint) (document.ResourceAccess, error)
}

type LegacyEvaluator struct {
	Documents legacyDocumentAccess
	Resources documentlibrary.Resolver
}

func (evaluator LegacyEvaluator) EvaluateLegacyDocumentResource(
	ctx context.Context,
	principal resourceaccess.Principal,
	resource documentlibrary.Resource,
	operation documentlibrary.Operation,
) (bool, error) {
	if resource.WorkspaceID != principal.WorkspaceID {
		return false, nil
	}
	if resource.TypeKey == documentlibrary.TypeBitable {
		// Legacy bitables had no object ACL; the canonical global Action was
		// their complete access boundary. The caller already checked it.
		return true, nil
	}
	if resource.TypeKey != documentlibrary.TypeRichText || evaluator.Documents == nil || evaluator.Resources == nil {
		return false, nil
	}
	_, binding, err := evaluator.Resources.ResolveResource(ctx, principal.WorkspaceID, resource.ID)
	if err != nil || binding.TypeKey != resource.TypeKey || binding.ResourceID != resource.ID {
		return false, nil
	}
	documentID, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil || documentID == 0 || uint64(uint(documentID)) != documentID {
		return false, nil
	}
	access, err := evaluator.Documents.EvaluateLegacyResourceAccess(ctx, document.Principal{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, ParticipantUserID: principal.ActorFactUserID,
	}, uint(documentID))
	if err != nil {
		return false, err
	}
	switch operation {
	case documentlibrary.OperationView, documentlibrary.OperationViewAccess, documentlibrary.OperationExport,
		documentlibrary.OperationAssetRead, documentlibrary.OperationCollaborate, documentlibrary.OperationCopy:
		return access.CanView, nil
	case documentlibrary.OperationComment:
		return access.CanComment, nil
	case documentlibrary.OperationEdit, documentlibrary.OperationAssetWrite, documentlibrary.OperationStructure,
		documentlibrary.OperationCellLease:
		return access.CanEdit, nil
	case documentlibrary.OperationManageAccess, documentlibrary.OperationViewLinks,
		documentlibrary.OperationManageLinks, documentlibrary.OperationMove, documentlibrary.OperationDelete:
		return access.CanManage, nil
	default:
		return false, nil
	}
}
