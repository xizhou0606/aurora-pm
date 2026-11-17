package resourceaccess

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"strconv"
	"strings"
	"time"
)

type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
	RequestRevoked  RequestStatus = "revoked"
	RequestExpired  RequestStatus = "expired"
)

const RequestLifetime = 72 * time.Hour

func RequestExpiresAt(created time.Time) time.Time { return created.Add(RequestLifetime) }
func RequestHasExpired(status RequestStatus, created, now time.Time) bool {
	return status == RequestPending && !now.Before(RequestExpiresAt(created))
}

type AccessRequest struct {
	GrantLevel      Level         `json:"grant_level"`
	GrantExpiresAt  *time.Time    `json:"grant_expires_at"`
	ID              uint          `json:"id"`
	ApplicantUserID uint          `json:"applicant_user_id"`
	ApplicantName   string        `json:"applicant_name"`
	Reason          string        `json:"reason"`
	NotifyOwner     bool          `json:"notify_owner"`
	Status          RequestStatus `json:"status"`
	Read            bool          `json:"read"`
	CreatedAt       time.Time     `json:"created_at"`
	ExpiresAt       time.Time     `json:"expires_at"`
	ReviewedAt      *time.Time    `json:"reviewed_at,omitempty"`
}
type RequestContext struct {
	ResourceType documentlibrary.TypeKey    `json:"resource_type"`
	ResourceID   documentlibrary.ResourceID `json:"resource_id"`
	OwnerUserID  uint                       `json:"owner_user_id"`
	OwnerName    string                     `json:"owner_name"`
	CanRequest   bool                       `json:"can_request"`
	CanReview    bool                       `json:"can_review"`
	HasAccess    bool                       `json:"has_access"`
	HasMore      bool                       `json:"has_more"`
	PendingCount int                        `json:"pending_count"`
	Items        []AccessRequest            `json:"items"`
}
type RequestMutation struct {
	GrantLevel    Level
	GrantDuration LinkDuration
	Principal     Principal
	ResourceID    documentlibrary.ResourceID
	OwnerUserID   uint
	RequestID     uint
	Reason        string
	NotifyOwner   bool
	Decision      RequestStatus
	Read          *bool
	Now           time.Time
}
type RequestStore interface {
	CountPendingAccessRequests(context.Context, Principal, documentlibrary.ResourceID, bool, time.Time) (int, error)
	ListAccessRequests(context.Context, Principal, documentlibrary.ResourceID, bool) ([]AccessRequest, error)
	CreateAccessRequest(context.Context, RequestMutation) (AccessRequest, error)
	UpdateAccessRequest(context.Context, RequestMutation) (AccessRequest, error)
}
type ApprovedRequestSource interface {
	ApprovedRequestResources(context.Context, Principal, []documentlibrary.ResourceID) (map[documentlibrary.ResourceID][]RequestGrant, error)
}
type RequestGrant struct {
	Level     Level
	ExpiresAt *time.Time
}
type RequestReview struct {
	Decision      RequestStatus
	Read          *bool
	GrantLevel    Level
	GrantDuration LinkDuration
}

func (g RequestGrant) Active(now time.Time) bool {
	return g.Level.Valid() && (g.ExpiresAt == nil || g.ExpiresAt.After(now))
}

type RequestService struct {
	policies         *PolicyService
	store            RequestStore
	authorizeRequest func(context.Context, Principal) error
	participantNames func(context.Context, []uint) (map[uint]string, error)
}

// Participant IDs come exclusively from the authorized resource and its scoped
// request records. Display lookup must not use the ACL subject picker: an
// admitted administrator can own a document without a membership row.
func (s *RequestService) ConfigureParticipantNames(resolve func(context.Context, []uint) (map[uint]string, error)) {
	s.participantNames = resolve
}

func (s *RequestService) ConfigureRequestAuthorizer(authorize func(context.Context, Principal) error) {
	s.authorizeRequest = authorize
}

