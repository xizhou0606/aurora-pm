package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	bitablegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/bitable/gorm/native"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
)

type bitableResourceAccessGate struct {
	resources *documentlibrary.Service
	policies  *resourceaccess.PolicyService
	create    bitable.Authorizer
}

func bitableResourcePrincipal(principal bitable.Principal) resourceaccess.Principal {
	actorFactUserID := principal.ActorFactUserID
	if actorFactUserID == 0 {
		actorFactUserID = principal.UserID
	}
	return resourceaccess.Principal{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, ActorFactUserID: actorFactUserID,
		LinkAccessSecret: principal.LinkAccessSecret,
	}
}

func (gate bitableResourceAccessGate) RequireBitableResource(
	ctx context.Context,
	principal bitable.Principal,
	resourceID documentlibrary.ResourceID,
	operation documentlibrary.Operation,
) (bitable.ResourceBinding, error) {
	if gate.resources == nil || gate.policies == nil {
		return bitable.ResourceBinding{}, bitable.ErrUnavailable
	}
	resource, binding, err := gate.resources.RequireTypeOperation(ctx, principal.WorkspaceID, resourceID, operation)
	if err != nil || resource.TypeKey != documentlibrary.TypeBitable || binding.TypeKey != documentlibrary.TypeBitable {
		return bitable.ResourceBinding{}, bitable.ErrNotFound
	}
	evaluation, err := gate.policies.Evaluate(ctx, bitableResourcePrincipal(principal), resourceaccess.EvaluationRequest{
		ResourceID: resource.ID,
		Operation:  operation,
	})
	if err != nil || !evaluation.Allowed || evaluation.PolicyRevision == 0 {
		if errors.Is(err, resourceaccess.ErrUnavailable) {
			return bitable.ResourceBinding{}, bitable.ErrUnavailable
		}
		return bitable.ResourceBinding{}, bitable.ErrNotFound
	}
	return bitable.ResourceBinding{
		ResourceID: resource.ID, NativeResourceID: binding.PayloadRef, OwnerUserID: resource.OwnerUserID,
		PolicyRevision: evaluation.PolicyRevision,
	}, nil
}

func (gate bitableResourceAccessGate) FilterBitableResources(
	ctx context.Context,
	principal bitable.Principal,
	nativeResourceIDs []string,
	operation documentlibrary.Operation,
) (map[string]bitable.ResourceBinding, error) {
	if gate.resources == nil || gate.policies == nil || principal.UserID == 0 || principal.WorkspaceID == 0 {
		return nil, bitable.ErrUnavailable
	}
	if len(nativeResourceIDs) == 0 {
		return map[string]bitable.ResourceBinding{}, nil
	}
	resources, err := gate.resources.ResolveContentResources(
		ctx, principal.WorkspaceID, documentlibrary.TypeBitable, nativeResourceIDs,
	)
	if err != nil {
		return nil, bitable.ErrUnavailable
	}
	requests := make([]resourceaccess.EvaluationRequest, 0, len(nativeResourceIDs))
	ordered := make([]documentlibrary.Resource, 0, len(nativeResourceIDs))
	for _, nativeID := range nativeResourceIDs {
		resource, ok := resources[nativeID]
		if !ok || resource.TypeKey != documentlibrary.TypeBitable || resource.Lifecycle != "active" || resource.ArchivedAt != nil {
			return nil, bitable.ErrUnavailable
		}
		ordered = append(ordered, resource)
		requests = append(requests, resourceaccess.EvaluationRequest{ResourceID: resource.ID, Operation: operation})
	}
	evaluations, err := gate.policies.BatchEvaluate(ctx, bitableResourcePrincipal(principal), requests)
	if err != nil || len(evaluations) != len(requests) {
		return nil, bitable.ErrUnavailable
	}
	result := make(map[string]bitable.ResourceBinding, len(ordered))
	for index, resource := range ordered {
		if !evaluations[index].Allowed {
			continue
		}
		result[nativeResourceIDs[index]] = bitable.ResourceBinding{
			ResourceID: resource.ID, NativeResourceID: nativeResourceIDs[index], OwnerUserID: resource.OwnerUserID,
			PolicyRevision: evaluations[index].PolicyRevision,
		}
	}
	return result, nil
}

