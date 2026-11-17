package identity

import (
	"strconv"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

func nativeSubjectCandidate(kind SubjectKind, item NativeSubjectRecord) SubjectCandidate {
	return SubjectCandidate{
		Kind: kind, ID: strconv.FormatUint(uint64(item.ID), 10),
		DisplayName: item.DisplayName, SecondaryText: item.SecondaryText,
	}
}

func nativeSubjectPage(kind SubjectKind, page NativeSubjectPage) SubjectPage {
	items := make([]SubjectCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, nativeSubjectCandidate(kind, item))
	}
	return SubjectPage{
		Items: items, Page: page.Page, PageSize: page.PageSize, Total: page.Total,
		HasMore: int64(page.Page*page.PageSize) < page.Total,
	}
}

func projectSetSubjectCandidate(item project.ProjectSetSubjectCandidate) SubjectCandidate {
	return SubjectCandidate{
		Kind: SubjectProjectSet, ID: item.ID,
		DisplayName: item.DisplayName, SecondaryText: item.SecondaryText,
	}
}

func projectSetSubjectPage(page project.ProjectSetSubjectPage) SubjectPage {
	items := make([]SubjectCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, projectSetSubjectCandidate(item))
	}
	return SubjectPage{
		Items: items, Page: page.Page, PageSize: page.PageSize, Total: page.Total,
		HasMore: int64(page.Page*page.PageSize) < page.Total,
	}
}

func opaqueIDs(ids []uint) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, strconv.FormatUint(uint64(id), 10))
	}
	return values
}

func indexResolvedSubjects(
	kind SubjectKind,
	refs []SubjectRef,
	items []SubjectCandidate,
	resolved map[SubjectRef]SubjectCandidate,
	err error,
) error {
	if err != nil {
		return err
	}
	byID := make(map[string]SubjectCandidate, len(items))
	for _, item := range items {
		if item.Kind != kind || item.ID == "" || item.DisplayName == "" {
			return ErrSubjectInvalid
		}
		byID[item.ID] = item
	}
	for _, ref := range refs {
		item, ok := byID[ref.ID]
		if !ok {
			return ErrSubjectInvalid
		}
		resolved[ref] = item
	}
	return nil
}

func indexSubjectMatches(
	kind SubjectKind,
	refs []SubjectRef,
	result SubjectMatchResult,
	matched map[SubjectRef]bool,
	err error,
) error {
	if err != nil {
		return err
	}
	if len(result.Matches) != len(refs) {
		return ErrSubjectInvalid
	}
	available := make(map[SubjectRef]bool, len(result.Matches))
	for _, match := range result.Matches {
		if match.Subject.Kind != kind {
			return ErrSubjectInvalid
		}
		available[match.Subject] = match.Matched
	}
	for _, ref := range refs {
		value, ok := available[ref]
		if !ok {
			return ErrSubjectInvalid
		}
		matched[ref] = value
	}
	return nil
}

func unmatchedSubjects(subjects []SubjectRef) SubjectMatchResult {
	result := SubjectMatchResult{Matches: make([]SubjectMatch, 0, len(subjects))}
	for _, subject := range subjects {
		result.Matches = append(result.Matches, SubjectMatch{Subject: subject})
	}
	return result
}

var _ SubjectDirectory = (*SubjectDirectoryService)(nil)
var _ SubjectCapabilityProvider = (*SubjectDirectoryService)(nil)
