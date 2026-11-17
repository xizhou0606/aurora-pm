package migrations

import (
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestDesignReviewReconciliationRunsAfterCurrentMainAndReplaysSchema(t *testing.T) {
	source, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	next, err := source.Next(20260910033028)
	if err != nil || next != 20260911000001 {
		t.Fatalf("next(20260910033028)=%d, %v; want 20260911000001", next, err)
	}

	next, err = source.Next(next)
	if err != nil || next != 20260911173000 {
		t.Fatalf("next design-review bridge=%d, %v", next, err)
	}

	legacy, err := FS.ReadFile("20260909031500_design_reviews.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := FS.ReadFile("20260911173000_reconcile_design_reviews.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.ReplaceAll(string(legacy), "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")
	expected = strings.ReplaceAll(expected, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ")
	expected = strings.ReplaceAll(expected, "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ")
	if !strings.Contains(string(bridge), strings.TrimSpace(expected)) {
		t.Fatal("reconciliation must replay the complete design-review schema additively")
	}
	for _, forbidden := range []string{"DROP TABLE ", "DROP COLUMN ", "TRUNCATE ", "DELETE FROM "} {
		if strings.Contains(strings.ToUpper(string(bridge)), forbidden) {
			t.Fatalf("reconciliation contains destructive SQL: %s", forbidden)
		}
	}

	down, err := FS.ReadFile("20260911173000_reconcile_design_reviews.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "RAISE EXCEPTION") || strings.Contains(strings.ToUpper(string(down)), "DROP TABLE") {
		t.Fatal("reconciliation rollback must fail closed without deleting review evidence")
	}
}
