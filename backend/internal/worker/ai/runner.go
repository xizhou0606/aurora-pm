package aiworker

import (
	"context"
	"log"
	"sync"
	"time"
)

// Run continuously consumes queued work with up to `concurrency` parallel
// generators. Claims are CAS-guarded, so extra loops (or extra pm-worker
// replicas) never process the same run twice. Individual provider failures
// are recorded on their run and do not terminate the consumer.
func (w *Worker) Run(ctx context.Context, pollInterval time.Duration, concurrency int) {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	if concurrency < 1 {
		concurrency = 1
	}
	var wg sync.WaitGroup
	for index := 0; index < concurrency; index++ {
		wg.Add(1)
		// Only the lead loop runs workspace-wide recovery/compensation scans;
		// followers go straight to claiming to keep the scans single-flight.
		lead := index == 0
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					processed, runErr := w.RunOnceRole(ctx, lead)
					if runErr != nil {
						log.Printf("AI run failed: %v", runErr)
					}
					if processed {
						continue
					}
				}
			}
		}()
	}
	wg.Wait()
}
