package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type DocumentPolicyQueries interface {
	GetPolicy(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) (resourceaccess.PolicyView, error)
}
type DocumentRequestQueries interface {
	Context(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) (resourceaccess.RequestContext, error)
	DocumentContext(context.Context, resourceaccess.Principal, uint) (resourceaccess.RequestContext, error)
	BitableContext(context.Context, resourceaccess.Principal, string) (resourceaccess.RequestContext, error)
}
type DocumentLinkQueries interface {
	List(context.Context, resourceaccess.Principal, documentlibrary.ResourceID) ([]resourceaccess.ShareLink, error)
}
type DocumentAccessTools struct {
	Mutations *resourceaccess.ToolMutationService
	Users     TaskObjectPrincipalResolver
	Policies  DocumentPolicyQueries
	Requests  DocumentRequestQueries
	Links     DocumentLinkQueries
	Subjects  identity.SubjectDirectory
}
type documentAccessReadHandler struct {
	service *DocumentAccessTools
	name    string
}

func DocumentAccessReadHandlerNames() []string {
	return []string{"document.resource.policy.get", "document.access_request.list", "document.access_request.context", "document.bitable.access_request.context", "document.share_subject.search", "document.share_link.list"}
}
func (s *DocumentAccessTools) Handlers() (map[string]toolruntime.ToolHandler, error) {
	if s == nil || s.Users == nil || s.Policies == nil || s.Requests == nil || s.Links == nil || s.Subjects == nil {
		return nil, resourceaccess.ErrUnavailable
	}
	result := map[string]toolruntime.ToolHandler{}
	for _, name := range DocumentAccessReadHandlerNames() {
		result[name] = documentAccessReadHandler{service: s, name: name}
	}
	if s.Mutations != nil {
		for _, name := range DocumentAccessWriteHandlerNames() {
			result[name] = documentAccessWriteHandler{service: s, name: name}
		}
	}
	return result, nil
}

type documentAccessInput struct {
	ResourceID   string               `json:"resource_id"`
	ResourceType string               `json:"resource_type"`
	ResourceRef  string               `json:"resource_ref"`
	ID           uint                 `json:"id"`
	Kind         identity.SubjectKind `json:"kind"`
	Query        string               `json:"q"`
	Limit        int                  `json:"limit"`
	Offset       int                  `json:"offset"`
}

func (h documentAccessReadHandler) Invoke(ctx context.Context, call toolruntime.ToolCall) (json.RawMessage, error) {
	if call.Authorization.PrincipalID == 0 || call.Authorization.WorkspaceID == 0 {
		return nil, toolruntime.ErrAuthorizationDenied
	}
	var input documentAccessInput
	if decodeInput(call.Input, &input) != nil {
		return nil, toolruntime.ErrToolInputInvalid
	}
	p, err := documentAccessPrincipal(ctx, h.service.Users, call.Authorization)
	if err != nil {
		return nil, err
	}
	value, err := h.service.read(ctx, p, h.name, input)
	if err != nil {
		return nil, documentResourceToolError(err)
	}
	return json.Marshal(value)
}
func (s *DocumentAccessTools) read(ctx context.Context, p resourceaccess.Principal, name string, i documentAccessInput) (any, error) {
	id := documentlibrary.ResourceID(i.ResourceID)
	switch name {
	case "document.resource.policy.get":
		return s.Policies.GetPolicy(ctx, p, id)
	case "document.access_request.list":
		view, err := s.Requests.Context(ctx, p, id)
		if err == nil && i.ResourceType != "" && string(view.ResourceType) != i.ResourceType {
			return nil, resourceaccess.ErrNotFound
		}
		return view, err
	case "document.access_request.context":
		return s.Requests.DocumentContext(ctx, p, i.ID)
	case "document.bitable.access_request.context":
		return s.Requests.BitableContext(ctx, p, i.ResourceRef)
	case "document.share_link.list":
		items, err := s.Links.List(ctx, p, id)
		if items == nil {
			items = []resourceaccess.ShareLink{}
		}
		return map[string]any{"items": items}, err
	case "document.share_subject.search":
		if i.Limit == 0 {
			i.Limit = 20
		}
		if !i.Kind.Valid() || i.Limit < 1 || i.Limit > identity.MaximumSubjectPageSize || i.Offset < 0 || i.Offset%i.Limit != 0 {
			return nil, resourceaccess.ErrInvalidInput
		}
		page, err := s.Subjects.SearchSubjects(ctx, p.WorkspaceID, identity.SubjectSearch{Kind: i.Kind, Query: i.Query, Page: i.Offset/i.Limit + 1, PageSize: i.Limit})
		return map[string]any{"items": page.Items, "limit": i.Limit, "offset": i.Offset, "total": page.Total, "has_more": page.HasMore, "subject_capabilities": resourceaccess.SubjectCapabilities(s.Subjects)}, err
	default:
		return nil, toolruntime.ErrToolUnknown
	}
}

