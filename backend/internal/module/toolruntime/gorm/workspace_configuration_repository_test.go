package toolruntimegorm

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/glebarez/sqlite"
	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWorkspaceConfigurationRepositoryPersistsPerWorkspaceAndIncrementsVersion(t *testing.T) {
	db := workspaceConfigurationTestDB(t)
	repository := NewWorkspaceConfigurationRepository(db)
	now := time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC)

	first, err := repository.Save(context.Background(), persistedWorkspaceConfiguration(11, 7, now))
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.WorkspaceID != 11 || first.AllowedTools[0] != "project.list" ||
		first.EffectiveCLIGrantTTLHours() != toolruntime.DefaultCLIGrantTTLHours {
		t.Fatalf("first=%+v", first)
	}
	updatedInput := persistedWorkspaceConfiguration(11, 9, now.Add(time.Minute))
	updatedInput.AllowedTools = []string{"task.list"}
	updatedInput.AllowedScopes = []string{"task:read"}
	updatedInput.CLIGrantTTLHours = 30
	updated, err := repository.Save(context.Background(), updatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.UpdatedBy != 9 || updated.AllowedTools[0] != "task.list" ||
		updated.EffectiveCLIGrantTTLHours() != 30 ||
		!updated.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("updated=%+v first=%+v", updated, first)
	}
	if _, err := repository.Save(context.Background(), persistedWorkspaceConfiguration(12, 8, now)); err != nil {
		t.Fatal(err)
	}
	workspace12, err := repository.FindByWorkspace(context.Background(), 12)
	if err != nil || workspace12.WorkspaceID != 12 || workspace12.UpdatedBy != 8 {
		t.Fatalf("workspace12=%+v err=%v", workspace12, err)
	}
	if _, err := repository.FindByWorkspace(context.Background(), 99); !errors.Is(err, toolruntime.ErrWorkspaceConfigurationNotFound) {
		t.Fatalf("missing err=%v", err)
	}
}

func TestWorkspaceConfigurationRepositoryFailsClosedForMalformedStoredEntitlements(t *testing.T) {
	db := workspaceConfigurationTestDB(t)
	now := time.Now().UTC()
	row := ExternalToolingWorkspaceConfiguration{
		WorkspaceID: 11, Mode: string(toolruntime.WorkspaceConfigurationWindow), AllowedTools: `{"not":"an array"}`, AllowedScopes: `["project:read"]`,
		EffectiveFrom: testTimePointer(now.Add(-time.Hour)), ExpiresAt: testTimePointer(now.Add(time.Hour)), UpdatedBy: 7, Version: 1,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewWorkspaceConfigurationRepository(db).FindByWorkspace(context.Background(), 11); !errors.Is(err, toolruntime.ErrWorkspaceConfigurationInvalid) {
		t.Fatalf("malformed err=%v", err)
	}
}

func TestWorkspaceConfigurationRepositoryRejectsNonCanonicalEntitlements(t *testing.T) {
	db := workspaceConfigurationTestDB(t)
	repository := NewWorkspaceConfigurationRepository(db)
	configuration := persistedWorkspaceConfiguration(11, 7, time.Now().UTC())
	configuration.AllowedTools = []string{"task.list", "project.list", "project.list"}
	configuration.AllowedScopes = []string{"task:read", "project:read", "project:read"}
	if _, err := repository.Save(context.Background(), configuration); !errors.Is(err, toolruntime.ErrWorkspaceConfigurationInvalid) {
		t.Fatalf("non-canonical save err=%v", err)
	}
	var count int64
	if err := db.Model(&ExternalToolingWorkspaceConfiguration{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("persisted non-canonical row count=%d err=%v", count, err)
	}
}

func TestWorkspaceConfigurationRepositoryPersistsThreeModesAndNullWindows(t *testing.T) {
	db := workspaceConfigurationTestDB(t)
	repository := NewWorkspaceConfigurationRepository(db)
	now := time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC)

	permanent := persistedWorkspaceConfiguration(11, 7, now)
	permanent.Mode = toolruntime.WorkspaceConfigurationPermanent
	permanent.EffectiveFrom, permanent.ExpiresAt = time.Time{}, time.Time{}
	stored, err := repository.Save(context.Background(), permanent)
	if err != nil || stored.Mode != toolruntime.WorkspaceConfigurationPermanent ||
		!stored.EffectiveFrom.IsZero() || !stored.ExpiresAt.IsZero() {
		t.Fatalf("permanent=%+v err=%v", stored, err)
	}
	var row ExternalToolingWorkspaceConfiguration
	if err := db.First(&row, "workspace_id = ?", 11).Error; err != nil || row.EffectiveFrom != nil || row.ExpiresAt != nil {
		t.Fatalf("permanent row=%+v err=%v", row, err)
	}

	disabled := permanent
	disabled.Mode = toolruntime.WorkspaceConfigurationDisabled
	disabled.UpdatedAt = now
	stored, err = repository.Save(context.Background(), disabled)
	if err != nil || stored.Mode != toolruntime.WorkspaceConfigurationDisabled ||
		!slices.Equal(stored.AllowedTools, []string{"project.list"}) ||
		!slices.Equal(stored.AllowedScopes, []string{"project:read"}) || stored.Version != 2 {
		t.Fatalf("disabled=%+v err=%v", stored, err)
	}
}

func TestWorkspaceConfigurationRepositoryPostgresUpsertQualifiesTargetVersion(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB, PreferSimpleProtocol: true,
	}), &gorm.Config{
		DisableAutomaticPing: true, SkipDefaultTransaction: true,
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := workspaceConfigurationRecord(persistedWorkspaceConfiguration(11, 7, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	result := db.Session(&gorm.Session{DryRun: true}).
		Clauses(workspaceConfigurationOnConflict(row)).Create(&row)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	generated := result.Statement.SQL.String()
	if !strings.Contains(generated, "pm_external_tooling_workspace_configurations.version + 1") {
		t.Fatalf("PostgreSQL upsert must qualify the target version column: %s", generated)
	}
	if strings.Contains(generated, `"version"=version + 1`) {
		t.Fatalf("PostgreSQL upsert contains ambiguous version increment: %s", generated)
	}
}

func workspaceConfigurationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(WorkspaceConfigurationMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	return db
}

func persistedWorkspaceConfiguration(workspaceID, updatedBy uint, now time.Time) toolruntime.WorkspaceConfiguration {
	return toolruntime.WorkspaceConfiguration{
		WorkspaceID: workspaceID, Mode: toolruntime.WorkspaceConfigurationWindow, AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), UpdatedBy: updatedBy,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), Version: 1,
	}
}

func testTimePointer(value time.Time) *time.Time { return &value }
