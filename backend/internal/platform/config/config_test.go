package config

import (
	"strings"
	"testing"
	"time"
)

func clearOptionalIntegrationEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PM_OBJECT_STORAGE_ENABLED",
		"PM_OBJECT_STORAGE_ENDPOINT",
		"PM_OBJECT_STORAGE_REGION",
		"PM_OBJECT_STORAGE_BUCKET",
		"PM_OBJECT_STORAGE_ACCESS_KEY",
		"PM_OBJECT_STORAGE_SECRET_KEY",
		"PM_OBJECT_STORAGE_USE_SSL",
		"PM_OBJECT_STORAGE_GLOBAL_PREFIX",
		"PM_OBJECT_STORAGE_TEAM_PREFIX",
		"PM_OBJECT_STORAGE_PUBLIC_BASE_URL",
		"PM_COLLAB_ENABLED",
		"PM_COLLAB_PUBLIC_URL",
		"PM_COLLAB_SERVICE_TOKEN",
		"PM_COLLAB_TOKEN_KEY",
		"PM_COLLAB_TICKET_TTL",
		"PM_COLLAB_GRANT_TTL",
		"PM_COLLAB_VALIDATION_TTL",
		"PM_NOTIFICATION_ENABLED",
		"PM_NOTIFICATION_HUB_RPC_BASE_URL",
		"PM_NOTIFICATION_HUB_INTERNAL_TOKEN",
		"PM_NOTIFICATION_PUBLIC_BASE_URL",
		"PM_NOTIFICATION_POLL_INTERVAL",
		"PM_NOTIFICATION_REQUEST_TIMEOUT",
		"PM_NOTIFICATION_CLAIM_TTL",
		"PM_NOTIFICATION_BATCH_SIZE",
		"PM_PUBLIC_BASE_URL",
		"PM_FEISHU_ENABLED",
		"PM_FEISHU_APP_ID",
		"PM_FEISHU_APP_SECRET",
		"PM_FEISHU_LOGIN_ENABLED",
		"PM_FEISHU_LOGIN_APP_ID",
		"PM_FEISHU_LOGIN_APP_SECRET",
		"PM_FEISHU_LOGIN_PREVIOUS_APP_ID",
		"PM_FEISHU_LOGIN_PREVIOUS_APP_SECRET",
		"PM_FEISHU_LOGIN_BASE_URL",
		"PM_FEISHU_LOGIN_TENANT_KEYS",
		"PM_FEISHU_LOGIN_REQUEST_TIMEOUT",
		"PM_GOOGLE_LOGIN_ENABLED",
		"PM_GOOGLE_LOGIN_CLIENT_ID",
		"PM_GOOGLE_LOGIN_CLIENT_SECRET",
		"PM_GOOGLE_LOGIN_REQUEST_TIMEOUT",
		"PM_TELEGRAM_LOGIN_ENABLED",
		"PM_TELEGRAM_LOGIN_CLIENT_ID",
		"PM_TELEGRAM_LOGIN_CLIENT_SECRET",
		"PM_TELEGRAM_LOGIN_REQUEST_TIMEOUT",
		"PM_SSO_ENABLED",
		"PM_SSO_AUTHORIZE_URL",
		"PM_SSO_TOKEN_URL",
		"PM_SSO_ISSUER",
		"PM_SSO_CLIENT_ID",
		"PM_SSO_CLIENT_SECRET",
		"PM_SSO_REQUEST_TIMEOUT",
		"PM_RBAC_MODE",
		"PM_RBAC_BOOTSTRAP_USER_ID",
		"PM_SHARED_AUTO_PROVISION",
		"PM_SHARED_TRUST_WORKSPACE_HEADER",
		"PM_USER_DIRECTORY_ENABLED",
		"PM_USER_DIRECTORY_RPC_BASE_URL",
		"PM_USER_DIRECTORY_INTERNAL_TOKEN",
		"PM_USER_DIRECTORY_REQUEST_TIMEOUT",
		"PM_REQUIREMENT_COVER_ENABLED",
		"PM_COVER_RENDERER_BASE_URL",
		"PM_COVER_RENDERER_INTERNAL_TOKEN",
		"PM_COVER_RENDERER_REQUEST_TIMEOUT",
		"AI_ENABLED",
		"AI_INTERNAL_SECRET",
		"PM_FIGMA_BASE_URL",
		"PM_FIGMA_TOKEN",
		"PM_FIGMA_REQUEST_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDesignReviewFigmaTimeout(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_FIGMA_TOKEN", "figma-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DesignReview.RequestTimeout != 45*time.Second {
		t.Fatalf("default Figma request timeout=%s", cfg.DesignReview.RequestTimeout)
	}

	t.Setenv("PM_FIGMA_REQUEST_TIMEOUT", "75s")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DesignReview.RequestTimeout != 75*time.Second {
		t.Fatalf("configured Figma request timeout=%s", cfg.DesignReview.RequestTimeout)
	}
}