func (gate bitableResourceAccessGate) AuthorizeBitableResource(
	ctx context.Context,
	principal bitable.Principal,
	binding bitable.ResourceBinding,
	operation documentlibrary.Operation,
) error {
	resolved, err := gate.RequireBitableResource(ctx, principal, binding.ResourceID, operation)
	if err != nil {
		return err
	}
	if resolved.NativeResourceID != binding.NativeResourceID || resolved.OwnerUserID != binding.OwnerUserID {
		return bitable.ErrNotFound
	}
	return nil
}

func (gate bitableResourceAccessGate) BatchAuthorizeBitableResources(
	ctx context.Context,
	principal bitable.Principal,
	bindings []bitable.ResourceBinding,
	operations []documentlibrary.Operation,
) (map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, error) {
	if gate.resources == nil || gate.policies == nil || principal.UserID == 0 || principal.WorkspaceID == 0 || len(operations) == 0 {
		return nil, bitable.ErrUnavailable
	}
	if len(bindings) == 0 {
		return map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool{}, nil
	}
	requests := make([]resourceaccess.EvaluationRequest, 0, len(bindings)*len(operations))
	for _, binding := range bindings {
		resource, content, err := gate.resources.ResolveResource(ctx, principal.WorkspaceID, binding.ResourceID)
		if err != nil || resource.TypeKey != documentlibrary.TypeBitable || resource.Lifecycle != "active" ||
			resource.ArchivedAt != nil || content.TypeKey != documentlibrary.TypeBitable ||
			content.PayloadRef != binding.NativeResourceID || resource.OwnerUserID != binding.OwnerUserID {
			return nil, bitable.ErrUnavailable
		}
		for _, operation := range operations {
			requests = append(requests, resourceaccess.EvaluationRequest{ResourceID: resource.ID, Operation: operation})
		}
	}
	evaluations, err := gate.policies.BatchEvaluate(ctx, bitableResourcePrincipal(principal), requests)
	if err != nil || len(evaluations) != len(requests) {
		return nil, bitable.ErrUnavailable
	}
	result := make(map[documentlibrary.ResourceID]map[documentlibrary.Operation]bool, len(bindings))
	for index, binding := range bindings {
		decisions := make(map[documentlibrary.Operation]bool, len(operations))
		for operationIndex, operation := range operations {
			decisions[operation] = evaluations[index*len(operations)+operationIndex].Allowed
		}
		result[binding.ResourceID] = decisions
	}
	return result, nil
}

func (gate bitableResourceAccessGate) RequireBitableCreate(ctx context.Context, principal bitable.Principal) error {
	if gate.create == nil {
		return bitable.ErrUnavailable
	}
	return gate.create.RequireAction(ctx, principal, accesscatalog.ActionBitablesResourcesCreate)
}

type bitablePrivateResourceCoordinator struct{ db *gorm.DB }

func (coordinator bitablePrivateResourceCoordinator) CreatePrivateBitable(
	ctx context.Context,
	principal bitable.Principal,
	name, operationID string,
) (bitable.Resource, bitable.ResourceBinding, error) {
	var result bitable.Resource
	var binding bitable.ResourceBinding
	err := coordinator.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		native := newBitableNativeRepository(tx)
		created, err := native.Create(ctx, principal, name, operationID)
		if err != nil {
			return err
		}
		resource, existing, found, err := resolveExistingBitableResource(ctx, tx, principal.WorkspaceID, created.ID)
		if err != nil {
			return err
		}
		if !found {
			resource, existing, err = createPrivateBitableResource(ctx, tx, principal, created)
			if err != nil {
				return err
			}
		}
		policyRevision, err := appendBitableResourceCommittedEvent(
			ctx, tx, principal, resource, created, operationID, "create",
		)
		if err != nil {
			return err
		}
		result, binding = created, bitableBinding(resource, existing, policyRevision)
		return nil
	})
	return result, binding, normalizeBitableCoordinatorError(err)
}

