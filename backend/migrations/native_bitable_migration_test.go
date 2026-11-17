package migrations

import (
	"strings"
	"testing"
)

func TestNativeBitableMigrationIsAdditiveAndRollbackPreservesData(t *testing.T) {
	up, err := FS.ReadFile("20260907120001_add_native_bitables.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260907120001_add_native_bitables.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{string(up)} {
		for _, forbidden := range []string{"DROP TABLE", "DROP COLUMN", "TRUNCATE", "DELETE FROM"} {
			if strings.Contains(strings.ToUpper(source), forbidden) {
				t.Fatalf("native bitable migration contains destructive SQL: %s", forbidden)
			}
		}
	}
	for _, table := range []string{"resources", "tables", "records", "receipts", "assets"} {
		if !strings.Contains(string(up), "CREATE TABLE IF NOT EXISTS pm_bitable_"+table) {
			t.Fatalf("missing additive table %s", table)
		}
	}
	guard := strings.Index(string(down), "RAISE EXCEPTION")
	firstDrop := strings.Index(string(down), "DROP TABLE")
	if guard < 0 || firstDrop < guard {
		t.Fatal("rollback must check existing business data before dropping tables")
	}
	for _, condition := range []string{"schema_version = 3", "SELECT 1 FROM pm_bitable_resources", "resource_ref <> ''", "aggregate_ref <> ''"} {
		if !strings.Contains(string(down), condition) {
			t.Fatalf("missing rollback data protection: %s", condition)
		}
	}
}