// Only the catalog's resource policy is handled here. Request-context access
// intentionally uses RequestService's narrower projection for non-viewers.
func (s *DocumentAccessTools) AuthorizeObjectAction(ctx context.Context, r toolruntime.ObjectActionRequest) (bool, error) {
	if r.PolicyCode != "document.resource-access" || r.Authorization.PrincipalID == 0 || r.Authorization.WorkspaceID == 0 {
		return false, nil
	}
	for _, name := range DocumentAccessWriteHandlerNames() {
		if r.ToolName != name {
			continue
		}
		var input documentAccessWriteInput
		if decodeInput(r.Input, &input) != nil {
			return false, nil
		}
		p, err := documentAccessPrincipal(ctx, s.Users, r.Authorization)
		if err != nil {
			return false, err
		}
		err = s.authorizeWrite(ctx, p, name, input)
		if errors.Is(err, resourceaccess.ErrForbidden) || errors.Is(err, resourceaccess.ErrNotFound) {
			return false, nil
		}
		return err == nil, documentResourceToolError(err)
	}
	switch r.ToolName {
	case "document.resource.policy.get", "document.access_request.list", "document.access_request.context", "document.bitable.access_request.context", "document.share_link.list":
	default:
		return false, nil
	}
	var input documentAccessInput
	if decodeInput(r.Input, &input) != nil {
		return false, nil
	}
	p, err := documentAccessPrincipal(ctx, s.Users, r.Authorization)
	if err != nil {
		return false, err
	}
	_, err = s.read(ctx, p, r.ToolName, input)
	if errors.Is(err, resourceaccess.ErrForbidden) || errors.Is(err, resourceaccess.ErrNotFound) {
		return false, nil
	}
	return err == nil, documentResourceToolError(err)
}
func documentResourceToolError(err error) error {
	switch {
	case errors.Is(err, resourceaccess.ErrConflict):
		return toolruntime.LookupBusinessError("DOCUMENT_ACCESS_CONFLICT")
	case errors.Is(err, resourceaccess.ErrIdempotencyConflict):
		return toolruntime.LookupBusinessError("DOCUMENT_IDEMPOTENCY_CONFLICT")
	case errors.Is(err, resourceaccess.ErrForbidden):
		return toolruntime.LookupBusinessError("DOCUMENT_FORBIDDEN")
	case errors.Is(err, resourceaccess.ErrNotFound):
		return toolruntime.LookupBusinessError("DOCUMENT_NOT_FOUND")
	case errors.Is(err, resourceaccess.ErrInvalidInput):
		return toolruntime.LookupBusinessError("DOCUMENT_INPUT_INVALID")
	case errors.Is(err, resourceaccess.ErrUnavailable), errors.Is(err, identity.ErrSubjectKindUnavailable):
		return toolruntime.LookupBusinessError("DOCUMENT_SERVICE_UNAVAILABLE")
	default:
		return err
	}
}

func documentAccessPrincipal(ctx context.Context, users TaskObjectPrincipalResolver, a toolruntime.ExecutionAuthorization) (resourceaccess.Principal, error) {
	if users == nil {
		return resourceaccess.Principal{}, toolruntime.ErrAuthorizationDenied
	}
	p, err := documentToolPrincipal(ctx, users, a)
	if err != nil {
		return resourceaccess.Principal{}, err
	}
	return resourceaccess.Principal{UserID: p.UserID, WorkspaceID: p.WorkspaceID, ActorFactUserID: p.ParticipantUserID}, nil
}
