package collaborationgorm

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func postgresLeaseRepositories(t *testing.T) (*Repository, *Repository) {
	t.Helper()
	dsn := os.Getenv("PM_TABLE_TEST_DSN")
	if dsn == "" {
		t.Skip("PM_TABLE_TEST_DSN not supplied; PostgreSQL collaboration locking not verified")
	}
	config, err := pgconn.ParseConfig(dsn)
	if err != nil || config.Host != "127.0.0.1" || config.Port != 5439 || config.Database != "pm_table_test" {
		t.Fatal("refusing non-task PostgreSQL test target")
	}
	for _, fallback := range config.Fallbacks {
		if fallback.Host != "127.0.0.1" || fallback.Port != 5439 {
			t.Fatal("refusing non-task PostgreSQL fallback")
		}
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	schema := "tst_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	openScoped := func() *gorm.DB {
		db, openErr := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if openErr != nil {
			t.Fatal(openErr)
		}
		return db
	}
	firstDB, secondDB := openScoped(), openScoped()
	if err = firstDB.AutoMigrate(&CellLeaseRecord{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, db := range []*gorm.DB{firstDB, secondDB} {
			if sqlDB, sqlErr := db.DB(); sqlErr == nil {
				_ = sqlDB.Close()
			}
		}
		if dropErr := base.Exec(`DROP SCHEMA "` + schema + `" CASCADE`).Error; dropErr != nil {
			t.Error(dropErr)
		}
	})
	first, _ := NewRepository(firstDB)
	second, _ := NewRepository(secondDB)
	return first, second
}

func TestPostgresCellLeaseCrossInstanceRaceHasOneWinner(t *testing.T) {
	first, second := postgresLeaseRepositories(t)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	services := []*collaboration.CellLeaseService{leaseService(t, first, access), leaseService(t, second, access)}
	principals := []collaboration.Principal{leasePrincipal(7, "connection-a", "Alice"), leasePrincipal(8, "connection-b", "Bob")}
	key := cellKey()
	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var wait sync.WaitGroup
	for index := range services {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, err := services[index].Acquire(context.Background(), principals[index], key, "editor-"+string(rune('a'+index)))
			errorsByAttempt <- err
		}(index)
	}
	close(start)
	wait.Wait()
	close(errorsByAttempt)
	winners, held := 0, 0
	for err := range errorsByAttempt {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, collaboration.ErrLeaseHeld):
			held++
		default:
			t.Fatal(err)
		}
	}
	if winners != 1 || held != 1 {
		t.Fatalf("winners=%d held=%d", winners, held)
	}
}

func TestPostgresDifferentFieldsCanAcquireConcurrently(t *testing.T) {
	first, second := postgresLeaseRepositories(t)
	access := &leaseAccessFake{decision: collaboration.AccessDecision{
		CanView: true, CanEdit: true, PolicyRevision: 4, ExpiresAt: time.Now().Add(time.Minute),
	}}
	keyA, keyB := cellKey(), cellKey()
	keyB.Room, keyB.TableID, keyB.RecordID = keyA.Room, keyA.TableID, keyA.RecordID
	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var wait sync.WaitGroup
	for _, attempt := range []struct {
		service   *collaboration.CellLeaseService
		principal collaboration.Principal
		key       collaboration.CellKey
		editor    string
	}{
		{leaseService(t, first, access), leasePrincipal(7, "connection-a", "Alice"), keyA, "editor-a"},
		{leaseService(t, second, access), leasePrincipal(8, "connection-b", "Bob"), keyB, "editor-b"},
	} {
		attempt := attempt
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := attempt.service.Acquire(context.Background(), attempt.principal, attempt.key, attempt.editor)
			errorsByAttempt <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByAttempt)
	for err := range errorsByAttempt {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPostgresBatchLeaseCrossInstanceRaceIsAtomicAndDeadlockFree(t *testing.T) {
	first, second := postgresLeaseRepositories(t)
	access := &leaseAccessFake{decision: allowedLeaseAccess(4)}
	services := []*collaboration.CellLeaseService{leaseService(t, first, access), leaseService(t, second, access)}
	principals := []collaboration.Principal{leasePrincipal(7, "connection-a", "Alice"), leasePrincipal(8, "connection-b", "Bob")}
	keyA, keyB := cellKey(), cellKey()
	keyB.Room, keyB.TableID, keyB.RecordID = keyA.Room, keyA.TableID, keyA.RecordID
	keySets := [][]collaboration.CellKey{{keyB, keyA}, {keyA, keyB}}
	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var wait sync.WaitGroup
	for index, repository := range []*Repository{first, second} {
		index, repository := index, repository
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			err := repository.db.Transaction(func(tx *gorm.DB) error {
				acquirer, createErr := NewCellLeaseBatchAcquirer(tx, access)
				if createErr != nil {
					return createErr
				}
				_, acquireErr := services[index].AcquireBatch(context.Background(), acquirer, principals[index],
					"editor-"+string(rune('a'+index)), keySets[index])
				return acquireErr
			})
			errorsByAttempt <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByAttempt)
	winners, held := 0, 0
	for err := range errorsByAttempt {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, collaboration.ErrLeaseHeld):
			held++
		default:
			t.Fatal(err)
		}
	}
	if winners != 1 || held != 1 {
		t.Fatalf("batch winners=%d held=%d", winners, held)
	}
	var active []CellLeaseRecord
	if err := first.db.Where("lease_id IS NOT NULL").Find(&active).Error; err != nil || len(active) != 2 ||
		active[0].OwnerConnectionID == nil || active[1].OwnerConnectionID == nil ||
		*active[0].OwnerConnectionID != *active[1].OwnerConnectionID {
		t.Fatalf("batch was partially owned: %#v, %v", active, err)
	}
}
