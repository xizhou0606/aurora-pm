package migrations

import (
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestBitableMainMergeBridgePreservesMainSources(t *testing.T) {
	bridge, err := FS.ReadFile("20260909130001_reconcile_bitable_main_merge.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"20260907070735_add_feishu_org_structure.up.sql",
		"20260907120000_add_scm_code_contrib.up.sql",
		"20260908120000_add_scm_daily_line_breakdown.up.sql",
		"20260908130000_add_scm_commit_diff_sample.up.sql",
		"20260908200000_add_users_feishu_job_title.up.sql",
	} {
		source, err := FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(bridge), strings.TrimSpace(string(source))) {
			t.Fatalf("bridge must preserve main migration %s", name)
		}
	}
	for _, forbidden := range []string{"DROP TABLE", "DROP COLUMN", "TRUNCATE", "DELETE FROM", "UPDATE ", "INSERT INTO"} {
		if strings.Contains(strings.ToUpper(string(bridge)), forbidden) {
			t.Fatalf("non-additive bridge: %s", forbidden)
		}
	}
	down, err := FS.ReadFile("20260909130001_reconcile_bitable_main_merge.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "RAISE EXCEPTION") || strings.Contains(string(down), "DROP TABLE") {
		t.Fatal("bridge rollback must fail closed without removing existing data")
	}
}

func TestBitableMainMergeMigrationOrdering(t *testing.T) {
	source, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for _, pair := range [][2]uint{
		{20260907120000, 20260907120001},
		{20260907120001, 20260907180000},
		{20260909130000, 20260909130001},
	} {
		next, err := source.Next(pair[0])
		if err != nil || next != pair[1] {
			t.Fatalf("next(%d)=%d, %v; want %d", pair[0], next, err, pair[1])
		}
	}
}
