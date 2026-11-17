package designreviewgorm

import (
	"context"
	"testing"
	"time"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func TestListReviewsUsesLatestNonAbandonedRound(t *testing.T) {
	db := openRecheckDB(t)
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	rootID := "62000000-0000-4000-8000-000000000001"
	abandonedID := "62000000-0000-4000-8000-000000000002"
	abandonedAt := now.Add(time.Minute)
	for _, row := range []Review{
		{ID: rootID, WorkspaceID: 7, ChainRootID: &rootID, RoundNumber: 1, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "completed", CreatedAt: now, UpdatedAt: now},
		{ID: abandonedID, WorkspaceID: 7, ChainRootID: &rootID, SourceReviewID: &rootID, RoundNumber: 2, CreatedByUserID: 42, ProjectSetIDSnapshot: 11, Status: "abandoned", AbandonedAt: &abandonedAt, CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
	} {
		validReviewRecord(&row)
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := NewRepository(db).ListReviews(context.Background(), 7, designreview.ReviewListFilter{})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != rootID {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
}
