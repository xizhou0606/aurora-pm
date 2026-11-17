package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	feishuLoginCallbackPath   = "/api/v1/core/auth/feishu/callback"
	googleLoginCallbackPath   = "/api/v1/core/auth/google/callback"
	ssoLoginCallbackPath      = "/api/v1/core/auth/sso/callback"
	telegramLoginCallbackPath = "/api/v1/core/auth/telegram/callback"
	pmPublicBaseURL           = "http://pm-main.example.invalid"
)

type SchemaMigrationMode string

const (
	SchemaMigrationApply    SchemaMigrationMode = "apply"
	SchemaMigrationValidate SchemaMigrationMode = "validate"
	SchemaMigrationOff      SchemaMigrationMode = "off"
)

type RBACMode string

const (
	RBACModePrepare RBACMode = "prepare"
	RBACModeEnforce RBACMode = "enforce"
)

type Config struct {
	Environment                 string
	HTTPAddr                    string
	PublicBaseURL               string
	DatabaseDriver              string
	DatabaseDSN                 string
	SchemaMigrationMode         SchemaMigrationMode
	RBACMode                    RBACMode
	RBACBootstrapUserID         uint
	LocalPreviewRequested       bool
	AuthMode                    string
	LocalJWTSecret              string
	SharedJWTSecret             string
	SharedJWTIssuer             string
	SharedJWTAudience           string
	SharedIntrospectionURL      string
	SharedIntrospectionTTL      time.Duration
	SharedIntrospectionTimeout  time.Duration
	SharedDevSecretFallback     bool
	TokenTTL                    time.Duration
	CORSOrigins                 []string
	Bootstrap                   Bootstrap
	ObjectStorage               ObjectStorage
	Collaboration               Collaboration
	Notification                Notification
	AI                          AI
	DesignReview                DesignReview
	Feishu                      Feishu
	FeishuLogin                 FeishuLogin
	GoogleLogin                 GoogleLogin
	TelegramLogin               TelegramLogin
	ExternalRegistrationEnabled bool
	SSO                         SSO
	RequirementCoverEnabled     bool
	CoverRenderer               CoverRenderer
}

type Bootstrap struct {
	Username      string
	Password      string
	DisplayName   string
	Email         string
	WorkspaceName string
	WorkspaceSlug string
	TeamName      string
}

// ObjectStorage configures the S3-compatible bucket used for PM-owned upload
// artifacts, including Tencent COS in production. Credentials stay server-side
// and are never exposed through runtime configuration.
type ObjectStorage struct {
	Enabled       bool
	Endpoint      string
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	GlobalPrefix  string
	TeamPrefix    bool
	PublicBaseURL string
}

// Collaboration configures the isolated Hocuspocus workload. The service
// token authenticates Node to Go; TokenKey encrypts short-lived browser,
// grant, and validation proofs. Neither value is exposed to the browser.
// TokenKey never encrypts document content (documents are persisted by the
// backend as usual); rotating it only invalidates in-flight proofs.
type Collaboration struct {
	InternalURL   string
	Enabled       bool
	PublicURL     string
	ServiceToken  string
	TokenKey      string
	TicketTTL     time.Duration
	GrantTTL      time.Duration
	ValidationTTL time.Duration
}

// Notification configures the transactional outbox publisher. PM owns the
// message text and delivers Feishu IM through the dedicated login application.
// PublicBaseURL is the standalone PM origin used in message links
// (pm-main), not the One2All host. HubRPCBaseURL and InternalToken are
// ignored leftovers from the retired One2All notification hub.
type Notification struct {
	Enabled        bool
	HubRPCBaseURL  string
	InternalToken  string
	PublicBaseURL  string
	PollInterval   time.Duration
	RequestTimeout time.Duration
	ClaimTTL       time.Duration
	BatchSize      int
}

type AI struct {
	Enabled        bool
	EmbeddedWorker bool
	RuntimeBaseURL string
	InternalSecret string
	Model          string
	RequestTimeout time.Duration
	// WorkerConcurrency bounds how many queued AI runs one worker process
	// generates in parallel. Claims are CAS-guarded, so more replicas or a
	// higher number are safe.
	// 默认 3（不低于 2）：report.* run 内部先做代码质量审查
	// （code.commit_quality_review）再做报告叙述，两段 runtime 调用在同一
	// run 内串行，run 内无法并行提速；不同 run 之间才靠该并发数并行执行
	// （每个消费循环一次认领一个 run），因此并发数是吞吐的主旋钮。
	WorkerConcurrency           int
	DesignReviewModel           string
	DesignReviewReasoningEffort string
	DesignReviewImageDetail     string
}