func (coordinator bitablePrivateResourceCoordinator) DuplicatePrivateBitable(
	ctx context.Context,
	principal bitable.Principal,
	source bitable.ResourceBinding,
	mutation bitable.ResourceMutation,
) (bitable.Resource, bitable.ResourceBinding, error) {
	var result bitable.Resource
	var binding bitable.ResourceBinding
	err := coordinator.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, err := newBitableNativeRepository(tx).MutateResource(
			ctx, principal, source.NativeResourceID, "duplicate", mutation,
		)
		if err != nil {
			return err
		}
		resource, existing, found, err := resolveExistingBitableResource(ctx, tx, principal.WorkspaceID, created.ID)
		if err != nil {
			return err
		}
		if !found {
			resource, existing, err = createPrivateBitableResource(ctx, tx, principal, created)
			if err != nil {
				return err
			}
		}
		policyRevision, err := appendBitableResourceCommittedEvent(
			ctx, tx, principal, resource, created, mutation.OperationID, "duplicate",
		)
		if err != nil {
			return err
		}
		result, binding = created, bitableBinding(resource, existing, policyRevision)
		return nil
	})
	return result, binding, normalizeBitableCoordinatorError(err)
}

func (coordinator bitablePrivateResourceCoordinator) MutateBitableResource(
	ctx context.Context,
	principal bitable.Principal,
	binding bitable.ResourceBinding,
	kind string,
	mutation bitable.ResourceMutation,
) (bitable.Resource, bitable.ResourceBinding, error) {
	var result bitable.Resource
	var updatedBinding bitable.ResourceBinding
	err := coordinator.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updated, err := newBitableNativeRepository(tx).MutateResource(
			ctx, principal, binding.NativeResourceID, kind, mutation,
		)
		if err != nil {
			return err
		}
		resources, err := documentlibrary.NewService(documentlibrarygorm.NewRepository(tx), documentlibrary.DefaultRegistry())
		if err != nil {
			return err
		}
		resource, content, err := resources.ResolveContentResource(
			ctx, principal.WorkspaceID, documentlibrary.TypeBitable, binding.NativeResourceID,
		)
		if err != nil || resource.ID != binding.ResourceID || content.ResourceID != binding.ResourceID {
			return bitable.ErrUnavailable
		}
		if kind == "delete" {
			if resource.Lifecycle == "active" && resource.ArchivedAt == nil {
				if err := resources.ArchiveResource(
					ctx, documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
					resource.ID, resource.Revision,
				); err != nil {
					return err
				}
				resource.Revision++
				resource.Lifecycle = "archived"
			}
		} else {
			parentRef := bitableParentRef(updated.ParentFolderID)
			if !sameBitablePublicMetadata(resource, updated, parentRef) {
				resource, err = resources.ReplaceResourceMetadata(
					ctx,
					documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
					resource.ID, resource.Revision, updated.Name, parentRef, updated.OwnerUserID,
				)
				if err != nil {
					return err
				}
			}
		}
		policyRevision, err := appendBitableResourceCommittedEvent(
			ctx, tx, principal, resource, updated, mutation.OperationID, kind,
		)
		if err != nil {
			return err
		}
		result, updatedBinding = updated, bitableBinding(resource, content, policyRevision)
		return nil
	})
	return result, updatedBinding, normalizeBitableCoordinatorError(err)
}

func newBitableNativeRepository(db *gorm.DB) *bitablegorm.Repository {
	repository := bitablegorm.NewRepository(db)
	repository.ConfigureMutationRecorderFactory(newBitableMutationRecorder)
	repository.ConfigureCellLeaseConflictGuardFactory(newBitableCellLeaseConflictGuard)
	return repository
}

func createPrivateBitableResource(
	ctx context.Context,
	tx *gorm.DB,
	principal bitable.Principal,
	created bitable.Resource,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	resources, err := documentlibrary.NewService(documentlibrarygorm.NewRepository(tx), documentlibrary.DefaultRegistry())
	if err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, err
	}
	resource, binding, err := resources.CreateResource(
		ctx,
		documentlibrary.Principal{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID},
		documentlibrary.TypeBitable,
		created.ID,
		created.Name,
		bitableParentRef(created.ParentFolderID),
	)
	if err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, err
	}
	if _, err := resourceaccessgorm.NewRepository(tx).CreatePrivatePolicy(
		ctx, bitableResourcePrincipal(principal), resource.ID, time.Now().UTC(),
	); err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, err
	}
	return resource, binding, nil
}

func resolveExistingBitableResource(
	ctx context.Context,
	tx *gorm.DB,
	workspaceID uint,
	nativeResourceID string,
) (documentlibrary.Resource, documentlibrary.ContentBinding, bool, error) {
	repository := documentlibrarygorm.NewRepository(tx)
	resource, binding, err := repository.ResolveContent(
		ctx, workspaceID, documentlibrary.TypeBitable, nativeResourceID,
	)
	if errors.Is(err, documentlibrary.ErrNotFound) {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, false, nil
	}
	if err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, false, err
	}
	policy, err := resourceaccessgorm.NewRepository(tx).GetPolicy(ctx, workspaceID, resource.ID)
	if err != nil || policy.Revision == 0 || resource.TypeKey != documentlibrary.TypeBitable {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, false, bitable.ErrConflict
	}
	return resource, binding, true, nil
}

