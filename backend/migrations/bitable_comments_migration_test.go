package migrations

import (
	"strings"
	"testing"
)

func TestBitableCommentsIncrementalMigration(t *testing.T) {
	up, e := FS.ReadFile("20260907180000_add_bitable_record_comments.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	down, e := FS.ReadFile("20260907180000_add_bitable_record_comments.down.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, part := range []string{"CREATE TABLE pm_bitable_comments", "REFERENCES pm_bitable_records(id)", "char_length(body)", "idx_pm_bitable_comments_scope"} {
		if !strings.Contains(string(up), part) {
			t.Fatal(part)
		}
	}
	if strings.Contains(string(up), "DROP TABLE") || strings.Contains(string(up), "INSERT INTO") {
		t.Fatal("destructive or default comment migration")
	}
	guard := strings.Index(string(down), "RAISE EXCEPTION")
	drop := strings.Index(string(down), "DROP TABLE")
	if guard < 0 || drop < guard {
		t.Fatal("unguarded rollback")
	}
}
