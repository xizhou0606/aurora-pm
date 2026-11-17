package migrations

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// This integration test shadows the real table with a transaction-local temp
// table. It never changes the application's receipts or migration cursor.
func TestInboxMessageCategoryPostgres(t *testing.T) {
	dsn := os.Getenv("PM_NOTIFICATION_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set PM_NOTIFICATION_TEST_DATABASE_DSN to a disposable local PostgreSQL")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test PostgreSQL configuration")
	}
	if config.Host != "127.0.0.1" && config.Host != "localhost" && config.Host != "::1" {
		t.Fatal("notification migration test requires a loopback PostgreSQL")
	}
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal("cannot connect to local test PostgreSQL")
	}
	defer conn.Close(ctx)
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
	rejected := func(sql string) {
		t.Helper()
		exec("SAVEPOINT expected_rejection")
		if _, err := tx.Exec(ctx, sql); err == nil {
			t.Fatal("expected category constraint rejection")
		}
		exec("ROLLBACK TO SAVEPOINT expected_rejection")
		exec("RELEASE SAVEPOINT expected_rejection")
	}
	exec(`CREATE TEMP TABLE notification_inbox_receipts (
 id BIGSERIAL PRIMARY KEY, category TEXT NOT NULL CHECK (category IN ('mention','access')), read_at TIMESTAMPTZ
) ON COMMIT DROP`)
	exec("INSERT INTO notification_inbox_receipts(category,read_at) VALUES ('mention',now()),('access',NULL)")
	rejected("INSERT INTO notification_inbox_receipts(category) VALUES ('message')")
	up, err := FS.ReadFile("20260917000000_inbox_message_category.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260917000000_inbox_message_category.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	exec(string(up))
	exec("INSERT INTO notification_inbox_receipts(category) VALUES ('message')")
	rejected("INSERT INTO notification_inbox_receipts(category) VALUES ('password_reset')")
	rejected(string(down))
	var total, read int
	if err := tx.QueryRow(ctx, "SELECT count(*),count(read_at) FROM notification_inbox_receipts").Scan(&total, &read); err != nil {
		t.Fatal(err)
	}
	if total != 3 || read != 1 {
		t.Fatalf("migration changed receipt history: rows=%d read=%d", total, read)
	}
	// Failed rollback retained the expanded constraint and existing messages.
	exec("INSERT INTO notification_inbox_receipts(category) VALUES ('message')")
	// Only the disposable fixture is removed to verify an empty-message downgrade.
	exec("DELETE FROM notification_inbox_receipts WHERE category='message'")
	exec(string(down))
	rejected("INSERT INTO notification_inbox_receipts(category) VALUES ('message')")
	// Legacy fresh installs may have been baselined from GORM without a CHECK.
	exec("ALTER TABLE notification_inbox_receipts DROP CONSTRAINT notification_inbox_receipts_category_check")
	exec(string(up))
	exec("INSERT INTO notification_inbox_receipts(category) VALUES ('message')")
	rejected("INSERT INTO notification_inbox_receipts(category) VALUES ('unknown')")
}
