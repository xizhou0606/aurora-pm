package migrations

import (
	"strings"
	"testing"
)

func TestCollaborationDocumentPersistenceMigrationHasAuthorityAndSizeGuards(t *testing.T) {
	up, err := FS.ReadFile("20260908104000_add_collaboration_document_persistence.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260908104000_add_collaboration_document_persistence.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(up)
	for _, expected := range []string{
		"CREATE TABLE collaboration_document_states",
		"CREATE TABLE collaboration_document_updates",
		"PRIMARY KEY (workspace_id, resource_id)",
		"UNIQUE (workspace_id, resource_id, durable_revision)",
		"UNIQUE (workspace_id, resource_id, operation_id)",
		"CHECK (durable_revision = base_durable_revision + 1)",
		"octet_length(update) > 0 AND octet_length(update) <= 262144",
		"octet_length(snapshot) > 0 AND octet_length(snapshot) <= 4194304",
		"jsonb_typeof(projection) = 'object'",
		"REFERENCES document_resources(workspace_id, id)",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	if strings.Contains(strings.ToUpper(text), "INSERT INTO") {
		t.Fatal("document collaboration migration must not seed or cut over business documents")
	}
	downText := string(down)
	guard := strings.Index(downText, "RAISE EXCEPTION")
	drop := strings.Index(downText, "DROP TABLE")
	if guard < 0 || drop < guard {
		t.Fatal("rollback must refuse deleting authoritative document facts")
	}
}
