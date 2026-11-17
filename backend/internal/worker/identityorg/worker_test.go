package identityorgworker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
)

type fakeOrgSyncer struct {
	mu        sync.Mutex
	calls     int
	deadlines []bool
	errOnCall map[int]error
	stats     identity.FeishuOrgSyncStats
}

func (fake *fakeOrgSyncer) SyncOnce(ctx context.Context) (identity.FeishuOrgSyncStats, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.calls++
	_, bounded := ctx.Deadline()
	fake.deadlines = append(fake.deadlines, bounded)
	if err, ok := fake.errOnCall[fake.calls]; ok {
		return identity.FeishuOrgSyncStats{}, err
	}
	return fake.stats, nil
}

func (fake *fakeOrgSyncer) callCount() int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.calls
}

func (fake *fakeOrgSyncer) allCallsBounded() bool {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, bounded := range fake.deadlines {
		if !bounded {
			return false
		}
	}
	return true
}

type fakeWorkspaceSource struct {
	ids []uint
	err error
}

func (fake fakeWorkspaceSource) ListFeishuBoundWorkspaceIDs(context.Context) ([]uint, error) {
	return fake.ids, fake.err
}

// TestDueAt is the table-driven contract for the daily 09:00 trigger: it
// reuses the report schedule's Due + TriggerAnchor so the org sync fires on
// the exact same 09:00 rhythm as the code sync and the report generation,
// once per occurrence (anchor dedup).
func TestDueAt(t *testing.T) {
	location := report.ScheduleLocation()
	at := func(year int, month time.Month, day, hour, minute int) time.Time {
		return time.Date(year, month, day, hour, minute, 0, 0, location)
	}
	tests := []struct {
		name          string
		previous      time.Time
		now           time.Time
		lastAnchor    string
		wantAnchor    string
		wantTriggered bool
	}{
		{"fires crossing 09:00", at(2026, 9, 4, 8, 59), at(2026, 9, 4, 9, 0), "", "2026-09-04", true},
		{"anchor dedup suppresses refire same day", at(2026, 9, 4, 9, 0), at(2026, 9, 4, 10, 0), "2026-09-04", "", false},
		{"silent before 09:00", at(2026, 9, 4, 2, 0), at(2026, 9, 4, 5, 0), "", "", false},
		{"crossing midnight without 09:00 stays silent", at(2026, 9, 4, 23, 0), at(2026, 9, 5, 2, 0), "2026-09-04", "", false},
		{"fires again next natural day", at(2026, 9, 4, 12, 0), at(2026, 9, 5, 9, 0), "2026-09-04", "2026-09-05", true},
		{"stalled ticker backfills the crossed 09:00", at(2026, 9, 4, 8, 0), at(2026, 9, 4, 11, 30), "", "2026-09-04", true},
		{"process started after 09:00 does not refire today", at(2026, 9, 4, 10, 0), at(2026, 9, 4, 11, 0), "", "", false},
		{"unknown anchor state still dedups by equality", at(2026, 9, 4, 8, 59), at(2026, 9, 4, 9, 1), "2026-09-04", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			anchor, ok := dueAt(location, test.previous, test.now, test.lastAnchor)
			if ok != test.wantTriggered || anchor != test.wantAnchor {
				t.Fatalf("dueAt(prev=%s, now=%s, last=%q) = (%q, %v), want (%q, %v)",
					test.previous, test.now, test.lastAnchor, anchor, ok, test.wantAnchor, test.wantTriggered)
			}
		})
	}
}

// TestDueAtFiresOnBusinessTimezoneNineOClock pins the trigger to 09:00 in the
// report schedule's business location, not the server's local clock: the same
// wall-clock moment expressed in UTC (01:00 for UTC+8) must fire.
func TestDueAtFiresOnBusinessTimezoneNineOClock(t *testing.T) {
	location := report.ScheduleLocation()
	_, offsetSeconds := time.Date(2026, 9, 4, 9, 0, 0, 0, location).Zone()
	if offsetSeconds != 8*3600 {
		t.Fatalf("schedule location offset = %ds, want UTC+8", offsetSeconds)
	}
	previous := time.Date(2026, 9, 4, 0, 55, 0, 0, time.UTC)
	now := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC) // 09:00 业务时区
	anchor, ok := dueAt(location, previous, now, "")
	if !ok || anchor != "2026-09-04" {
		t.Fatalf("dueAt(utc 01:00) = (%q, %v), want (2026-09-04, true)", anchor, ok)
	}
}

