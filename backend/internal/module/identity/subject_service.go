package identity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

type SubjectDirectoryService struct {
	native            NativeSubjectStore
	projectSets       project.ProjectSetSubjectDirectory
	organizationUnits OrganizationUnitProvider
}

func NewSubjectDirectoryService(
	native NativeSubjectStore,
	projectSets project.ProjectSetSubjectDirectory,
	organizationUnits OrganizationUnitProvider,
) (*SubjectDirectoryService, error) {
	if native == nil {
		return nil, ErrSubjectKindUnavailable
	}
	return &SubjectDirectoryService{native: native, projectSets: projectSets, organizationUnits: organizationUnits}, nil
}

func (service *SubjectDirectoryService) SubjectCapabilities() map[SubjectKind]SubjectKindCapability {
	return map[SubjectKind]SubjectKindCapability{
		SubjectUser:             {Available: service != nil && service.native != nil},
		SubjectRole:             {Available: service != nil && service.native != nil},
		SubjectUserGroup:        {Available: service != nil && service.native != nil},
		SubjectOrganizationUnit: {Available: service != nil && service.organizationUnits != nil},
		SubjectProjectSet:       {Available: service != nil && service.projectSets != nil},
	}
}

func (service *SubjectDirectoryService) SearchSubjects(
	ctx context.Context, workspaceID uint, search SubjectSearch,
) (SubjectPage, error) {
	if workspaceID == 0 || !search.Kind.Valid() {
		return SubjectPage{}, ErrSubjectInvalid
	}
	search.Query = strings.TrimSpace(search.Query)
	search.Page, search.PageSize = normalizeSubjectPage(search.Page, search.PageSize)
	switch search.Kind {
	case SubjectOrganizationUnit:
		if service.organizationUnits == nil {
			return SubjectPage{}, ErrSubjectKindUnavailable
		}
		return service.organizationUnits.SearchOrganizationUnits(ctx, workspaceID, search)
	case SubjectProjectSet:
		if service.projectSets == nil {
			return SubjectPage{}, ErrSubjectKindUnavailable
		}
		page, err := service.projectSets.SearchProjectSetSubjects(ctx, workspaceID, project.ProjectSetSubjectSearch{
			Query: search.Query, Page: search.Page, PageSize: search.PageSize,
		})
		return projectSetSubjectPage(page), err
	default:
		page, err := service.native.SearchNativeSubjects(ctx, workspaceID, search.Kind, search.Query, search.Page, search.PageSize)
		if err != nil {
			return SubjectPage{}, err
		}
		return nativeSubjectPage(search.Kind, page), nil
	}
}

func (service *SubjectDirectoryService) ResolveSubjects(
	ctx context.Context, workspaceID uint, subjects []SubjectRef,
) ([]SubjectCandidate, error) {
	if workspaceID == 0 {
		return nil, ErrSubjectInvalid
	}
	if len(subjects) == 0 {
		return []SubjectCandidate{}, nil
	}
	resolved := make(map[SubjectRef]SubjectCandidate, len(subjects))
	for kind, refs := range groupSubjectRefs(subjects) {
		if err := service.resolveKind(ctx, workspaceID, kind, refs, resolved); err != nil {
			return nil, err
		}
	}
	result := make([]SubjectCandidate, 0, len(subjects))
	for _, subject := range subjects {
		candidate, ok := resolved[subject]
		if !ok {
			return nil, ErrSubjectInvalid
		}
		result = append(result, candidate)
	}
	return result, nil
}

func (service *SubjectDirectoryService) MatchSubjects(
	ctx context.Context, request SubjectMatchRequest,
) (SubjectMatchResult, error) {
	if request.WorkspaceID == 0 || request.UserID == 0 {
		return SubjectMatchResult{}, ErrSubjectInvalid
	}
	active, err := service.native.HasActiveWorkspaceUser(ctx, request.WorkspaceID, request.UserID)
	if err != nil {
		return SubjectMatchResult{}, err
	}
	if !active {
		return unmatchedSubjects(request.Subjects), nil
	}
	matched := make(map[SubjectRef]bool, len(request.Subjects))
	for kind, refs := range groupSubjectRefs(request.Subjects) {
		if err := service.matchKind(ctx, request, kind, refs, matched); err != nil {
			return SubjectMatchResult{}, err
		}
	}
	result := SubjectMatchResult{Matches: make([]SubjectMatch, 0, len(request.Subjects))}
	for _, subject := range request.Subjects {
		result.Matches = append(result.Matches, SubjectMatch{Subject: subject, Matched: matched[subject]})
	}
	return result, nil
}

