package aiworker

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"time"
)

const (
	staleAIRunGrace = time.Minute
)

// recoverStaleAIRuns returns runs abandoned by a terminated worker to the
// queue. Retaining started_at lets the next claim advance the fence beyond the
// abandoned attempt, so a late response cannot finish the replacement run.
func (w *Worker) recoverStaleAIRuns() (bool, error) {
	now := w.now()
	cutoff := now.Add(-(w.cfg.AI.RequestTimeout + staleAIRunGrace))
	result := w.db.Model(&aigorm.AIRun{}).
		Where("status = ? AND started_at IS NOT NULL AND started_at < ?", "running", cutoff).
		Updates(map[string]any{
			"status":        "queued",
			"completed_at":  nil,
			"error_message": "",
		})
	return result.RowsAffected > 0, result.Error
}
