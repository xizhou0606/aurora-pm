package migrations

import (
	"strings"
	"testing"
)

func TestCollaborationCellLeaseMigrationHasAuthorityAndRollbackGuards(t *testing.T) {
	up, err := FS.ReadFile("20260908103000_add_collaboration_cell_leases.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260908103000_add_collaboration_cell_leases.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(up)
	for _, expected := range []string{
		"CREATE TABLE collaboration_cell_leases",
		"PRIMARY KEY (workspace_id, resource_id, table_id, record_id, field_id)",
		"table_id uuid NOT NULL",
		"record_id uuid NOT NULL",
		"field_id uuid NOT NULL",
		"fencing_token bigint NOT NULL DEFAULT 0",
		"FOREIGN KEY (workspace_id, resource_id) REFERENCES document_resources(workspace_id, id)",
		"owner_connection_id IS NULL",
		"owner_connection_id IS NOT NULL",
		"WHERE lease_id IS NOT NULL",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	if strings.Contains(strings.ToUpper(text), "INSERT INTO") {
		t.Fatal("cell lease migration must not seed business data")
	}
	downText := string(down)
	guard := strings.Index(downText, "RAISE EXCEPTION")
	drop := strings.Index(downText, "DROP TABLE")
	if guard < 0 || drop < guard || !strings.Contains(downText, "expires_at > clock_timestamp()") {
		t.Fatal("rollback must refuse dropping live leases")
	}
}
