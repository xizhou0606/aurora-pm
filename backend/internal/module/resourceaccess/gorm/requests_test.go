package resourceaccessgorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestAccessRequestsDeduplicateReviewReadRevokeAndTenantIsolation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	viewer := resourceaccess.LevelViewer
	storedViewer := string(viewer)
	seedPolicy(t, db, "dr_request", resourceaccess.ModeWorkspace, &storedViewer, now)
	repo := NewRepository(db)
	owner := uint(7)
	repo.ConfigureRequestOwnerGuard(func(_ *gorm.DB, m resourceaccess.RequestMutation) error {
		if m.OwnerUserID != owner || m.RequestID != 0 && m.Principal.UserID != owner {
			return resourceaccess.ErrNotFound
		}
		return nil
	})
	events := &policyEventAppenderStub{}
	repo.ConfigurePolicyRevisionEventAppenderFactory(func(*gorm.DB) (resourceaccess.PolicyRevisionEventAppender, error) { return events, nil })
	notifications := 0
	repo.ConfigureRequestNotificationAppender(func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error { notifications++; return nil })
	create := resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 8}, ResourceID: "dr_request", OwnerUserID: 7, Reason: "编辑需求", Now: now}
	request, err := repo.CreateAccessRequest(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repo.CreateAccessRequest(ctx, create)
	if err != nil || replay.ID != request.ID || notifications != 0 {
		t.Fatalf("duplicate or checkbox: %+v %v notifications=%d", replay, err, notifications)
	}
	read := true
	review := resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: create.ResourceID, OwnerUserID: 7, RequestID: request.ID, Read: &read, Now: now}
	marked, err := repo.UpdateAccessRequest(ctx, review)
	if err != nil || !marked.Read || marked.Status != resourceaccess.RequestPending {
		t.Fatalf("read changed approval: %+v %v", marked, err)
	}
	count, _ := repo.CountPendingAccessRequests(ctx, review.Principal, review.ResourceID, true, now)
	if count != 1 {
		t.Fatalf("read changed pending count: %d", count)
	}
	review.Read = nil
	review.Decision = resourceaccess.RequestApproved
	approved, err := repo.UpdateAccessRequest(ctx, review)
	if err != nil || !approved.Read {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	if _, err = repo.UpdateAccessRequest(ctx, review); err != nil {
		t.Fatal(err)
	}
	if notifications != 1 || len(events.events) != 1 || events.events[0].PreviousRevision != 1 || events.events[0].PolicyRevision != 2 {
		t.Fatalf("idempotency/revision: %+v notifications=%d", events, notifications)
	}
	grants, err := repo.ApprovedRequestResources(ctx, create.Principal, []documentlibrary.ResourceID{create.ResourceID})
	if err != nil || len(grants[create.ResourceID]) != 1 || grants[create.ResourceID][0].Level != resourceaccess.LevelEditor {
		t.Fatalf("grants=%v err=%v", grants, err)
	}
	other := create.Principal
	other.WorkspaceID = 2
	grants, err = repo.ApprovedRequestResources(ctx, other, []documentlibrary.ResourceID{create.ResourceID})
	if err != nil || len(grants) != 0 {
		t.Fatalf("cross workspace grant: %v %v", grants, err)
	}
	rows, _ := repo.ListAccessRequests(ctx, other, create.ResourceID, true)
	if len(rows) != 0 {
		t.Fatal("cross workspace list")
	}
	otherMutation := review
	otherMutation.Principal.WorkspaceID = 2
	if _, err = repo.UpdateAccessRequest(ctx, otherMutation); !errors.Is(err, resourceaccess.ErrNotFound) {
		t.Fatalf("cross workspace update=%v", err)
	}
	owner = 9
	if _, err = repo.UpdateAccessRequest(ctx, review); !errors.Is(err, resourceaccess.ErrNotFound) {
		t.Fatalf("stale owner=%v", err)
	}
	review.Principal.UserID = 9
	review.OwnerUserID = 9
	review.Decision = resourceaccess.RequestRevoked
	if _, err = repo.UpdateAccessRequest(ctx, review); err != nil {
		t.Fatal(err)
	}
	grants, _ = repo.ApprovedRequestResources(ctx, create.Principal, []documentlibrary.ResourceID{create.ResourceID})
	if len(grants) != 0 {
		t.Fatal("revoked grant remained")
	}
	policy, err := repo.GetPolicy(ctx, 1, create.ResourceID)
	if err != nil || policy.Mode != resourceaccess.ModeWorkspace || policy.WorkspaceLevel == nil || *policy.WorkspaceLevel != viewer || policy.Revision != 3 {
		t.Fatalf("existing policy changed: %+v %v", policy, err)
	}
	if events.events[0].IdempotencyKey == events.events[1].IdempotencyKey {
		t.Fatal("revoke reused approval event key")
	}
	create.NotifyOwner = true
	if _, err = repo.CreateAccessRequest(ctx, create); !errors.Is(err, resourceaccess.ErrNotFound) {
		t.Fatal("old owner accepted")
	}
	create.OwnerUserID = 9
	if _, err = repo.CreateAccessRequest(ctx, create); err != nil || notifications != 3 {
		t.Fatalf("notify checked: %v count=%d", err, notifications)
	}
}

