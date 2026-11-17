package migrations

import (
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestMainSCMReconciliationOrderingAndSeed(t *testing.T) {
	source, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for _, pair := range [][2]uint{
		{20260909130005, 20260910033026},
		{20260910033026, 20260910033027},
		{20260910033027, 20260910033028},
	} {
		next, err := source.Next(pair[0])
		if err != nil || next != pair[1] {
			t.Fatalf("next(%d)=%d, %v", pair[0], next, err)
		}
	}
	bridge, err := FS.ReadFile("20260910033028_reconcile_main_scm_and_release_notes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	seed, err := FS.ReadFile("20260909110000_seed_release_notes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bridge), strings.TrimSpace(string(seed))) {
		t.Fatal("must replay main's frozen idempotent seed")
	}
	for _, fragment := range []string{"ADD COLUMN IF NOT EXISTS full_synced_at", "DROP COLUMN IF EXISTS collected_since", "DROP COLUMN IF EXISTS sync_interval_minutes", "DROP COLUMN IF EXISTS overlap_days"} {
		if !strings.Contains(string(bridge), fragment) {
			t.Fatalf("missing final SCM schema: %s", fragment)
		}
	}
	down, err := FS.ReadFile("20260910033028_reconcile_main_scm_and_release_notes.down.sql")
	if err != nil || !strings.Contains(string(down), "RAISE EXCEPTION") {
		t.Fatal("rollback must fail closed")
	}
}
