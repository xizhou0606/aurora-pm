package toolruntimegorm

import "time"

type ExternalOAuthClient struct {
	ID           string `gorm:"primaryKey;size:255"`
	Name         string `gorm:"size:255;not null"`
	Channel      string `gorm:"size:16;not null;index"`
	RedirectURIs string `gorm:"type:text;not null"`
	CreatedAt    time.Time
}

func (ExternalOAuthClient) TableName() string {
	return "pm_external_oauth_clients"
}

func OAuthMigrationRecords() []any {
	return []any{&ExternalOAuthClient{}, &ExternalAuthorizationRequest{}}
}
