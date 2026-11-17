package database

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const RuntimeSettingLocalJWTSecret = "auth.local_jwt_secret"

// RuntimeSetting stores PM-owned values that must survive process replacement
// and are not deployment configuration. Secret values are never serialized or
// included in diagnostic errors.
type RuntimeSetting struct {
	Key       string    `gorm:"column:setting_key;primaryKey;size:191"`
	Value     string    `gorm:"column:setting_value;type:text;not null" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (RuntimeSetting) TableName() string { return "pm_runtime_settings" }

// LoadLocalJWTSecret reads the PM session-signing key from the PM database.
// The key is deliberately not resolved from process environment or files.
func LoadLocalJWTSecret(db *gorm.DB) (string, error) {
	var setting RuntimeSetting
	err := db.Where("setting_key = ?", RuntimeSettingLocalJWTSecret).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errors.New("PM runtime setting auth.local_jwt_secret is not configured")
	}
	if err != nil {
		return "", errors.New("unable to load PM runtime setting auth.local_jwt_secret")
	}
	value := strings.TrimSpace(setting.Value)
	if len(value) < 32 {
		return "", errors.New("PM runtime setting auth.local_jwt_secret must contain at least 32 characters")
	}
	return value, nil
}
