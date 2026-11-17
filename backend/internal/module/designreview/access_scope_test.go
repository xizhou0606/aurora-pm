package designreview

import (
	"context"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

type accessScopeRepository struct {
	Repository
	requirements []RequirementOption
	reviews      []DesignReviewSummary
	detail       DesignReviewDetail
	assetKinds   []string
	queued       bool
}

func (repository *accessScopeRepository) ListRequirementOptions(context.Context, uint) ([]RequirementOption, error) {
	return append([]RequirementOption(nil), repository.requirements...), nil
}

func (repository *accessScopeRepository) ListReviews(context.Context, uint, ReviewListFilter) ([]DesignReviewSummary, int64, error) {
	return append([]DesignReviewSummary(nil), repository.reviews...), int64(len(repository.reviews)), nil
}

func (repository *accessScopeRepository) GetReview(context.Context, uint, string) (DesignReviewDetail, error) {
	return repository.detail, nil
}

func (repository *accessScopeRepository) ListAssetKinds(context.Context, uint, string) ([]string, error) {
	return append([]string(nil), repository.assetKinds...), nil
}

func (repository *accessScopeRepository) QueueAnalysis(context.Context, uint, string, uint) error {
	repository.queued = true
	return nil
}

type projectAccessStub struct {
	allowed  map[uint]bool
	errs     map[uint]error
	admin    bool
	adminErr error
	users    []uint
}

func (stub *projectAccessStub) CanAccessProject(_ context.Context, _ uint, projectSetID uint, userID uint) (bool, error) {
	stub.users = append(stub.users, userID)
	return stub.allowed[projectSetID], stub.errs[projectSetID]
}

func (stub *projectAccessStub) IsAccessAdministrator(context.Context, uint, uint) (bool, error) {
	return stub.admin, stub.adminErr
}

func TestReviewAuthorizationSeparatesPrincipalAndFactUserIDs(t *testing.T) {
	repository := &accessScopeRepository{requirements: []RequirementOption{{ID: 1, ProjectSetID: 11}}}
	projects := &projectAccessStub{allowed: map[uint]bool{11: true}}
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)}}
	service := &Service{repository: repository, projects: projects, actions: actions}

	items, err := service.ListRequirements(context.Background(), 7, ActorIdentity{PrincipalUserID: 109, FactUserID: 1})

	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if len(actions.users) != 1 || actions.users[0] != 109 {
		t.Fatalf("RBAC action lookup users=%v want=[109]", actions.users)
	}
	if len(projects.users) != 1 || projects.users[0] != 1 {
		t.Fatalf("project access users=%v want=[1]", projects.users)
	}
}

func TestReviewQueriesFilterSavedProjectScope(t *testing.T) {
	repository := &accessScopeRepository{
		requirements: []RequirementOption{{ID: 1, ProjectSetID: 11}, {ID: 2, ProjectSetID: 12}, {ID: 3, ProjectSetID: 13}},
		reviews: []DesignReviewSummary{
			{ID: "visible", ProjectSetID: 11, RoundNumber: 1, CreatedByUserID: 42},
			{ID: "forbidden", ProjectSetID: 12, RoundNumber: 1, CreatedByUserID: 42},
			{ID: "lookup-failed", ProjectSetID: 13, RoundNumber: 1, CreatedByUserID: 42},
		},
	}
	projects := &projectAccessStub{allowed: map[uint]bool{11: true}, errs: map[uint]error{13: errors.New("scope lookup failed")}}
	service := &Service{repository: repository, projects: projects, actions: &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)}}}
	requirements, err := service.ListRequirements(context.Background(), 7, testActor(42))
	if err != nil || len(requirements) != 1 || requirements[0].ID != 1 {
		t.Fatalf("requirements=%+v err=%v", requirements, err)
	}
	page, err := service.ListReviews(context.Background(), 7, testActor(42), ReviewListFilter{Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "visible" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestDeletedProjectSnapshotFailsClosedExceptForWorkspaceAdministrator(t *testing.T) {
	repository := &accessScopeRepository{detail: DesignReviewDetail{DesignReviewSummary: DesignReviewSummary{ID: "review", ProjectSetID: 99}}}
	actions := &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)}}
	for _, test := range []struct {
		name     string
		projects *projectAccessStub
		wantErr  error
	}{
		{name: "ordinary user", projects: &projectAccessStub{allowed: map[uint]bool{}, errs: map[uint]error{99: errors.New("project removed")}}, wantErr: ErrForbidden},
		{name: "administrator", projects: &projectAccessStub{allowed: map[uint]bool{}, errs: map[uint]error{99: errors.New("project removed")}, admin: true}},
		{name: "administrator lookup failed", projects: &projectAccessStub{allowed: map[uint]bool{}, adminErr: errors.New("admin lookup failed")}, wantErr: ErrForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &Service{repository: repository, projects: test.projects, actions: actions}
			_, err := service.GetReview(context.Background(), 7, testActor(42), "11111111-1111-4111-8111-111111111111")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error=%v want=%v", err, test.wantErr)
			}
		})
	}
}

func TestInitialAnalysisAndRetryUseDistinctActionsAndStates(t *testing.T) {
	view := string(accesscatalog.ActionDesignReviewsReviewsView)
	create := string(accesscatalog.ActionDesignReviewsReviewsCreate)
	manage := string(accesscatalog.ActionDesignReviewsAnalysisManage)
	for _, test := range []struct {
		name      string
		status    string
		actions   []string
		retry     bool
		wantErr   error
		wantQueue bool
	}{
		{name: "initial", status: "draft", actions: []string{view, create}, wantQueue: true},
		{name: "initial rejects failed", status: "failed", actions: []string{view, create}, wantErr: ErrConflict},
		{name: "retry", status: "failed", actions: []string{view, manage}, retry: true, wantQueue: true},
		{name: "retry rejects draft", status: "draft", actions: []string{view, manage}, retry: true, wantErr: ErrConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &accessScopeRepository{
				detail:     DesignReviewDetail{DesignReviewSummary: DesignReviewSummary{ID: "review", ProjectSetID: 11, Status: test.status}, AnalysisScope: AnalysisScope{}},
				assetKinds: []string{"design_original", "implementation_original", "design_structure", "implementation_structure"},
			}
			service := &Service{
				repository:   repository,
				projects:     &projectAccessStub{allowed: map[uint]bool{11: true}},
				actions:      &effectiveActionReaderStub{codes: test.actions},
				availability: Availability{ObjectStorage: true, VisualModel: true},
			}
			var err error
			if test.retry {
				err = service.RetryAnalysis(context.Background(), 7, testActor(42), "11111111-1111-4111-8111-111111111111")
			} else {
				err = service.QueueAnalysis(context.Background(), 7, testActor(42), "11111111-1111-4111-8111-111111111111")
			}
			if !errors.Is(err, test.wantErr) || repository.queued != test.wantQueue {
				t.Fatalf("err=%v queued=%v wantErr=%v wantQueue=%v", err, repository.queued, test.wantErr, test.wantQueue)
			}
		})
	}
}
