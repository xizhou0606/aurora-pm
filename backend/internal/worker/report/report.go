// Package reportworker owns the scheduled reporting cadence: daily reports at
// 09:00 (covering the previous day), weekly reports Monday 09:00 (covering
// the previous complete week), monthly reports on the 1st 09:00 (covering the
// previous complete month) — all in the business timezone. It is an internal
// pm-worker execution role, not a separately deployed unit.
package reportworker

import (
	"context"
	"log"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/report"
)

const tickInterval = time.Minute

// Scheduler drives the report scheduler on a fixed tick.
type Scheduler struct {
	scheduler *report.ReportScheduler
	tick      func(time.Duration) <-chan time.Time
	now       func() time.Time
}

func New(scheduler *report.ReportScheduler) *Scheduler {
	return &Scheduler{scheduler: scheduler, now: time.Now}
}

// Run blocks until ctx is cancelled, firing each cadence once per 09:00
// occurrence. Two layers keep one occurrence from generating twice:
//
//  1. the (previous, now] trigger window (report.Due): a 09:00 crossing is
//     observed by exactly one tick, so restarts and overlapping ticks are safe;
//  2. the per-cadence last-fired anchor (report.TriggerAnchor): the trigger
//     date recorded after a successful pass, so a clock anomaly can never
//     re-run the same window inside this process lifetime.
//
// The durable guarantee underneath both is the per-subject run dedup table in
// the report domain. The former 10-minute restart gate is deliberately gone:
// with trigger-window semantics a stalled ticker backfills the missed 09:00 on
// its next tick (回补) instead of skipping it, and dedup absorbs the extra pass.
func (worker *Scheduler) Run(ctx context.Context) {
	if worker == nil || worker.scheduler == nil {
		return
	}
	nowFn := worker.now
	if nowFn == nil {
		nowFn = time.Now
	}
	location := report.ScheduleLocation()
	schedules := []report.PeriodType{
		report.PeriodDaily,
		report.PeriodWeekly,
		report.PeriodMonthly,
	}
	// 上次触发锚点：每个粒度记录最近一次成功触发的 trigger date，同一窗口
	// 不重复生成；跨进程的持久去重仍由 report 域的 dedup 表承担。
	lastAnchors := make(map[report.PeriodType]string, len(schedules))
	previous := nowFn()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := nowFn()
			for _, periodType := range schedules {
				if !report.Due(periodType, location, previous, now) {
					continue
				}
				anchor, ok := report.TriggerAnchor(periodType, location, now)
				if ok && lastAnchors[periodType] == anchor {
					continue
				}
				summaries, err := worker.scheduler.GenerateForAllWorkspaces(ctx, periodType, location)
				if err != nil {
					log.Printf("report scheduler %s pass error: %v", periodType, err)
					continue
				}
				if ok {
					lastAnchors[periodType] = anchor
				}
				created, skipped, failed := 0, 0, 0
				for _, summary := range summaries {
					created += summary.Created
					skipped += summary.Skipped
					failed += summary.Failed
				}
				passAnchor := report.AnchorDateFor(location, now)
				if len(summaries) > 0 {
					passAnchor = summaries[0].AnchorDate
				}
				log.Printf("report scheduler %s pass (anchor %s): workspaces=%d created=%d skipped=%d failed=%d",
					periodType, passAnchor, len(summaries), created, skipped, failed)
			}
			previous = now
		}
	}
}
