package migrations

import (
	"strings"
	"testing"
)

func TestCollaborationMigrationHasDurabilityAndRollbackGuards(t *testing.T) {
	up, err := FS.ReadFile("20260908102000_add_collaboration_events_and_presence.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260908102000_add_collaboration_events_and_presence.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	text := string(up)
	for _, expected := range []string{
		"CREATE TABLE collaboration_resource_streams",
		"CREATE TABLE collaboration_committed_events",
		"CREATE TABLE collaboration_presence",
		"UNIQUE (workspace_id, resource_id, sequence)",
		"UNIQUE (workspace_id, resource_id, operation_id)",
		"jsonb_typeof(payload) = 'object'",
		"REFERENCES document_resources(workspace_id, id)",
		"REFERENCES collaboration_resource_streams(workspace_id, resource_id)",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	if strings.Contains(strings.ToUpper(text), "INSERT INTO") {
		t.Fatal("collaboration migration must not seed business or presence data")
	}

	downText := string(down)
	guard := strings.Index(downText, "RAISE EXCEPTION")
	firstDrop := strings.Index(downText, "DROP TABLE")
	if guard < 0 || firstDrop < guard {
		t.Fatal("rollback must reject deleting committed history before dropping tables")
	}
}
