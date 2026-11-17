package auth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type Identity struct {
	UserID        uint
	Issuer        string
	Subject       string
	Username      string
	DisplayName   string
	Email         string
	Role          string
	WorkspaceID   uint
	WorkspaceName string
	Source        string
}

type Tokens struct {
	cfg config.Config
}

func NewTokens(cfg config.Config) *Tokens {
	return &Tokens{cfg: cfg}
}

func (t *Tokens) SignLocal(userID uint, username, role string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"uid":       userID,
		"sub":       username,
		"role":      role,
		"token_use": "pm_local",
		"iat":       now.Unix(),
		"exp":       now.Add(t.cfg.TokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(t.cfg.LocalJWTSecret))
}

// SignSession signs the common PM session used by both OAuth entry points.
// The token contains a PM user identity only; role and workspace facts are
// deliberately excluded from the session contract.
func (t *Tokens) SignSession(userID uint, username, displayName, email string) (string, error) {
	if userID == 0 || len(t.cfg.LocalJWTSecret) < 32 {
		return "", errors.New("PM session signing is not configured")
	}
	now := time.Now().UTC()
	ttl := t.cfg.TokenTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	claims := jwt.MapClaims{
		"iss":          PMSessionIssuer,
		"uid":          userID,
		"sub":          fmt.Sprintf("user:%d", userID),
		"username":     strings.TrimSpace(username),
		"display_name": strings.TrimSpace(displayName),
		"email":        strings.TrimSpace(email),
		"token_use":    "pm_session",
		"iat":          now.Unix(),
		"exp":          now.Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(t.cfg.LocalJWTSecret))
}

func (t *Tokens) VerifySession(raw string) (Identity, error) {
	claims, err := parse(raw, t.cfg.LocalJWTSecret, PMSessionIssuer, "")
	if err != nil {
		return Identity{}, err
	}
	identity := identityFromClaims(claims)
	if claimString(claims, "token_use") != "pm_session" || identity.UserID == 0 ||
		identity.Subject != fmt.Sprintf("user:%d", identity.UserID) {
		return Identity{}, errors.New("not a PM session token")
	}
	identity.Issuer = PMSessionIssuer
	identity.Source = "pm_session"
	return identity, nil
}

func (t *Tokens) VerifyLocal(raw string) (Identity, error) {
	if t.cfg.AuthMode != "local" && t.cfg.AuthMode != "hybrid" {
		return Identity{}, errors.New("local tokens are disabled")
	}
	claims, err := parse(raw, t.cfg.LocalJWTSecret, "", "")
	if err != nil {
		return Identity{}, err
	}
	identity := identityFromClaims(claims)
	if identity.UserID == 0 || claimString(claims, "token_use") != "pm_local" {
		return Identity{}, errors.New("not a PM local token")
	}
	identity.Source = "local"
	return identity, nil
}

func (t *Tokens) VerifySharedFallback(raw string) (Identity, error) {
	if !t.cfg.SharedDevSecretFallback {
		return Identity{}, errors.New("shared token local verification is disabled")
	}
	claims, err := parse(raw, t.cfg.SharedJWTSecret, t.cfg.SharedJWTIssuer, t.cfg.SharedJWTAudience)
	if err != nil {
		return Identity{}, err
	}
	identity := identityFromClaims(claims)
	if identity.Subject == "" && identity.UserID == 0 {
		return Identity{}, errors.New("shared token has no subject")
	}
	identity.Source = "shared_jwt"
	identity.Issuer = t.sharedIssuer()
	return identity, nil
}

func (t *Tokens) sharedIssuer() string {
	if value := strings.TrimSpace(t.cfg.SharedJWTIssuer); value != "" {
		return value
	}
	return strings.TrimSpace(t.cfg.SharedIntrospectionURL)
}

func parse(raw, secret, issuer, audience string) (jwt.MapClaims, error) {
	if len(secret) < 32 {
		return nil, errors.New("jwt secret is not configured")
	}
	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	}
	if issuer != "" {
		options = append(options, jwt.WithIssuer(issuer))
	}
	if audience != "" {
		options = append(options, jwt.WithAudience(audience))
	}
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg())
		}
		return []byte(secret), nil
	}, options...)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

func identityFromClaims(claims jwt.MapClaims) Identity {
	subject := claimString(claims, "sub")
	username := claimString(claims, "username")
	if username == "" {
		username = claimString(claims, "preferred_username")
	}
	if username == "" {
		username = subject
	}
	return Identity{
		Issuer:        claimString(claims, "iss"),
		UserID:        claimUint(claims, "uid", "user_id"),
		Subject:       subject,
		Username:      username,
		DisplayName:   claimString(claims, "display_name", "name"),
		Email:         claimString(claims, "email"),
		Role:          claimString(claims, "role"),
		WorkspaceID:   claimUint(claims, "workspace_id", "wid"),
		WorkspaceName: claimString(claims, "workspace_name"),
	}
}

func claimString(claims jwt.MapClaims, keys ...string) string {
	for _, key := range keys {
		switch value := claims[key].(type) {
		case string:
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		case fmt.Stringer:
			return strings.TrimSpace(value.String())
		}
	}
	return ""
}

func claimUint(claims jwt.MapClaims, keys ...string) uint {
	for _, key := range keys {
		switch value := claims[key].(type) {
		case float64:
			if value > 0 {
				return uint(value)
			}
		case jsonNumber:
			parsed, _ := strconv.ParseUint(string(value), 10, 64)
			return uint(parsed)
		case string:
			parsed, _ := strconv.ParseUint(value, 10, 64)
			if parsed > 0 {
				return uint(parsed)
			}
		}
	}
	return 0
}

type jsonNumber string
