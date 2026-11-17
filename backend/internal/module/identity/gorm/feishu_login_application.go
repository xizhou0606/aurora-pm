package identitygorm

import (
	"context"
	"time"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

const FeishuLoginApplicationID uint = 1

// FeishuLoginApplication is the singleton configuration for PM's standalone
// Feishu login application. Other Feishu integrations do not read this table.
type FeishuLoginApplication struct {
	ID        uint   `gorm:"primaryKey;autoIncrement:false"`
	AppID     string `gorm:"size:191;not null"`
	AppSecret string `gorm:"type:text;not null" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (FeishuLoginApplication) TableName() string { return "feishu_login_applications" }

type FeishuLoginApplicationStore struct {
	db *gorm.DB
}

func NewFeishuLoginApplicationStore(db *gorm.DB) *FeishuLoginApplicationStore {
	return &FeishuLoginApplicationStore{db: db}
}

func (s *FeishuLoginApplicationStore) FindFeishuLoginApplication(ctx context.Context) (identity.FeishuLoginApplicationCredentials, error) {
	var value FeishuLoginApplication
	err := s.db.WithContext(ctx).First(&value, FeishuLoginApplicationID).Error
	return identity.FeishuLoginApplicationCredentials{AppID: value.AppID, AppSecret: value.AppSecret}, err
}

var _ identity.FeishuLoginApplicationRepository = (*FeishuLoginApplicationStore)(nil)
