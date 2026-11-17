package migrations

import (
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"strings"
	"testing"
)

func TestPersonalDirectoryReconciliationAfterMain(t *testing.T) {
	source, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	next, err := source.Next(20260911173000)
	if err != nil || next != 20260914000001 {
		t.Fatalf("next=%d err=%v", next, err)
	}
	original, err := FS.ReadFile("20260911000001_personal_document_directory.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := FS.ReadFile("20260914000001_reconcile_personal_document_directory.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.NewReplacer("ADD COLUMN ", "ADD COLUMN IF NOT EXISTS ", "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ").Replace(string(original))
	if !strings.Contains(string(bridge), strings.TrimSpace(expected)) {
		t.Fatal("bridge must preserve the complete original schema")
	}
	down, err := FS.ReadFile("20260914000001_reconcile_personal_document_directory.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "RAISE EXCEPTION") || strings.Contains(string(down), "DROP TABLE") {
		t.Fatal("rollback must preserve existing personal directories")
	}
}
