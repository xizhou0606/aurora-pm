package api

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

// DocumentCollaborationCutoverExecutor is the non-HTTP command seam. Only the
// app composition root may assemble its cross-module dependencies.
type DocumentCollaborationCutoverExecutor interface {
	Inspect(context.Context, collaboration.Principal, collaboration.DocumentSeed) (bool, error)
	SeedAndCutover(context.Context, collaboration.Principal, collaboration.DocumentSeed) (collaboration.DocumentState, error)
}

// AssembleDocumentCollaborationCutoverExecutor constructs the same canonical
// resource, action, membership and projection checks used by online
// collaboration. It does not prepare schemas, bootstrap data or execute a
// cutover; the controlled maintenance command owns those lifecycle decisions.
func AssembleDocumentCollaborationCutoverExecutor(
	db *gorm.DB,
	assetConfig config.ObjectStorage,
) (DocumentCollaborationCutoverExecutor, error) {
	if db == nil {
		return nil, collaboration.ErrInvalidInput
	}
	actions := identity.NewAccessService(identitygorm.NewAccessRepository(db), nil)
	_, documents, err := assembleDocumentRoutes(db, actions, assetConfig, false)
	if err != nil {
		return nil, err
	}
	memberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, err
	}
	resourceAccess, err := assembleResourceAccess(db, actions, memberships, documents)
	if err != nil {
		return nil, err
	}
	return documentCollaborationCutover{
		db: db, resources: resourceAccess.Resources, policies: resourceAccess.Policies,
		access: collaborationAccessEvaluator{policies: resourceAccess.Policies},
		projection: collaborationProjectionValidator{
			documents: documents, resources: resourceAccess.Resources, policies: resourceAccess.Policies,
		},
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// documentCollaborationCutover is intentionally not an HTTP handler. A
// controlled seed job must generate and attest the Yjs bytes, then invoke this
// app-owned transaction for explicitly approved synthetic/document IDs.
type documentCollaborationCutover struct {
	db         *gorm.DB
	resources  documentlibrary.Resolver
	policies   *resourceaccess.PolicyService
	access     collaborationAccessEvaluator
	projection collaborationProjectionValidator
	now        func() time.Time
}

// Inspect performs the same authority, source and projection checks as apply,
// but does not open a write transaction or seed collaboration state. The bool
// reports an exact, already-completed cutover so controlled runners can count
// idempotent replays separately.
func (cutover documentCollaborationCutover) Inspect(
	ctx context.Context,
	principal collaboration.Principal,
	seed collaboration.DocumentSeed,
) (bool, error) {
	documentID, _, normalizedSeed, err := cutover.prepare(ctx, principal, seed)
	if err != nil {
		return false, err
	}
	source, err := documentgorm.NewRepository(cutover.db).GetDocument(ctx, seed.Room.WorkspaceID, documentID)
	if err != nil {
		return false, mapDocumentCollaborationValidationError(err)
	}
	if uint64(source.Revision) != normalizedSeed.SourceRevision || source.ContentHash != normalizedSeed.SourceHash {
		return false, collaboration.ErrConflict
	}
	repository, err := collaborationgorm.NewRepository(cutover.db)
	if err != nil {
		return false, err
	}
	state, stateErr := repository.LoadDocumentState(ctx, normalizedSeed.Room)
	if source.CollaborationCutoverAt == nil {
		if errors.Is(stateErr, collaboration.ErrNotFound) {
			return false, nil
		}
		// A durable seed without the legacy cutover marker cannot be produced by
		// the atomic coordinator and needs operator investigation.
		return false, collaboration.ErrDataCorrupt
	}
	if stateErr != nil || state.SourceRevision != normalizedSeed.SourceRevision ||
		state.SourceHash != normalizedSeed.SourceHash || state.ProjectionHash != normalizedSeed.ProjectionHash {
		return false, collaboration.ErrDataCorrupt
	}
	return true, nil
}

func (cutover documentCollaborationCutover) SeedAndCutover(
	ctx context.Context,
	principal collaboration.Principal,
	seed collaboration.DocumentSeed,
) (collaboration.DocumentState, error) {
	documentID, decision, seed, err := cutover.prepare(ctx, principal, seed)
	if err != nil {
		return collaboration.DocumentState{}, err
	}

	var state collaboration.DocumentState
	err = cutover.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		policy, lockErr := resourceaccessgorm.NewRepository(tx).LockCurrentPolicy(
			ctx, seed.Room.WorkspaceID, documentlibrary.ResourceID(seed.Room.ResourceID),
		)
		if lockErr != nil || policy.Revision != decision.PolicyRevision {
			return collaboration.ErrStalePolicy
		}
		source, lockErr := documentgorm.LockDocumentForCollaboration(
			ctx, tx, seed.Room.WorkspaceID, uint(documentID),
		)
		if lockErr != nil {
			return lockErr
		}
		if uint64(source.Revision) != seed.SourceRevision || source.ContentHash != seed.SourceHash {
			return collaboration.ErrConflict
		}
		store, storeErr := collaborationgorm.NewDocumentTransaction(tx)
		if storeErr != nil {
			return storeErr
		}
		state, storeErr = store.SeedDocument(ctx, seed)
		if storeErr != nil {
			return storeErr
		}
		if source.CollaborationCutoverAt != nil {
			return nil
		}
		return documentgorm.MarkDocumentCollaborationCutover(
			ctx, tx, seed.Room.WorkspaceID, uint(documentID), source.Revision, source.ContentHash, cutover.now().UTC(),
		)
	})
	if err != nil {
		if errors.Is(err, document.ErrRevisionConflict) {
			return collaboration.DocumentState{}, collaboration.ErrConflict
		}
		return collaboration.DocumentState{}, err
	}
	return state, nil
}

func (cutover documentCollaborationCutover) prepare(
	ctx context.Context,
	principal collaboration.Principal,
	seed collaboration.DocumentSeed,
) (uint, collaboration.AccessDecision, collaboration.DocumentSeed, error) {
	if cutover.db == nil || cutover.resources == nil || cutover.policies == nil || principal.WorkspaceID == 0 ||
		principal.UserID == 0 || principal.WorkspaceID != seed.Room.WorkspaceID || cutover.now == nil {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, collaboration.ErrInvalidInput
	}
	resource, binding, err := cutover.resources.ResolveResource(
		ctx, seed.Room.WorkspaceID, documentlibrary.ResourceID(seed.Room.ResourceID),
	)
	if err != nil || resource.TypeKey != documentlibrary.TypeRichText || binding.TypeKey != documentlibrary.TypeRichText ||
		resource.Lifecycle != "active" || resource.ArchivedAt != nil {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, collaboration.ErrForbidden
	}
	documentID, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil || documentID == 0 || uint64(uint(documentID)) != documentID {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, collaboration.ErrDataCorrupt
	}
	decision, err := cutover.access.EvaluateDocumentCollaborationAccess(ctx, principal, seed.Room)
	if err != nil || !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, collaboration.ErrForbidden
	}
	seed.ActorUserID, seed.PolicyRevision = principal.UserID, decision.PolicyRevision
	if collaboration.ValidateDocumentSeed(seed) != nil {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, collaboration.ErrInvalidInput
	}
	if err = cutover.projection.ValidateDocumentProjection(ctx, principal, seed.Room, seed.Projection); err != nil {
		return 0, collaboration.AccessDecision{}, collaboration.DocumentSeed{}, err
	}
	return uint(documentID), decision, seed, nil
}
