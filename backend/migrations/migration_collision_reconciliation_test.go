package migrations

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

func reconciliationSQL(t *testing.T, title string) (uint, string) {
	t.Helper()
	names, err := fs.Glob(FS, "*_"+title+".up.sql")
	if err != nil || len(names) != 1 {
		t.Fatalf("expected one %s migration: %v, %v", title, names, err)
	}
	version, err := strconv.ParseUint(strings.SplitN(names[0], "_", 2)[0], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	body, err := FS.ReadFile(names[0])
	if err != nil {
		t.Fatal(err)
	}
	return uint(version), string(body)
}

func TestCollisionReconciliationSourceOrdering(t *testing.T) {
	schedule, _ := reconciliationSQL(t, "scm_sync_schedule")
	history, _ := reconciliationSQL(t, "reconcile_external_authorization_history")
	source, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for _, pair := range [][2]uint{{20260918000000, 20260918000001}, {20260918000001, schedule}, {schedule, history}} {
		next, err := source.Next(pair[0])
		if err != nil || next != pair[1] {
			t.Fatalf("next(%d) = %d, %v; want %d", pair[0], next, err, pair[1])
		}
	}
}

// Temp fixtures shadow application tables. The transaction always rolls back;
// neither business rows nor schema_migrations are changed by this test.
func TestCollisionReconciliationPostgres(t *testing.T) {
	dsn := os.Getenv("PM_NOTIFICATION_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set PM_NOTIFICATION_TEST_DATABASE_DSN to a disposable local PostgreSQL")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test PostgreSQL configuration")
	}
	if config.Host != "127.0.0.1" && config.Host != "localhost" && config.Host != "::1" {
		t.Fatal("migration test requires a loopback PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("cannot connect to local test PostgreSQL")
	}
	defer conn.Close(ctx)
	_, schedule := reconciliationSQL(t, "scm_sync_schedule")
	version, history := reconciliationSQL(t, "reconcile_external_authorization_history")
	originalBytes, err := FS.ReadFile("20260918000000_external_authorization_history.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	original := string(originalBytes)
	_, minutes := reconciliationSQL(t, "scm_sync_schedule_minutes")
	downNames, err := fs.Glob(FS, strconv.FormatUint(uint64(version), 10)+"_*.down.sql")
	if err != nil || len(downNames) != 1 {
		t.Fatal("missing reconciliation rollback")
	}
	down, err := FS.ReadFile(downNames[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"auth_owned_collision", "scm_owned_collision", "before_collision"} {
		t.Run(state, func(t *testing.T) {
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			exec := func(sql string) {
				t.Helper()
				if _, err := tx.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			exec(`CREATE TEMP TABLE pm_external_device_authorizations (device_code_digest TEXT PRIMARY KEY, workspace_id TEXT, created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE pm_external_authorization_requests (workspace_id TEXT, created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE scm_collection_settings (workspace_id TEXT PRIMARY KEY) ON COMMIT DROP;
INSERT INTO pm_external_device_authorizations VALUES ('old-digest','workspace',now());
INSERT INTO scm_collection_settings VALUES ('workspace')`)
			if state != "scm_owned_collision" {
				exec(original)
			}
			if state == "auth_owned_collision" {
				exec(`UPDATE pm_external_device_authorizations SET record_id='d_existing', user_code='ABCD-EFGH'`)
			}
			if state == "scm_owned_collision" {
				exec(schedule)
				exec(`UPDATE scm_collection_settings SET daily_sync_hour=13, weekly_sync_weekday=4, monthly_sync_day=17`)
			}
			exec(minutes)
			exec(`UPDATE scm_collection_settings SET daily_sync_minute=23, weekly_sync_hour=14, weekly_sync_minute=24, monthly_sync_hour=15, monthly_sync_minute=25`)
			var beforeID string
			if state != "scm_owned_collision" {
				if err := tx.QueryRow(ctx, `SELECT record_id FROM pm_external_device_authorizations`).Scan(&beforeID); err != nil {
					t.Fatal(err)
				}
			}
			exec(schedule)
			exec(history)
			var firstID string
			if err := tx.QueryRow(ctx, `SELECT record_id FROM pm_external_device_authorizations`).Scan(&firstID); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(firstID, "d_") || (beforeID != "" && firstID != beforeID) {
				t.Fatal("history backfill lost or replaced public identifier")
			}
			exec(schedule)
			exec(history)
			var id string
			var code *string
			var count int
			if err := tx.QueryRow(ctx, `SELECT record_id,user_code,(SELECT count(*) FROM pm_external_device_authorizations) FROM pm_external_device_authorizations`).Scan(&id, &code, &count); err != nil {
				t.Fatal(err)
			}
			if id != firstID || count != 1 {
				t.Fatal("rerun changed history identifiers or rows")
			}
			if state == "auth_owned_collision" {
				if code == nil || *code != "ABCD-EFGH" {
					t.Fatal("existing confirmation code changed")
				}
			} else if code != nil {
				t.Fatal("migration invented unrecoverable confirmation code")
			}
			var got [8]int
			if err := tx.QueryRow(ctx, `SELECT daily_sync_hour,weekly_sync_weekday,monthly_sync_day,daily_sync_minute,weekly_sync_hour,weekly_sync_minute,monthly_sync_hour,monthly_sync_minute FROM scm_collection_settings`).Scan(&got[0], &got[1], &got[2], &got[3], &got[4], &got[5], &got[6], &got[7]); err != nil {
				t.Fatal(err)
			}
			want := [8]int{9, 1, 1, 23, 14, 24, 15, 25}
			if state == "scm_owned_collision" {
				want[0], want[1], want[2] = 13, 4, 17
			}
			if got != want {
				t.Fatalf("schedule changed: got %v want %v", got, want)
			}
			exec("SAVEPOINT rollback_rejection")
			if _, err := tx.Exec(ctx, string(down)); err == nil {
				t.Fatal("reconciliation rollback must fail closed")
			}
			exec("ROLLBACK TO SAVEPOINT rollback_rejection")
			if err := tx.QueryRow(ctx, `SELECT record_id FROM pm_external_device_authorizations`).Scan(&id); err != nil || id != firstID {
				t.Fatal("rejected rollback changed history")
			}
		})
	}
}