func TestCollaborationRequiresIsolatedSecretsAndBoundedTTL(t *testing.T) {
	base := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		Collaboration: Collaboration{
			Enabled: true, PublicURL: "ws://localhost:1234", ServiceToken: strings.Repeat("s", 32),
			TokenKey: strings.Repeat("k", 32), TicketTTL: time.Minute, GrantTTL: 30 * time.Second,
			ValidationTTL: 10 * time.Second,
		},
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	missing := base
	missing.Collaboration.ServiceToken = "short"
	if err := missing.Validate(); err == nil || !strings.Contains(err.Error(), "PM_COLLAB_SERVICE_TOKEN") {
		t.Fatalf("short service token error=%v", err)
	}
	wrongKeyLength := base
	wrongKeyLength.Collaboration.TokenKey = strings.Repeat("k", 33)
	if err := wrongKeyLength.Validate(); err == nil || !strings.Contains(err.Error(), "exact 32-byte") {
		t.Fatalf("wrong token key length error=%v", err)
	}
	unsafe := base
	unsafe.Collaboration.PublicURL = "https://example.test/socket?token=secret"
	if err := unsafe.Validate(); err == nil || !strings.Contains(err.Error(), "PM_COLLAB_PUBLIC_URL") {
		t.Fatalf("unsafe public URL error=%v", err)
	}
	unbounded := base
	unbounded.Collaboration.GrantTTL = 2 * time.Minute
	if err := unbounded.Validate(); err == nil || !strings.Contains(err.Error(), "safety bounds") {
		t.Fatalf("unbounded grant TTL error=%v", err)
	}
}

func TestObjectStorageRequiresCompleteCredentialsWhenEnabled(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		ObjectStorage:  ObjectStorage{Enabled: true, Endpoint: "minio:9000", Bucket: "pm"},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PM_OBJECT_STORAGE_ACCESS_KEY") ||
		!strings.Contains(err.Error(), "PM_OBJECT_STORAGE_SECRET_KEY") {
		t.Fatalf("Validate() error=%v", err)
	}
}

func TestNotificationRequiresPublicBase(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		Notification: Notification{Enabled: true, PollInterval: time.Second,
			RequestTimeout: time.Second, ClaimTTL: time.Minute, BatchSize: 10},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PM_NOTIFICATION_PUBLIC_BASE_URL") {
		t.Fatalf("Validate() error=%v", err)
	}
}

func TestNotificationClaimTTLExceedsFullDispatchBudget(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		Notification: Notification{
			Enabled: true, PublicBaseURL: "https://pm-main.example.invalid",
			PollInterval: time.Second, RequestTimeout: 10 * time.Second,
			ClaimTTL: 40 * time.Second, BatchSize: 10,
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "four request timeouts") {
		t.Fatalf("Validate() error=%v", err)
	}
}

func TestLoadNotificationEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_NOTIFICATION_ENABLED", "true")
	t.Setenv("PM_NOTIFICATION_PUBLIC_BASE_URL", "https://pm-main.example.invalid/")
	t.Setenv("PM_NOTIFICATION_POLL_INTERVAL", "3s")
	t.Setenv("PM_NOTIFICATION_REQUEST_TIMEOUT", "7s")
	t.Setenv("PM_NOTIFICATION_CLAIM_TTL", "90s")
	t.Setenv("PM_NOTIFICATION_BATCH_SIZE", "12")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Notification.Enabled || cfg.Notification.PublicBaseURL != "https://pm-main.example.invalid" ||
		cfg.Notification.PollInterval != 3*time.Second || cfg.Notification.RequestTimeout != 7*time.Second ||
		cfg.Notification.ClaimTTL != 90*time.Second || cfg.Notification.BatchSize != 12 {
		t.Fatalf("loaded notification=%+v", cfg.Notification)
	}
}

func TestLoadNotificationPublicBaseFallsBackToPMPublicURL(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_NOTIFICATION_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notification.PublicBaseURL != "http://pm-main.example.invalid" {
		t.Fatalf("loaded notification public base=%q", cfg.Notification.PublicBaseURL)
	}
}

func TestLoadNotificationPublicBaseRewritesOne2AllHost(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_NOTIFICATION_ENABLED", "true")
	t.Setenv("PM_NOTIFICATION_PUBLIC_BASE_URL", "http://one2all-main.example.invalid/one2all")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notification.PublicBaseURL != "http://pm-main.example.invalid" {
		t.Fatalf("loaded notification public base=%q", cfg.Notification.PublicBaseURL)
	}
}

func TestCanonicalNotificationPublicBaseURL(t *testing.T) {
	fallback := "http://pm-main.example.invalid"
	cases := []struct {
		in, want string
	}{
		{in: "", want: fallback},
		{in: "http://pm-main.example.invalid/", want: "http://pm-main.example.invalid"},
		{in: "http://one2all-main.example.invalid/one2all", want: fallback},
		{in: "https://one2all.example.test", want: fallback},
		{in: "http://192.168.113.115:32634", want: "http://192.168.113.115:32634"},
	}
	for _, tc := range cases {
		got := canonicalNotificationPublicBaseURL(tc.in, fallback)
		if got != tc.want {
			t.Fatalf("canonicalNotificationPublicBaseURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadDoesNotReadLocalJWTSecretFromEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_LOCAL_JWT_SECRET", "environment-secret-that-is-at-least-thirty-two-characters")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalJWTSecret != "" {
		t.Fatalf("Load read PM_LOCAL_JWT_SECRET from environment")
	}
}

func TestLoadPreviousFeishuLoginApplication(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_FEISHU_LOGIN_PREVIOUS_APP_ID", "cli_previous")
	t.Setenv("PM_FEISHU_LOGIN_PREVIOUS_APP_SECRET", "previous-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FeishuLogin.PreviousAppID != "cli_previous" || cfg.FeishuLogin.PreviousAppSecret != "previous-secret" {
		t.Fatalf("previous login application=%+v", cfg.FeishuLogin)
	}
}

func TestLoadRejectsPartialPreviousFeishuLoginApplication(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_FEISHU_LOGIN_PREVIOUS_APP_ID", "cli_previous")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PM_FEISHU_LOGIN_PREVIOUS_APP_ID") {
		t.Fatalf("Load() error=%v", err)
	}
}

func TestLoadObjectStorageEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_OBJECT_STORAGE_ENABLED", "true")
	t.Setenv("PM_OBJECT_STORAGE_ENDPOINT", "https://minio.example.test")
	t.Setenv("PM_OBJECT_STORAGE_BUCKET", "pm-materials")
	t.Setenv("PM_OBJECT_STORAGE_ACCESS_KEY", "access")
	t.Setenv("PM_OBJECT_STORAGE_SECRET_KEY", "secret")
	t.Setenv("PM_OBJECT_STORAGE_USE_SSL", "true")
	t.Setenv("PM_OBJECT_STORAGE_GLOBAL_PREFIX", "/pm/native/")
	t.Setenv("PM_OBJECT_STORAGE_TEAM_PREFIX", "false")
	t.Setenv("PM_OBJECT_STORAGE_PUBLIC_BASE_URL", "https://cdn.example.test/materials/")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ObjectStorage.Enabled || cfg.ObjectStorage.Endpoint != "https://minio.example.test" ||
		cfg.ObjectStorage.Bucket != "pm-materials" || !cfg.ObjectStorage.UseSSL ||
		cfg.ObjectStorage.GlobalPrefix != "pm/native" || cfg.ObjectStorage.TeamPrefix ||
		cfg.ObjectStorage.PublicBaseURL != "https://cdn.example.test/materials" {
		t.Fatalf("loaded object storage=%+v", cfg.ObjectStorage)
	}
}

func TestLoadDualLoginEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "shared_jwt")
	t.Setenv("PM_SHARED_AUTH_INTROSPECTION_URL", "http://auth.example.test/auth/me")
	t.Setenv("PM_FEISHU_LOGIN_BASE_URL", "https://open.feishu.cn/")
	t.Setenv("PM_FEISHU_LOGIN_TENANT_KEYS", "tenant-one, tenant-two")
	t.Setenv("PM_SSO_ENABLED", "true")
	t.Setenv("PM_SSO_AUTHORIZE_URL", "https://sso.example.test/authorize")
	t.Setenv("PM_SSO_TOKEN_URL", "https://sso.example.test/token")
	t.Setenv("PM_SSO_ISSUER", "one2all")
	t.Setenv("PM_SSO_CLIENT_ID", "pm")
	t.Setenv("PM_SSO_CLIENT_SECRET", "sso-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "http://pm-main.example.invalid" ||
		cfg.FeishuLogin.BaseURL != "https://open.feishu.cn" ||
		cfg.FeishuLogin.RedirectURL != "http://pm-main.example.invalid/api/v1/core/auth/feishu/callback" ||
		len(cfg.FeishuLogin.TenantKeys) != 2 || cfg.FeishuLogin.TenantKeys[0] != "tenant-one" || cfg.FeishuLogin.TenantKeys[1] != "tenant-two" ||
		!cfg.SSO.Enabled || cfg.SSO.Issuer != "one2all" || cfg.SSO.ClientID != "pm" ||
		cfg.SSO.RedirectURL != "http://pm-main.example.invalid/api/v1/core/auth/sso/callback" {
		t.Fatalf("dual login config=%+v %+v", cfg.FeishuLogin, cfg.SSO)
	}
}

func TestLoadGoogleLoginEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_PUBLIC_BASE_URL", "http://127.0.0.1:8081/")
	t.Setenv("PM_GOOGLE_LOGIN_ENABLED", "true")
	t.Setenv("PM_GOOGLE_LOGIN_CLIENT_ID", "google-client")
	t.Setenv("PM_GOOGLE_LOGIN_CLIENT_SECRET", "google-secret")
	t.Setenv("PM_GOOGLE_LOGIN_REQUEST_TIMEOUT", "7s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.GoogleLogin.Enabled || cfg.GoogleLogin.ClientID != "google-client" ||
		cfg.GoogleLogin.ClientSecret != "google-secret" || cfg.GoogleLogin.RequestTimeout != 7*time.Second ||
		cfg.GoogleLogin.RedirectURL != "http://127.0.0.1:8081/api/v1/core/auth/google/callback" {
		t.Fatalf("Google login config=%+v", cfg.GoogleLogin)
	}
}

