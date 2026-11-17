package document

import (
	"context"
	"sort"
)

type objectRole uint8

const (
	roleNone objectRole = iota
	roleViewer
	roleCommenter
	roleEditor
	roleManager
)

type accessState struct {
	actions  map[string]struct{}
	scopeAll bool
}

func (service *Service) accessState(ctx context.Context, principal Principal) (accessState, error) {
	if principal.UserID == 0 || principal.WorkspaceID == 0 {
		return accessState{}, ErrForbidden
	}
	codes, err := service.actions.EffectiveDocumentActionCodes(ctx, principal.UserID, principal.WorkspaceID)
	if err != nil {
		return accessState{}, err
	}
	state := accessState{actions: make(map[string]struct{}, len(codes))}
	for _, code := range codes {
		state.actions[code] = struct{}{}
	}
	_, state.scopeAll = state.actions[string(ActionScopeAll)]
	return state, nil
}

func (state accessState) has(action string) bool {
	_, ok := state.actions[action]
	return ok
}

func (service *Service) documentRole(
	ctx context.Context,
	principal Principal,
	state accessState,
	documentID uint,
) (objectRole, error) {
	if service.resourceAccess != nil {
		access, err := service.resourceAccess.ResolveDocumentAccess(ctx, principal, documentID)
		if err != nil {
			return roleNone, err
		}
		switch {
		case access.CanManage:
			return roleManager, nil
		case access.CanEdit:
			return roleEditor, nil
		case access.CanComment:
			return roleCommenter, nil
		case access.CanView:
			return roleViewer, nil
		default:
			return roleNone, nil
		}
	}
	return service.legacyDocumentRole(ctx, principal, state, documentID)
}

func (service *Service) legacyDocumentRole(
	ctx context.Context,
	principal Principal,
	state accessState,
	documentID uint,
) (objectRole, error) {
	if state.scopeAll {
		return roleManager, nil
	}
	bindings, err := service.store.ListBindings(ctx, principal.WorkspaceID, documentID)
	if err != nil {
		return roleNone, err
	}
	role := roleNone
	participantUserID := principal.requirementParticipantID()
	for _, binding := range bindings {
		participants, err := service.requirements.ResolveRequirementParticipants(
			ctx,
			principal.WorkspaceID,
			binding.RequirementID,
		)
		if err != nil || !participants.Active || participants.WorkspaceID != principal.WorkspaceID {
			continue
		}
		if includesUser(participants.TesterUserIDs, participantUserID) && role < roleCommenter {
			role = roleCommenter
		}
		if includesUser(editorParticipantIDs(participants), participantUserID) {
			role = roleEditor
		}
	}
	return role, nil
}

// EvaluateLegacyResourceAccess is consumed only by the unified policy's
// explicit legacy adapter. It never calls the unified resolver, preventing a
// legacy-policy recursion and keeping scope.all out of every non-legacy mode.
func (service *Service) EvaluateLegacyResourceAccess(
	ctx context.Context,
	principal Principal,
	documentID uint,
) (ResourceAccess, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return ResourceAccess{}, err
	}
	role, err := service.legacyDocumentRole(ctx, principal, state, documentID)
	if err != nil {
		return ResourceAccess{}, err
	}
	return ResourceAccess{
		CanView: role >= roleViewer, CanComment: role >= roleCommenter,
		CanEdit: role >= roleEditor, CanManage: role >= roleManager,
	}, nil
}

func allowedActions(state accessState, role objectRole) []string {
	candidates := []struct {
		code string
		role objectRole
	}{
		{string(ActionNodesView), roleViewer},
		{string(ActionContentView), roleViewer},
		{string(ActionContentUpdate), roleEditor},
		{string(ActionContentExport), roleViewer},
		{string(ActionAssetsView), roleViewer},
		{string(ActionAssetsUpload), roleEditor},
		{string(ActionAssetsDownload), roleViewer},
		{string(ActionAssetsDelete), roleEditor},
		{string(ActionCommentsView), roleViewer},
		{string(ActionCommentsCreate), roleCommenter},
		{string(ActionCommentsReply), roleCommenter},
		{string(ActionCommentsEditOwn), roleCommenter},
		{string(ActionCommentsDeleteOwn), roleCommenter},
		{string(ActionCommentsResolve), roleCommenter},
		{string(ActionCommentsReopen), roleCommenter},
		{string(ActionPreferencesUpdate), roleViewer},
	}
	if role == roleManager {
		candidates = append(candidates,
			struct {
				code string
				role objectRole
			}{string(ActionNodesCreate), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionNodesCopy), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionShortcutsCreate), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionOwnershipTransfer), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionNodesUpdate), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionNodesMove), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionNodesArchive), roleManager},
			struct {
				code string
				role objectRole
			}{string(ActionBindingsUpdate), roleManager},
		)
	}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.code == string(ActionContentExport) && !state.has(string(ActionContentView)) {
			continue
		}
		if role >= candidate.role {
			if _, ok := state.actions[candidate.code]; ok {
				result = append(result, candidate.code)
			}
		}
	}
	sort.Strings(result)
	return result
}

func editorParticipantIDs(value RequirementParticipants) []uint {
	result := make([]uint, 0)
	result = append(result, value.ProductOwnerUserIDs...)
	result = append(result, value.DesignOwnerUserIDs...)
	result = append(result, value.FrontendOwnerUserIDs...)
	result = append(result, value.BackendOwnerUserIDs...)
	return append(result, value.DeveloperUserIDs...)
}

func includesUser(ids []uint, userID uint) bool {
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

func uniqueIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