type DesignReview struct {
	FigmaBaseURL        string
	FigmaToken          string
	ExtensionVersion    string
	CollectorVersion    string
	MobilewrightVersion string
	RequestTimeout      time.Duration
}

type Feishu struct {
	Enabled         bool
	AppID           string
	AppSecret       string
	DomainAllowlist []string
	MaxBlocks       int
	MaxContentChars int
}

// FeishuLogin is separate from the PM document-reader credentials. It owns
// only the standalone PM browser login OAuth client.
type FeishuLogin struct {
	AppID             string
	AppSecret         string
	PreviousAppID     string
	PreviousAppSecret string
	BaseURL           string
	RedirectURL       string
	RequestTimeout    time.Duration
	TenantKeys        []string
}

// GoogleLogin configures PM as an OpenID Connect relying party. The callback
// is always derived from PublicBaseURL so credentials cannot redirect browser
// sessions to an arbitrary origin.
type GoogleLogin struct {
	Enabled        bool
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	RequestTimeout time.Duration
}

// TelegramLogin configures PM for Telegram's OIDC authorization-code flow.
// The integration uses PKCE S256 in addition to the confidential client
// secret, and derives its callback from PublicBaseURL.
type TelegramLogin struct {
	Enabled        bool
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	RequestTimeout time.Duration
}

// SSO is PM's relying-party configuration for an already authenticated SaaS
// Control browser. The external service remains the authorization-code issuer;
// PM only starts the handoff and exchanges the code server-to-server.
type SSO struct {
	Enabled        bool
	AuthorizeURL   string
	TokenURL       string
	Issuer         string
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	RequestTimeout time.Duration
}