func TestRunScheduledSyncSyncsEveryFeishuBoundWorkspace(t *testing.T) {
	syncer := &fakeOrgSyncer{}
	worker := New(syncer, fakeWorkspaceSource{ids: []uint{7, 2}})
	worker.runScheduledSync(context.Background())
	if got := syncer.callCount(); got != 2 {
		t.Fatalf("sync calls = %d, want 2 (one per Feishu-bound workspace)", got)
	}
	if !syncer.allCallsBounded() {
		t.Fatal("every sync call must carry a timeout deadline")
	}
}

func TestRunScheduledSyncKeepsGoingAfterFailure(t *testing.T) {
	t.Run("sync failure only logs", func(t *testing.T) {
		syncer := &fakeOrgSyncer{errOnCall: map[int]error{1: errors.New("feishu down")}}
		worker := New(syncer, fakeWorkspaceSource{ids: []uint{7, 2}})
		worker.runScheduledSync(context.Background())
		if got := syncer.callCount(); got != 2 {
			t.Fatalf("sync calls = %d, want 2: one failure must not abort the pass", got)
		}
	})
	t.Run("in-progress conflict only skips", func(t *testing.T) {
		syncer := &fakeOrgSyncer{errOnCall: map[int]error{1: identity.ErrFeishuOrgSyncInProgress}}
		worker := New(syncer, fakeWorkspaceSource{ids: []uint{7, 2}})
		worker.runScheduledSync(context.Background())
		if got := syncer.callCount(); got != 2 {
			t.Fatalf("sync calls = %d, want 2: an in-progress conflict must not abort the pass", got)
		}
	})
}

func TestRunScheduledSyncSkipsWithoutWork(t *testing.T) {
	t.Run("enumeration error syncs nothing", func(t *testing.T) {
		syncer := &fakeOrgSyncer{}
		worker := New(syncer, fakeWorkspaceSource{err: errors.New("db down")})
		worker.runScheduledSync(context.Background())
		if got := syncer.callCount(); got != 0 {
			t.Fatalf("sync calls = %d, want 0 on enumeration failure", got)
		}
	})
	t.Run("no Feishu-bound workspaces syncs nothing", func(t *testing.T) {
		syncer := &fakeOrgSyncer{}
		worker := New(syncer, fakeWorkspaceSource{})
		worker.runScheduledSync(context.Background())
		if got := syncer.callCount(); got != 0 {
			t.Fatalf("sync calls = %d, want 0 without Feishu-bound workspaces", got)
		}
	})
}

type scriptedClock struct {
	mu    sync.Mutex
	value time.Time
	reads int
}

func (clock *scriptedClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.reads++
	return clock.value
}

func (clock *scriptedClock) Set(moment time.Time) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.value = moment
}

func (clock *scriptedClock) readCount() int {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.reads
}

// TestRunLoopSyncsOncePerDailyAnchor drives the real Run loop with a
// millisecond tick and a scripted clock: the tick that crosses 09:00 fires
// exactly one pass, and the anchor dedup keeps the rest of the day silent.
func TestRunLoopSyncsOncePerDailyAnchor(t *testing.T) {
	location := report.ScheduleLocation()
	at := func(hour, minute int) time.Time {
		return time.Date(2026, 9, 4, hour, minute, 0, 0, location)
	}
	clock := &scriptedClock{value: at(8, 0)}
	syncer := &fakeOrgSyncer{}
	worker := &Worker{sync: syncer, workspaces: fakeWorkspaceSource{ids: []uint{7}}, tick: time.Millisecond, now: clock.Now}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	// 等 Run 循环先以 08:00 完成至少一次判定,再推进时钟到 09:00 之后。
	deadline := time.Now().Add(2 * time.Second)
	for clock.readCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clock.readCount() < 3 {
		t.Fatal("worker loop did not start ticking")
	}
	clock.Set(at(9, 1))
	deadline = time.Now().Add(2 * time.Second)
	for syncer.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if syncer.callCount() == 0 {
		t.Fatal("crossing 09:00 must trigger exactly the first scheduled sync")
	}
	// 当天继续推进:锚点去重保证不再触发。
	clock.Set(at(10, 0))
	time.Sleep(30 * time.Millisecond)
	if got := syncer.callCount(); got != 1 {
		t.Fatalf("sync calls = %d, want 1: the daily anchor must not refire", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run must stop when ctx is done")
	}
}
