package project

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrProjectSetSubjectInvalid = errors.New("project set subject is invalid")

type ProjectSetSubjectSearch struct {
	Query    string
	Page     int
	PageSize int
}

type ProjectSetSubjectCandidate struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	SecondaryText string `json:"secondary_text,omitempty"`
}

type ProjectSetSubjectPage struct {
	Items    []ProjectSetSubjectCandidate `json:"items"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
	Total    int64                        `json:"total"`
}

// ProjectSetSubjectDirectory is the project module's narrow public port for
// resource-sharing subjects. It exposes actual membership facts and never
// expands an administrator's broad project visibility into membership.
type ProjectSetSubjectDirectory interface {
	SearchProjectSetSubjects(context.Context, uint, ProjectSetSubjectSearch) (ProjectSetSubjectPage, error)
	ResolveProjectSetSubjects(context.Context, uint, []string) ([]ProjectSetSubjectCandidate, error)
	MatchProjectSetSubjects(context.Context, uint, uint, []string) (map[string]bool, error)
}

type ProjectSetSubjectRecord struct {
	ID   uint
	Name string
	Code string
}

type ProjectSetSubjectRecordPage struct {
	Items    []ProjectSetSubjectRecord
	Page     int
	PageSize int
	Total    int64
}

type ProjectSetSubjectStore interface {
	SearchProjectSetSubjectRecords(context.Context, uint, string, int, int) (ProjectSetSubjectRecordPage, error)
	ResolveProjectSetSubjectRecords(context.Context, uint, []uint) ([]ProjectSetSubjectRecord, error)
	MatchProjectSetSubjectRecords(context.Context, uint, uint, []uint) (map[uint]bool, error)
}

type ProjectSetSubjectService struct{ store ProjectSetSubjectStore }

func NewProjectSetSubjectService(store ProjectSetSubjectStore) (*ProjectSetSubjectService, error) {
	if store == nil {
		return nil, ErrProjectSetSubjectInvalid
	}
	return &ProjectSetSubjectService{store: store}, nil
}

func (service *ProjectSetSubjectService) SearchProjectSetSubjects(
	ctx context.Context, workspaceID uint, search ProjectSetSubjectSearch,
) (ProjectSetSubjectPage, error) {
	if workspaceID == 0 {
		return ProjectSetSubjectPage{}, ErrProjectSetSubjectInvalid
	}
	search.Query = strings.TrimSpace(search.Query)
	search.Page, search.PageSize = normalizeProjectSubjectPage(search.Page, search.PageSize)
	page, err := service.store.SearchProjectSetSubjectRecords(
		ctx, workspaceID, search.Query, search.Page, search.PageSize,
	)
	if err != nil {
		return ProjectSetSubjectPage{}, err
	}
	return projectSetSubjectPageFromRecords(page), nil
}

func (service *ProjectSetSubjectService) ResolveProjectSetSubjects(
	ctx context.Context, workspaceID uint, opaqueIDs []string,
) ([]ProjectSetSubjectCandidate, error) {
	if workspaceID == 0 {
		return nil, ErrProjectSetSubjectInvalid
	}
	if len(opaqueIDs) == 0 {
		return []ProjectSetSubjectCandidate{}, nil
	}
	ids, err := parseProjectSetSubjectIDs(opaqueIDs)
	if err != nil {
		return nil, err
	}
	records, err := service.store.ResolveProjectSetSubjectRecords(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]ProjectSetSubjectRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	result := make([]ProjectSetSubjectCandidate, 0, len(ids))
	for _, id := range ids {
		record, ok := byID[id]
		if !ok {
			return nil, ErrProjectSetSubjectInvalid
		}
		result = append(result, projectSetSubjectCandidateFromRecord(record))
	}
	return result, nil
}

func (service *ProjectSetSubjectService) MatchProjectSetSubjects(
	ctx context.Context, workspaceID, userID uint, opaqueIDs []string,
) (map[string]bool, error) {
	if workspaceID == 0 || userID == 0 {
		return nil, ErrProjectSetSubjectInvalid
	}
	if len(opaqueIDs) == 0 {
		return map[string]bool{}, nil
	}
	ids, err := parseProjectSetSubjectIDs(opaqueIDs)
	if err != nil {
		return nil, err
	}
	facts, err := service.store.MatchProjectSetSubjectRecords(ctx, workspaceID, userID, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(ids))
	for index, id := range ids {
		result[opaqueIDs[index]] = facts[id]
	}
	return result, nil
}

func parseProjectSetSubjectIDs(values []string) ([]uint, error) {
	ids := make([]uint, 0, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 || uint64(uint(parsed)) != parsed {
			return nil, fmt.Errorf("%w: invalid opaque ID", ErrProjectSetSubjectInvalid)
		}
		if value != strconv.FormatUint(parsed, 10) {
			return nil, fmt.Errorf("%w: non-canonical opaque ID", ErrProjectSetSubjectInvalid)
		}
		ids = append(ids, uint(parsed))
	}
	return ids, nil
}

func normalizeProjectSubjectPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func projectSetSubjectCandidateFromRecord(record ProjectSetSubjectRecord) ProjectSetSubjectCandidate {
	return ProjectSetSubjectCandidate{
		ID: strconv.FormatUint(uint64(record.ID), 10), DisplayName: record.Name, SecondaryText: record.Code,
	}
}

func projectSetSubjectPageFromRecords(page ProjectSetSubjectRecordPage) ProjectSetSubjectPage {
	items := make([]ProjectSetSubjectCandidate, 0, len(page.Items))
	for _, record := range page.Items {
		items = append(items, projectSetSubjectCandidateFromRecord(record))
	}
	return ProjectSetSubjectPage{Items: items, Page: page.Page, PageSize: page.PageSize, Total: page.Total}
}

var _ ProjectSetSubjectDirectory = (*ProjectSetSubjectService)(nil)
