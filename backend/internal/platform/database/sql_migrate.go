package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	postgresmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/gorm"

	pmmigrations "github.com/xizhou0606/aurora-pm/backend/migrations"
)

func isPostgres(db *gorm.DB) bool {
	return db.Dialector.Name() == "postgres"
}

func latestSQLMigrationVersion() (uint, error) {
	entries, err := pmmigrations.FS.ReadDir(".")
	if err != nil {
		return 0, err
	}
	var latest uint
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		migration, err := source.DefaultParse(entry.Name())
		if err != nil {
			continue
		}
		if migration.Version > latest {
			latest = migration.Version
		}
	}
	if latest == 0 {
		return 0, fmt.Errorf("no golang-migrate SQL files in backend/migrations")
	}
	return latest, nil
}

func shouldRunLegacyGormMigrate(sqlVersion uint, sqlErr error, gormCurrent bool) bool {
	if sqlErr == nil && sqlVersion >= 1 {
		return false
	}
	if gormCurrent {
		return false
	}
	return true
}

func shouldBaselineFreshPostgres(
	freshInstall bool,
	sqlVersion uint,
	sqlErr error,
	gormCurrent bool,
) bool {
	return freshInstall && shouldRunLegacyGormMigrate(sqlVersion, sqlErr, gormCurrent)
}

func postgresSchemaIsEmpty(db *gorm.DB) (bool, error) {
	var tableCount int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = current_schema()
		  AND table_type = 'BASE TABLE'
	`).Scan(&tableCount).Error; err != nil {
		return false, fmt.Errorf("inspect postgres schema: %w", err)
	}
	return tableCount == 0, nil
}

func migratePostgres(db *gorm.DB) error {
	freshInstall, err := postgresSchemaIsEmpty(db)
	if err != nil {
		return err
	}
	sourceDriver, err := iofs.New(pmmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open SQL migration source: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("sql database: %w", err)
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sql connection: %w", err)
	}
	driver, err := postgresmigrate.WithConnection(ctx, conn, &postgresmigrate.Config{
		MultiStatementEnabled: true,
		MultiStatementMaxSize: postgresmigrate.DefaultMultiStatementMaxSize,
	})
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("postgres migration driver: %w", err)
	}
	migrator, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		_ = driver.Close()
		return fmt.Errorf("golang-migrate: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()

	sqlVersion, dirty, sqlErr := migrator.Version()
	if sqlErr != nil && !errors.Is(sqlErr, migrate.ErrNilVersion) {
		return fmt.Errorf("read schema_migrations: %w", sqlErr)
	}
	if dirty {
		return fmt.Errorf("schema_migrations is dirty at version %d; repair before starting", sqlVersion)
	}
	gormCurrent, err := schemaVersionApplied(db, currentSchemaVersion)
	if err != nil {
		return err
	}
	if shouldRunLegacyGormMigrate(sqlVersion, sqlErr, gormCurrent) {
		if err := migrateLegacyGORM(db); err != nil {
			return err
		}
		if shouldBaselineFreshPostgres(freshInstall, sqlVersion, sqlErr, gormCurrent) {
			// The current GORM records create the complete schema for a brand-new
			// install. Replaying historical post-cutover SQL would try to add and
			// rename columns that already exist. Existing databases still follow
			// the normal incremental migration path below.
			latest, err := latestSQLMigrationVersion()
			if err != nil {
				return err
			}
			if err := migrator.Force(int(latest)); err != nil {
				return fmt.Errorf("record fresh postgres migration baseline: %w", err)
			}
			return db.FirstOrCreate(&schemaMigration{Version: currentSchemaVersion}).Error
		}
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply SQL migrations: %w", err)
	}
	return db.FirstOrCreate(&schemaMigration{Version: currentSchemaVersion}).Error
}

func validatePostgresSQLMigrations(db *gorm.DB) error {
	latest, err := latestSQLMigrationVersion()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable("schema_migrations") {
		return fmt.Errorf(
			"database SQL migration version %d is not applied; run the designated migrator with PM_SCHEMA_MIGRATION_MODE=apply",
			latest,
		)
	}
	var row struct {
		Version uint `gorm:"column:version"`
		Dirty   bool `gorm:"column:dirty"`
	}
	if err := db.Table("schema_migrations").Take(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf(
				"database SQL migration version %d is not applied; run the designated migrator with PM_SCHEMA_MIGRATION_MODE=apply",
				latest,
			)
		}
		return err
	}
	if row.Dirty {
		return fmt.Errorf("schema_migrations is dirty at version %d; repair before starting", row.Version)
	}
	if row.Version != latest {
		return fmt.Errorf(
			"database SQL migration version %d is not current %d; run the designated migrator with PM_SCHEMA_MIGRATION_MODE=apply",
			row.Version, latest,
		)
	}
	return nil
}
