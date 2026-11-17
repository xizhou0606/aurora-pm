package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

type subjectStoreStub struct {
	factID  uint
	active  bool
	matched map[uint]bool
}

func (store *subjectStoreStub) HasActiveWorkspaceUser(context.Context, uint, uint) (bool, error) {
	return store.active, nil
}

func (*subjectStoreStub) SearchNativeSubjects(
	context.Context, uint, SubjectKind, string, int, int,
) (NativeSubjectPage, error) {
	return NativeSubjectPage{}, nil
}

func (*subjectStoreStub) ResolveNativeSubjects(
	_ context.Context, _ uint, _ SubjectKind, ids []uint,
) ([]NativeSubjectRecord, error) {
	result := make([]NativeSubjectRecord, 0, len(ids))
	for _, id := range ids {
		result = append(result, NativeSubjectRecord{ID: id, DisplayName: "resolved"})
	}
	return result, nil
}

func (store *subjectStoreStub) MatchNativeSubjects(
	_ context.Context, _, _ uint, _ SubjectKind, ids []uint,
) (map[uint]bool, error) {
	result := make(map[uint]bool, len(ids))
	for _, id := range ids {
		result[id] = store.matched[id]
	}
	return result, nil
}

type projectSubjectDirectoryStub struct {
	matched map[string]bool
	userID  uint
}

func (*projectSubjectDirectoryStub) SearchProjectSetSubjects(
	context.Context, uint, project.ProjectSetSubjectSearch,
) (project.ProjectSetSubjectPage, error) {
	return project.ProjectSetSubjectPage{}, nil
}

func (*projectSubjectDirectoryStub) ResolveProjectSetSubjects(
	_ context.Context, _ uint, ids []string,
) ([]project.ProjectSetSubjectCandidate, error) {
	result := make([]project.ProjectSetSubjectCandidate, 0, len(ids))
	for _, id := range ids {
		result = append(result, project.ProjectSetSubjectCandidate{ID: id, DisplayName: "project"})
	}
	return result, nil
}

func (directory *projectSubjectDirectoryStub) MatchProjectSetSubjects(
	_ context.Context, _ uint, userID uint, ids []string,
) (map[string]bool, error) {
	directory.userID = userID
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = directory.matched[id]
	}
	return result, nil
}

type organizationProviderStub struct {
	matched bool
	last    []SubjectRef
}

func (*organizationProviderStub) SearchOrganizationUnits(
	_ context.Context, _ uint, search SubjectSearch,
) (SubjectPage, error) {
	return SubjectPage{Page: search.Page, PageSize: search.PageSize}, nil
}

func (provider *organizationProviderStub) ResolveOrganizationUnits(
	_ context.Context, _ uint, refs []SubjectRef,
) ([]SubjectCandidate, error) {
	provider.last = append([]SubjectRef(nil), refs...)
	result := make([]SubjectCandidate, 0, len(refs))
	for _, ref := range refs {
		result = append(result, SubjectCandidate{Kind: SubjectOrganizationUnit, ID: ref.ID, DisplayName: "unit"})
	}
	return result, nil
}

func (provider *organizationProviderStub) MatchOrganizationUnits(
	_ context.Context, request SubjectMatchRequest,
) (SubjectMatchResult, error) {
	provider.last = append([]SubjectRef(nil), request.Subjects...)
	result := SubjectMatchResult{Matches: make([]SubjectMatch, 0, len(request.Subjects))}
	for _, ref := range request.Subjects {
		result.Matches = append(result.Matches, SubjectMatch{Subject: ref, Matched: provider.matched})
	}
	return result, nil
}

