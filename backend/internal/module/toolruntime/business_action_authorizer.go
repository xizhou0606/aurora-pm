package toolruntime

import (
	"context"
	"encoding/json"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

// EffectiveActionReader exposes the authoritative role + user-group Action
// union without coupling Tool Runtime to identity persistence.
type EffectiveActionReader interface {
	EffectiveActionCodes(context.Context, uint) ([]string, error)
}

type ObjectActionRequest struct {
	PolicyCode    string
	ActionCode    string
	ToolName      string
	Authorization ExecutionAuthorization
	Input         json.RawMessage
}

// ObjectActionAuthorizer evaluates a catalog-declared object policy. It is
// called only after the principal lacks the global Action permission.
type ObjectActionAuthorizer interface {
	AuthorizeObjectAction(context.Context, ObjectActionRequest) (bool, error)
}

type CatalogBusinessActionAuthorizer struct {
	actions EffectiveActionReader
	objects ObjectActionAuthorizer
}

func NewCatalogBusinessActionAuthorizer(
	actions EffectiveActionReader,
	objects ObjectActionAuthorizer,
) (*CatalogBusinessActionAuthorizer, error) {
	if actions == nil {
		return nil, ErrExecutorInvalid
	}
	return &CatalogBusinessActionAuthorizer{actions: actions, objects: objects}, nil
}

func (authorizer *CatalogBusinessActionAuthorizer) AuthorizeBusinessAction(
	ctx context.Context,
	request BusinessActionRequest,
) (BusinessActionDecision, error) {
	if authorizer == nil || authorizer.actions == nil || request.Authorization.PrincipalID == 0 ||
		request.Authorization.WorkspaceID == 0 {
		return BusinessActionDenied, ErrAuthorizationDenied
	}
	action, mapped := accesscatalog.ActionForTool(request.ToolName)
	if !mapped {
		// Baseline and explicitly actionless Tools are governed by their exact
		// Grant/scope and catalog classification, not editable role Actions.
		return BusinessActionAllowed, nil
	}
	codes, err := authorizer.actions.EffectiveActionCodes(ctx, request.Authorization.PrincipalID)
	if err != nil {
		return BusinessActionDenied, err
	}
	for _, grantingAction := range accesscatalog.GrantingActions(action) {
		for _, code := range codes {
			if code == string(grantingAction) {
				return BusinessActionAllowed, nil
			}
		}
	}
	policy, conditional := accesscatalog.ObjectPolicyForAction(action)
	if !conditional || authorizer.objects == nil {
		return BusinessActionDenied, nil
	}
	if len(request.Input) == 0 {
		return BusinessActionConditional, nil
	}
	allowed, err := authorizer.objects.AuthorizeObjectAction(ctx, ObjectActionRequest{
		PolicyCode: string(policy.Code), ActionCode: string(action), ToolName: request.ToolName,
		Authorization: request.Authorization, Input: append(json.RawMessage(nil), request.Input...),
	})
	if err != nil {
		return BusinessActionDenied, err
	}
	if allowed {
		return BusinessActionAllowed, nil
	}
	return BusinessActionDenied, nil
}

func (authorizer *CatalogBusinessActionAuthorizer) PreflightBusinessActions(
	ctx context.Context,
	toolNames []string,
	authorization ExecutionAuthorization,
) (map[string]BusinessActionDecision, error) {
	if authorizer == nil || authorizer.actions == nil || authorization.PrincipalID == 0 || authorization.WorkspaceID == 0 {
		return nil, ErrAuthorizationDenied
	}
	codes, err := authorizer.actions.EffectiveActionCodes(ctx, authorization.PrincipalID)
	if err != nil {
		return nil, err
	}
	allowedActions := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		allowedActions[code] = struct{}{}
	}
	result := make(map[string]BusinessActionDecision, len(toolNames))
	for _, toolName := range toolNames {
		action, mapped := accesscatalog.ActionForTool(toolName)
		if !mapped {
			result[toolName] = BusinessActionAllowed
			continue
		}
		for _, grantingAction := range accesscatalog.GrantingActions(action) {
			if _, allowed := allowedActions[string(grantingAction)]; allowed {
				result[toolName] = BusinessActionAllowed
				break
			}
		}
		if result[toolName] == BusinessActionAllowed {
			continue
		}
		if _, conditional := accesscatalog.ObjectPolicyForAction(action); conditional && authorizer.objects != nil {
			result[toolName] = BusinessActionConditional
		} else {
			result[toolName] = BusinessActionDenied
		}
	}
	return result, nil
}

var _ BusinessActionAuthorizer = (*CatalogBusinessActionAuthorizer)(nil)

// ObjectPolicyAuthorizers dispatches only the policy selected by the catalog.
// It does not combine unrelated policies into a permissive OR decision.
type ObjectPolicyAuthorizers map[string]ObjectActionAuthorizer

func (authorizers ObjectPolicyAuthorizers) AuthorizeObjectAction(ctx context.Context, request ObjectActionRequest) (bool, error) {
	authorizer := authorizers[request.PolicyCode]
	if authorizer == nil {
		return false, nil
	}
	return authorizer.AuthorizeObjectAction(ctx, request)
}
