package migrations

import (
	"strings"
	"testing"
)

func TestPlanUnitRequirementsLockMigration(t *testing.T) {
	up, err := FS.ReadFile("20260914103001_add_plan_unit_requirements_lock.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := FS.ReadFile("20260914103001_add_plan_unit_requirements_lock.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	upText := string(up)
	for _, expected := range []string{
		"ALTER TABLE plan_units ADD COLUMN IF NOT EXISTS requirements_locked_at TIMESTAMPTZ NULL",
		"ALTER TABLE plan_units ADD COLUMN IF NOT EXISTS requirements_locked_by_user_id BIGINT NULL",
	} {
		if !strings.Contains(upText, expected) {
			t.Fatalf("up migration missing %q", expected)
		}
	}
	if strings.Count(upText, "ALTER TABLE plan_units") != 2 {
		t.Fatalf("up migration must only touch plan_units: %s", upText)
	}
	downText := string(down)
	for _, expected := range []string{
		"DROP COLUMN IF EXISTS requirements_locked_by_user_id",
		"DROP COLUMN IF EXISTS requirements_locked_at",
	} {
		if !strings.Contains(downText, expected) {
			t.Fatalf("down migration missing %q", expected)
		}
	}
}
