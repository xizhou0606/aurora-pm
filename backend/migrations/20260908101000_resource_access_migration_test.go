package migrations

import (
	"strings"
	"testing"
)

func TestResourceAccessMigrationBackfillsLegacyWithoutGrantingAccess(t *testing.T) {
	up, err := FS.ReadFile("20260908101000_add_resource_access_policies_and_links.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260908101000_add_resource_access_policies_and_links.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	upText := string(up)
	for _, expected := range []string{
		"CREATE TABLE document_resource_access_policies",
		"CREATE TABLE document_resource_access_rules",
		"CREATE TABLE document_resource_share_links",
		"CREATE TABLE document_resource_link_access_contexts",
		"CREATE TABLE document_resource_access_receipts",
		"CREATE TABLE document_resource_access_audits",
		"CREATE TABLE document_resource_access_outbox",
		"CREATE TABLE document_share_link_exchange_limits",
		"SELECT\n workspace_id, id, 'legacy', NULL, 1",
		"include_descendants = false OR subject_kind = 'organization_unit'",
		"secret_hash ~ '^[0-9a-f]{64}$'",
	} {
		if !strings.Contains(upText, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"INSERT INTO document_resource_access_rules",
		"INSERT INTO document_resource_share_links",
		"INSERT INTO document_resource_link_access_contexts",
	} {
		if strings.Contains(upText, forbidden) {
			t.Fatalf("migration must not invent access: %s", forbidden)
		}
	}
	downText := string(down)
	guard, firstDrop := strings.Index(downText, "RAISE EXCEPTION"), strings.Index(downText, "DROP TABLE")
	if guard < 0 || firstDrop < guard {
		t.Fatal("rollback must reject runtime access writes before dropping tables")
	}
	for _, expected := range []string{
		"mode <> 'legacy'", "revision <> 1", "document_resource_access_rules",
		"document_resource_share_links", "document_share_link_exchange_limits",
	} {
		if !strings.Contains(downText, expected) {
			t.Fatalf("down migration missing guard %q", expected)
		}
	}
}
