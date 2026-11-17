package api

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	bitablegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/bitable/gorm/native"
	bitablehttp "github.com/xizhou0606/aurora-pm/backend/internal/module/bitable/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
)

type bitablePolicy struct {
	access      *identity.AccessService
	memberships *identity.WorkspaceMembershipService
}

func (p bitablePolicy) RequireAction(ctx context.Context, actor bitable.Principal, action accesscatalog.ActionCode) error {
	if err := p.memberships.VerifyWorkspaceMembership(ctx, actor.UserID, actor.WorkspaceID); err != nil {
		return bitable.ErrForbidden
	}
	codes, err := p.access.EffectiveActionCodes(ctx, actor.UserID)
	if err != nil {
		return err
	}
	for _, code := range codes {
		if code == string(action) {
			return nil
		}
	}
	return bitable.ErrForbidden
}

type bitableMutationRecorder struct{ service *audit.MutationEffectService }

func (r bitableMutationRecorder) RecordBitableMutation(ctx context.Context, e bitable.MutationEffect) error {
	return r.service.RecordMutationEffect(ctx, audit.MutationEffect{
		WorkspaceID: e.Principal.WorkspaceID, PrincipalID: e.Principal.UserID, Resource: "bitable", ResourceRef: e.ResourceID,
		Action: e.Kind, Target: e.TableID, EventType: "pm.bitable." + e.Kind,
		Payload: map[string]any{"resource_id": e.ResourceID, "table_id": e.TableID, "operation_id": e.OperationID, "record_ids": e.RecordIDs},
	})
}
func newBitableMutationRecorder(tx *gorm.DB) (bitable.MutationRecorder, error) {
	service, err := audit.NewMutationEffectService(auditgorm.NewMutationEffectRepository(tx))
	if err != nil {
		return nil, err
	}
	return bitableMutationRecorder{service}, nil
}

type bitableCellAccessEvaluator struct {
	policies *resourceaccess.PolicyService
	store    *resourceaccessgorm.Repository
}