func bitableBinding(
	resource documentlibrary.Resource,
	binding documentlibrary.ContentBinding,
	policyRevision uint64,
) bitable.ResourceBinding {
	return bitable.ResourceBinding{
		ResourceID: resource.ID, NativeResourceID: binding.PayloadRef, OwnerUserID: resource.OwnerUserID,
		PolicyRevision: policyRevision,
	}
}

func appendBitableResourceCommittedEvent(
	ctx context.Context,
	tx *gorm.DB,
	principal bitable.Principal,
	resource documentlibrary.Resource,
	native bitable.Resource,
	operationID, kind string,
) (uint64, error) {
	eventTypes := map[string]string{
		"create": "bitable.resource.create.committed", "duplicate": "bitable.resource.duplicate.committed",
		"name": "bitable.resource.name.committed", "location": "bitable.resource.location.committed",
		"owner": "bitable.resource.owner.committed", "delete": "bitable.resource.delete.committed",
	}
	eventType, ok := eventTypes[kind]
	if tx == nil || !ok || principal.WorkspaceID == 0 || principal.UserID == 0 ||
		resource.ID == "" || resource.WorkspaceID != principal.WorkspaceID || native.ID == "" {
		return 0, bitable.ErrUnavailable
	}
	policy, err := resourceaccessgorm.NewRepository(tx).LockCurrentPolicy(
		ctx, principal.WorkspaceID, resource.ID,
	)
	if err != nil || policy.Revision == 0 {
		return 0, bitable.ErrUnavailable
	}
	payload, err := json.Marshal(map[string]any{
		"resource_revision": resource.Revision,
		"native_revision":   native.Revision,
		"lifecycle":         resource.Lifecycle,
		"type_key":          documentlibrary.TypeBitable,
	})
	if err != nil {
		return 0, err
	}
	appender, err := collaborationgorm.NewAppender(tx)
	if err != nil {
		return 0, bitable.ErrUnavailable
	}
	if _, err := appender.Append(ctx, collaboration.AppendInput{
		Room:        collaboration.Room{WorkspaceID: principal.WorkspaceID, ResourceID: string(resource.ID)},
		OperationID: operationID, ActorUserID: principal.UserID, Type: eventType,
		Payload: payload, PolicyRevision: policy.Revision,
	}); err != nil {
		return 0, bitable.ErrUnavailable
	}
	return policy.Revision, nil
}

func bitableParentRef(parentFolderID *uint) *string {
	if parentFolderID == nil {
		return nil
	}
	value := fmt.Sprintf("document-node:%d", *parentFolderID)
	return &value
}

func sameBitablePublicMetadata(resource documentlibrary.Resource, native bitable.Resource, parentRef *string) bool {
	if strings.TrimSpace(resource.Name) != strings.TrimSpace(native.Name) || resource.OwnerUserID != native.OwnerUserID {
		return false
	}
	if resource.ParentRef == nil || parentRef == nil {
		return resource.ParentRef == nil && parentRef == nil
	}
	return *resource.ParentRef == *parentRef
}

func normalizeBitableCoordinatorError(err error) error {
	if err == nil || errors.Is(err, bitable.ErrInvalidInput) || errors.Is(err, bitable.ErrConflict) ||
		errors.Is(err, bitable.ErrNotFound) || errors.Is(err, bitable.ErrForbidden) || errors.Is(err, bitable.ErrUnavailable) {
		return err
	}
	if errors.Is(err, documentlibrary.ErrConflict) || errors.Is(err, resourceaccess.ErrConflict) {
		return bitable.ErrConflict
	}
	if errors.Is(err, documentlibrary.ErrNotFound) || errors.Is(err, resourceaccess.ErrNotFound) {
		return bitable.ErrNotFound
	}
	return bitable.ErrUnavailable
}

var _ bitable.ResourceAccessGate = bitableResourceAccessGate{}
var _ bitable.PrivateResourceCoordinator = bitablePrivateResourceCoordinator{}
