package docsource

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

type sourceFixture struct {
	denied   bool
	row      requirement.DocumentSource
	snapshot Snapshot
	reads    int
}

func (f *sourceFixture) Authorize(context.Context, uint, string, string) (identity.AuthorizationResult, error) {
	if f.denied {
		return identity.AuthorizationForbidden, nil
	}
	return identity.AuthorizationAllowed, nil
}
func (f *sourceFixture) VerifyWorkspaceMembership(context.Context, uint, uint) error {
	if f.denied {
		return ErrUnavailable
	}
	return nil
}
func (f *sourceFixture) FindActiveUserByID(_ context.Context, id uint) (identity.User, error) {
	return identity.User{ID: id}, nil
}
func (f *sourceFixture) RequireProjectAccess(context.Context, uint, uint, uint) error {
	if f.denied {
		return ErrUnavailable
	}
	return nil
}
func (f *sourceFixture) ReadDocumentSource(_ context.Context, _ uint, id uint) (requirement.DocumentSource, error) {
	r := f.row
	r.RequirementID = id
	return r, nil
}
func (f *sourceFixture) Kind() string { return Internal }
func (f *sourceFixture) Read(context.Context, ReadContext, SourceRef) (Snapshot, error) {
	f.reads++
	return f.snapshot, nil
}
func newSourceFixture(t *testing.T) (*Service, *sourceFixture, ReadContext, map[string]any) {
	t.Helper()
	f := &sourceFixture{row: requirement.DocumentSource{Version: 1, ProjectID: 7, ResourceIDs: []string{"doc"}}, snapshot: Snapshot{Type: Internal, Content: "正文", ContentHash: "hash", Revision: "1", Available: true}}
	registry, err := NewRegistry(f)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{Registry: registry, Users: f, Requirements: f, Actions: f, Memberships: f, Projects: f}
	return service, f, ReadContext{UserID: 1, FactUserID: 1, WorkspaceID: 7, Method: "POST", Route: "/test"}, map[string]any{"capability": "qa.test_cases", "test_case_context": map[string]any{"requirements": []any{map[string]any{"id": 1}, map[string]any{"id": 2}}}}
}
func TestSourceManifestDeduplicatesWithoutLosingRequirementOwnership(t *testing.T) {
	s, f, actor, input := newSourceFixture(t)
	m, err := s.Prepare(context.Background(), actor, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 1 || len(m.Sources[0].Requirements) != 2 || f.reads != 1 {
		t.Fatalf("manifest %+v reads=%d", m, f.reads)
	}
	snapshots, err := s.Read(context.Background(), m)
	if err != nil || len(snapshots) != 1 || len(snapshots[0].RequirementIDs) != 2 {
		t.Fatalf("snapshots=%+v err=%v", snapshots, err)
	}
}
func TestSourceVerificationRejectsChangedFactsAndPermissions(t *testing.T) {
	for _, kind := range []string{"unlink", "replace", "move", "edit", "revoke", "identity"} {
		t.Run(kind, func(t *testing.T) {
			s, f, actor, input := newSourceFixture(t)
			m, err := s.Prepare(context.Background(), actor, input)
			if err != nil {
				t.Fatal(err)
			}
			before, err := s.Read(context.Background(), m)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "unlink":
				f.row.ResourceIDs = nil
			case "replace":
				f.row.ResourceIDs = []string{"other"}
			case "move":
				f.row.ProjectID++
			case "edit":
				f.snapshot.Revision = "2"
			case "revoke":
				f.denied = true
			case "identity":
				m.Context.FactUserID = 99
			}
			if err := s.Verify(context.Background(), m, before, false); err == nil {
				t.Fatal("accepted stale or unauthorized source")
			}
		})
	}
}
func TestProviderRegistryFailsClosed(t *testing.T) {
	f := &sourceFixture{}
	if _, err := NewRegistry(f, f); err == nil {
		t.Fatal("duplicate accepted")
	}
	r, _ := NewRegistry(f)
	if _, err := r.Read(context.Background(), ReadContext{}, SourceRef{Kind: "missing"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Read(ctx, ReadContext{}, SourceRef{Kind: Internal}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
