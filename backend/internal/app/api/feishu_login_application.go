package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

func resolveLocalJWTSecret(ctx context.Context, cfg *config.Config, db *gorm.DB) error {
	if cfg.LocalPreview() {
		// Generate once inside Postgres so signing material never appears in logs.
		if err := db.WithContext(ctx).Exec(`INSERT INTO pm_runtime_settings (setting_key, setting_value, created_at, updated_at)
    VALUES (?, replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', ''), now(), now())
    ON CONFLICT (setting_key) DO NOTHING`, database.RuntimeSettingLocalJWTSecret).Error; err != nil {
			return err
		}
	}
	secret, err := database.LoadLocalJWTSecret(db.WithContext(ctx))
	if err != nil {
		return err
	}
	cfg.LocalJWTSecret = secret
	return nil
}

// resolveFeishuLoginApplication reads the dedicated PM browser-login
// application. Other Feishu integrations use their own configuration.
func resolveFeishuLoginApplication(ctx context.Context, cfg *config.Config, db *gorm.DB) error {
	if cfg.LocalPreview() {
		return nil
	}
	credentials, err := identitygorm.NewFeishuLoginApplicationStore(db).FindFeishuLoginApplication(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("dedicated Feishu login application is not configured in the database")
		}
		return fmt.Errorf("load dedicated Feishu login application: %w", err)
	}
	cfg.FeishuLogin.AppID = strings.TrimSpace(credentials.AppID)
	cfg.FeishuLogin.AppSecret = strings.TrimSpace(credentials.AppSecret)
	return nil
}
