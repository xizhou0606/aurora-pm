package documentread

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

type ActionResolver struct {
	Access      *identity.AccessService
	Memberships *identity.WorkspaceMembershipService
}

func (resolver ActionResolver) EffectiveDocumentActionCodes(
	ctx context.Context,
	userID uint,
	workspaceID uint,
) ([]string, error) {
	if err := resolver.Memberships.VerifyWorkspaceMembership(ctx, userID, workspaceID); err != nil {
		return nil, document.ErrForbidden
	}
	return resolver.Access.EffectiveActionCodes(ctx, userID)
}

type RequirementResolver struct {
	Service *requirement.DocumentParticipantService
}

func (resolver RequirementResolver) ResolveRequirementParticipants(
	ctx context.Context,
	workspaceID uint,
	requirementID uint,
) (document.RequirementParticipants, error) {
	value, err := resolver.Service.ResolveDocumentParticipants(ctx, workspaceID, requirementID)
	if err != nil {
		return document.RequirementParticipants{}, err
	}
	return document.RequirementParticipants{
		RequirementID: value.RequirementID, WorkspaceID: value.WorkspaceID, Version: value.Version, Active: value.Active,
		ProductOwnerUserIDs: value.ProductOwnerUserIDs, DesignOwnerUserIDs: value.DesignOwnerUserIDs,
		FrontendOwnerUserIDs: value.FrontendOwnerUserIDs, BackendOwnerUserIDs: value.BackendOwnerUserIDs,
		TesterUserIDs: value.TesterUserIDs, DeveloperUserIDs: value.DeveloperUserIDs,
	}, nil
}
