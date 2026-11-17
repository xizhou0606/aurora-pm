package api

import (
	"context"
	documentread "github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const collaborationConnectionHeader = "X-PM-Collaboration-Connection"

var collaborationResourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

type collaborationRuntime struct {
	Tickets             collaborationhttp.ConnectionTicketIssuer
	MountRoutes         func(*gin.RouterGroup)
	MountInternalRoutes func(*gin.RouterGroup)
}

type collaborationRequestResolver struct {
	resources documentlibrary.Resolver
}

func (resolver collaborationRequestResolver) ResolveCollaborationPrincipal(
	c *gin.Context,
) (collaboration.Principal, error) {
	principalValue, principalOK := c.Get(pmauth.PrincipalContextKey)
	principal, principalTyped := principalValue.(pmauth.Principal)
	workspaceValue, workspaceOK := c.Get(pmauth.WorkspaceIDContextKey)
	workspaceID, workspaceTyped := workspaceValue.(uint)
	connectionID := strings.TrimSpace(c.GetHeader(collaborationConnectionHeader))
	parsedConnectionID, connectionErr := uuid.Parse(connectionID)
	linkAccessSecret := strings.TrimSpace(c.GetHeader("X-PM-Resource-Access"))
	if !principalOK || !principalTyped || !workspaceOK || !workspaceTyped || principal.UserID == 0 || workspaceID == 0 ||
		connectionErr != nil || parsedConnectionID == uuid.Nil || parsedConnectionID.String() != connectionID ||
		len(linkAccessSecret) > 256 {
		return collaboration.Principal{}, collaboration.ErrForbidden
	}
	displayName := strings.TrimSpace(principal.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(principal.Username)
	}
	if displayName == "" {
		return collaboration.Principal{}, collaboration.ErrForbidden
	}
	return collaboration.Principal{
		WorkspaceID: workspaceID, UserID: principal.UserID, DisplayName: displayName,
		ConnectionID: connectionID, LinkAccessSecret: linkAccessSecret,
	}, nil
}

func (resolver collaborationRequestResolver) ResolveCollaborationRoom(
	c *gin.Context,
	principal collaboration.Principal,
) (collaboration.Room, error) {
	resourceID := strings.TrimSpace(c.Param("resourceId"))
	if resolver.resources == nil || principal.WorkspaceID == 0 || !collaborationResourceIDPattern.MatchString(resourceID) {
		return collaboration.Room{}, collaboration.ErrForbidden
	}
	resource, _, err := resolver.resources.ResolveResource(
		c.Request.Context(), principal.WorkspaceID, documentlibrary.ResourceID(resourceID),
	)
	if err != nil || resource.ID != documentlibrary.ResourceID(resourceID) || resource.Lifecycle != "active" ||
		resource.ArchivedAt != nil {
		return collaboration.Room{}, collaboration.ErrForbidden
	}
	return collaboration.Room{WorkspaceID: principal.WorkspaceID, ResourceID: resourceID}, nil
}

type collaborationAccessEvaluator struct{ policies *resourceaccess.PolicyService }

type documentPolicyRevisionLocker interface {
	LockCurrentPolicy(context.Context, uint, documentlibrary.ResourceID) (resourceaccess.Policy, error)
}

type collaborationActiveResourceLocker interface {
	LockActiveResource(context.Context, uint, documentlibrary.ResourceID) (documentlibrary.Resource, documentlibrary.ContentBinding, error)
}

type collaborationAssetLocker interface {
	LockCollaborationAssetReferences(context.Context, uint, uint, []uint) error
}

type collaborationPolicyEvaluator interface {
	Evaluate(context.Context, resourceaccess.Principal, resourceaccess.EvaluationRequest) (resourceaccess.Evaluation, error)
	BatchEvaluate(context.Context, resourceaccess.Principal, []resourceaccess.EvaluationRequest) ([]resourceaccess.Evaluation, error)
}

type collaborationDocumentCommitGuard struct {
	resources collaborationActiveResourceLocker
	assets    collaborationAssetLocker
	policies  collaborationPolicyEvaluator
	policyDB  documentPolicyRevisionLocker
}

func (guard collaborationDocumentCommitGuard) VerifyDocumentCommit(
	ctx context.Context,
	principal collaboration.Principal,
	input collaboration.DocumentUpdate,
) error {
	if guard.resources == nil || guard.assets == nil || guard.policies == nil || guard.policyDB == nil ||
		principal.WorkspaceID == 0 || principal.UserID == 0 || principal.WorkspaceID != input.Room.WorkspaceID ||
		input.PolicyRevision == 0 {
		return collaboration.ErrForbidden
	}
	references, err := extractCollaborationProjectionReferences(input.Projection)
	if err != nil {
		return err
	}
	resource, binding, err := guard.resources.LockActiveResource(
		ctx, input.Room.WorkspaceID, documentlibrary.ResourceID(input.Room.ResourceID),
	)
	if err != nil || resource.TypeKey != documentlibrary.TypeRichText || binding.TypeKey != documentlibrary.TypeRichText ||
		binding.ResourceID != resource.ID {
		return collaboration.ErrForbidden
	}
	documentID64, err := strconv.ParseUint(binding.PayloadRef, 10, 64)
	if err != nil || documentID64 == 0 || uint64(uint(documentID64)) != documentID64 {
		return collaboration.ErrDataCorrupt
	}

	bitableIDs := make([]documentlibrary.ResourceID, 0, len(references.BitableResourceIDs))
	for _, value := range references.BitableResourceIDs {
		bitableIDs = append(bitableIDs, documentlibrary.ResourceID(value))
	}
	sort.Slice(bitableIDs, func(left, right int) bool { return bitableIDs[left] < bitableIDs[right] })
	for _, resourceID := range bitableIDs {
		embedded, embeddedBinding, lockErr := guard.resources.LockActiveResource(ctx, input.Room.WorkspaceID, resourceID)
		if lockErr != nil || embedded.TypeKey != documentlibrary.TypeBitable ||
			embeddedBinding.TypeKey != documentlibrary.TypeBitable || embeddedBinding.ResourceID != embedded.ID {
			return collaboration.ErrForbidden
		}
	}

	assetIDs := make([]uint, 0, len(references.AssetIDs))
	for _, value := range references.AssetIDs {
		assetID, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil || assetID == 0 || uint64(uint(assetID)) != assetID {
			return collaboration.ErrInvalidInput
		}
		assetIDs = append(assetIDs, uint(assetID))
	}
	if err = guard.assets.LockCollaborationAssetReferences(
		ctx, input.Room.WorkspaceID, uint(documentID64), assetIDs,
	); err != nil {
		return mapDocumentCollaborationValidationError(err)
	}

	policyIDs := append([]documentlibrary.ResourceID{resource.ID}, bitableIDs...)
	sort.Slice(policyIDs, func(left, right int) bool { return policyIDs[left] < policyIDs[right] })
	for _, resourceID := range policyIDs {
		policy, lockErr := guard.policyDB.LockCurrentPolicy(ctx, input.Room.WorkspaceID, resourceID)
		if lockErr != nil {
			return mapCollaborationResourceAccessError(lockErr)
		}
		if resourceID == resource.ID && policy.Revision != input.PolicyRevision {
			return collaboration.ErrStalePolicy
		}
	}

	resourcePrincipal := collaborationResourcePrincipal(principal)
	mainDecisions, err := guard.policies.BatchEvaluate(ctx, resourcePrincipal, []resourceaccess.EvaluationRequest{
		{ResourceID: resource.ID, Operation: documentlibrary.OperationCollaborate},
		{ResourceID: resource.ID, Operation: documentlibrary.OperationEdit},
	})
	if err != nil || len(mainDecisions) != 2 || !mainDecisions[0].Allowed || !mainDecisions[1].Allowed ||
		mainDecisions[0].PolicyRevision != input.PolicyRevision || mainDecisions[1].PolicyRevision != input.PolicyRevision {
		return collaboration.ErrForbidden
	}
	for _, resourceID := range bitableIDs {
		decision, evaluateErr := guard.policies.Evaluate(ctx, resourcePrincipal, resourceaccess.EvaluationRequest{
			ResourceID: resourceID, Operation: documentlibrary.OperationView,
		})
		if evaluateErr != nil || !decision.Allowed || decision.PolicyRevision == 0 {
			return collaboration.ErrForbidden
		}
	}
	return nil
}

type unavailableCollaborationTicketIssuer struct{}

func (unavailableCollaborationTicketIssuer) IssueDocumentConnectionTicket(
	context.Context, collaboration.Principal, collaboration.Room,
) (collaborationhttp.ConnectionTicket, error) {
	return collaborationhttp.ConnectionTicket{}, collaboration.ErrForbidden
}

type activeCollaborationTicketIssuer struct {
	authority *collaborationDocumentAuthority
	documents collaborationAuthorityDocumentLoader
	publicURL string
}

type collaborationAuthorityDocumentLoader interface {
	Load(context.Context, collaboration.Principal, collaboration.Room) (collaboration.DocumentState, error)
}

func (issuer activeCollaborationTicketIssuer) IssueDocumentConnectionTicket(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (collaborationhttp.ConnectionTicket, error) {
	if issuer.authority == nil || issuer.documents == nil || issuer.publicURL == "" {
		return collaborationhttp.ConnectionTicket{}, collaboration.ErrForbidden
	}
	state, err := issuer.documents.Load(ctx, principal, room)
	if err != nil || state.DurableRevision == 0 || state.Room != room {
		if err != nil {
			return collaborationhttp.ConnectionTicket{}, err
		}
		return collaborationhttp.ConnectionTicket{}, collaboration.ErrDataCorrupt
	}
	ticket, expiresAt, err := issuer.authority.IssueDocumentConnectionTicket(ctx, principal, room)
	if err != nil {
		return collaborationhttp.ConnectionTicket{}, err
	}
	return collaborationhttp.ConnectionTicket{
		Ticket: ticket, WebSocketURL: issuer.publicURL, ExpiresAt: expiresAt,
	}, nil
}

func (evaluator collaborationAccessEvaluator) EvaluateCollaborationAccess(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (collaboration.AccessDecision, error) {
	if evaluator.policies == nil || principal.WorkspaceID != room.WorkspaceID {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	requests := []resourceaccess.EvaluationRequest{
		{ResourceID: documentlibrary.ResourceID(room.ResourceID), Operation: documentlibrary.OperationCollaborate},
		{ResourceID: documentlibrary.ResourceID(room.ResourceID), Operation: documentlibrary.OperationEdit},
	}
	evaluations, err := evaluator.policies.BatchEvaluate(ctx, collaborationResourcePrincipal(principal), requests)
	if err != nil {
		return collaboration.AccessDecision{}, mapCollaborationResourceAccessError(err)
	}
	if len(evaluations) != len(requests) || evaluations[0].PolicyRevision == 0 ||
		evaluations[0].PolicyRevision != evaluations[1].PolicyRevision {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	expiresAt := time.Now().UTC().Add(10 * time.Second)
	for _, evaluation := range evaluations {
		if evaluation.ExpiresAt != nil && evaluation.ExpiresAt.Before(expiresAt) {
			expiresAt = evaluation.ExpiresAt.UTC()
		}
	}
	return collaboration.AccessDecision{
		CanView: evaluations[0].Allowed, CanEdit: evaluations[1].Allowed,
		PolicyRevision: evaluations[0].PolicyRevision, ExpiresAt: expiresAt,
	}, nil
}

func (evaluator collaborationAccessEvaluator) EvaluateDocumentCollaborationAccess(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
) (collaboration.AccessDecision, error) {
	return evaluator.EvaluateCollaborationAccess(ctx, principal, room)
}

func (evaluator collaborationAccessEvaluator) EvaluateCellEditAccess(
	ctx context.Context,
	principal collaboration.Principal,
	key collaboration.CellKey,
) (collaboration.AccessDecision, error) {
	if evaluator.policies == nil || principal.WorkspaceID != key.Room.WorkspaceID {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	evaluation, err := evaluator.policies.Evaluate(ctx, collaborationResourcePrincipal(principal), resourceaccess.EvaluationRequest{
		ResourceID: documentlibrary.ResourceID(key.Room.ResourceID),
		Operation:  documentlibrary.OperationCellLease,
	})
	if err != nil {
		return collaboration.AccessDecision{}, mapCollaborationResourceAccessError(err)
	}
	if !evaluation.Allowed || evaluation.PolicyRevision == 0 {
		return collaboration.AccessDecision{}, collaboration.ErrForbidden
	}
	expiresAt := time.Now().UTC().Add(collaboration.DefaultCellLeaseTTL)
	if evaluation.ExpiresAt != nil && evaluation.ExpiresAt.Before(expiresAt) {
		expiresAt = evaluation.ExpiresAt.UTC()
	}
	return collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: evaluation.PolicyRevision,
		ExpiresAt: expiresAt,
	}, nil
}

func collaborationResourcePrincipal(principal collaboration.Principal) resourceaccess.Principal {
	return resourceaccess.Principal{
		WorkspaceID:      principal.WorkspaceID,
		UserID:           principal.UserID,
		LinkAccessSecret: principal.LinkAccessSecret,
	}
}

func assembleCollaborationRoutes(
	db *gorm.DB,
	resourceAccess resourceAccessRuntime,
	documents *document.Service,
	collaborationConfig config.Collaboration,
) (collaborationRuntime, error) {
	if db == nil || resourceAccess.Resources == nil || resourceAccess.Policies == nil || documents == nil {
		return collaborationRuntime{}, collaboration.ErrInvalidInput
	}
	repository, err := collaborationgorm.NewRepository(db)
	if err != nil {
		return collaborationRuntime{}, err
	}
	repository.ConfigureDocumentCommitGuardFactory(func(tx *gorm.DB) (collaboration.DocumentCommitGuard, error) {
		txPolicies, _, _, guardErr := newResourcePolicyService(
			tx, resourceAccess.Actions, resourceAccess.Memberships, documents,
		)
		if guardErr != nil {
			return nil, guardErr
		}
		return collaborationDocumentCommitGuard{
			resources: documentlibrarygorm.NewRepository(tx),
			assets:    documentgorm.NewRepository(tx),
			policies:  txPolicies,
			policyDB:  resourceaccessgorm.NewRepository(tx),
		}, nil
	})
	documents.ConfigureCollaborationProjection(documentread.CollaborationProjectionReader{States: repository})
	evaluator := collaborationAccessEvaluator{policies: resourceAccess.Policies}
	projectionValidator := collaborationProjectionValidator{
		documents: documents, resources: resourceAccess.Resources, policies: resourceAccess.Policies,
	}
	documentCollaboration, err := collaboration.NewDocumentCollaborationService(
		repository, repository, evaluator, projectionValidator,
	)
	if err != nil {
		return collaborationRuntime{}, err
	}
	subscriptions, err := collaboration.NewSubscriptionService(repository, repository, evaluator, collaboration.StreamOptions{
		PollInterval: 250 * time.Millisecond, ReauthorizeInterval: 5 * time.Second, ReplayLimit: 200,
	})
	if err != nil {
		return collaborationRuntime{}, err
	}
	leases, err := collaboration.NewCellLeaseService(repository, evaluator, collaboration.DefaultCellLeaseTTL)
	if err != nil {
		return collaborationRuntime{}, err
	}
	presence, err := collaboration.NewPresenceService(repository, evaluator)
	if err != nil {
		return collaborationRuntime{}, err
	}
	resolver := collaborationRequestResolver{resources: resourceAccess.Resources}
	streamHandler, err := collaborationhttp.NewHandler(resolver, resolver, subscriptions)
	if err != nil {
		return collaborationRuntime{}, err
	}
	leaseHandler, err := collaborationhttp.NewLeaseHandler(resolver, resolver, leases)
	if err != nil {
		return collaborationRuntime{}, err
	}
	presenceHandler, err := collaborationhttp.NewPresenceHandler(resolver, resolver, presence)
	if err != nil {
		return collaborationRuntime{}, err
	}
	var ticketIssuer collaborationhttp.ConnectionTicketIssuer = unavailableCollaborationTicketIssuer{}
	var mountInternalRoutes func(*gin.RouterGroup)
	if collaborationConfig.Enabled {
		tokens, tokenErr := securetoken.New([]byte(collaborationConfig.TokenKey))
		if tokenErr != nil {
			return collaborationRuntime{}, tokenErr
		}
		authority, authorityErr := newCollaborationDocumentAuthority(
			tokens, evaluator, documentCollaboration, projectionValidator,
			collaborationConfig.TicketTTL, collaborationConfig.GrantTTL, collaborationConfig.ValidationTTL,
			func() time.Time { return time.Now().UTC() },
		)
		if authorityErr != nil {
			return collaborationRuntime{}, authorityErr
		}
		serviceTokens, verifierErr := newCollaborationServiceTokenVerifier(collaborationConfig.ServiceToken)
		if verifierErr != nil {
			return collaborationRuntime{}, verifierErr
		}
		authorityHandler, handlerErr := collaborationhttp.NewAuthorityHandler(serviceTokens, authority)
		if handlerErr != nil {
			return collaborationRuntime{}, handlerErr
		}
		ticketIssuer = activeCollaborationTicketIssuer{
			authority: authority, documents: documentCollaboration, publicURL: collaborationConfig.PublicURL,
		}
		mountInternalRoutes = authorityHandler.RegisterRoutes
	}
	ticketHandler, err := collaborationhttp.NewTicketHandler(resolver, resolver, ticketIssuer)
	if err != nil {
		return collaborationRuntime{}, err
	}
	return collaborationRuntime{Tickets: ticketIssuer, MountRoutes: func(group *gin.RouterGroup) {
		ticketHandler.RegisterRoutes(group)
		streamHandler.RegisterRoutes(group)
		leaseHandler.RegisterRoutes(group)
		presenceHandler.RegisterRoutes(group)
	}, MountInternalRoutes: mountInternalRoutes}, nil
}

var _ collaborationhttp.PrincipalResolver = collaborationRequestResolver{}
var _ collaborationhttp.RoomResolver = collaborationRequestResolver{}
var _ collaboration.AccessEvaluator = collaborationAccessEvaluator{}
var _ collaboration.DocumentCollaborationAccessEvaluator = collaborationAccessEvaluator{}
var _ collaboration.DocumentCommitGuard = collaborationDocumentCommitGuard{}
var _ collaboration.CellLeaseAccessEvaluator = collaborationAccessEvaluator{}

var _ collaborationhttp.ConnectionTicketIssuer = unavailableCollaborationTicketIssuer{}
var _ collaborationhttp.ConnectionTicketIssuer = activeCollaborationTicketIssuer{}
