package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func DocumentAccessWriteHandlerNames() []string {
	return []string{"document.resource.policy.update", "document.access_request.create", "document.access_request.decide", "document.share_link.revoke"}
}

type documentAccessWriteInput struct {
	ResourceID             documentlibrary.ResourceID   `json:"resource_id"`
	Mode                   resourceaccess.Mode          `json:"mode"`
	WorkspaceLevel         *resourceaccess.Level        `json:"workspace_level"`
	Rules                  []resourceaccess.Rule        `json:"rules"`
	ExpectedPolicyRevision uint64                       `json:"expected_policy_revision"`
	Reason                 string                       `json:"reason"`
	NotifyOwner            *bool                        `json:"notify_owner"`
	RequestID              uint                         `json:"request_id"`
	Decision               resourceaccess.RequestStatus `json:"decision"`
	GrantLevel             resourceaccess.Level         `json:"grant_level"`
	GrantDuration          resourceaccess.LinkDuration  `json:"grant_duration"`
	Read                   *bool                        `json:"read"`
	LinkID                 string                       `json:"link_id"`
}

type documentAccessWriteHandler struct {
	service *DocumentAccessTools
	name    string
}

func (h documentAccessWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (h documentAccessWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if call.ApplyKey == "" || call.ChangeSetID == "" {
		return nil, toolruntime.ErrChangeSetRequired
	}
	p, err := documentAccessPrincipal(ctx, h.service.Users, call.Authorization)
	if err != nil {
		return nil, err
	}
	var input documentAccessWriteInput
	if decodeInput(call.Input, &input) != nil {
		return nil, toolruntime.ErrToolInputInvalid
	}
	if err := h.service.authorizeWrite(ctx, p, h.name, input); err != nil {
		return nil, documentResourceToolError(err)
	}
	digest := sha256.Sum256(append([]byte(h.name+"\n"), call.Input...))
	output, err := h.service.Mutations.Apply(ctx, p, input.ResourceID, call.ApplyKey, hex.EncodeToString(digest[:]), func(s *resourceaccess.ToolMutationService) (json.RawMessage, error) {
		var result any
		var err error
		switch h.name {
		case "document.resource.policy.update":
			result, err = s.Policies.ReplacePolicy(ctx, p, input.ResourceID, resourceaccess.ReplacePolicyInput{Mode: input.Mode, WorkspaceLevel: input.WorkspaceLevel, Rules: input.Rules, ExpectedPolicyRevision: input.ExpectedPolicyRevision, IdempotencyKey: call.ApplyKey})
		case "document.access_request.create":
			result, err = s.Requests.Create(ctx, p, input.ResourceID, input.Reason, input.NotifyOwner == nil || *input.NotifyOwner)
		case "document.access_request.decide":
			result, err = s.Requests.Update(ctx, p, input.ResourceID, input.RequestID, resourceaccess.RequestReview{Decision: input.Decision, GrantLevel: input.GrantLevel, GrantDuration: input.GrantDuration, Read: input.Read})
		case "document.share_link.revoke":
			err = s.Links.Revoke(ctx, p, input.ResourceID, input.LinkID)
			result = map[string]any{"resource_id": input.ResourceID, "link_id": input.LinkID, "revoked": true}
		default:
			return nil, toolruntime.ErrToolUnknown
		}
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	return output, documentResourceToolError(err)
}

func (s *DocumentAccessTools) authorizeWrite(ctx context.Context, p resourceaccess.Principal, name string, input documentAccessWriteInput) error {
	if s.Mutations == nil || s.Mutations.Policies == nil || s.Mutations.Requests == nil {
		return resourceaccess.ErrUnavailable
	}
	switch name {
	case "document.access_request.create":
		// Match HTTP's request-context gate. Existing body access is not required.
		_, err := s.Mutations.Requests.Context(ctx, p, input.ResourceID)
		return err
	case "document.resource.policy.update", "document.access_request.decide":
		return s.Mutations.Policies.AuthorizeOperation(ctx, p, input.ResourceID, documentlibrary.OperationManageAccess)
	case "document.share_link.revoke":
		return s.Mutations.Policies.AuthorizeOperation(ctx, p, input.ResourceID, documentlibrary.OperationManageLinks)
	default:
		return toolruntime.ErrToolUnknown
	}
}