type requestTestClock struct{ now time.Time }

func TestApprovalPersistsLevelDurationAndConflictingReplay(t *testing.T) {
	for _, duration := range []resourceaccess.LinkDuration{resourceaccess.LinkDuration24Hours, resourceaccess.LinkDuration7Days, resourceaccess.LinkDurationNever} {
		t.Run(string(duration), func(t *testing.T) {
			db := testDB(t)
			ctx := context.Background()
			now := time.Now().UTC()
			seedPolicy(t, db, "dr_terms", resourceaccess.ModePrivate, nil, now)
			repo := NewRepository(db)
			repo.ConfigureRequestClock(&requestTestClock{now: now})
			repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error { return nil })
			events := &policyEventAppenderStub{}
			repo.ConfigurePolicyRevisionEventAppenderFactory(func(*gorm.DB) (resourceaccess.PolicyRevisionEventAppender, error) { return events, nil })
			repo.ConfigureRequestNotificationAppender(func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error { return nil })
			request, err := repo.CreateAccessRequest(ctx, resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 8}, ResourceID: "dr_terms", OwnerUserID: 7, Reason: "edit"})
			if err != nil {
				t.Fatal(err)
			}
			mutation := resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: "dr_terms", OwnerUserID: 7, RequestID: request.ID, Decision: resourceaccess.RequestApproved, GrantLevel: resourceaccess.LevelViewer, GrantDuration: duration}
			approved, err := repo.UpdateAccessRequest(ctx, mutation)
			expected := resourceaccess.GrantExpiry(now, duration)
			if err != nil || approved.GrantLevel != resourceaccess.LevelViewer || (approved.GrantExpiresAt == nil) != (expected == nil) {
				t.Fatalf("approved=%+v err=%v", approved, err)
			}
			if expected != nil && !approved.GrantExpiresAt.Equal(*expected) {
				t.Fatal("grant expiry mismatch")
			}
			if _, err = repo.UpdateAccessRequest(ctx, mutation); err != nil || len(events.events) != 1 {
				t.Fatalf("replay=%v", err)
			}
			mutation.GrantLevel = resourceaccess.LevelEditor
			if _, err = repo.UpdateAccessRequest(ctx, mutation); !errors.Is(err, resourceaccess.ErrConflict) {
				t.Fatalf("conflicting replay=%v", err)
			}
		})
	}
}

func (c *requestTestClock) Now() time.Time { return c.now }

