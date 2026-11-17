package aiworker

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

func TestRunActivityWatcherIgnoresTransientDatabaseErrorAndCancelsOnConfirmedStatus(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_run_activity_transient_error?mode=memory&cache=shared",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now().UTC()
	run := aigorm.AIRun{WorkspaceID: 1, Status: "running", StartedAt: &startedAt}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	const callbackName = "test:transient_run_activity_error"
	errorInjected := make(chan struct{})
	injected := false
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != "ai_runs" {
			return
		}
		injected = true
		tx.AddError(errors.New("transient run activity query failure"))
		close(errorInjected)
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Callback().Query().Remove(callbackName) }()

	requestCtx, stopWatching := New(db, cfg, nil).contextWhileRunIsActive(context.Background(), &run)
	defer stopWatching()

	select {
	case <-errorInjected:
	case <-time.After(4 * runStatusPollInterval):
		t.Fatal("run activity query was not polled")
	}
	select {
	case <-requestCtx.Done():
		t.Fatal("transient database error cancelled an active model request")
	case <-time.After(runStatusPollInterval / 2):
	}

	if err := db.Model(&aigorm.AIRun{}).Where("id = ? AND workspace_id = ?", run.ID, run.WorkspaceID).Update("status", "cancelled").Error; err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestCtx.Done():
	case <-time.After(4 * runStatusPollInterval):
		t.Fatal("confirmed run cancellation did not stop the model request")
	}
}

func TestRunActivityWatcherCancelsWhenClaimIsReplaced(t *testing.T) {
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    "file:worker_run_activity_replaced_claim?mode=memory&cache=shared",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	run := aigorm.AIRun{WorkspaceID: 1, Status: "running", StartedAt: &startedAt}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	requestCtx, stopWatching := New(db, cfg, nil).contextWhileRunIsActive(context.Background(), &run)
	defer stopWatching()
	replacementStartedAt := startedAt.Add(time.Second)
	if err := db.Model(&aigorm.AIRun{}).
		Where("id = ? AND workspace_id = ? AND status = ?", run.ID, run.WorkspaceID, "running").
		Update("started_at", replacementStartedAt).Error; err != nil {
		t.Fatal(err)
	}
	select {
	case <-requestCtx.Done():
	case <-time.After(4 * runStatusPollInterval):
		t.Fatal("replaced claim did not stop the old model request")
	}
}
