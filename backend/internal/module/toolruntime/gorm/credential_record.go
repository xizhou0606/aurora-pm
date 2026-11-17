package toolruntimegorm

import "time"

type credentialBindingRecord struct {
	PrincipalID uint   `gorm:"not null;index"`
	WorkspaceID uint   `gorm:"not null;index"`
	GrantID     uint   `gorm:"not null;index"`
	ClientID    string `gorm:"size:255;not null;index"`
	Channel     string `gorm:"size:16;not null"`
	Audience    string `gorm:"size:32;not null"`
}

// ExternalAuthorizationCode stores only the code digest. PKCEChallenge is the
// one-way S256 challenge supplied by the client, not a verifier or token.
type ExternalAuthorizationCode struct {
	CodeDigest             string                  `gorm:"primaryKey;size:64"`
	AuthorizationRequestID string                  `gorm:"size:128;not null;uniqueIndex:ux_tool_code_request"`
	Binding                credentialBindingRecord `gorm:"embedded"`
	PKCEChallenge          string                  `gorm:"size:64;not null"`
	RedirectURI            string                  `gorm:"size:2048;not null"`
	ExpiresAt              time.Time               `gorm:"not null;index"`
	ConsumedAt             *time.Time              `gorm:"index"`
	CreatedAt              time.Time
}

func (ExternalAuthorizationCode) TableName() string {
	return "pm_external_authorization_codes"
}

type ExternalCredentialFamily struct {
	ID                string                  `gorm:"primaryKey;size:64"`
	Binding           credentialBindingRecord `gorm:"embedded"`
	AbsoluteExpiresAt time.Time               `gorm:"not null;index"`
	RevokedAt         *time.Time              `gorm:"index"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ExternalCredentialFamily) TableName() string {
	return "pm_external_credential_families"
}

type ExternalAccessCredential struct {
	TokenDigest string                  `gorm:"primaryKey;size:64"`
	FamilyID    string                  `gorm:"size:64;not null;index"`
	Binding     credentialBindingRecord `gorm:"embedded"`
	ExpiresAt   time.Time               `gorm:"not null;index"`
	CreatedAt   time.Time
}

func (ExternalAccessCredential) TableName() string {
	return "pm_external_access_credentials"
}

type ExternalRefreshCredential struct {
	TokenDigest string                  `gorm:"primaryKey;size:64"`
	FamilyID    string                  `gorm:"size:64;not null;index"`
	Binding     credentialBindingRecord `gorm:"embedded"`
	ExpiresAt   time.Time               `gorm:"not null;index"`
	ConsumedAt  *time.Time              `gorm:"index"`
	RevokedAt   *time.Time              `gorm:"index"`
	CreatedAt   time.Time
}

func (ExternalRefreshCredential) TableName() string {
	return "pm_external_refresh_credentials"
}

func ExternalCredentialMigrationRecords() []any {
	return []any{
		&ExternalAuthorizationCode{},
		&ExternalCredentialFamily{},
		&ExternalAccessCredential{},
		&ExternalRefreshCredential{},
	}
}
