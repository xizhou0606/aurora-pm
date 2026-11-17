// Package codecontribworker runs the periodic code-contribution collection.
// The sync model is fixed and predictable: a repository's first successful
// pass pulls ALL history across every ref, and afterwards the workspace's
// configured daily trigger (workspace timezone, 默认 09:00) syncs the
// appropriate calendar window — 日常同步昨天、每周触发日同步上周、每月触发日
// 同步上月，时机由统计口径配置，故障恢复自动从上次成功日补齐。Natural-key
// upserts make every pass idempotent: restarts neither miss data nor
// double-count.
package codecontribworker

import (
	"context"
	"log"
	"time"

	codecontrib "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib"
)

// syncRunner is the sync surface the scheduler drives. *codecontrib.SyncService
// satisfies it; tests substitute an in-memory fake.
type syncRunner interface {
	SyncDueWorkspaces(ctx context.Context, now time.Time) (codecontrib.WorkspaceSyncResult, error)
}

// Scheduler ticks every minute and hands the current time to the sync
// service; per-repository trigger judgment (initial full pull, the
// workspace's configured daily window, same-day idempotency) lives in the
// sync service.
type Scheduler struct {
	service syncRunner
	tick    time.Duration
	now     func() time.Time
}

// New wires the scheduler to the sync service.
func New(service *codecontrib.SyncService) *Scheduler {
	return &Scheduler{service: service, tick: time.Minute, now: time.Now}
}

// Run blocks until ctx is done.
func (scheduler *Scheduler) Run(ctx context.Context) {
	log.Printf("codecontrib worker: started (tick %s, per-workspace configurable schedule)", scheduler.tick)
	nowFn := scheduler.now
	if nowFn == nil {
		nowFn = time.Now
	}
	ticker := time.NewTicker(scheduler.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("codecontrib worker: stopped")
			return
		case <-ticker.C:
		}
		result, err := scheduler.service.SyncDueWorkspaces(ctx, nowFn())
		if err != nil {
			log.Printf("codecontrib worker: sync pass failed: %v", err)
			continue
		}
		logOutcomes(result)
	}
}

func logOutcomes(result codecontrib.WorkspaceSyncResult) {
	for _, outcome := range result.RepoOutcomes {
		if outcome.Error != "" {
			log.Printf("codecontrib worker: repo %d (%s) failed: %s",
				outcome.RepoID, outcome.Path, outcome.Error)
			continue
		}
		log.Printf("codecontrib worker: repo %d (%s) synced %d commits, %d requests",
			outcome.RepoID, outcome.Path, outcome.Commits, outcome.PullRequests)
	}
}