func (evaluator bitableCellAccessEvaluator) EvaluateCellEditAccess(
	ctx context.Context,
	principal collaboration.Principal,
	key collaboration.CellKey,
) (collaboration.AccessDecision, error) {
	if evaluator.policies == nil || evaluator.store == nil || principal.UserID == 0 || principal.WorkspaceID == 0 ||
		principal.WorkspaceID != key.Room.WorkspaceID || key.Room.ResourceID == "" {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	locked, err := evaluator.store.LockCurrentPolicy(
		ctx, principal.WorkspaceID, documentlibrary.ResourceID(key.Room.ResourceID),
	)
	if err != nil {
		return collaboration.AccessDecision{}, mapCollaborationResourceAccessError(err)
	}
	evaluation, err := evaluator.policies.Evaluate(ctx, resourceaccess.Principal{
		WorkspaceID:      principal.WorkspaceID,
		UserID:           principal.UserID,
		LinkAccessSecret: principal.LinkAccessSecret,
	}, resourceaccess.EvaluationRequest{
		ResourceID: documentlibrary.ResourceID(key.Room.ResourceID),
		Operation:  documentlibrary.OperationCellLease,
	})
	if err != nil || !evaluation.Allowed || evaluation.PolicyRevision == 0 || evaluation.PolicyRevision != locked.Revision {
		if err != nil {
			return collaboration.AccessDecision{}, mapCollaborationResourceAccessError(err)
		}
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	return collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: evaluation.PolicyRevision,
		ExpiresAt: time.Now().UTC().Add(collaboration.DefaultCellLeaseTTL),
	}, nil
}

type bitableCellMutationGuard struct {
	delegate collaboration.CellMutationGuard
}

func (guard bitableCellMutationGuard) VerifyCellMutation(
	ctx context.Context,
	principal bitable.Principal,
	proof bitable.CellMutationProof,
) error {
	if guard.delegate == nil {
		return bitable.ErrUnavailable
	}
	_, err := guard.delegate.VerifyCellMutation(ctx, collaboration.Principal{
		WorkspaceID:      principal.WorkspaceID,
		UserID:           principal.UserID,
		ConnectionID:     proof.ConnectionID,
		LinkAccessSecret: principal.LinkAccessSecret,
	}, collaboration.CellLeaseProof{
		Key: collaboration.CellKey{
			Room:    collaboration.Room{WorkspaceID: proof.WorkspaceID, ResourceID: proof.ResourceID},
			TableID: proof.TableID, RecordID: proof.RecordID, FieldID: proof.FieldID,
		},
		LeaseID: proof.LeaseID, FencingToken: proof.FencingToken,
		ConnectionID: proof.ConnectionID, EditorInstanceID: proof.EditorInstanceID,
	})
	return mapBitableCellMutationError(err)
}

type bitableCellLeaseConflictGuard struct {
	delegate *collaborationgorm.CellLeaseConflictGuard
}

func newBitableCellLeaseConflictGuard(tx *gorm.DB) (bitable.CellLeaseConflictGuard, error) {
	delegate, err := collaborationgorm.NewCellLeaseConflictGuard(tx)
	if err != nil {
		return nil, err
	}
	return bitableCellLeaseConflictGuard{delegate: delegate}, nil
}

func (guard bitableCellLeaseConflictGuard) RequireNoConflictingCellLeases(
	ctx context.Context,
	principal bitable.Principal,
	scopes []bitable.CellLeaseConflictScope,
) error {
	if guard.delegate == nil || principal.WorkspaceID == 0 || principal.UserID == 0 || len(scopes) == 0 {
		return bitable.ErrUnavailable
	}
	keys := make([]collaboration.CellKey, 0)
	for _, scope := range scopes {
		if scope.WorkspaceID != principal.WorkspaceID || scope.ResourceID == "" || scope.TableID == "" ||
			len(scope.RecordIDs) == 0 || len(scope.FieldIDs) == 0 ||
			len(scope.RecordIDs) > 500 || len(scope.FieldIDs) > 500 || len(scope.RecordIDs)*len(scope.FieldIDs) > 500 {
			return bitable.ErrInvalidInput
		}
		for _, recordID := range scope.RecordIDs {
			for _, fieldID := range scope.FieldIDs {
				keys = append(keys, collaboration.CellKey{
					Room:    collaboration.Room{WorkspaceID: scope.WorkspaceID, ResourceID: scope.ResourceID},
					TableID: scope.TableID, RecordID: recordID, FieldID: fieldID,
				})
			}
		}
	}
	err := guard.delegate.RequireNoActiveCellLeases(ctx, keys)
	if errors.Is(err, collaboration.ErrLeaseHeld) {
		return bitable.ErrCellLeaseConflict
	}
	if errors.Is(err, collaboration.ErrInvalidInput) {
		return bitable.ErrInvalidInput
	}
	if err != nil {
		return bitable.ErrUnavailable
	}
	return nil
}

type bitableCommittedEventAppender struct {
	resources *documentlibrarygorm.Repository
	policies  *resourceaccessgorm.Repository
	delegate  collaboration.EventAppender
}

func (appender bitableCommittedEventAppender) AppendBitableCommittedEvent(
	ctx context.Context,
	event bitable.CommittedEvent,
) error {
	if appender.resources == nil || appender.policies == nil || appender.delegate == nil {
		return bitable.ErrUnavailable
	}
	resource, binding, err := appender.resources.LockActiveResource(
		ctx, event.WorkspaceID, documentlibrary.ResourceID(event.ResourceID),
	)
	if err != nil || resource.TypeKey != documentlibrary.TypeBitable || binding.TypeKey != documentlibrary.TypeBitable ||
		binding.ResourceID != resource.ID {
		return bitable.ErrForbidden
	}
	if _, err := appender.policies.LockPolicyRevision(
		ctx, event.WorkspaceID, documentlibrary.ResourceID(event.ResourceID), event.PolicyRevision,
	); err != nil {
		if errors.Is(err, resourceaccess.ErrConflict) || errors.Is(err, resourceaccess.ErrNotFound) {
			return bitable.ErrForbidden
		}
		return bitable.ErrUnavailable
	}
	_, err = appender.delegate.Append(ctx, collaboration.AppendInput{
		Room:        collaboration.Room{WorkspaceID: event.WorkspaceID, ResourceID: event.ResourceID},
		OperationID: event.OperationID, ActorUserID: event.ActorUserID, Type: event.Type,
		Payload: event.Payload, PolicyRevision: event.PolicyRevision,
	})
	if err != nil {
		return bitable.ErrUnavailable
	}
	return nil
}

func mapBitableCellMutationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, collaboration.ErrLeaseExpired), errors.Is(err, collaboration.ErrStaleLease),
		errors.Is(err, collaboration.ErrStalePolicy), errors.Is(err, collaboration.ErrNotFound):
		return bitable.ErrCellLeaseLost
	case errors.Is(err, collaboration.ErrForbidden):
		return bitable.ErrForbidden
	case errors.Is(err, collaboration.ErrInvalidInput):
		return bitable.ErrInvalidInput
	default:
		return bitable.ErrUnavailable
	}
}