func (service *SubjectDirectoryService) resolveKind(
	ctx context.Context, workspaceID uint, kind SubjectKind, refs []SubjectRef, resolved map[SubjectRef]SubjectCandidate,
) error {
	if kind == SubjectOrganizationUnit {
		if service.organizationUnits == nil {
			return ErrSubjectKindUnavailable
		}
		items, err := service.organizationUnits.ResolveOrganizationUnits(ctx, workspaceID, refs)
		return indexResolvedSubjects(kind, refs, items, resolved, err)
	}
	ids, err := nativeSubjectIDs(kind, refs)
	if err != nil {
		return err
	}
	if kind == SubjectProjectSet {
		if service.projectSets == nil {
			return ErrSubjectKindUnavailable
		}
		items, err := service.projectSets.ResolveProjectSetSubjects(ctx, workspaceID, opaqueIDs(ids))
		candidates := make([]SubjectCandidate, 0, len(items))
		for _, item := range items {
			candidates = append(candidates, projectSetSubjectCandidate(item))
		}
		return indexResolvedSubjects(kind, refs, candidates, resolved, err)
	}
	items, err := service.native.ResolveNativeSubjects(ctx, workspaceID, kind, ids)
	if err != nil {
		return err
	}
	candidates := make([]SubjectCandidate, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, nativeSubjectCandidate(kind, item))
	}
	return indexResolvedSubjects(kind, refs, candidates, resolved, nil)
}

func (service *SubjectDirectoryService) matchKind(
	ctx context.Context, request SubjectMatchRequest, kind SubjectKind, refs []SubjectRef, matched map[SubjectRef]bool,
) error {
	if kind == SubjectOrganizationUnit {
		if service.organizationUnits == nil {
			return ErrSubjectKindUnavailable
		}
		result, err := service.organizationUnits.MatchOrganizationUnits(ctx, SubjectMatchRequest{
			WorkspaceID: request.WorkspaceID, UserID: request.UserID, Subjects: refs,
		})
		return indexSubjectMatches(kind, refs, result, matched, err)
	}
	ids, err := nativeSubjectIDs(kind, refs)
	if err != nil {
		return err
	}
	var facts map[uint]bool
	if kind == SubjectProjectSet {
		if service.projectSets == nil {
			return ErrSubjectKindUnavailable
		}
		factID, err := service.native.ResolveSubjectFactUserID(ctx, request.WorkspaceID, request.UserID)
		if err != nil || factID == 0 {
			return ErrSubjectInvalid
		}
		projectFacts, matchErr := service.projectSets.MatchProjectSetSubjects(ctx, request.WorkspaceID, factID, opaqueIDs(ids))
		if matchErr != nil {
			return matchErr
		}
		facts = make(map[uint]bool, len(ids))
		for _, id := range ids {
			facts[id] = projectFacts[strconv.FormatUint(uint64(id), 10)]
		}
	} else if kind == SubjectUser {
		facts = make(map[uint]bool, len(ids))
		for _, id := range ids {
			facts[id] = id == request.UserID
		}
	} else {
		facts, err = service.native.MatchNativeSubjects(ctx, request.WorkspaceID, request.UserID, kind, ids)
		if err != nil {
			return err
		}
	}
	for _, ref := range refs {
		id, _ := parseSubjectID(ref.ID)
		matched[ref] = facts[id]
	}
	return nil
}

func groupSubjectRefs(subjects []SubjectRef) map[SubjectKind][]SubjectRef {
	grouped := make(map[SubjectKind][]SubjectRef)
	for _, subject := range subjects {
		grouped[subject.Kind] = append(grouped[subject.Kind], subject)
	}
	return grouped
}

func nativeSubjectIDs(kind SubjectKind, refs []SubjectRef) ([]uint, error) {
	if !kind.Valid() || kind == SubjectOrganizationUnit {
		return nil, ErrSubjectInvalid
	}
	ids := make([]uint, 0, len(refs))
	for _, ref := range refs {
		if ref.Kind != kind || ref.IncludeDescendants {
			return nil, ErrSubjectInvalid
		}
		id, err := parseSubjectID(ref.ID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseSubjectID(value string) (uint, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || uint64(uint(parsed)) != parsed {
		return 0, fmt.Errorf("%w: invalid opaque ID", ErrSubjectInvalid)
	}
	if value != strconv.FormatUint(parsed, 10) {
		return 0, fmt.Errorf("%w: non-canonical opaque ID", ErrSubjectInvalid)
	}
	return uint(parsed), nil
}

func normalizeSubjectPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultSubjectPageSize
	}
	if pageSize > MaximumSubjectPageSize {
		pageSize = MaximumSubjectPageSize
	}
	return page, pageSize
}
