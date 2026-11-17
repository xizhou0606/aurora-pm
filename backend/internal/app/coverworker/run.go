// Package coverworker owns pm-cover-worker process assembly.
package coverworker

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"github.com/xizhou0606/aurora-pm/backend/internal/worker/requirementcover"
)

// Run starts the internal requirement-cover worker and stops it on termination.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if !cfg.RequirementCoverEnabled || !cfg.Feishu.Enabled || !cfg.ObjectStorage.Enabled {
		log.Print("pm-cover-worker is idle because requirement-cover generation, Feishu, or object storage is disabled")
		<-ctx.Done()
		return nil
	}
	if err := cfg.ValidateCoverWorkerRuntime(); err != nil {
		return fmt.Errorf("cover worker runtime configuration error: %w", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	if err := database.PrepareSchema(db, cfg.SchemaMigrationMode); err != nil {
		return fmt.Errorf("schema preparation error: %w", err)
	}
	store, err := objectstore.New(cfg.ObjectStorage)
	if err != nil {
		return fmt.Errorf("object storage error: %w", err)
	}
	renderer, err := feishu.NewRemoteDocumentCoverRenderer(
		cfg.CoverRenderer.BaseURL, cfg.CoverRenderer.InternalToken, cfg.CoverRenderer.RequestTimeout,
	)
	if err != nil {
		return fmt.Errorf("cover renderer error: %w", err)
	}
	reader := feishu.New(cfg.Feishu, feishu.WithDocumentCoverRenderer(renderer))
	worker, err := requirementcover.NewWorker(db, cfg, reader, store)
	if err != nil {
		return fmt.Errorf("cover worker error: %w", err)
	}
	worker.Run(ctx)
	return nil
}
