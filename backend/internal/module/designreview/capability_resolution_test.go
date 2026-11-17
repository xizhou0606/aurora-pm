package designreview

import (
	"context"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

func TestListReviewDeleteCapabilityRequiresActionAndCreator(t *testing.T) {
	repository := &accessScopeRepository{reviews: []DesignReviewSummary{{
		ID: "review", ProjectSetID: 11, RoundNumber: 1, CreatedByUserID: 42,
	}}}
	projects := &projectAccessStub{allowed: map[uint]bool{11: true}}
	service := &Service{
		repository: repository,
		projects:   projects,
		actions:    &effectiveActionReaderStub{codes: []string{string(accesscatalog.ActionDesignReviewsReviewsView)}},
	}

	page, err := service.ListReviews(context.Background(), 7, testActor(42), ReviewListFilter{Page: 1, PageSize: 20})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].CanDelete {
		t.Fatal("creator must not receive can_delete without the delete Action")
	}

	service.actions = &effectiveActionReaderStub{codes: []string{
		string(accesscatalog.ActionDesignReviewsReviewsView),
		string(accesscatalog.ActionDesignReviewsReviewsDelete),
	}}
	page, err = service.ListReviews(context.Background(), 7, testActor(42), ReviewListFilter{Page: 1, PageSize: 20})
	if err != nil || !page.Items[0].CanDelete {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestReviewDetailResolvesDifferenceAndDeepReviewCapabilities(t *testing.T) {
	actor, other := uint(42), uint(77)
	repository := &accessScopeRepository{detail: DesignReviewDetail{
		DesignReviewSummary: DesignReviewSummary{
			ID: "11111111-1111-4111-8111-111111111111", ProjectSetID: 11,
			RoundNumber: 2, CreatedByUserID: 9, Status: "completed",
		},
		Differences: []DesignDifference{
			{ID: "ai", Source: "ai", ResolutionStatus: "unresolved"},
			{ID: "own-manual", Source: "manual", CreatedByUserID: &actor, ResolutionStatus: "unresolved"},
			{ID: "other-manual", Source: "manual", CreatedByUserID: &other, ResolutionStatus: "resolved"},
		},
		Rounds:   []ReviewRound{{RoundNumber: 1, Status: "completed"}, {RoundNumber: 2, Status: "completed"}},
		DeepJobs: []DeepReviewJob{{ID: "job", CreatedByUserID: actor, Status: "failed"}},
	}}
	service := detailCapabilityService(repository)

	detail, err := service.GetReview(context.Background(), 7, testActor(actor), repository.detail.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Capabilities.CanManageDifferences || !detail.Capabilities.CanManageAnalysis {
		t.Fatalf("capabilities=%+v", detail.Capabilities)
	}
	if item := detail.Differences[0]; !item.CanEdit || item.CanDelete || !item.CanVerify || !item.CanResolve || item.CanReopen {
		t.Fatalf("AI difference capabilities=%+v", item)
	}
	if item := detail.Differences[1]; !item.CanEdit || !item.CanDelete || !item.CanVerify || !item.CanResolve {
		t.Fatalf("own manual difference capabilities=%+v", item)
	}
	if item := detail.Differences[2]; item.CanEdit || item.CanDelete || !item.CanVerify || item.CanResolve || !item.CanReopen {
		t.Fatalf("other manual difference capabilities=%+v", item)
	}
	if !detail.DeepJobs[0].CanRetry || detail.DeepJobs[0].CanCancel {
		t.Fatalf("deep job capabilities=%+v", detail.DeepJobs[0])
	}
}

func TestHistoricalReviewCapabilitiesAreFrozenUntilNewerRoundIsAbandoned(t *testing.T) {
	actor := uint(42)
	repository := &accessScopeRepository{detail: DesignReviewDetail{
		DesignReviewSummary: DesignReviewSummary{
			ID: "11111111-1111-4111-8111-111111111111", ProjectSetID: 11,
			RoundNumber: 1, CreatedByUserID: actor, Status: "completed",
		},
		Differences: []DesignDifference{{ID: "difference", Source: "ai", ResolutionStatus: "unresolved"}},
		Rounds:      []ReviewRound{{RoundNumber: 1, Status: "completed"}, {RoundNumber: 2, Status: "completed"}},
		DeepJobs:    []DeepReviewJob{{ID: "job", CreatedByUserID: actor, Status: "failed"}},
	}}
	service := detailCapabilityService(repository)

	detail, err := service.GetReview(context.Background(), 7, testActor(actor), repository.detail.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Capabilities.CanManageDifferences || detail.Capabilities.CanManageAnalysis ||
		detail.Differences[0].CanEdit || detail.Differences[0].CanVerify || detail.DeepJobs[0].CanRetry {
		t.Fatalf("historical detail must be frozen: %+v", detail)
	}

	repository.detail.Rounds[1].Status = "abandoned"
	detail, err = service.GetReview(context.Background(), 7, testActor(actor), repository.detail.ID)
	if err != nil || !detail.Capabilities.CanManageDifferences || !detail.Capabilities.CanManageAnalysis {
		t.Fatalf("abandoned newer round must not freeze source: capabilities=%+v err=%v", detail.Capabilities, err)
	}
}

func detailCapabilityService(repository Repository) *Service {
	return &Service{
		repository: repository,
		projects:   &projectAccessStub{allowed: map[uint]bool{11: true}},
		actions: &effectiveActionReaderStub{codes: []string{
			string(accesscatalog.ActionDesignReviewsReviewsView),
			string(accesscatalog.ActionDesignReviewsDifferencesManage),
			string(accesscatalog.ActionDesignReviewsAnalysisManage),
		}},
	}
}