type CoverRenderer struct {
	BaseURL        string
	InternalToken  string
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	notificationInternalToken := strings.TrimSpace(os.Getenv("PM_NOTIFICATION_HUB_INTERNAL_TOKEN"))
	environment := env("PM_ENV", "development")
	publicBaseURL := pmPublicBaseURL
	if configured := strings.TrimRight(strings.TrimSpace(os.Getenv("PM_PUBLIC_BASE_URL")), "/"); configured != "" {
		publicBaseURL = configured
	}
	notificationPublicBaseURL := canonicalNotificationPublicBaseURL(
		os.Getenv("PM_NOTIFICATION_PUBLIC_BASE_URL"), publicBaseURL,
	)
	cfg := Config{
		Environment:           environment,
		LocalPreviewRequested: envBool("PM_LOCAL_PREVIEW", false),
		HTTPAddr:              env("PM_HTTP_ADDR", ":8081"),
		PublicBaseURL:         publicBaseURL,
		DatabaseDriver:        strings.ToLower(env("PM_DATABASE_DRIVER", "sqlite")),
		DatabaseDSN: developmentDatabaseDSN(
			environment,
			strings.ToLower(env("PM_DATABASE_DRIVER", "sqlite")),
			env("PM_DATABASE_DSN", "pm.db"),
		),
		SchemaMigrationMode: SchemaMigrationMode(strings.ToLower(strings.TrimSpace(env("PM_SCHEMA_MIGRATION_MODE", string(SchemaMigrationApply))))),
		RBACMode:            RBACMode(strings.ToLower(strings.TrimSpace(env("PM_RBAC_MODE", string(RBACModePrepare))))),
		RBACBootstrapUserID: uint(envUint64("PM_RBAC_BOOTSTRAP_USER_ID", 0)),
		AuthMode:            strings.ToLower(env("PM_AUTH_MODE", "local")),
		// PM local session signing is resolved from the PM database after the
		// database connection and schema are ready. It is never read from env.
		LocalJWTSecret:             "",
		SharedJWTSecret:            os.Getenv("PM_SHARED_JWT_SECRET"),
		SharedJWTIssuer:            os.Getenv("PM_SHARED_JWT_ISSUER"),
		SharedJWTAudience:          os.Getenv("PM_SHARED_JWT_AUDIENCE"),
		SharedIntrospectionURL:     strings.TrimSpace(os.Getenv("PM_SHARED_AUTH_INTROSPECTION_URL")),
		SharedIntrospectionTTL:     envDuration("PM_SHARED_AUTH_CACHE_TTL", 30*time.Second),
		SharedIntrospectionTimeout: envDuration("PM_SHARED_AUTH_TIMEOUT", 3*time.Second),
		SharedDevSecretFallback:    envBool("PM_SHARED_ALLOW_DEV_SECRET_FALLBACK", false),
		TokenTTL:                   envDuration("PM_TOKEN_TTL", 7*24*time.Hour),
		CORSOrigins:                csv(os.Getenv("PM_CORS_ORIGINS")),
		Bootstrap: Bootstrap{
			Username:      os.Getenv("PM_BOOTSTRAP_ADMIN_USERNAME"),
			Password:      os.Getenv("PM_BOOTSTRAP_ADMIN_PASSWORD"),
			DisplayName:   env("PM_BOOTSTRAP_ADMIN_DISPLAY_NAME", "PM Administrator"),
			Email:         os.Getenv("PM_BOOTSTRAP_ADMIN_EMAIL"),
			WorkspaceName: env("PM_BOOTSTRAP_WORKSPACE_NAME", "Default Workspace"),
			WorkspaceSlug: env("PM_BOOTSTRAP_WORKSPACE_SLUG", "default"),
			TeamName:      env("PM_BOOTSTRAP_TEAM_NAME", "Default Team"),
		},
		ObjectStorage: ObjectStorage{
			Enabled:       envBool("PM_OBJECT_STORAGE_ENABLED", false),
			Endpoint:      strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_ENDPOINT")),
			Region:        strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_REGION")),
			Bucket:        strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_BUCKET")),
			AccessKey:     strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_ACCESS_KEY")),
			SecretKey:     strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_SECRET_KEY")),
			UseSSL:        envBool("PM_OBJECT_STORAGE_USE_SSL", false),
			GlobalPrefix:  strings.Trim(strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_GLOBAL_PREFIX")), "/"),
			TeamPrefix:    envBool("PM_OBJECT_STORAGE_TEAM_PREFIX", true),
			PublicBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("PM_OBJECT_STORAGE_PUBLIC_BASE_URL")), "/"),
		},
		Collaboration: Collaboration{
			Enabled:       envBool("PM_COLLAB_ENABLED", false),
			InternalURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("PM_COLLAB_INTERNAL_URL")), "/"),
			PublicURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("PM_COLLAB_PUBLIC_URL")), "/"),
			ServiceToken:  strings.TrimSpace(os.Getenv("PM_COLLAB_SERVICE_TOKEN")),
			TokenKey:      strings.TrimSpace(os.Getenv("PM_COLLAB_TOKEN_KEY")),
			TicketTTL:     envDuration("PM_COLLAB_TICKET_TTL", time.Minute),
			GrantTTL:      envDuration("PM_COLLAB_GRANT_TTL", 30*time.Second),
			ValidationTTL: envDuration("PM_COLLAB_VALIDATION_TTL", 10*time.Second),
		},
		Notification: Notification{
			Enabled:        envBool("PM_NOTIFICATION_ENABLED", false),
			HubRPCBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("PM_NOTIFICATION_HUB_RPC_BASE_URL")), "/"),
			InternalToken:  notificationInternalToken,
			PublicBaseURL:  notificationPublicBaseURL,
			PollInterval:   envDuration("PM_NOTIFICATION_POLL_INTERVAL", 2*time.Second),
			RequestTimeout: envDuration("PM_NOTIFICATION_REQUEST_TIMEOUT", 8*time.Second),
			ClaimTTL:       envDuration("PM_NOTIFICATION_CLAIM_TTL", 2*time.Minute),
			BatchSize:      envInt("PM_NOTIFICATION_BATCH_SIZE", 20),
		},
		AI: AI{
			Enabled:                     envBool("AI_ENABLED", false),
			EmbeddedWorker:              envBool("PM_AI_EMBEDDED_WORKER", false),
			RuntimeBaseURL:              strings.TrimRight(strings.TrimSpace(env("AI_RUNTIME_BASE_URL", "http://pm-ai-runtime:8091")), "/"),
			InternalSecret:              strings.TrimSpace(os.Getenv("AI_INTERNAL_SECRET")),
			Model:                       env("AI_MODEL", "qwen3.6-max-preview"),
			RequestTimeout:              envDuration("AI_REQUEST_TIMEOUT", 360*time.Second),
			WorkerConcurrency:           clampInt(envInt("PM_AI_WORKER_CONCURRENCY", 3), 1, 16),
			DesignReviewModel:           strings.TrimSpace(os.Getenv("AI_DESIGN_REVIEW_MODEL")),
			DesignReviewReasoningEffort: env("AI_DESIGN_REVIEW_REASONING_EFFORT", "medium"),
			DesignReviewImageDetail:     env("AI_DESIGN_REVIEW_IMAGE_DETAIL", "high"),
		},
		DesignReview: DesignReview{
			FigmaBaseURL:        strings.TrimRight(env("PM_FIGMA_BASE_URL", "https://api.figma.com"), "/"),
			FigmaToken:          strings.TrimSpace(os.Getenv("PM_FIGMA_TOKEN")),
			ExtensionVersion:    env("PM_DESIGN_REVIEW_EXTENSION_VERSION", "0.1.0"),
			CollectorVersion:    env("PM_DESIGN_REVIEW_COLLECTOR_VERSION", "0.1.0"),
			MobilewrightVersion: env("PM_DESIGN_REVIEW_MOBILEWRIGHT_VERSION", "0.0.53"),
			RequestTimeout:      envDuration("PM_FIGMA_REQUEST_TIMEOUT", 45*time.Second),
		},
		Feishu: Feishu{
			Enabled:         envBool("PM_FEISHU_ENABLED", false),
			AppID:           strings.TrimSpace(os.Getenv("PM_FEISHU_APP_ID")),
			AppSecret:       strings.TrimSpace(os.Getenv("PM_FEISHU_APP_SECRET")),
			DomainAllowlist: csv(env("PM_FEISHU_DOMAIN_ALLOWLIST", ".feishu.cn")),
			MaxBlocks:       envInt("PM_FEISHU_MAX_BLOCKS", 3000),
			MaxContentChars: envInt("PM_FEISHU_MAX_CONTENT_CHARS", 250000),
		},
		FeishuLogin: FeishuLogin{
			PreviousAppID:     strings.TrimSpace(os.Getenv("PM_FEISHU_LOGIN_PREVIOUS_APP_ID")),
			PreviousAppSecret: strings.TrimSpace(os.Getenv("PM_FEISHU_LOGIN_PREVIOUS_APP_SECRET")),
			BaseURL:           strings.TrimRight(strings.TrimSpace(env("PM_FEISHU_LOGIN_BASE_URL", "https://open.feishu.cn")), "/"),
			RedirectURL:       loginCallbackURL(publicBaseURL, feishuLoginCallbackPath),
			RequestTimeout:    envDuration("PM_FEISHU_LOGIN_REQUEST_TIMEOUT", 10*time.Second),
			TenantKeys:        csv(os.Getenv("PM_FEISHU_LOGIN_TENANT_KEYS")),
		},
		GoogleLogin: GoogleLogin{
			Enabled:        envBool("PM_GOOGLE_LOGIN_ENABLED", false),
			ClientID:       strings.TrimSpace(os.Getenv("PM_GOOGLE_LOGIN_CLIENT_ID")),
			ClientSecret:   strings.TrimSpace(os.Getenv("PM_GOOGLE_LOGIN_CLIENT_SECRET")),
			RedirectURL:    loginCallbackURL(publicBaseURL, googleLoginCallbackPath),
			RequestTimeout: envDuration("PM_GOOGLE_LOGIN_REQUEST_TIMEOUT", 10*time.Second),
		},
		TelegramLogin: TelegramLogin{
			Enabled:        envBool("PM_TELEGRAM_LOGIN_ENABLED", false),
			ClientID:       strings.TrimSpace(os.Getenv("PM_TELEGRAM_LOGIN_CLIENT_ID")),
			ClientSecret:   strings.TrimSpace(os.Getenv("PM_TELEGRAM_LOGIN_CLIENT_SECRET")),
			RedirectURL:    loginCallbackURL(publicBaseURL, telegramLoginCallbackPath),
			RequestTimeout: envDuration("PM_TELEGRAM_LOGIN_REQUEST_TIMEOUT", 10*time.Second),
		},
		ExternalRegistrationEnabled: envBool("PM_EXTERNAL_REGISTRATION_ENABLED", false),
		SSO: SSO{
			Enabled:        envBool("PM_SSO_ENABLED", false),
			AuthorizeURL:   strings.TrimSpace(os.Getenv("PM_SSO_AUTHORIZE_URL")),
			TokenURL:       strings.TrimSpace(os.Getenv("PM_SSO_TOKEN_URL")),
			Issuer:         strings.TrimSpace(os.Getenv("PM_SSO_ISSUER")),
			ClientID:       strings.TrimSpace(os.Getenv("PM_SSO_CLIENT_ID")),
			ClientSecret:   strings.TrimSpace(os.Getenv("PM_SSO_CLIENT_SECRET")),
			RedirectURL:    loginCallbackURL(publicBaseURL, ssoLoginCallbackPath),
			RequestTimeout: envDuration("PM_SSO_REQUEST_TIMEOUT", 10*time.Second),
		},
		RequirementCoverEnabled: envBool("PM_REQUIREMENT_COVER_ENABLED", false),
		CoverRenderer: CoverRenderer{
			BaseURL:        strings.TrimRight(strings.TrimSpace(os.Getenv("PM_COVER_RENDERER_BASE_URL")), "/"),
			InternalToken:  strings.TrimSpace(os.Getenv("PM_COVER_RENDERER_INTERNAL_TOKEN")),
			RequestTimeout: envDuration("PM_COVER_RENDERER_REQUEST_TIMEOUT", 10*time.Second),
		},
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.DatabaseDriver {
	case "sqlite", "postgres", "mysql":
	default:
		return fmt.Errorf("unsupported PM_DATABASE_DRIVER %q", c.DatabaseDriver)
	}
	if err := c.validateDevelopmentDatabaseTarget(); err != nil {
		return err
	}
	switch c.SchemaMigrationMode {
	case "", SchemaMigrationApply, SchemaMigrationValidate, SchemaMigrationOff:
	default:
		return fmt.Errorf("unsupported PM_SCHEMA_MIGRATION_MODE %q: expected apply, validate, or off", c.SchemaMigrationMode)
	}
	switch c.RBACMode {
	case "", RBACModePrepare, RBACModeEnforce:
	default:
		return fmt.Errorf("unsupported PM_RBAC_MODE %q: expected prepare or enforce", c.RBACMode)
	}
	switch c.AuthMode {
	case "local", "shared_jwt", "hybrid":
	default:
		return fmt.Errorf("unsupported PM_AUTH_MODE %q", c.AuthMode)
	}
	production := strings.EqualFold(c.Environment, "production")
	if production && c.RBACMode == RBACModeEnforce && c.RBACBootstrapUserID == 0 {
		return errors.New("production PM_RBAC_MODE=enforce requires PM_RBAC_BOOTSTRAP_USER_ID")
	}
	if strings.TrimSpace(c.LocalJWTSecret) != "" {
		if err := c.validateSessionSigning(c.SSO.Enabled); err != nil {
			return err
		}
	}
	if c.SharedDevSecretFallback && production {
		return errors.New("PM_SHARED_ALLOW_DEV_SECRET_FALLBACK is forbidden in production")
	}
	if c.AuthMode != "local" && c.SharedIntrospectionURL == "" && !c.SharedDevSecretFallback {
		return errors.New("shared_jwt/hybrid requires PM_SHARED_AUTH_INTROSPECTION_URL")
	}
	if production && c.AuthMode != "local" && strings.TrimSpace(c.SharedJWTIssuer) == "" {
		return errors.New("production shared_jwt/hybrid requires a stable PM_SHARED_JWT_ISSUER")
	}
	if c.SharedDevSecretFallback && len(c.SharedJWTSecret) < 32 {
		return errors.New("development shared secret fallback requires PM_SHARED_JWT_SECRET with at least 32 characters")
	}
	hasUser := strings.TrimSpace(c.Bootstrap.Username) != ""
	hasPass := c.Bootstrap.Password != ""
	if hasUser != hasPass {
		return errors.New("bootstrap username and password must be configured together")
	}
	if hasPass && len(c.Bootstrap.Password) < 12 {
		return errors.New("bootstrap password must contain at least 12 characters")
	}
	if c.ObjectStorage.Enabled {
		missing := make([]string, 0, 4)
		for _, setting := range []struct {
			key   string
			value string
		}{
			{key: "PM_OBJECT_STORAGE_ENDPOINT", value: c.ObjectStorage.Endpoint},
			{key: "PM_OBJECT_STORAGE_BUCKET", value: c.ObjectStorage.Bucket},
			{key: "PM_OBJECT_STORAGE_ACCESS_KEY", value: c.ObjectStorage.AccessKey},
			{key: "PM_OBJECT_STORAGE_SECRET_KEY", value: c.ObjectStorage.SecretKey},
		} {
			if strings.TrimSpace(setting.value) == "" {
				missing = append(missing, setting.key)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("object storage enabled but required settings are missing: %s", strings.Join(missing, ", "))
		}
	}
	if c.Collaboration.InternalURL != "" {
		target, err := url.Parse(c.Collaboration.InternalURL)
		if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Path != "" && target.Path != "/") {
			return errors.New("PM_COLLAB_INTERNAL_URL must be an http(s) origin without credentials, query or fragment")
		}
	}
	if c.Collaboration.Enabled {
		if len(c.Collaboration.ServiceToken) < 32 || len([]byte(c.Collaboration.TokenKey)) != 32 {
			return errors.New("PM_COLLAB_ENABLED requires PM_COLLAB_SERVICE_TOKEN with at least 32 characters and an exact 32-byte PM_COLLAB_TOKEN_KEY")
		}
		parsed, err := url.Parse(c.Collaboration.PublicURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("PM_COLLAB_PUBLIC_URL must be a ws(s) URL without credentials, query, or fragment")
		}
		if production && parsed.Scheme != "wss" {
			return errors.New("production PM_COLLAB_PUBLIC_URL must use wss")
		}
		if c.Collaboration.TicketTTL <= 0 || c.Collaboration.TicketTTL > 5*time.Minute ||
			c.Collaboration.GrantTTL <= 0 || c.Collaboration.GrantTTL > time.Minute ||
			c.Collaboration.ValidationTTL <= 0 || c.Collaboration.ValidationTTL > 30*time.Second {
			return errors.New("collaboration ticket, grant, and validation TTLs exceed their safety bounds")
		}
	}
	if c.Notification.Enabled {
		if strings.TrimSpace(c.Notification.PublicBaseURL) == "" {
			return errors.New("notifications enabled but required settings are missing: PM_NOTIFICATION_PUBLIC_BASE_URL")
		}
		parsed, err := url.Parse(c.Notification.PublicBaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("PM_NOTIFICATION_PUBLIC_BASE_URL must be an http(s) URL without credentials, query, or fragment")
		}
		if c.Notification.PollInterval <= 0 || c.Notification.RequestTimeout <= 0 ||
			c.Notification.ClaimTTL <= 4*c.Notification.RequestTimeout {
			return errors.New("notification durations must be positive and claim TTL must exceed four request timeouts")
		}
		if c.Notification.BatchSize < 1 || c.Notification.BatchSize > 100 {
			return errors.New("PM_NOTIFICATION_BATCH_SIZE must be between 1 and 100")
		}
	}
	if c.AI.Enabled {
		if production && c.AI.EmbeddedWorker {
			return errors.New("PM_AI_EMBEDDED_WORKER is development-only; production must use pm-worker")
		}
		if len(c.AI.InternalSecret) < 32 {
			return errors.New("AI_ENABLED requires AI_INTERNAL_SECRET with at least 32 characters")
		}
		if strings.TrimSpace(c.AI.Model) == "" {
			return errors.New("AI_ENABLED requires AI_MODEL")
		}
		parsed, err := url.Parse(c.AI.RuntimeBaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("AI_RUNTIME_BASE_URL must be an http(s) URL without credentials, query, or fragment")
		}
		if c.AI.RequestTimeout <= 0 {
			return errors.New("AI_REQUEST_TIMEOUT must be positive")
		}
	}
	if model := strings.TrimSpace(c.AI.DesignReviewModel); model != "" {
		if !c.AI.Enabled {
			return errors.New("AI_DESIGN_REVIEW_MODEL requires AI_ENABLED")
		}
		if c.AI.DesignReviewReasoningEffort != "low" && c.AI.DesignReviewReasoningEffort != "medium" && c.AI.DesignReviewReasoningEffort != "high" {
			return errors.New("AI_DESIGN_REVIEW_REASONING_EFFORT must be low, medium, or high")
		}
		if c.AI.DesignReviewImageDetail != "low" && c.AI.DesignReviewImageDetail != "high" && c.AI.DesignReviewImageDetail != "auto" {
			return errors.New("AI_DESIGN_REVIEW_IMAGE_DETAIL must be low, high, or auto")
		}
	}
	if strings.TrimSpace(c.DesignReview.FigmaToken) != "" {
		parsed, err := url.Parse(c.DesignReview.FigmaBaseURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("PM_FIGMA_BASE_URL must be a root https URL without credentials, query, or fragment")
		}
		if c.DesignReview.RequestTimeout <= 0 {
			return errors.New("PM_FIGMA_REQUEST_TIMEOUT must be positive")
		}
	}
	if c.Feishu.Enabled {
		if c.Feishu.AppID == "" || c.Feishu.AppSecret == "" {
			return errors.New("PM_FEISHU_ENABLED requires PM_FEISHU_APP_ID and PM_FEISHU_APP_SECRET")
		}
		if len(c.Feishu.DomainAllowlist) == 0 || c.Feishu.MaxBlocks < 1 || c.Feishu.MaxContentChars < 1 {
			return errors.New("Feishu allowlist and content limits must be configured")
		}
	}
	if c.SSO.Enabled {
		missing := make([]string, 0, 5)
		for _, setting := range []struct {
			key   string
			value string
		}{
			{"PM_SSO_AUTHORIZE_URL", c.SSO.AuthorizeURL},
			{"PM_SSO_TOKEN_URL", c.SSO.TokenURL},
			{"PM_SSO_ISSUER", c.SSO.Issuer},
			{"PM_SSO_CLIENT_ID", c.SSO.ClientID},
			{"PM_SSO_CLIENT_SECRET", c.SSO.ClientSecret},
		} {
			if strings.TrimSpace(setting.value) == "" {
				missing = append(missing, setting.key)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("PM_SSO_ENABLED requires %s", strings.Join(missing, ", "))
		}
		if err := validateRemoteURL("PM_SSO_AUTHORIZE_URL", c.SSO.AuthorizeURL, false, production); err != nil {
			return err
		}
		if err := validateRemoteURL("PM_SSO_TOKEN_URL", c.SSO.TokenURL, false, production); err != nil {
			return err
		}
		if c.SSO.RedirectURL != loginCallbackURL(c.PublicBaseURL, ssoLoginCallbackPath) {
			return errors.New("SSO callback must use the PM public web entry")
		}
		if c.SSO.RequestTimeout <= 0 {
			return errors.New("PM_SSO_REQUEST_TIMEOUT must be positive")
		}
	}
	if c.GoogleLogin.Enabled {
		missing := make([]string, 0, 2)
		if strings.TrimSpace(c.GoogleLogin.ClientID) == "" {
			missing = append(missing, "PM_GOOGLE_LOGIN_CLIENT_ID")
		}
		if strings.TrimSpace(c.GoogleLogin.ClientSecret) == "" {
			missing = append(missing, "PM_GOOGLE_LOGIN_CLIENT_SECRET")
		}
		if len(missing) > 0 {
			return fmt.Errorf("PM_GOOGLE_LOGIN_ENABLED requires %s", strings.Join(missing, ", "))
		}
		if c.GoogleLogin.RedirectURL != loginCallbackURL(c.PublicBaseURL, googleLoginCallbackPath) {
			return errors.New("Google login callback must use the PM public web entry")
		}
		if err := validateGoogleCallbackURL("PM_PUBLIC_BASE_URL", c.GoogleLogin.RedirectURL, production); err != nil {
			return err
		}
		if c.GoogleLogin.RequestTimeout <= 0 {
			return errors.New("PM_GOOGLE_LOGIN_REQUEST_TIMEOUT must be positive")
		}
	}
	if c.TelegramLogin.Enabled {
		missing := make([]string, 0, 2)
		if strings.TrimSpace(c.TelegramLogin.ClientID) == "" {
			missing = append(missing, "PM_TELEGRAM_LOGIN_CLIENT_ID")
		}
		if strings.TrimSpace(c.TelegramLogin.ClientSecret) == "" {
			missing = append(missing, "PM_TELEGRAM_LOGIN_CLIENT_SECRET")
		}
		if len(missing) > 0 {
			return fmt.Errorf("PM_TELEGRAM_LOGIN_ENABLED requires %s", strings.Join(missing, ", "))
		}
		if c.TelegramLogin.RedirectURL != loginCallbackURL(c.PublicBaseURL, telegramLoginCallbackPath) {
			return errors.New("Telegram login callback must use the PM public web entry")
		}
		if err := validateRemoteURL("PM_PUBLIC_BASE_URL", c.TelegramLogin.RedirectURL, false, production); err != nil {
			return err
		}
		if c.TelegramLogin.RequestTimeout <= 0 {
			return errors.New("PM_TELEGRAM_LOGIN_REQUEST_TIMEOUT must be positive")
		}
	}
	rendererConfigured := c.CoverRenderer.BaseURL != "" || c.CoverRenderer.InternalToken != ""
	if rendererConfigured {
		if c.CoverRenderer.BaseURL == "" || len(c.CoverRenderer.InternalToken) < 32 {
			return errors.New("cover renderer requires PM_COVER_RENDERER_BASE_URL and PM_COVER_RENDERER_INTERNAL_TOKEN with at least 32 characters")
		}
		parsed, err := url.Parse(c.CoverRenderer.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") {
			return errors.New("PM_COVER_RENDERER_BASE_URL must be a root http(s) URL without credentials, query, or fragment")
		}
		if c.CoverRenderer.RequestTimeout <= 0 {
			return errors.New("PM_COVER_RENDERER_REQUEST_TIMEOUT must be positive")
		}
	}
	if (c.FeishuLogin.PreviousAppID == "") != (c.FeishuLogin.PreviousAppSecret == "") {
		return errors.New("PM_FEISHU_LOGIN_PREVIOUS_APP_ID and PM_FEISHU_LOGIN_PREVIOUS_APP_SECRET must be configured together")
	}
	return nil
}