func (s *PolicyService) RequestStore() RequestStore { store, _ := s.store.(RequestStore); return store }
func NewRequestService(policies *PolicyService, store RequestStore) *RequestService {
	return &RequestService{policies: policies, store: store}
}
func (s *RequestService) Context(ctx context.Context, p Principal, id documentlibrary.ResourceID) (RequestContext, error) {
	if !validPrincipal(p) {
		return RequestContext{}, ErrInvalidInput
	}
	resource, _, err := s.policies.resolver.RequireTypeOperation(ctx, p.WorkspaceID, id, documentlibrary.OperationView)
	if err != nil {
		return RequestContext{}, ErrNotFound
	}
	if (resource.TypeKey != documentlibrary.TypeRichText && resource.TypeKey != documentlibrary.TypeBitable) || resource.WorkspaceID != p.WorkspaceID || resource.ArchivedAt != nil || resource.Lifecycle == "archived" {
		return RequestContext{}, ErrNotFound
	}
	// Metadata disclosure is limited to this enabled workspace's admitted users.
	// This global check does not require an existing object-level read grant.
	globalView, err := s.policies.authorizer.AuthorizeDocumentResourceOperation(ctx, p, resource, documentlibrary.OperationViewAccess)
	if err != nil {
		return RequestContext{}, ErrUnavailable
	}
	if !globalView {
		return RequestContext{}, ErrNotFound
	}
	active, _, err := s.policies.subjectMatches(ctx, p, map[documentlibrary.ResourceID]Policy{})
	if err != nil {
		return RequestContext{}, err
	}
	if !active {
		return RequestContext{}, ErrNotFound
	}
	globalEdit, err := s.policies.authorizer.AuthorizeDocumentResourceOperation(ctx, p, resource, documentlibrary.OperationEdit)
	if err != nil {
		return RequestContext{}, ErrUnavailable
	}
	_, manageErr := s.policies.requireManagement(ctx, p, id)
	edit, err := s.policies.Evaluate(ctx, p, EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationEdit})
	if err != nil {
		return RequestContext{}, err
	}
	view, err := s.policies.Evaluate(ctx, p, EvaluationRequest{ResourceID: id, Operation: documentlibrary.OperationView})
	if err != nil {
		return RequestContext{}, err
	}
	result := RequestContext{ResourceID: id, ResourceType: resource.TypeKey, OwnerUserID: resource.OwnerUserID, CanReview: manageErr == nil, CanRequest: globalEdit && !edit.Allowed && resource.OwnerUserID != p.UserID}
	result.HasAccess = view.Allowed
	if resource.OwnerUserID == 0 {
		result.CanRequest = false
	}
	result.Items, err = s.store.ListAccessRequests(ctx, p, id, result.CanReview)
	if err != nil {
		return RequestContext{}, normalizeStoreError(err)
	}
	if len(result.Items) > 100 {
		result.HasMore = true
		result.Items = result.Items[:100]
	}
	now := s.policies.clock.Now().UTC()
	for i := range result.Items {
		result.Items[i].ExpiresAt = RequestExpiresAt(result.Items[i].CreatedAt)
		if RequestHasExpired(result.Items[i].Status, result.Items[i].CreatedAt, now) {
			result.Items[i].Status = RequestExpired
		}
	}
	ids := []uint{resource.OwnerUserID}
	for _, item := range result.Items {
		ids = append(ids, item.ApplicantUserID)
	}
	if s.participantNames == nil {
		return RequestContext{}, ErrUnavailable
	}
	names, err := s.participantNames(ctx, ids)
	if err != nil {
		return RequestContext{}, ErrUnavailable
	}
	result.OwnerName = names[resource.OwnerUserID]
	for i := range result.Items {
		result.Items[i].ApplicantName = names[result.Items[i].ApplicantUserID]
	}
	result.PendingCount, err = s.store.CountPendingAccessRequests(ctx, p, id, result.CanReview, now)
	if err != nil {
		return RequestContext{}, normalizeStoreError(err)
	}
	if !result.CanReview && result.PendingCount > 0 {
		result.CanRequest = false
	}
	return result, nil
}