func TestSubjectDirectoryOrganizationProviderIsExplicitlyUnavailable(t *testing.T) {
	service, err := NewSubjectDirectoryService(&subjectStoreStub{active: true}, &projectSubjectDirectoryStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := SubjectRef{Kind: SubjectOrganizationUnit, ID: "ou-1", IncludeDescendants: true}
	if _, err := service.SearchSubjects(context.Background(), 7, SubjectSearch{Kind: SubjectOrganizationUnit}); !errors.Is(err, ErrSubjectKindUnavailable) {
		t.Fatalf("search error=%v", err)
	}
	if _, err := service.ResolveSubjects(context.Background(), 7, []SubjectRef{ref}); !errors.Is(err, ErrSubjectKindUnavailable) {
		t.Fatalf("resolve error=%v", err)
	}
	if _, err := service.MatchSubjects(context.Background(), SubjectMatchRequest{WorkspaceID: 7, UserID: 9, Subjects: []SubjectRef{ref}}); !errors.Is(err, ErrSubjectKindUnavailable) {
		t.Fatalf("match error=%v", err)
	}
}

func TestSubjectDirectoryKeepsOrganizationDescendantSemanticsAtProviderBoundary(t *testing.T) {
	provider := &organizationProviderStub{matched: true}
	service, err := NewSubjectDirectoryService(
		&subjectStoreStub{active: true}, &projectSubjectDirectoryStub{}, provider,
	)
	if err != nil {
		t.Fatal(err)
	}
	ref := SubjectRef{Kind: SubjectOrganizationUnit, ID: "ou-1", IncludeDescendants: true}
	result, err := service.MatchSubjects(context.Background(), SubjectMatchRequest{
		WorkspaceID: 7, UserID: 9, Subjects: []SubjectRef{ref},
	})
	if err != nil || len(result.Matches) != 1 || !result.Matches[0].Matched || !provider.last[0].IncludeDescendants {
		t.Fatalf("result=%+v provider refs=%+v err=%v", result, provider.last, err)
	}
	provider.matched = false // models deactivation or member removal in the owning implementation
	result, err = service.MatchSubjects(context.Background(), SubjectMatchRequest{
		WorkspaceID: 7, UserID: 9, Subjects: []SubjectRef{ref},
	})
	if err != nil || result.Matches[0].Matched {
		t.Fatalf("removed membership result=%+v err=%v", result, err)
	}
}

func TestSubjectDirectoryMatchesEveryAvailableKindAndFailsClosedForInactiveUser(t *testing.T) {
	store := &subjectStoreStub{active: true, matched: map[uint]bool{2: true, 3: true}}
	projects := &projectSubjectDirectoryStub{matched: map[string]bool{"4": true}}
	service, err := NewSubjectDirectoryService(store, projects, &organizationProviderStub{matched: true})
	if err != nil {
		t.Fatal(err)
	}
	refs := []SubjectRef{
		{Kind: SubjectUser, ID: "9"}, {Kind: SubjectRole, ID: "2"},
		{Kind: SubjectUserGroup, ID: "3"}, {Kind: SubjectProjectSet, ID: "4"},
		{Kind: SubjectOrganizationUnit, ID: "ou-1"},
	}
	result, err := service.MatchSubjects(context.Background(), SubjectMatchRequest{WorkspaceID: 7, UserID: 9, Subjects: refs})
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range result.Matches {
		if !match.Matched {
			t.Fatalf("unexpected miss: %+v", match)
		}
	}
	store.active = false
	result, err = service.MatchSubjects(context.Background(), SubjectMatchRequest{WorkspaceID: 7, UserID: 9, Subjects: refs})
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range result.Matches {
		if match.Matched {
			t.Fatalf("inactive user matched: %+v", match)
		}
	}
}

func TestSubjectDirectoryRejectsExternalOrNonCanonicalNumericIDs(t *testing.T) {
	service, err := NewSubjectDirectoryService(&subjectStoreStub{active: true}, &projectSubjectDirectoryStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"external-7", "07", " 7"} {
		_, err := service.ResolveSubjects(context.Background(), 7, []SubjectRef{{Kind: SubjectUser, ID: id}})
		if !errors.Is(err, ErrSubjectInvalid) {
			t.Fatalf("id=%q error=%v", id, err)
		}
	}
}

func TestSubjectCapabilitiesExposeOrganizationPlaceholderAsUnavailable(t *testing.T) {
	service, err := NewSubjectDirectoryService(&subjectStoreStub{active: true}, &projectSubjectDirectoryStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := service.SubjectCapabilities()
	for _, kind := range []SubjectKind{SubjectUser, SubjectRole, SubjectUserGroup, SubjectProjectSet} {
		if !capabilities[kind].Available {
			t.Fatalf("subject kind %s should be available", kind)
		}
	}
	if capabilities[SubjectOrganizationUnit].Available {
		t.Fatal("organization placeholder must stay unavailable until a provider is injected")
	}
}

func (s *subjectStoreStub) ResolveSubjectFactUserID(_ context.Context, _ uint, id uint) (uint, error) {
	if s.factID > 0 {
		return s.factID, nil
	}
	return id, nil
}
func TestProjectSubjectsResolveCanonicalPrincipalToParticipantFact(t *testing.T) {
	projects := &projectSubjectDirectoryStub{matched: map[string]bool{"7": true}}
	service, err := NewSubjectDirectoryService(&subjectStoreStub{active: true, factID: 99}, projects, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.MatchSubjects(context.Background(), SubjectMatchRequest{WorkspaceID: 7, UserID: 11, Subjects: []SubjectRef{{Kind: SubjectProjectSet, ID: "7"}}})
	if err != nil || projects.userID != 99 {
		t.Fatalf("project fact user=%d err=%v", projects.userID, err)
	}
}