// LocalPreview uses a dedicated local Postgres database and password login.
func (c Config) LocalPreview() bool {
	return c.LocalPreviewRequested && c.Environment == "development" && c.AuthMode == "local" && c.DatabaseDriver == "postgres"
}

func (c Config) ValidateAPIRuntime() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := c.validateSessionSigning(true); err != nil {
		return err
	}
	if c.LocalPreview() {
		return nil
	}
	if c.FeishuLogin.AppID == "" || c.FeishuLogin.AppSecret == "" {
		return errors.New("PM Feishu login application is not configured")
	}
	if c.FeishuLogin.RedirectURL == "" {
		return errors.New("PM Feishu login callback is not configured")
	}
	if c.FeishuLogin.RedirectURL != loginCallbackURL(c.PublicBaseURL, feishuLoginCallbackPath) {
		return errors.New("Feishu login callback must use the PM public web entry")
	}
	if (c.FeishuLogin.PreviousAppID == "") != (c.FeishuLogin.PreviousAppSecret == "") {
		return errors.New("PM_FEISHU_LOGIN_PREVIOUS_APP_ID and PM_FEISHU_LOGIN_PREVIOUS_APP_SECRET must be configured together")
	}
	production := strings.EqualFold(c.Environment, "production")
	if err := validateRemoteURL("PM_FEISHU_LOGIN_BASE_URL", c.FeishuLogin.BaseURL, false, production); err != nil {
		return err
	}
	if c.FeishuLogin.RequestTimeout <= 0 {
		return errors.New("PM_FEISHU_LOGIN_REQUEST_TIMEOUT must be positive")
	}
	for _, tenantKey := range c.FeishuLogin.TenantKeys {
		tenantKey = strings.TrimSpace(tenantKey)
		if tenantKey == "" || len(tenantKey) > 191 {
			return errors.New("PM_FEISHU_LOGIN_TENANT_KEYS contains an invalid tenant key")
		}
	}
	return nil
}

