package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

func TestResolveFeishuLoginApplicationUsesDatabaseWithoutChangingOtherFeishuConfig(t *testing.T) {
	cfg, db := openFeishuLoginApplicationTestDB(t)
	cfg.Feishu = config.Feishu{Enabled: true, AppID: "reader-app", AppSecret: "reader-secret"}
	if err := db.Create(&identitygorm.FeishuLoginApplication{
		ID: identitygorm.FeishuLoginApplicationID, AppID: "database-login-app", AppSecret: "database-login-secret",
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := resolveFeishuLoginApplication(context.Background(), &cfg, db); err != nil {
		t.Fatal(err)
	}
	if cfg.FeishuLogin.AppID != "database-login-app" || cfg.FeishuLogin.AppSecret != "database-login-secret" {
		t.Fatalf("database login application was not authoritative: %+v", cfg.FeishuLogin)
	}
	if cfg.Feishu.AppID != "reader-app" || cfg.Feishu.AppSecret != "reader-secret" {
		t.Fatalf("non-login Feishu configuration changed: %+v", cfg.Feishu)
	}
}

func TestResolveFeishuLoginApplicationRejectsEnvironmentCredentialsWithoutDatabaseRecord(t *testing.T) {
	cfg, db := openFeishuLoginApplicationTestDB(t)
	cfg.FeishuLogin.AppID = "environment-login-app"
	cfg.FeishuLogin.AppSecret = "environment-login-secret"

	err := resolveFeishuLoginApplication(context.Background(), &cfg, db)
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("environment credentials must not be a fallback: %v", err)
	}
}

func TestResolveFeishuLoginApplicationRejectsMissingDedicatedConfig(t *testing.T) {
	cfg, db := openFeishuLoginApplicationTestDB(t)
	err := resolveFeishuLoginApplication(context.Background(), &cfg, db)
	if err == nil || !strings.Contains(err.Error(), "dedicated Feishu login application") {
		t.Fatalf("missing login application error=%v", err)
	}
}

func openFeishuLoginApplicationTestDB(t *testing.T) (config.Config, *gorm.DB) {
	t.Helper()
	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:api_feishu_login_application_%s?mode=memory&cache=shared", t.Name()),
		FeishuLogin:    config.FeishuLogin{},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return cfg, db
}
