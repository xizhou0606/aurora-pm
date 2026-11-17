package database

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"gorm.io/gorm"

	pmmigrations "github.com/xizhou0606/aurora-pm/backend/migrations"
)

const latestTimestampMigrationVersion uint = 20260921060827

func TestLatestSQLMigrationVersionReadsCutoverFile(t *testing.T) {
	version, err := latestSQLMigrationVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != latestTimestampMigrationVersion {
		t.Fatalf("最新 SQL 版本=%d，期望=%d（迁移撞号修复）", version, latestTimestampMigrationVersion)
	}
}

func TestPostCutoverSQLMigrationsUseUniquePairedUTCTimestamps(t *testing.T) {
	entries, err := pmmigrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^(\d{14})_([a-z0-9]+(?:_[a-z0-9]+)*)\.(up|down)\.sql$`)
	paired := map[string]map[string]bool{}
	descriptionsByVersion := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || isSQLMigrationCompatibilityMarker(name) {
			continue
		}
		parts := pattern.FindStringSubmatch(name)
		if parts == nil {
			t.Fatalf("切换后的 migration %q 必须使用 UTC YYYYMMDDHHMMSS_description.(up|down).sql", name)
		}
		version, parseErr := strconv.ParseUint(parts[1], 10, 64)
		if parseErr != nil || version <= 1 {
			t.Fatalf("migration %q 的时间戳版本无效", name)
		}
		if description, exists := descriptionsByVersion[parts[1]]; exists && description != parts[2] {
			t.Fatalf("migration 版本 %s 被 %q 和 %q 重复使用", parts[1], description, parts[2])
		}
		descriptionsByVersion[parts[1]] = parts[2]
		key := fmt.Sprintf("%s_%s", parts[1], parts[2])
		if paired[key] == nil {
			paired[key] = map[string]bool{}
		}
		if paired[key][parts[3]] {
			t.Fatalf("migration %q 重复定义了 %s 方向", key, parts[3])
		}
		paired[key][parts[3]] = true
	}
	for key, directions := range paired {
		if !directions["up"] || !directions["down"] {
			t.Fatalf("migration %q 必须同时提供 up 和 down 文件", key)
		}
	}
}

func TestSQLMigrationCompatibilityMarkersStayPaired(t *testing.T) {
	for _, name := range []string{
		"000001_cutover_to_versioned_sql.up.sql",
		"000001_cutover_to_versioned_sql.down.sql",
		"000003_legacy_project_participants.up.sql",
		"000003_legacy_project_participants.down.sql",
	} {
		if _, err := pmmigrations.FS.ReadFile(name); err != nil {
			t.Fatalf("compatibility migration %q is missing: %v", name, err)
		}
	}
}

func isSQLMigrationCompatibilityMarker(name string) bool {
	return strings.HasPrefix(name, "000001_cutover_to_versioned_sql.") ||
		strings.HasPrefix(name, "000003_legacy_project_participants.")
}

func TestShouldRunLegacyGormMigrate(t *testing.T) {
	tests := []struct {
		name           string
		sqlVersion     uint
		sqlErr         error
		gormCurrent    bool
		wantLegacyGorm bool
	}{
		{name: "empty postgres", sqlErr: migrate.ErrNilVersion, wantLegacyGorm: true},
		{name: "already cut over", sqlVersion: 1, wantLegacyGorm: false},
		{name: "future SQL versions", sqlVersion: 2, wantLegacyGorm: false},
		{name: "synced GORM schema before cutover", sqlErr: migrate.ErrNilVersion, gormCurrent: true, wantLegacyGorm: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldRunLegacyGormMigrate(test.sqlVersion, test.sqlErr, test.gormCurrent)
			if got != test.wantLegacyGorm {
				t.Fatalf("got %v, want %v", got, test.wantLegacyGorm)
			}
		})
	}
}

func TestShouldBaselineFreshPostgres(t *testing.T) {
	tests := []struct {
		name         string
		freshInstall bool
		sqlVersion   uint
		sqlErr       error
		gormCurrent  bool
		want         bool
	}{
		{name: "brand-new database", freshInstall: true, sqlErr: migrate.ErrNilVersion, want: true},
		{name: "existing legacy database", sqlErr: migrate.ErrNilVersion},
		{name: "already cut over", freshInstall: true, sqlVersion: 1},
		{name: "gorm baseline already recorded", freshInstall: true, sqlErr: migrate.ErrNilVersion, gormCurrent: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := shouldBaselineFreshPostgres(
				test.freshInstall,
				test.sqlVersion,
				test.sqlErr,
				test.gormCurrent,
			)
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidatePostgresSQLMigrations(t *testing.T) {
	t.Run("missing table", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		err := validatePostgresSQLMigrations(db)
		if err == nil || !strings.Contains(err.Error(), "PM_SCHEMA_MIGRATION_MODE=apply") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("dirty", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		createSQLSchemaMigrations(t, db, 1, true)
		err := validatePostgresSQLMigrations(db)
		if err == nil || !strings.Contains(err.Error(), "dirty") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("stale version", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		createSQLSchemaMigrations(t, db, 0, false)
		err := validatePostgresSQLMigrations(db)
		if err == nil || !strings.Contains(err.Error(), "not current") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("current", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		createSQLSchemaMigrations(t, db, latestTimestampMigrationVersion, false)
		if err := validatePostgresSQLMigrations(db); err != nil {
			t.Fatal(err)
		}
	})
}

func createSQLSchemaMigrations(t *testing.T, db *gorm.DB, version uint, dirty bool) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE schema_migrations (version bigint not null primary key, dirty boolean not null)`).Error; err != nil {
		t.Fatal(err)
	}
	dirtyValue := 0
	if dirty {
		dirtyValue = 1
	}
	if err := db.Exec(`INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)`, version, dirtyValue).Error; err != nil {
		t.Fatal(err)
	}
}