func TestGoogleLoginRequiresCredentialsAndSecureProductionCallback(t *testing.T) {
	base := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		PublicBaseURL: "http://pm.test",
		GoogleLogin:   GoogleLogin{Enabled: true, RedirectURL: "http://pm.test/api/v1/core/auth/google/callback", RequestTimeout: time.Second},
	}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "PM_GOOGLE_LOGIN_CLIENT_ID") ||
		!strings.Contains(err.Error(), "PM_GOOGLE_LOGIN_CLIENT_SECRET") {
		t.Fatalf("missing Google credentials error=%v", err)
	}

	base.GoogleLogin.ClientID = "google-client"
	base.GoogleLogin.ClientSecret = "google-secret"
	base.GoogleLogin.RedirectURL = "http://other.test/api/v1/core/auth/google/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "public web entry") {
		t.Fatalf("non-canonical Google callback error=%v", err)
	}

	base.Environment = "production"
	base.PublicBaseURL = "http://pm.test"
	base.GoogleLogin.RedirectURL = "http://pm.test/api/v1/core/auth/google/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("insecure production Google callback error=%v", err)
	}

	base.PublicBaseURL = "http://localhost:8081"
	base.GoogleLogin.RedirectURL = "http://localhost:8081/api/v1/core/auth/google/callback"
	if err := base.Validate(); err != nil {
		t.Fatalf("production localhost Google callback should be allowed: %v", err)
	}

	base.PublicBaseURL = "http://localhost.example.com:8081"
	base.GoogleLogin.RedirectURL = "http://localhost.example.com:8081/api/v1/core/auth/google/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("non-loopback production Google callback error=%v", err)
	}

	base.PublicBaseURL = "http://127.0.0.1:8081"
	base.GoogleLogin.RedirectURL = "http://127.0.0.1:8081/api/v1/core/auth/google/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("non-registered loopback alias Google callback error=%v", err)
	}
}

func TestLoadTelegramLoginEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_PUBLIC_BASE_URL", "http://127.0.0.1:8081/")
	t.Setenv("PM_TELEGRAM_LOGIN_ENABLED", "true")
	t.Setenv("PM_TELEGRAM_LOGIN_CLIENT_ID", "123456789")
	t.Setenv("PM_TELEGRAM_LOGIN_CLIENT_SECRET", "telegram-secret")
	t.Setenv("PM_TELEGRAM_LOGIN_REQUEST_TIMEOUT", "8s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TelegramLogin.Enabled || cfg.TelegramLogin.ClientID != "123456789" ||
		cfg.TelegramLogin.ClientSecret != "telegram-secret" || cfg.TelegramLogin.RequestTimeout != 8*time.Second ||
		cfg.TelegramLogin.RedirectURL != "http://127.0.0.1:8081/api/v1/core/auth/telegram/callback" {
		t.Fatalf("Telegram login config=%+v", cfg.TelegramLogin)
	}
}

func TestTelegramLoginRequiresCredentialsAndCanonicalHTTPSCallback(t *testing.T) {
	base := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		PublicBaseURL: "http://pm.test",
		TelegramLogin: TelegramLogin{Enabled: true, RedirectURL: "http://pm.test/api/v1/core/auth/telegram/callback", RequestTimeout: time.Second},
	}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "PM_TELEGRAM_LOGIN_CLIENT_ID") ||
		!strings.Contains(err.Error(), "PM_TELEGRAM_LOGIN_CLIENT_SECRET") {
		t.Fatalf("missing Telegram credentials error=%v", err)
	}

	base.TelegramLogin.ClientID = "123456789"
	base.TelegramLogin.ClientSecret = "telegram-secret"
	base.TelegramLogin.RedirectURL = "http://other.test/api/v1/core/auth/telegram/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "public web entry") {
		t.Fatalf("non-canonical Telegram callback error=%v", err)
	}

	base.Environment = "production"
	base.PublicBaseURL = "http://pm.test"
	base.TelegramLogin.RedirectURL = "http://pm.test/api/v1/core/auth/telegram/callback"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("insecure production Telegram callback error=%v", err)
	}
}

func TestLoadUsesLocalPublicBaseURLOutsideProduction(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "development")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_PUBLIC_BASE_URL", "http://127.0.0.1:8081/")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "http://127.0.0.1:8081" ||
		cfg.FeishuLogin.RedirectURL != "http://127.0.0.1:8081/api/v1/core/auth/feishu/callback" ||
		cfg.SSO.RedirectURL != "http://127.0.0.1:8081/api/v1/core/auth/sso/callback" {
		t.Fatalf("local public base URL was not applied: %+v %+v", cfg.FeishuLogin, cfg.SSO)
	}
}