func (s *RequestService) DocumentContext(ctx context.Context, p Principal, documentID uint) (RequestContext, error) {
	if !validPrincipal(p) || documentID == 0 {
		return RequestContext{}, ErrInvalidInput
	}
	resource, _, err := s.policies.resolver.ResolveContentResource(ctx, p.WorkspaceID, documentlibrary.TypeRichText, strconv.FormatUint(uint64(documentID), 10))
	if err != nil {
		return RequestContext{}, ErrNotFound
	}
	return s.Context(ctx, p, resource.ID)
}
func (s *RequestService) BitableContext(ctx context.Context, p Principal, ref string) (RequestContext, error) {
	if !validPrincipal(p) || strings.TrimSpace(ref) == "" {
		return RequestContext{}, ErrInvalidInput
	}
	// Bitable routes accept either a document-library resource UUID or a native UUID.
	resource, _, err := s.policies.resolver.ResolveResource(ctx, p.WorkspaceID, documentlibrary.ResourceID(ref))
	if err != nil {
		resource, _, err = s.policies.resolver.ResolveContentResource(ctx, p.WorkspaceID, documentlibrary.TypeBitable, ref)
	}
	if err != nil || resource.TypeKey != documentlibrary.TypeBitable {
		return RequestContext{}, ErrNotFound
	}
	return s.Context(ctx, p, resource.ID)
}
func (s *RequestService) Create(ctx context.Context, p Principal, id documentlibrary.ResourceID, reason string, notify bool) (AccessRequest, error) {
	if s.authorizeRequest == nil {
		return AccessRequest{}, ErrUnavailable
	}
	if err := s.authorizeRequest(ctx, p); err != nil {
		return AccessRequest{}, err
	}
	view, err := s.Context(ctx, p, id)
	if err != nil {
		return AccessRequest{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 1000 {
		return AccessRequest{}, ErrInvalidInput
	}
	for _, item := range view.Items {
		if item.ApplicantUserID == p.UserID && item.Status == "pending" {
			return item, nil
		}
	}
	if !view.CanRequest {
		return AccessRequest{}, ErrForbidden
	}
	result, err := s.store.CreateAccessRequest(ctx, RequestMutation{Principal: p, ResourceID: id, OwnerUserID: view.OwnerUserID, Reason: reason, NotifyOwner: notify, Now: s.policies.clock.Now().UTC()})
	return result, normalizeStoreError(err)
}
func (s *RequestService) Update(ctx context.Context, p Principal, id documentlibrary.ResourceID, requestID uint, input RequestReview) (AccessRequest, error) {
	decision, read := input.Decision, input.Read
	if decision == RequestApproved {
		if input.GrantLevel == "" {
			input.GrantLevel = LevelEditor
		}
		if input.GrantDuration == "" {
			input.GrantDuration = LinkDurationNever
		}
		if !input.GrantLevel.Valid() || !input.GrantDuration.Valid() {
			return AccessRequest{}, ErrInvalidInput
		}
	} else if input.GrantLevel != "" || input.GrantDuration != "" {
		return AccessRequest{}, ErrInvalidInput
	}
	resource, err := s.policies.requireManagement(ctx, p, id)
	if err != nil {
		return AccessRequest{}, err
	}
	if requestID == 0 || (decision != "" && RequestStatus(decision) != RequestApproved && RequestStatus(decision) != RequestRejected && RequestStatus(decision) != RequestRevoked) || (decision == "" && read == nil) {
		return AccessRequest{}, ErrInvalidInput
	}
	result, err := s.store.UpdateAccessRequest(ctx, RequestMutation{Principal: p, ResourceID: id, OwnerUserID: resource.OwnerUserID, RequestID: requestID, Decision: decision, Read: read, GrantLevel: input.GrantLevel, GrantDuration: input.GrantDuration, Now: s.policies.clock.Now().UTC()})
	return result, normalizeStoreError(err)
}
