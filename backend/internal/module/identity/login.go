package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
)

const (
	FlowFeishu         = "feishu"
	FlowSSO            = "sso"
	FlowGoogle         = "google"
	FlowTelegram       = "telegram"
	defaultStateTTL    = 10 * time.Minute
	defaultCodeTTL     = 2 * time.Minute
	feishuTenantMaxLen = 191
	feishuOpenIDMax    = 191
	feishuNameMax      = 128
	feishuAvatarMax    = 512
)

var (
	ErrInvalidLoginState             = errors.New("invalid or expired login state")
	ErrInvalidLoginCode              = errors.New("invalid or expired login code")
	ErrLoginUserNotFound             = errors.New("login user is not provisioned")
	ErrLoginUserDisabled             = errors.New("login user is disabled")
	ErrLoginTenantNotRegistered      = errors.New("Feishu tenant is not registered")
	ErrLoginTenantDisabled           = errors.New("Feishu tenant is disabled")
	ErrLoginFeishuIdentityInvalid    = errors.New("Feishu identity is invalid")
	ErrLoginFeishuIdentityAmbiguous  = errors.New("Feishu identity matches multiple PM users")
	ErrLoginFeishuIdentityConflict   = errors.New("Feishu identity is already bound to another PM user")
	ErrLoginGoogleIdentityInvalid    = errors.New("Google identity is invalid")
	ErrLoginGoogleEmailUnverified    = errors.New("Google email is not verified")
	ErrLoginGoogleEmailAmbiguous     = errors.New("Google email matches multiple PM users")
	ErrLoginGoogleIdentityConflict   = errors.New("Google identity is already bound to another PM user")
	ErrLoginProviderUnavailable      = errors.New("login provider is unavailable")
	ErrLoginTelegramIdentityInvalid  = errors.New("Telegram identity is invalid")
	ErrLoginTelegramIdentityConflict = errors.New("Telegram identity conflicts with an existing binding")
)

type LoginOptions struct {
	StateTTL time.Duration
	CodeTTL  time.Duration
	Now      func() time.Time
	Random   io.Reader
}

type LoginService struct {
	repo     LoginRepository
	stateTTL time.Duration
	codeTTL  time.Duration
	now      func() time.Time
	random   io.Reader
}

func NewLoginService(repo LoginRepository, options LoginOptions) *LoginService {
	if options.StateTTL <= 0 {
		options.StateTTL = defaultStateTTL
	}
	if options.CodeTTL <= 0 {
		options.CodeTTL = defaultCodeTTL
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	return &LoginService{repo: repo, stateTTL: options.StateTTL, codeTTL: options.CodeTTL, now: options.Now, random: options.Random}
}

// NormalizeReturnTo accepts only a relative same-origin browser path. It
// deliberately rejects scheme-relative paths such as //evil.example.
func NormalizeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "/"
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.RequestURI()
}

func (s *LoginService) Start(ctx context.Context, flow, returnTo string) (string, error) {
	if !validFlow(flow) {
		return "", ErrInvalidLoginState
	}
	raw, digest, err := s.secret()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	transaction := &LoginTransaction{
		StateHash: digest, Flow: flow, ReturnTo: NormalizeReturnTo(returnTo),
		ExpiresAt: now.Add(s.stateTTL), CreatedAt: now,
	}
	if err := s.repo.CreateLoginTransaction(ctx, transaction); err != nil {
		return "", err
	}
	return raw, nil
}

// startGoogle creates independent state and nonce values. Only their digests
// are persisted, so a database reader cannot replay an in-flight OIDC login.
func (s *LoginService) startGoogle(ctx context.Context, returnTo string) (string, string, error) {
	state, stateDigest, err := s.secret()
	if err != nil {
		return "", "", err
	}
	nonce, nonceDigest, err := s.secret()
	if err != nil {
		return "", "", err
	}
	now := s.now().UTC()
	transaction := &LoginTransaction{
		StateHash: stateDigest, NonceHash: nonceDigest, Flow: FlowGoogle,
		ReturnTo: NormalizeReturnTo(returnTo), ExpiresAt: now.Add(s.stateTTL), CreatedAt: now,
	}
	if err := s.repo.CreateLoginTransaction(ctx, transaction); err != nil {
		return "", "", err
	}
	return state, nonce, nil
}

