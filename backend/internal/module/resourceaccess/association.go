package resourceaccess

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type RequirementRuleSource interface {
	ResourceLinkProjectSets(context.Context, uint, []string) (map[string][]uint, error)
}

func (s *PolicyService) ConfigureRequirementSource(source RequirementRuleSource) {
	s.requirementSource = source
}

func (s *PolicyService) applyRequirementSources(ctx context.Context, w uint, policies map[documentlibrary.ResourceID]Policy) error {
	if s.requirementSource == nil {
		return nil
	}
	ids := []string{}
	for id, p := range policies {
		if p.Mode == ModeRestricted {
			ids = append(ids, string(id))
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sources, err := s.requirementSource.ResourceLinkProjectSets(ctx, w, ids)
	if err != nil {
		return ErrUnavailable
	}
	for id, sets := range sources {
		p := policies[documentlibrary.ResourceID(id)]
		if p.Mode != ModeRestricted {
			continue
		}
		p.Rules = append([]Rule(nil), p.Rules...)
		for _, setID := range sets {
			if setID > 0 {
				p.Rules = append(p.Rules, Rule{SubjectKind: identity.SubjectProjectSet, SubjectID: uintString(setID), Level: LevelViewer})
			}
		}
		policies[p.ResourceID] = p
	}
	return nil
}

// TouchAssociation is called in the SAME transaction as the owning requirement
// link mutation. It preserves direct rules and independently shared links.
func (s *PolicyService) TouchAssociation(ctx context.Context, p Principal, id documentlibrary.ResourceID, revision uint64, key string, adding bool) error {
	current, err := s.store.GetPolicy(ctx, p.WorkspaceID, id)
	if err != nil {
		return err
	}
	if current.Mode == ModeLegacy {
		return ErrForbidden
	}
	mode := current.Mode
	if adding && (mode == ModePrivate || mode == ModeLink) {
		mode = ModeRestricted
	}
	_, err = s.ReplacePolicy(ctx, p, id, ReplacePolicyInput{Mode: mode, WorkspaceLevel: current.WorkspaceLevel, Rules: current.Rules, ExpectedPolicyRevision: revision, IdempotencyKey: key})
	return err
}
