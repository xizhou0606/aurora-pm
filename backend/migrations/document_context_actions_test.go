package migrations

import (
	"strings"
	"testing"
)

func TestDocumentContextActionMigrationKeepsSystemAndReferenceConstraints(t *testing.T) {
	up, err := FS.ReadFile("20260904150000_add_document_context_actions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260904150000_add_document_context_actions.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"CHECK (type IN ('folder', 'document', 'shortcut'))",
		"chk_document_nodes_shortcut_target",
		"chk_document_nodes_system_shape",
		"ux_document_favorites_folder",
		"ux_document_favorite_reference",
		"document_user_preferences",
		"document_asset_references",
		"owner_user_id",
	} {
		if !strings.Contains(string(up), expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	for _, expected := range []string{
		"DROP TABLE IF EXISTS document_asset_references",
		"DROP TABLE IF EXISTS document_user_preferences",
		"DROP COLUMN IF EXISTS owner_user_id",
		"ADD CONSTRAINT chk_document_nodes_type CHECK (type IN ('folder', 'document'))",
	} {
		if !strings.Contains(string(down), expected) {
			t.Fatalf("down migration missing %q", expected)
		}
	}
}
