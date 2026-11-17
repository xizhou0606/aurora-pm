package migrations

import (
	"strings"
	"testing"
)

func TestDocumentSchemaVersionMigrationAllowsV1AndV2(t *testing.T) {
	up, err := FS.ReadFile("20260904165230_expand_document_schema_version.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260904165230_expand_document_schema_version.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(up), "CHECK (schema_version IN (1, 2))") {
		t.Fatal("up migration must allow canonical document schema versions 1 and 2")
	}
	for _, expected := range []string{
		"WHERE schema_version <> 1",
		"CHECK (schema_version = 1)",
	} {
		if !strings.Contains(string(down), expected) {
			t.Fatalf("down migration missing %q", expected)
		}
	}
}
