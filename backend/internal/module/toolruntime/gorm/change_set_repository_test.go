package toolruntimegorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

func TestChangeSetRepositoryEnforcesBindingIdempotencyCASAndTerminalScrubbing(t *testing.T) {
	repository, db := newChangeSetRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	authorization := changeSetAuthorization()

	succeeded := createChangeSetDraft("change-set-succeeded-000000000000000000000000", "digest-a", "input-a", authorization, now)
	created, err := repository.CreateOrFind(context.Background(), succeeded)
	if err != nil || !created.Created || created.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	reused, err := repository.CreateOrFind(context.Background(), succeeded)
	if err != nil || reused.Created || reused.ChangeSet.ID != created.ChangeSet.ID {
		t.Fatalf("reused=%+v err=%v", reused, err)
	}
	conflict := succeeded
	conflict.ID = "change-set-conflict-0000000000000000000000000"
	conflict.InputDigest = "digest-b"
	if _, err := repository.CreateOrFind(context.Background(), conflict); !errors.Is(err, toolruntime.ErrChangeSetConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	foreignBindings := map[string]toolruntime.ExecutionAuthorization{}
	for name, mutate := range map[string]func(*toolruntime.ExecutionAuthorization){
		"principal": func(value *toolruntime.ExecutionAuthorization) { value.PrincipalID++ },
		"workspace": func(value *toolruntime.ExecutionAuthorization) { value.WorkspaceID++ },
		"grant":     func(value *toolruntime.ExecutionAuthorization) { value.GrantID++ },
		"client":    func(value *toolruntime.ExecutionAuthorization) { value.ClientID = "other-client" },
		"channel":   func(value *toolruntime.ExecutionAuthorization) { value.Channel = toolruntime.DeliveryMCP },
		"audience":  func(value *toolruntime.ExecutionAuthorization) { value.Audience = toolruntime.AudienceMCP },
	} {
		foreign := authorization
		mutate(&foreign)
		foreignBindings[name] = foreign
	}
	for name, foreign := range foreignBindings {
		if _, err := repository.FindBound(context.Background(), succeeded.ID, foreign, now); !errors.Is(err, toolruntime.ErrChangeSetNotFound) {
			t.Fatalf("foreign %s show err=%v", name, err)
		}
		if _, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
			ID: succeeded.ID, Authorization: foreign, ClaimID: "foreign-claim-00000000000000000000000000000", Now: now, ClaimLease: time.Second,
		}); !errors.Is(err, toolruntime.ErrChangeSetNotFound) {
			t.Fatalf("foreign %s confirm err=%v", name, err)
		}
	}

	claim, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: succeeded.ID, Authorization: authorization, ClaimID: "claim-0000000000000000000000000000000000001", Now: now, ClaimLease: 30 * time.Second,
	})
	if err != nil || claim.ChangeSet.Status != toolruntime.ChangeSetConfirmed || string(claim.Input) != `{"title":"input-a"}` {
		t.Fatalf("claim=%+v err=%v", claim, err)
	}
	if _, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: succeeded.ID, Authorization: authorization, ClaimID: "claim-0000000000000000000000000000000000002", Now: now.Add(time.Second), ClaimLease: 30 * time.Second,
	}); !errors.Is(err, toolruntime.ErrChangeSetInProgress) {
		t.Fatalf("concurrent claim err=%v", err)
	}
	replayed, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: succeeded.ID, Authorization: authorization, ClaimID: "claim-0000000000000000000000000000000000003", Now: now.Add(time.Minute), ClaimLease: 30 * time.Second,
	})
	if err != nil || replayed.ClaimID == claim.ClaimID {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	completed, err := repository.Succeed(context.Background(), toolruntime.ChangeSetFinishCommand{
		ID: succeeded.ID, Authorization: authorization, ClaimID: replayed.ClaimID,
		Result: json.RawMessage(`{"id":42}`), Now: now.Add(time.Minute),
	})
	if err != nil || completed.Status != toolruntime.ChangeSetSucceeded || string(completed.Result) != `{"id":42}` {
		t.Fatalf("completed=%+v err=%v", completed, err)
	}
	assertChangeSetInputScrubbed(t, db, succeeded.ID)

	failed := createChangeSetDraft("change-set-failed-000000000000000000000000000", "digest-f", "input-f", authorization, now)
	failed.IdempotencyDigest = "idempotency-f"
	_, _ = repository.CreateOrFind(context.Background(), failed)
	failedClaim, _ := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: failed.ID, Authorization: authorization, ClaimID: "claim-failed-0000000000000000000000000000000", Now: now, ClaimLease: time.Second,
	})
	failedResult, err := repository.Fail(context.Background(), toolruntime.ChangeSetFinishCommand{
		ID: failed.ID, Authorization: authorization, ClaimID: failedClaim.ClaimID, ErrorCode: "TOOL_EXECUTION_FAILED", Now: now,
	})
	if err != nil || failedResult.Status != toolruntime.ChangeSetFailed || failedResult.ErrorCode != "TOOL_EXECUTION_FAILED" {
		t.Fatalf("failed=%+v err=%v", failedResult, err)
	}
	assertChangeSetInputScrubbed(t, db, failed.ID)

	rejected := createChangeSetDraft("change-set-rejected-0000000000000000000000000", "digest-r", "input-r", authorization, now)
	rejected.IdempotencyDigest = "idempotency-r"
	_, _ = repository.CreateOrFind(context.Background(), rejected)
	rejectedResult, err := repository.RejectPending(context.Background(), rejected.ID, authorization, "TOOL_UAT_BLOCKED", now)
	if err != nil || rejectedResult.Status != toolruntime.ChangeSetRejected {
		t.Fatalf("rejected=%+v err=%v", rejectedResult, err)
	}
	assertChangeSetInputScrubbed(t, db, rejected.ID)

	claimedRejected := createChangeSetDraft("change-set-claim-rejected-00000000000000000000", "digest-cr", "input-cr", authorization, now)
	claimedRejected.IdempotencyDigest = "idempotency-cr"
	_, _ = repository.CreateOrFind(context.Background(), claimedRejected)
	claimedRejectClaim, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: claimedRejected.ID, Authorization: authorization, ClaimID: "claim-rejected-00000000000000000000000000000", Now: now, ClaimLease: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	claimedRejectedResult, err := repository.RejectClaim(context.Background(), toolruntime.ChangeSetFinishCommand{
		ID: claimedRejected.ID, Authorization: authorization, ClaimID: claimedRejectClaim.ClaimID,
		ErrorCode: "TOOL_AUTHORIZATION_DENIED", Now: now,
	})
	if err != nil || claimedRejectedResult.Status != toolruntime.ChangeSetRejected {
		t.Fatalf("claimed rejected=%+v err=%v", claimedRejectedResult, err)
	}
	assertChangeSetInputScrubbed(t, db, claimedRejected.ID)

	expired := createChangeSetDraft("change-set-expired-00000000000000000000000000", "digest-e", "input-e", authorization, now)
	expired.IdempotencyDigest, expired.ExpiresAt = "idempotency-e", now.Add(time.Second)
	_, _ = repository.CreateOrFind(context.Background(), expired)
	expiredResult, err := repository.FindBound(context.Background(), expired.ID, authorization, now.Add(time.Minute))
	if err != nil || expiredResult.Status != toolruntime.ChangeSetExpired || expiredResult.ErrorCode != "CHANGE_SET_EXPIRED" {
		t.Fatalf("expired=%+v err=%v", expiredResult, err)
	}
	assertChangeSetInputScrubbed(t, db, expired.ID)

	var events []ToolChangeSetEvent
	if err := db.Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) < 12 {
		t.Fatalf("events=%+v", events)
	}
	for _, event := range events {
		encoded, _ := json.Marshal(event)
		if string(encoded) == "" || containsAny(string(encoded), "input-a", "input-f", "input-cr", "private database failure") {
			t.Fatalf("event leaked private data: %s", encoded)
		}
	}
}

