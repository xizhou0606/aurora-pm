package migrations

import (
	"strings"
	"testing"
)

func TestUnifiedDocumentResourceMigrationIsScopedAndAddsNoBusinessRows(t *testing.T) {
	up, err := FS.ReadFile("20260908100000_add_unified_document_resources.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260908100000_add_unified_document_resources.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	text := string(up)
	for _, expected := range []string{
		"CREATE TABLE document_resources",
		"CREATE TABLE document_resource_bindings",
		"FOREIGN KEY(workspace_id, resource_id, type_key)",
		"UNIQUE(workspace_id, type_key, payload_ref)",
		"WHERE d.archived_at IS NULL AND n.archived_at IS NULL",
		"WHERE r.deleted = false",
		"'rich_text'",
		"'bitable'",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"INSERT INTO users",
		"INSERT INTO document_nodes",
		"INSERT INTO documents (",
		"INSERT INTO pm_bitable_resources",
		"INSERT INTO access_",
	} {
		if strings.Contains(strings.ToUpper(text), strings.ToUpper(forbidden)) {
			t.Fatalf("migration must not create default business data: %s", forbidden)
		}
	}

	downText := string(down)
	guard := strings.Index(downText, "RAISE EXCEPTION")
	firstDrop := strings.Index(downText, "DROP TABLE")
	if guard < 0 || firstDrop < guard {
		t.Fatal("rollback must reject changed or newly created resources before dropping tables")
	}
	for _, expected := range []string{"LEFT JOIN document_resource_bindings", "b.resource_id IS NULL", "revision <> 1"} {
		if !strings.Contains(downText, expected) {
			t.Fatalf("down migration missing safety guard %q", expected)
		}
	}
}