// startTelegram persists the raw PKCE verifier only in the short-lived
// server-side transaction. State and nonce remain digest-only values.
func (s *LoginService) startTelegram(ctx context.Context, returnTo string) (string, string, string, error) {
	state, stateDigest, err := s.secret()
	if err != nil {
		return "", "", "", err
	}
	nonce, nonceDigest, err := s.secret()
	if err != nil {
		return "", "", "", err
	}
	verifier, _, err := s.secret()
	if err != nil {
		return "", "", "", err
	}
	now := s.now().UTC()
	transaction := &LoginTransaction{
		StateHash: stateDigest, NonceHash: nonceDigest, PKCEVerifier: verifier, Flow: FlowTelegram,
		ReturnTo: NormalizeReturnTo(returnTo), ExpiresAt: now.Add(s.stateTTL), CreatedAt: now,
	}
	if err := s.repo.CreateLoginTransaction(ctx, transaction); err != nil {
		return "", "", "", err
	}
	return state, nonce, pkceS256Challenge(verifier), nil
}

func (s *LoginService) ConsumeState(ctx context.Context, flow, raw string) (string, error) {
	if !validFlow(flow) {
		return "", ErrInvalidLoginState
	}
	transaction, err := s.repo.ConsumeLoginTransaction(ctx, flow, digest(raw), s.now().UTC())
	if err != nil || transaction.Flow != flow {
		return "", ErrInvalidLoginState
	}
	return NormalizeReturnTo(transaction.ReturnTo), nil
}

func (s *LoginService) consumeGoogleState(ctx context.Context, raw string) (LoginTransaction, error) {
	transaction, err := s.repo.ConsumeLoginTransaction(ctx, FlowGoogle, digest(raw), s.now().UTC())
	if err != nil || transaction.Flow != FlowGoogle || strings.TrimSpace(transaction.NonceHash) == "" {
		return LoginTransaction{}, ErrInvalidLoginState
	}
	transaction.ReturnTo = NormalizeReturnTo(transaction.ReturnTo)
	return transaction, nil
}

func (s *LoginService) consumeTelegramState(ctx context.Context, raw string) (LoginTransaction, error) {
	transaction, err := s.repo.ConsumeLoginTransaction(ctx, FlowTelegram, digest(raw), s.now().UTC())
	if err != nil || transaction.Flow != FlowTelegram || strings.TrimSpace(transaction.NonceHash) == "" ||
		strings.TrimSpace(transaction.PKCEVerifier) == "" {
		return LoginTransaction{}, ErrInvalidLoginState
	}
	transaction.ReturnTo = NormalizeReturnTo(transaction.ReturnTo)
	return transaction, nil
}

func (s *LoginService) IssueCode(ctx context.Context, flow string, userID uint) (string, error) {
	if !validFlow(flow) || userID == 0 {
		return "", ErrInvalidLoginCode
	}
	raw, digest, err := s.secret()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	code := &LoginCode{
		CodeHash: digest, UserID: userID, Flow: flow,
		ExpiresAt: now.Add(s.codeTTL), CreatedAt: now,
	}
	if err := s.repo.CreateLoginCode(ctx, code); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *LoginService) ExchangeCode(ctx context.Context, raw string) (User, string, error) {
	code, err := s.repo.ConsumeLoginCode(ctx, digest(raw), s.now().UTC())
	if err != nil || !validFlow(code.Flow) || code.UserID == 0 {
		return User{}, "", ErrInvalidLoginCode
	}
	user, err := s.repo.FindActiveUserByID(ctx, code.UserID)
	if err != nil {
		return User{}, "", ErrLoginUserNotFound
	}
	if !user.Active {
		return User{}, "", ErrLoginUserDisabled
	}
	return user, code.Flow, nil
}

