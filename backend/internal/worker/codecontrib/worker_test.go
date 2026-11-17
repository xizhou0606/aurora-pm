package codecontribworker

import (
	"context"
	"testing"
	"time"

	codecontrib "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
)

type fakeSyncRunner struct {
	calls  int
	times  []time.Time
	result codecontrib.WorkspaceSyncResult
	err    error
}

func (fake *fakeSyncRunner) SyncDueWorkspaces(_ context.Context, now time.Time) (codecontrib.WorkspaceSyncResult, error) {
	fake.calls++
	fake.times = append(fake.times, now)
	return fake.result, fake.err
}

// The scheduler is a thin clock: every tick hands the current time to the
// sync service, which owns all trigger judgment (initial full pull, the
// daily 09:00 window, same-day idempotency).
func TestRunTicksPassCurrentTimeToSyncService(t *testing.T) {
	runner := &fakeSyncRunner{}
	location := time.FixedZone("CST", 8*3600)
	clock := time.Date(2026, 9, 9, 9, 0, 0, 0, location)
	scheduler := &Scheduler{service: runner, tick: 5 * time.Millisecond, now: func() time.Time {
		clock = clock.Add(5 * time.Millisecond)
		return clock
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for runner.calls < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if runner.calls < 3 {
		t.Fatalf("scheduler ticks = %d, want at least 3", runner.calls)
	}
	for _, callTime := range runner.times {
		if callTime.Location().String() != location.String() {
			t.Fatalf("tick time location = %s, want the scheduler clock's location", callTime.Location())
		}
	}
}
