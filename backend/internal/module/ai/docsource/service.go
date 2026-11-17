package docsource

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

type Users interface {
	FindActiveUserByID(context.Context, uint) (identity.User, error)
}

type Memberships interface {
	VerifyWorkspaceMembership(context.Context, uint, uint) error
}
type ProjectAccess interface {
	RequireProjectAccess(context.Context, uint, uint, uint) error
}
type Service struct {
	Users        Users
	Registry     *Registry
	Requirements requirement.DocumentSourceReader
	Actions      identity.Authorizer
	Memberships  Memberships
	Projects     ProjectAccess
}

func Supported(capability string) bool {
	return capability == "requirement.completeness" || capability == "requirement.task_plan" || capability == "qa.test_cases"
}
func (s *Service) authorize(ctx context.Context, actor ReadContext) error {
	if s == nil || s.Actions == nil || s.Memberships == nil || actor.UserID == 0 || actor.FactUserID == 0 || actor.WorkspaceID == 0 {
		return ErrUnavailable
	}
	if s.Users == nil {
		return ErrUnavailable
	}
	user, err := s.Users.FindActiveUserByID(ctx, actor.UserID)
	if err != nil || user.ID != actor.UserID {
		return ErrUnavailable
	}
	factID := user.ID
	if factID != actor.FactUserID {
		return ErrUnavailable
	}
	if err := s.Memberships.VerifyWorkspaceMembership(ctx, actor.UserID, actor.WorkspaceID); err != nil {
		return ErrUnavailable
	}
	decision, err := s.Actions.Authorize(ctx, actor.UserID, actor.Method, actor.Route)
	if err != nil || decision != identity.AuthorizationAllowed {
		return ErrUnavailable
	}
	return nil
}

// Prepare only resolves server-owned associations; external reads happen in the worker.
func (s *Service) Prepare(ctx context.Context, actor ReadContext, input map[string]any) (*Manifest, error) {
	if err := s.authorize(ctx, actor); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Subject struct {
			ID uint `json:"id"`
		}
		Capability      string
		TestCaseContext struct {
			Requirements []struct {
				ID uint `json:"id"`
			} `json:"requirements"`
		} `json:"test_case_context"`
	}
	if json.Unmarshal(raw, &decoded) != nil {
		return nil, ErrUnavailable
	}
	ids := []uint{}
	if decoded.Capability == "qa.test_cases" {
		for _, r := range decoded.TestCaseContext.Requirements {
			ids = append(ids, r.ID)
		}
	} else if Supported(decoded.Capability) {
		ids = append(ids, decoded.Subject.ID)
	}
	return s.prepareIDs(ctx, actor, ids, true)
}
func (s *Service) prepareIDs(ctx context.Context, actor ReadContext, ids []uint, checkContent bool) (*Manifest, error) {
	if s.Requirements == nil || s.Projects == nil {
		return nil, ErrUnavailable
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	m := &Manifest{Version: 1, Context: actor, Requirements: []RequirementRef{}, Sources: []SourceRef{}}
	seen := map[uint]bool{}
	sources := map[string]int{}
	for _, id := range ids {
		if id == 0 {
			return nil, ErrUnavailable
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		row, err := s.Requirements.ReadDocumentSource(ctx, actor.WorkspaceID, id)
		if err != nil {
			return nil, ErrUnavailable
		}
		if err = s.Projects.RequireProjectAccess(ctx, actor.WorkspaceID, row.ProjectID, actor.FactUserID); err != nil {
			return nil, ErrUnavailable
		}
		stamp := RequirementRef{ID: id, Version: row.Version, ProjectID: row.ProjectID}
		m.Requirements = append(m.Requirements, stamp)
		ref := SourceRef{URL: strings.TrimSpace(row.ProductDocURL), Kind: Feishu}
		if len(row.ResourceIDs) > 1 || len(row.ResourceIDs) > 0 && ref.URL != "" {
			return nil, ErrUnavailable
		}
		if len(row.ResourceIDs) == 1 {
			ref = SourceRef{Kind: Internal, ResourceID: row.ResourceIDs[0], ConverterVersion: document.AnalysisProjectionVersion}
		} else if ref.URL == "" {
			continue
		}
		key := ref.Kind + ":" + ref.ResourceID + ref.URL
		if index, ok := sources[key]; ok {
			m.Sources[index].Requirements = append(m.Sources[index].Requirements, stamp)
			continue
		}
		ref.Requirements = []RequirementRef{stamp}
		if checkContent && ref.Kind == Internal {
			if _, err := s.Registry.Read(ctx, actor, ref); err != nil {
				return nil, err
			}
		}
		sources[key] = len(m.Sources)
		m.Sources = append(m.Sources, ref)
	}
	return m, nil
}
func (s *Service) Validate(ctx context.Context, m *Manifest) error {
	if m == nil || m.Version != 1 {
		return ErrUnavailable
	}
	if err := s.authorize(ctx, m.Context); err != nil {
		return err
	}
	ids := make([]uint, 0, len(m.Requirements))
	for _, r := range m.Requirements {
		ids = append(ids, r.ID)
	}
	current, err := s.prepareIDs(ctx, m.Context, ids, false)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current.Requirements, m.Requirements) || !reflect.DeepEqual(current.Sources, m.Sources) {
		return ErrChanged
	}
	return nil
}
func (s *Service) Read(ctx context.Context, m *Manifest) ([]Snapshot, error) {
	if err := s.Validate(ctx, m); err != nil {
		return nil, err
	}
	snapshots := make([]Snapshot, 0, len(m.Sources))
	for _, ref := range m.Sources {
		snapshot, err := s.Registry.Read(ctx, m.Context, ref)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}
func (s *Service) Verify(ctx context.Context, m *Manifest, before []Snapshot, internalOnly bool) error {
	if err := s.Validate(ctx, m); err != nil {
		return err
	}
	if len(before) != len(m.Sources) {
		return ErrUnavailable
	}
	for i, ref := range m.Sources {
		if internalOnly && ref.Kind != Internal {
			continue
		}
		after, err := s.Registry.Read(ctx, m.Context, ref)
		if err != nil {
			return err
		}
		b := before[i]
		if after.Revision != b.Revision || after.ContentHash != b.ContentHash || after.Title != b.Title || after.ProjectionHash != b.ProjectionHash {
			return ErrChanged
		}
	}
	return nil
}