// RegisterFeishuTenants creates configured tenant registrations without
// changing the enabled state of an existing tenant. It is called only from
// PM's composition root, never from a browser request.
func (s *LoginService) RegisterFeishuTenants(ctx context.Context, tenantKeys []string) error {
	keys := make([]string, 0, len(tenantKeys))
	seen := make(map[string]struct{}, len(tenantKeys))
	for _, tenantKey := range tenantKeys {
		tenantKey = strings.TrimSpace(tenantKey)
		if tenantKey == "" || len(tenantKey) > feishuTenantMaxLen {
			return ErrLoginFeishuIdentityInvalid
		}
		if _, exists := seen[tenantKey]; exists {
			continue
		}
		seen[tenantKey] = struct{}{}
		keys = append(keys, tenantKey)
	}
	if len(keys) == 0 {
		return nil
	}
	return s.repo.RegisterFeishuTenants(ctx, keys)
}

// ResolveOrProvisionFeishuUser admits an employee of a registered Feishu
// company. A previously persisted login-application open_id restores the
// same PM user across directory-name changes; exact company/name matching is
// the first-login and application-credential-change fallback. A first-company
// bootstrap links legacy PM users to the verified company in one transaction.
func (s *LoginService) ResolveOrProvisionFeishuUser(ctx context.Context, provider FeishuIdentity) (User, error) {
	provider = normalizeFeishuIdentity(provider)
	if provider.TenantKey == "" || provider.DisplayName == "" ||
		len(provider.TenantKey) > feishuTenantMaxLen {
		return User{}, ErrLoginFeishuIdentityInvalid
	}
	user, err := s.repo.ProvisionFeishuUser(ctx, provider)
	if err != nil {
		return User{}, err
	}
	if !user.Active {
		return User{}, ErrLoginUserDisabled
	}
	return user, nil
}

func (s *LoginService) ResolveSSOUser(ctx context.Context, issuer, subject string) (User, error) {
	issuer, subject = strings.TrimSpace(issuer), strings.TrimSpace(subject)
	if issuer == "" || subject == "" {
		return User{}, ErrLoginUserNotFound
	}
	user, err := s.repo.FindActiveUserByExternalIdentity(ctx, issuer, subject)
	if err != nil {
		return User{}, ErrLoginUserNotFound
	}
	if !user.Active {
		return User{}, ErrLoginUserDisabled
	}
	return user, nil
}

func (s *LoginService) secret() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(s.random, buffer); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(buffer)
	return raw, digest(raw), nil
}

func digest(raw string) string {
	return digestBytes([]byte(strings.TrimSpace(raw)))
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// FeishuUsername is a stable PM-local fallback username for a company user
// that does not yet exist in the imported users table. It is derived only
// from the company key and exact directory name, never an app-scoped open_id.
func FeishuUsername(tenantKey, displayName string) string {
	return "feishu_" + digest(tenantKey+"\x00"+displayName)
}

// FeishuExternalSubject keeps newly created company users distinct from the
// imported One2All identity namespace without storing provider user IDs.
func FeishuExternalSubject(tenantKey, displayName string) string {
	return "feishu-name:" + digest(tenantKey+"\x00"+displayName)
}

func normalizeFeishuIdentity(value FeishuIdentity) FeishuIdentity {
	value.TenantKey = strings.TrimSpace(value.TenantKey)
	value.OpenID = strings.TrimSpace(value.OpenID)
	if len(value.OpenID) > feishuOpenIDMax {
		value.OpenID = ""
	}
	value.DisplayName = trimRunes(strings.TrimSpace(value.DisplayName), feishuNameMax)
	value.AvatarURL = trimRunes(strings.TrimSpace(value.AvatarURL), feishuAvatarMax)
	return value
}

func trimRunes(value string, limit int) string {
	if limit < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func pkceS256Challenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func validFlow(flow string) bool {
	return flow == FlowFeishu || flow == FlowSSO || flow == FlowGoogle || flow == FlowTelegram
}
