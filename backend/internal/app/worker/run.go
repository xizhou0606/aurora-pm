// Package worker owns pm-worker process assembly. It deliberately contains no
// AI business behavior or database query logic.
package worker

import (
	"context"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"os/signal"
	"strings"
	"syscall"

	"gorm.io/gorm"

	scmapp "github.com/xizhou0606/aurora-pm/backend/internal/app/scm"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"github.com/xizhou0606/aurora-pm/backend/internal/worker/ai"
	codecontribworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/codecontrib"
	designreviewworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/designreview"
	identityorgworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/identityorg"
	reportworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/report"
	workflowdelivery "github.com/xizhou0606/aurora-pm/backend/internal/worker/workflowdelivery"
)

// Run creates the process lifetime context used by the standalone worker.
func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return RunContext(ctx)
}

// RunContext is the testable worker entrypoint. Cancellation stops polling
// after the in-flight run observes the supplied context.
func RunContext(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	if err := database.PrepareSchema(db, cfg.SchemaMigrationMode); err != nil {
		return fmt.Errorf("schema preparation error: %w", err)
	}
	var reader feishu.Reader
	var orgReader *feishu.SDKReader
	if cfg.Feishu.Enabled {
		orgReader = feishu.New(cfg.Feishu)
		reader = orgReader
	}
	// The scheduled reporting cadence shares the AI run pipeline and stops
	// with the same process lifetime context.
	if cfg.AI.Enabled {
		scheduler, err := AssembleReportScheduler(db, cfg)
		if err != nil {
			return fmt.Errorf("report scheduler bootstrap error: %w", err)
		}
		go reportworker.New(scheduler).Run(ctx)
	}
	// Code contribution collection is settings-driven and AI-independent:
	// it is a no-op until a workspace binds repositories. 触发判定在 sync
	// service 内：接入全量 + 每日 09:00 定时自然窗。
	scmRuntime, err := scmapp.Assemble(db)
	if err != nil {
		return fmt.Errorf("codecontrib bootstrap error: %w", err)
	}
	go codecontribworker.New(scmRuntime.Sync).Run(ctx)
	// The delivery reconcile consumer re-evaluates requirement gates when
	// evidence or scope changes; it is core workflow behavior, not AI-bound.
	deliveryConsumer, err := workflowdelivery.NewConsumer(db)
	if err != nil {
		return fmt.Errorf("delivery reconcile consumer bootstrap error: %w", err)
	}
	go deliveryConsumer.Run(ctx)
	// The scheduled Feishu contact org sync shares the daily 09:00 cadence and
	// only covers workspaces owning a Feishu-bound member. It stays silent in
	// test environments; sync failures log and the next occurrence retries.
	if cfg.Feishu.Enabled && !strings.EqualFold(cfg.Environment, "test") {
		orgSync, err := identityorgworker.NewSyncService(orgReader, db)
		if err != nil {
			return fmt.Errorf("feishu org sync bootstrap error: %w", err)
		}
		go identityorgworker.New(orgSync, identityorgworker.NewGormWorkspaceSource(db)).Run(ctx)
	}
	go designreviewworker.New(db, cfg).Run(ctx, 0)
	NewAIWorker(db, cfg, reader).Run(ctx, 0, cfg.AI.WorkerConcurrency)
	return nil
}

// AssembleReportScheduler wires the report scheduler to the same module-owned
// read surfaces the interactive reporting API uses. Both process assemblies
// (standalone pm-worker and pm-api's embedded local-development worker) start
// the scheduler through this constructor.
func AssembleReportScheduler(db *gorm.DB, cfg config.Config) (*report.ReportScheduler, error) {
	directory, err := identity.NewDirectoryService(identitygorm.NewReportingDirectoryRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report directory bootstrap error: %w", err)
	}
	tasks, err := task.NewBIReadService(taskgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report task facts bootstrap error: %w", err)
	}
	bugs, err := quality.NewBIReadService(qualitygorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report bug facts bootstrap error: %w", err)
	}
	requirements, err := requirement.NewBIReadService(requirementgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report requirement facts bootstrap error: %w", err)
	}
	runs, err := ai.NewReportRunStartService(aigorm.NewReportRunRepository(db, cfg.AI.Enabled, cfg.AI.Model), cfg.AI.Enabled)
	if err != nil {
		return nil, fmt.Errorf("AI report run bootstrap error: %w", err)
	}
	dedup, err := ai.NewReportRunDedupService(aigorm.NewReportRunRepository(db, cfg.AI.Enabled, cfg.AI.Model))
	if err != nil {
		return nil, fmt.Errorf("AI report run dedup bootstrap error: %w", err)
	}
	runReads, err := ai.NewReportRunReadService(aigorm.NewReportRunRepository(db, cfg.AI.Enabled, cfg.AI.Model))
	if err != nil {
		return nil, fmt.Errorf("AI report run read bootstrap error: %w", err)
	}
	// Scheduled reports must carry the same code-contribution evidence as the
	// interactive API path; otherwise 定时周报/月报 silently lose the 代码活动
	// section and the AI narrative loses its code facts.
	scmRuntime, err := scmapp.Assemble(db)
	if err != nil {
		return nil, fmt.Errorf("codecontrib bootstrap error: %w", err)
	}
	service, err := report.NewService(report.Dependencies{
		Directory:     directory,
		Tasks:         tasks,
		Bugs:          bugs,
		Requirements:  requirements,
		Runs:          runs,
		RunResults:    runReads,
		Contributions: scmRuntime.ReportReads,
	})
	if err != nil {
		return nil, err
	}
	return report.NewReportScheduler(service, dedup, directory)
}

// NewAIWorker keeps embedded and standalone source adapters identical.
func NewAIWorker(db *gorm.DB, cfg config.Config, reader feishu.Reader) *aiworker.Worker {
	worker := aiworker.New(db, cfg, reader)
	worker.ConfigureDocumentSources(func(tx *gorm.DB) (*docsource.Service, error) { return documentread.NewSources(tx, reader) })
	return worker
}