func TestLoadUsesConfiguredProductionPublicBaseURL(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "production")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_PUBLIC_BASE_URL", "https://pm-prod.example.invalid")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "https://pm-prod.example.invalid" ||
		cfg.FeishuLogin.RedirectURL != "https://pm-prod.example.invalid/api/v1/core/auth/feishu/callback" {
		t.Fatalf("production public base URL was not applied: %+v", cfg.FeishuLogin)
	}
}

func TestLoadFallsBackToDefaultProductionPublicBaseURL(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "production")
	t.Setenv("PM_AUTH_MODE", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicBaseURL != "http://pm-main.example.invalid" ||
		cfg.FeishuLogin.RedirectURL != "http://pm-main.example.invalid/api/v1/core/auth/feishu/callback" {
		t.Fatalf("production public base URL fallback changed: %+v", cfg.FeishuLogin)
	}
}

func TestLoadUsesSevenDayDefaultSessionTTL(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_TOKEN_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TokenTTL != 7*24*time.Hour {
		t.Fatalf("default token TTL=%s, want 7d", cfg.TokenTTL)
	}
}

func TestLoadIgnoresRetiredFeishuLoginEnvironmentSettings(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_FEISHU_LOGIN_ENABLED", "true")
	t.Setenv("PM_FEISHU_LOGIN_APP_ID", "legacy-app")
	t.Setenv("PM_FEISHU_LOGIN_APP_SECRET", "legacy-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FeishuLogin.AppID != "" || cfg.FeishuLogin.AppSecret != "" {
		t.Fatalf("legacy Feishu login credentials were loaded: %+v", cfg.FeishuLogin)
	}
}

func TestFeishuLoginCredentialsMayResolveAfterDatabaseOpen(t *testing.T) {
	cfg := Config{
		Environment:    "test",
		DatabaseDriver: "sqlite",
		AuthMode:       "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		PublicBaseURL:  "http://pm.test",
		FeishuLogin: FeishuLogin{
			BaseURL:     "https://open.feishu.cn",
			RedirectURL: "http://pm.test/api/v1/core/auth/feishu/callback", RequestTimeout: time.Second,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("database-resolvable configuration error=%v", err)
	}
	if err := cfg.ValidateAPIRuntime(); err == nil || !strings.Contains(err.Error(), "login application") {
		t.Fatalf("unresolved API login application error=%v", err)
	}
	cfg.FeishuLogin.AppID = "database-app"
	cfg.FeishuLogin.AppSecret = "database-secret"
	if err := cfg.ValidateAPIRuntime(); err != nil {
		t.Fatalf("resolved API login application error=%v", err)
	}
}

func TestLocalPreviewRequiresExplicitLocalDevelopment(t *testing.T) {
	cfg := Config{Environment: "development", AuthMode: "local", DatabaseDriver: "postgres", LocalPreviewRequested: true}
	if !cfg.LocalPreview() {
		t.Fatal("local preview unavailable")
	}
	for _, alter := range []func(*Config){
		func(c *Config) { c.Environment = "production" },
		func(c *Config) { c.AuthMode = "hybrid" },
		func(c *Config) { c.DatabaseDriver = "sqlite" },
		func(c *Config) { c.LocalPreviewRequested = false },
	} {
		other := cfg
		alter(&other)
		if other.LocalPreview() {
			t.Fatal("local preview enabled outside its intended scope")
		}
	}
}

func TestValidateAPIRuntimeRequiresDatabaseSessionSecret(t *testing.T) {
	cfg := Config{
		Environment:    "production",
		DatabaseDriver: "sqlite",
		AuthMode:       "local",
		PublicBaseURL:  "http://pm.internal.test",
		FeishuLogin: FeishuLogin{
			AppID: "app", AppSecret: "secret", BaseURL: "https://open.feishu.cn",
			RedirectURL: "http://pm.internal.test/api/v1/core/auth/feishu/callback", RequestTimeout: time.Second,
		},
	}
	if err := cfg.ValidateAPIRuntime(); err == nil || !strings.Contains(err.Error(), "PM database") {
		t.Fatalf("missing database session secret error=%v", err)
	}
}

func TestFeishuLoginAllowsInternalHTTPPublicWebEntry(t *testing.T) {
	cfg := Config{
		Environment:    "production",
		DatabaseDriver: "sqlite",
		AuthMode:       "local",
		LocalJWTSecret: "production-secret-that-is-at-least-thirty-two-characters",
		PublicBaseURL:  "http://pm.internal.test",
		FeishuLogin: FeishuLogin{
			AppID: "app", AppSecret: "secret", BaseURL: "https://open.feishu.cn",
			RedirectURL: "http://pm.internal.test/api/v1/core/auth/feishu/callback", RequestTimeout: time.Second,
		},
	}
	if err := cfg.ValidateAPIRuntime(); err != nil {
		t.Fatalf("valid internal HTTP callback error=%v", err)
	}
	cfg.FeishuLogin.RedirectURL = "http://pm-api.internal.test/api/v1/core/auth/feishu/callback"
	if err := cfg.ValidateAPIRuntime(); err == nil || !strings.Contains(err.Error(), "public web entry") {
		t.Fatalf("direct API callback error=%v", err)
	}
}

func TestSSORequiresIssuer(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters",
		PublicBaseURL:  "http://pm.test",
		SSO: SSO{
			Enabled: true, AuthorizeURL: "http://sso.test/authorize", TokenURL: "http://sso.test/token",
			ClientID: "pm", ClientSecret: "secret", RedirectURL: "http://pm.test/api/v1/core/auth/sso/callback",
			RequestTimeout: time.Second,
		},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PM_SSO_ISSUER") {
		t.Fatalf("missing issuer error=%v", err)
	}
}