func TestChangeSetTerminalEventFailureRollsBackAndCanReplay(t *testing.T) {
	repository, db := newChangeSetRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	authorization := changeSetAuthorization()
	draft := createChangeSetDraft("change-set-event-failure-000000000000000000000", "digest-event", "input-event", authorization, now)
	if _, err := repository.CreateOrFind(context.Background(), draft); err != nil {
		t.Fatal(err)
	}
	claim, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: "claim-event-failure-000000000000000000000000", Now: now, ClaimLease: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	const callback = "test:fail_change_set_terminal_event"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == (ToolChangeSetEvent{}).TableName() {
			if event, ok := tx.Statement.Dest.(*ToolChangeSetEvent); ok && event.Status == string(toolruntime.ChangeSetSucceeded) {
				tx.AddError(errors.New("private event database failure"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	command := toolruntime.ChangeSetFinishCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: claim.ClaimID,
		Result: json.RawMessage(`{"id":42}`), Now: now.Add(time.Second),
	}
	if _, err := repository.Succeed(context.Background(), command); err == nil {
		t.Fatal("terminal event failure must fail the entire transition")
	}
	var retained ToolChangeSet
	if err := db.First(&retained, "id = ?", draft.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retained.Status != string(toolruntime.ChangeSetConfirmed) || retained.InputJSON == "" || retained.ResultJSON != "" || retained.ApplyClaimID != claim.ClaimID {
		t.Fatalf("terminal transition was not rolled back: %+v", retained)
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.Succeed(context.Background(), command)
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	assertChangeSetInputScrubbed(t, db, draft.ID)
}

func TestChangeSetReleaseClaimUsesExactBindingAndClaimCAS(t *testing.T) {
	repository, db := newChangeSetRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	authorization := changeSetAuthorization()
	draft := createChangeSetDraft("change-set-release-0000000000000000000000000", "digest-release", "input-release", authorization, now)
	if _, err := repository.CreateOrFind(context.Background(), draft); err != nil {
		t.Fatal(err)
	}
	claim, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: "claim-release-00000000000000000000000000000",
		Now: now, ClaimLease: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	wrong := toolruntime.ChangeSetReleaseCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: "wrong-claim-000000000000000000000000000000",
		ErrorCode: "CHANGE_SET_RETRYABLE", Now: now.Add(time.Second),
	}
	if _, err := repository.ReleaseClaim(context.Background(), wrong); !errors.Is(err, toolruntime.ErrChangeSetConflict) {
		t.Fatalf("wrong claim err=%v", err)
	}
	foreign := wrong
	foreign.ClaimID, foreign.Authorization.WorkspaceID = claim.ClaimID, authorization.WorkspaceID+1
	if _, err := repository.ReleaseClaim(context.Background(), foreign); !errors.Is(err, toolruntime.ErrChangeSetNotFound) {
		t.Fatalf("foreign binding err=%v", err)
	}
	released, err := repository.ReleaseClaim(context.Background(), toolruntime.ChangeSetReleaseCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: claim.ClaimID,
		ErrorCode: "CHANGE_SET_RETRYABLE", Now: now.Add(time.Second),
	})
	if err != nil || released.Status != toolruntime.ChangeSetConfirmed {
		t.Fatalf("released=%+v err=%v", released, err)
	}
	var row ToolChangeSet
	if err := db.First(&row, "id = ?", draft.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.ApplyClaimID != "" || row.ApplyClaimedAt != nil || row.InputJSON == "" || row.Status != string(toolruntime.ChangeSetConfirmed) {
		t.Fatalf("released row=%+v", row)
	}
	var retryEvents int64
	if err := db.Model(&ToolChangeSetEvent{}).Where(
		"change_set_id = ? AND status = ? AND error_code = ?", draft.ID, string(toolruntime.ChangeSetConfirmed), "CHANGE_SET_RETRYABLE",
	).Count(&retryEvents).Error; err != nil || retryEvents != 1 {
		t.Fatalf("retry events=%d err=%v", retryEvents, err)
	}
	reclaimed, err := repository.ConfirmAndClaim(context.Background(), toolruntime.ChangeSetClaimCommand{
		ID: draft.ID, Authorization: authorization, ClaimID: "claim-retry-000000000000000000000000000000",
		Now: now.Add(2 * time.Second), ClaimLease: time.Minute,
	})
	if err != nil || reclaimed.ClaimID == claim.ClaimID || string(reclaimed.Input) != `{"title":"input-release"}` {
		t.Fatalf("reclaimed=%+v err=%v", reclaimed, err)
	}
}

func newChangeSetRepository(t *testing.T) (*ChangeSetRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return NewChangeSetRepository(db), db
}

func createChangeSetDraft(id, digest, title string, authorization toolruntime.ExecutionAuthorization, now time.Time) toolruntime.ChangeSetDraft {
	input := json.RawMessage(`{"title":"` + title + `"}`)
	return toolruntime.ChangeSetDraft{
		ChangeSet: toolruntime.ChangeSet{
			ID: id, Status: toolruntime.ChangeSetPending, ToolName: "project.update", ToolVersion: "1.0.0",
			Risk: toolruntime.RiskMutation, AuditEvent: "tool.project.update", Authorization: authorization,
			InputDigest: digest, InputSize: len(input), ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now,
		},
		IdempotencyDigest: "idempotency-a", Input: input,
	}
}

func changeSetAuthorization() toolruntime.ExecutionAuthorization {
	return toolruntime.ExecutionAuthorization{
		PrincipalID: 7, WorkspaceID: 11, GrantID: 13, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
}

func assertChangeSetInputScrubbed(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	var row ToolChangeSet
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if row.InputJSON != "" {
		t.Fatalf("change set %s retained raw input %q", id, row.InputJSON)
	}
	encoded, _ := json.Marshal(changeSetSnapshot(row))
	if containsAny(string(encoded), "input-a", "input-f", "input-r", "input-cr", "input-e", "private database failure") {
		t.Fatalf("public change set leaked private data: %s", encoded)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