func mapCollaborationResourceAccessError(err error) error {
	if errors.Is(err, resourceaccess.ErrNotFound) || errors.Is(err, resourceaccess.ErrForbidden) ||
		errors.Is(err, resourceaccess.ErrConflict) {
		return collaboration.ErrForbidden
	}
	return collaboration.ErrDataCorrupt
}

func assembleBitableRoutes(
	db *gorm.DB,
	access *identity.AccessService,
	resourceAccess resourceAccessRuntime,
	cfg config.ObjectStorage,
	documents ...*document.Service,
) (func(*gin.RouterGroup), error) {
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, err
	}
	repo := bitablegorm.NewRepository(db)
	repo.ConfigureMutationRecorderFactory(newBitableMutationRecorder)
	documentService := (*document.Service)(nil)
	if len(documents) > 0 {
		documentService = documents[0]
	}
	repo.ConfigureCellMutationGuardFactory(func(tx *gorm.DB) (bitable.CellMutationGuard, error) {
		policies, _, _, err := newResourcePolicyService(tx, access, memberships, documentService)
		if err != nil {
			return nil, err
		}
		delegate, err := collaborationgorm.NewCellMutationGuard(tx, bitableCellAccessEvaluator{
			policies: policies,
			store:    resourceaccessgorm.NewRepository(tx),
		})
		if err != nil {
			return nil, err
		}
		return bitableCellMutationGuard{delegate: delegate}, nil
	})
	repo.ConfigureCellLeaseConflictGuardFactory(newBitableCellLeaseConflictGuard)
	repo.ConfigureCommittedEventAppenderFactory(func(tx *gorm.DB) (bitable.CommittedEventAppender, error) {
		delegate, err := collaborationgorm.NewAppender(tx)
		if err != nil {
			return nil, err
		}
		return bitableCommittedEventAppender{
			resources: documentlibrarygorm.NewRepository(tx),
			policies:  resourceaccessgorm.NewRepository(tx),
			delegate:  delegate,
		}, nil
	})
	service, err := bitable.NewService(repo, repo, bitablePolicy{access, memberships}, repo)
	if err != nil {
		return nil, err
	}
	service.ConfigureUnifiedResourceAccess(
		bitableResourceAccessGate{
			resources: resourceAccess.Resources,
			policies:  resourceAccess.Policies,
			create:    bitablePolicy{access, memberships},
		},
		bitablePrivateResourceCoordinator{db: db},
	)
	people, err := identity.NewToolDirectoryService(identitygorm.NewToolDirectoryRepository(db))
	if err != nil {
		return nil, err
	}
	service.ConfigurePeopleDirectory(people)
	if documentService != nil {
		service.ConfigureFolderDirectory(bitableFolderDirectory{documentService})
	}
	repo.ConfigureValueNormalizer(service.NormalizeRecordUsers)
	storage, err := objectstore.New(cfg)
	if err != nil {
		return nil, err
	}
	if storage != nil {
		service.ConfigureAssetStorage(storage)
	}
	handler, err := bitablehttp.NewHandler(service)
	if err != nil {
		return nil, err
	}
	return handler.RegisterRoutes, nil
}

type bitableFolderDirectory struct{ service *document.Service }

func (d bitableFolderDirectory) ValidateBitableFolder(ctx context.Context, p bitable.Principal, id uint) error {
	if err := d.service.ValidateResourceFolder(ctx, document.Principal{UserID: p.UserID, WorkspaceID: p.WorkspaceID}, id); err != nil {
		return bitable.ErrForbidden
	}
	return nil
}

func (d bitableFolderDirectory) ReadBitableLocation(ctx context.Context, p bitable.Principal, id string, seed *uint) (*uint, uint64, error) {
	location, err := d.service.PersonalResourceLocation(ctx, document.Principal{UserID: p.UserID, WorkspaceID: p.WorkspaceID}, id, seed)
	return location.ParentID, location.Revision, err
}
func (d bitableFolderDirectory) MoveBitableLocation(ctx context.Context, p bitable.Principal, id string, parent *uint, revision uint64, operation string) (*uint, uint64, error) {
	location, err := d.service.MovePersonalResource(ctx, document.Principal{UserID: p.UserID, WorkspaceID: p.WorkspaceID}, id, parent, revision, operation)
	if errors.Is(err, document.ErrDirectoryConflict) {
		return nil, 0, bitable.ErrConflict
	}
	if err != nil {
		return nil, 0, bitable.ErrForbidden
	}
	return location.ParentID, location.Revision, nil
}