func TestLoadIgnoresRetiredSharedIdentityConfiguration(t *testing.T) {
	for _, key := range []string{
		"PM_SHARED_AUTO_PROVISION",
		"PM_SHARED_TRUST_WORKSPACE_HEADER",
		"PM_USER_DIRECTORY_ENABLED",
		"PM_USER_DIRECTORY_RPC_BASE_URL",
		"PM_USER_DIRECTORY_INTERNAL_TOKEN",
		"PM_USER_DIRECTORY_REQUEST_TIMEOUT",
	} {
		t.Run(key, func(t *testing.T) {
			clearOptionalIntegrationEnv(t)
			t.Setenv("PM_ENV", "test")
			t.Setenv("PM_AUTH_MODE", "local")
			t.Setenv(key, "retired")

			if _, err := Load(); err != nil {
				t.Fatalf("legacy setting %s blocked startup: %v", key, err)
			}
		})
	}
}

func TestRBACEnforcementRequiresBootstrapUserInProduction(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "production-secret-that-is-at-least-thirty-two-characters",
		RBACMode:       RBACModeEnforce,
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PM_RBAC_BOOTSTRAP_USER_ID") {
		t.Fatalf("missing RBAC bootstrap user error=%v", err)
	}
	cfg.RBACBootstrapUserID = 12
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid RBAC enforcement config error=%v", err)
	}
}

func TestLoadRBACEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_RBAC_MODE", "enforce")
	t.Setenv("PM_RBAC_BOOTSTRAP_USER_ID", "42")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RBACMode != RBACModeEnforce || cfg.RBACBootstrapUserID != 42 {
		t.Fatalf("RBAC config=%+v", cfg)
	}
}

