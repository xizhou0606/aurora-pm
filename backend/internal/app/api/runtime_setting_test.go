package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

func TestResolveLocalJWTSecretUsesDatabaseRecord(t *testing.T) {
	cfg := config.Config{LocalJWTSecret: "environment-or-file-value"}
	db, err := database.Open(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:api_runtime_setting_%s?mode=memory&cache=shared", t.Name()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	const secret = "database-secret-that-is-at-least-thirty-two-characters"
	if err := db.Exec(
		"INSERT INTO pm_runtime_settings (setting_key, setting_value) VALUES (?, ?)",
		database.RuntimeSettingLocalJWTSecret, secret,
	).Error; err != nil {
		t.Fatal(err)
	}

	if err := resolveLocalJWTSecret(context.Background(), &cfg, db); err != nil {
		t.Fatal(err)
	}
	if cfg.LocalJWTSecret != secret {
		t.Fatalf("resolved local JWT secret=%q, want database value", cfg.LocalJWTSecret)
	}
}

func TestResolveLocalJWTSecretRejectsMissingDatabaseRecord(t *testing.T) {
	db, err := database.Open(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:api_runtime_setting_missing_%s?mode=memory&cache=shared", t.Name()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := resolveLocalJWTSecret(context.Background(), &cfg, db); err == nil || !strings.Contains(err.Error(), "runtime setting") {
		t.Fatalf("missing database setting error=%v", err)
	}
}