func (c Config) validateSessionSigning(pmSessionEnabled bool) error {
	if c.AuthMode != "shared_jwt" && !pmSessionEnabled {
		return nil
	}
	secret := strings.TrimSpace(c.LocalJWTSecret)
	if secret == "" {
		return errors.New("PM local session signing key is not configured in the PM database")
	}
	if len(secret) < 32 {
		return errors.New("PM local session signing key must contain at least 32 characters")
	}
	return nil
}

func (c Config) ValidateCoverWorkerRuntime() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !c.RequirementCoverEnabled || !c.Feishu.Enabled || !c.ObjectStorage.Enabled || c.CoverRenderer.BaseURL == "" || len(c.CoverRenderer.InternalToken) < 32 {
		return errors.New("cover worker requires PM_REQUIREMENT_COVER_ENABLED, Feishu, object storage, PM_COVER_RENDERER_BASE_URL, and PM_COVER_RENDERER_INTERNAL_TOKEN")
	}
	return nil
}

// canonicalNotificationPublicBaseURL is the origin Feishu IM links open.
// Shared-JWT installs sometimes copy MAIN_PUBLIC_BASE (the One2All host
// console) into PM_NOTIFICATION_PUBLIC_BASE_URL; those links 404 outside
// the host shell, so PM keeps the standalone public origin instead.
func canonicalNotificationPublicBaseURL(configured, fallback string) string {
	fallback = strings.TrimRight(strings.TrimSpace(fallback), "/")
	configured = strings.TrimRight(strings.TrimSpace(configured), "/")
	if configured == "" {
		return fallback
	}
	parsed, err := url.Parse(configured)
	if err != nil || parsed.Host == "" {
		return configured
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.Path, "/")
	if strings.Contains(host, "one2all") || path == "one2all" || strings.HasPrefix(path, "one2all/") {
		if fallback != "" {
			return fallback
		}
	}
	return configured
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envUint64(key string, fallback uint64) uint64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func loginCallbackURL(publicBaseURL, callbackPath string) string {
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" {
		return ""
	}
	return publicBaseURL + callbackPath
}

func validateRemoteURL(key, value string, allowPath bool, production bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an http(s) URL without credentials or fragment", key)
	}
	if production && parsed.Scheme != "https" {
		return fmt.Errorf("production %s must use https", key)
	}
	if !allowPath && parsed.RawQuery != "" {
		return fmt.Errorf("%s must not contain a query", key)
	}
	return nil
}

func validateGoogleCallbackURL(key, value string, production bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	localhostHTTP := err == nil && parsed.Scheme == "http" && strings.EqualFold(parsed.Hostname(), "localhost")
	return validateRemoteURL(key, value, false, production && !localhostHTTP)
}

func csv(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