func TestCoverWorkerRuntimeRequiresRemoteCoverRenderer(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseDriver: "sqlite", AuthMode: "shared_jwt",
		LocalJWTSecret:  "api-login-secret-that-is-at-least-thirty-two-characters",
		SharedJWTIssuer: "one2all", SharedIntrospectionURL: "http://one2all-user-service:8080/api/v1/core/auth/me",
		PublicBaseURL: "http://pm.internal.test",
		Feishu: Feishu{
			Enabled: true, AppID: "app-id", AppSecret: "app-secret",
			DomainAllowlist: []string{"feishu.cn"}, MaxBlocks: 10, MaxContentChars: 100,
		},
		FeishuLogin: FeishuLogin{
			AppID: "login-app", AppSecret: "login-secret", BaseURL: "https://open.feishu.cn",
			RedirectURL: "http://pm.internal.test/api/v1/core/auth/feishu/callback", RequestTimeout: time.Second,
		},
		RequirementCoverEnabled: true,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("shared worker configuration should not require renderer credentials: %v", err)
	}
	if err := cfg.ValidateAPIRuntime(); err != nil {
		t.Fatalf("API must not need renderer credentials: %v", err)
	}
	if err := cfg.ValidateCoverWorkerRuntime(); err == nil || !strings.Contains(err.Error(), "PM_COVER_RENDERER_BASE_URL") {
		t.Fatalf("missing renderer validation error=%v", err)
	}
	cfg.CoverRenderer = CoverRenderer{
		BaseURL:        "http://pm-cover-renderer:8092",
		InternalToken:  "renderer-token-that-is-at-least-32-characters",
		RequestTimeout: time.Second,
	}
	cfg.ObjectStorage = ObjectStorage{Enabled: true, Endpoint: "minio:9000", Bucket: "pm", AccessKey: "key", SecretKey: "secret"}
	if err := cfg.ValidateCoverWorkerRuntime(); err != nil {
		t.Fatalf("valid renderer config error=%v", err)
	}
}

func TestLoadCoverRendererEnvironment(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_COVER_RENDERER_BASE_URL", "http://pm-cover-renderer:8092/")
	t.Setenv("PM_COVER_RENDERER_INTERNAL_TOKEN", "renderer-token-that-is-at-least-32-characters")
	t.Setenv("PM_COVER_RENDERER_REQUEST_TIMEOUT", "7s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CoverRenderer.BaseURL != "http://pm-cover-renderer:8092" ||
		cfg.CoverRenderer.RequestTimeout != 7*time.Second ||
		cfg.CoverRenderer.InternalToken == "" {
		t.Fatalf("loaded cover renderer=%+v", cfg.CoverRenderer)
	}
}

func TestSharedAuthUsesPMLocalUserRegistry(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "hybrid",
		LocalJWTSecret:         "test-secret-that-is-at-least-thirty-two-characters",
		SharedIntrospectionURL: "http://one2all-user-service:8080/api/v1/core/auth/me",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("shared auth should rely on the PM user registry: %v", err)
	}
}

func TestLoadSchemaMigrationMode(t *testing.T) {
	clearOptionalIntegrationEnv(t)
	t.Setenv("PM_ENV", "test")
	t.Setenv("PM_AUTH_MODE", "local")
	t.Setenv("PM_SCHEMA_MIGRATION_MODE", " VALIDATE ")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaMigrationMode != SchemaMigrationValidate {
		t.Fatalf("schema migration mode=%q, want %q", cfg.SchemaMigrationMode, SchemaMigrationValidate)
	}
}

func TestSchemaMigrationModeRejectsUnknownValue(t *testing.T) {
	cfg := Config{
		Environment: "test", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret:      "test-secret-that-is-at-least-thirty-two-characters",
		SchemaMigrationMode: "automatic",
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PM_SCHEMA_MIGRATION_MODE") {
		t.Fatalf("invalid schema migration mode error=%v", err)
	}
}

func TestEmbeddedAIWorkerIsDevelopmentOnly(t *testing.T) {
	cfg := Config{
		Environment: "production", DatabaseDriver: "sqlite", AuthMode: "local",
		LocalJWTSecret: "production-secret-that-is-at-least-thirty-two-characters",
		AI:             AI{Enabled: true, EmbeddedWorker: true, InternalSecret: "ai-secret-that-is-at-least-thirty-two-characters", RuntimeBaseURL: "http://ai-runtime:8091", Model: "test", RequestTimeout: time.Second},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "development-only") {
		t.Fatalf("embedded production worker error=%v", err)
	}
}