func TestRequestExpiryUsesClockAfterOwnerLockAndAllowsReapplication(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	seedPolicy(t, db, "dr_expiry", resourceaccess.ModePrivate, nil, base)
	repo := NewRepository(db)
	clock := &requestTestClock{now: base}
	repo.ConfigureRequestClock(clock)
	repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error { return nil })
	create := resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 8}, ResourceID: "dr_expiry", OwnerUserID: 7, Reason: "edit", Now: base}
	request, err := repo.CreateAccessRequest(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate waiting for the resource lock across the exact 72-hour deadline.
	repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error {
		clock.now = base.Add(resourceaccess.RequestLifetime)
		return nil
	})
	_, err = repo.UpdateAccessRequest(ctx, resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: create.ResourceID, OwnerUserID: 7, RequestID: request.ID, Decision: resourceaccess.RequestApproved, Now: base.Add(resourceaccess.RequestLifetime - time.Nanosecond)})
	if !errors.Is(err, resourceaccess.ErrConflict) {
		t.Fatalf("expired approval=%v", err)
	}
	var stored AccessRequestRecord
	if err = db.First(&stored, request.ID).Error; err != nil || stored.Status != resourceaccess.RequestExpired {
		t.Fatalf("stored=%+v error=%v", stored, err)
	}
	count, err := repo.CountPendingAccessRequests(ctx, create.Principal, create.ResourceID, true, clock.now)
	if err != nil || count != 0 {
		t.Fatalf("expired count=%d err=%v", count, err)
	}
	grants, err := repo.ApprovedRequestResources(ctx, create.Principal, []documentlibrary.ResourceID{create.ResourceID})
	if err != nil || len(grants) != 0 {
		t.Fatalf("expired grants=%v err=%v", grants, err)
	}
	replacement, err := repo.CreateAccessRequest(ctx, create)
	if err != nil || replacement.ID == request.ID || replacement.Status != resourceaccess.RequestPending {
		t.Fatalf("reapplication=%+v err=%v", replacement, err)
	}
}

func TestRequestListPrioritizesPendingOverOneHundredHistoryRows(t *testing.T) {
	db := testDB(t)
	now := time.Now().UTC()
	rows := []AccessRequestRecord{{WorkspaceID: 1, ResourceID: "dr_list", ApplicantUserID: 8, Status: resourceaccess.RequestPending, CreatedAt: now.Add(-time.Hour)}}
	for i := 0; i < 101; i++ {
		rows = append(rows, AccessRequestRecord{WorkspaceID: 1, ResourceID: "dr_list", ApplicantUserID: 9, Status: resourceaccess.RequestRejected, CreatedAt: now})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	listed, err := NewRepository(db).ListAccessRequests(context.Background(), resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, "dr_list", true)
	if err != nil || len(listed) != 101 || listed[0].Status != resourceaccess.RequestPending {
		t.Fatalf("list length=%d err=%v", len(listed), err)
	}
}

func TestRequestNotificationFailureRollsBackMutation(t *testing.T) {
	db := testDB(t)
	now := time.Now().UTC()
	seedPolicy(t, db, "dr_rollback", resourceaccess.ModePrivate, nil, now)
	repo := NewRepository(db)
	repo.ConfigureRequestOwnerGuard(func(*gorm.DB, resourceaccess.RequestMutation) error { return nil })
	repo.ConfigureRequestNotificationAppender(func(*gorm.DB, resourceaccess.RequestMutation, uint, uint) error { return errors.New("outbox failed") })
	mutation := resourceaccess.RequestMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 8}, ResourceID: "dr_rollback", OwnerUserID: 7, Reason: "edit", NotifyOwner: true, Now: now}
	if _, err := repo.CreateAccessRequest(context.Background(), mutation); err == nil {
		t.Fatal("expected failure")
	}
	var count int64
	db.Model(&AccessRequestRecord{}).Count(&count)
	if count != 0 {
		t.Fatal("request committed without notification")
	}
}
