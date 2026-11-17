package native

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type postgresRecorder struct{}

func (postgresRecorder) RecordBitableMutation(context.Context, bitable.MutationEffect) error {
	return nil
}
func postgresRepository(t *testing.T) (*Repository, bitable.Scope, bitable.Bootstrap) {
	t.Helper()
	dsn := os.Getenv("PM_TABLE_TEST_DSN")
	if dsn == "" {
		t.Skip("PM_TABLE_TEST_DSN not supplied; PostgreSQL locking not verified")
	}
	// This test is intentionally bound to the task-owned disposable database.
	config, parseErr := pgconn.ParseConfig(dsn)
	if parseErr != nil || config.Host != "127.0.0.1" || config.Port != 5439 || config.Database != "pm_table_test" {
		t.Fatal("refusing non-task PostgreSQL test target")
	}
	for _, fallback := range config.Fallbacks {
		if fallback.Host != "127.0.0.1" || fallback.Port != 5439 {
			t.Fatal("refusing non-task PostgreSQL fallback")
		}
	}
	base, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("cannot open disposable PostgreSQL")
	}
	schema := "tst_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; e != nil {
		t.Fatal("cannot create disposable test schema")
	}
	db, e := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal("cannot open scoped PostgreSQL")
	}
	sql, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sql.SetMaxOpenConns(12)
	t.Cleanup(func() {
		_ = sql.Close()
		if err := base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; err != nil {
			t.Error("cannot clean exact disposable test schema")
		}
		conn, _ := base.DB()
		if conn != nil {
			_ = conn.Close()
		}
	})
	if e = db.AutoMigrate(MigrationRecords()...); e != nil {
		t.Fatal(e)
	}
	repo := NewRepository(db)
	repo.ConfigureMutationRecorderFactory(func(*gorm.DB) (bitable.MutationRecorder, error) { return postgresRecorder{}, nil })
	repo.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return &mutationGuard{}, nil })
	repo.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) { return &leaseConflictGuard{}, nil })
	repo.ConfigureCommittedEventAppenderFactory(func(*gorm.DB) (bitable.CommittedEventAppender, error) { return discardEventAppender{}, nil })
	p := bitable.Principal{UserID: 10, WorkspaceID: 20}
	resource, e := repo.Create(context.Background(), p, "PG synthetic", uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	s := bitable.Scope{Principal: p, ResourceID: resource.ID, DocumentResourceID: "document-" + resource.ID, PolicyRevision: 1, TableID: resource.Tables[0].ID}
	b, e := repo.Bootstrap(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	return repo, s, b
}
func TestPostgresConcurrentCAS(t *testing.T) {
	r, s, b := postgresRepository(t)
	row := mustCreate(t, r, s, b, map[string]any{b.Fields[0].ID: "before"})
	start := make(chan struct{})
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := r.Execute(context.Background(), s, batch(b.SchemaRevision, update(row.ID, b.Fields[0].ID, row.FieldRevisions[b.Fields[0].ID], "after")))
			results <- e
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	committed := 0
	for e := range results {
		if e == nil {
			committed++
		} else if !errors.Is(e, bitable.ErrConflict) {
			t.Fatal(e)
		}
	}
	if committed != 1 {
		t.Fatalf("expected one winner, got %d", committed)
	}
	page, e := r.Query(context.Background(), s, query(b))
	if e != nil || page.Records[0].Revision != 2 {
		t.Fatal(e)
	}
}

func TestPostgresConcurrentDifferentFieldsOnSameRowBothCommit(t *testing.T) {
	r, s, b := postgresRepository(t)
	second := addTextField(t, r, s, b.SchemaRevision, "Second")
	b, err := r.Bootstrap(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	first := b.Fields[0]
	row := mustCreate(t, r, s, b, map[string]any{first.ID: "a", second.ID: "b"})
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, input := range []bitable.Command{
		update(row.ID, first.ID, row.FieldRevisions[first.ID], "a-1"),
		update(row.ID, second.ID, row.FieldRevisions[second.ID], "b-1"),
	} {
		wg.Add(1)
		go func(command bitable.Command) {
			defer wg.Done()
			<-start
			_, executeErr := r.Execute(context.Background(), s, batch(b.SchemaRevision, command))
			results <- executeErr
		}(input)
	}
	close(start)
	wg.Wait()
	close(results)
	for executeErr := range results {
		if executeErr != nil {
			t.Fatalf("different field conflicted: %v", executeErr)
		}
	}
	page, err := r.Query(context.Background(), s, query(b))
	if err != nil || page.Records[0].Values[first.ID] != "a-1" || page.Records[0].Values[second.ID] != "b-1" {
		t.Fatalf("concurrent merge lost a field: %#v %v", page, err)
	}
}
func TestPostgresConcurrentCreateIdempotency(t *testing.T) {
	r, s, _ := postgresRepository(t)
	key := uuid.NewString()
	start := make(chan struct{})
	results := make(chan bitable.Resource, 6)
	failures := make(chan error, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, e := r.Create(context.Background(), s.Principal, "Same resource", key)
			results <- v
			failures <- e
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	id := ""
	for v := range results {
		if id == "" {
			id = v.ID
		}
		if id != v.ID {
			t.Fatal("duplicate resources created")
		}
	}
}
