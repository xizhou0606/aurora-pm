package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

var (
	errSessionExpired          = errors.New("shared session expired")
	errAuthUpstreamUnavailable = errors.New("shared auth upstream unavailable")
	errIdentityNotProvisioned  = errors.New("shared identity not provisioned")
)

type cachedIdentity struct {
	Identity  pmauth.Identity
	ExpiresAt time.Time
}

func (s *Server) authenticateToken(c *gin.Context, raw string) (pmauth.Identity, error) {
	// OAuth entry points issue a PM-owned session even when the compatibility
	// auth mode is shared_jwt. Verify it before consulting the external token
	// introspector so both browser entries converge to one local principal.
	if identity, err := s.tokens.VerifySession(raw); err == nil {
		return identity, nil
	}
	if s.cfg.AuthMode == "local" || s.cfg.AuthMode == "hybrid" {
		if identity, err := s.tokens.VerifyLocal(raw); err == nil {
			return identity, nil
		}
	}
	if s.cfg.AuthMode != "shared_jwt" && s.cfg.AuthMode != "hybrid" {
		return pmauth.Identity{}, errSessionExpired
	}
	if s.cfg.SharedIntrospectionURL != "" {
		identity, err := s.introspectToken(c.Request.Context(), raw)
		if err == nil {
			return identity, nil
		}
		if !s.cfg.SharedDevSecretFallback {
			return pmauth.Identity{}, err
		}
	}
	identity, err := s.tokens.VerifySharedFallback(raw)
	if err != nil {
		return pmauth.Identity{}, errSessionExpired
	}
	return identity, nil
}

func (s *Server) introspectToken(
	ctx context.Context,
	raw string,
) (pmauth.Identity, error) {
	key := tokenCacheKey(raw)
	if identity, ok := s.cachedIntrospection(key); ok {
		return identity, nil
	}
	timeout := s.cfg.SharedIntrospectionTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, s.cfg.SharedIntrospectionURL, nil)
	if err != nil {
		return pmauth.Identity{}, errAuthUpstreamUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+raw)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return pmauth.Identity{}, errAuthUpstreamUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return pmauth.Identity{}, errSessionExpired
	}
	if response.StatusCode != http.StatusOK {
		return pmauth.Identity{}, errAuthUpstreamUnavailable
	}
	var envelope struct {
		Data struct {
			ID          uint   `json:"id"`
			Issuer      string `json:"issuer"`
			Subject     string `json:"subject"`
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		} `json:"data"`
	}
	if json.NewDecoder(response.Body).Decode(&envelope) != nil || envelope.Data.ID == 0 {
		return pmauth.Identity{}, errIdentityNotProvisioned
	}
	issuer := s.sharedIdentityIssuer()
	if advertised := strings.TrimSpace(envelope.Data.Issuer); advertised != "" && advertised != issuer {
		return pmauth.Identity{}, errIdentityNotProvisioned
	}
	subject := strings.TrimSpace(envelope.Data.Subject)
	if subject == "" {
		subject = sharedIdentitySubject(issuer, envelope.Data.ID)
	}
	identity := pmauth.Identity{
		UserID: envelope.Data.ID, Issuer: issuer, Subject: subject,
		Username: envelope.Data.Username, DisplayName: envelope.Data.DisplayName,
		Email: envelope.Data.Email, Source: "shared_jwt",
	}
	s.storeIntrospection(key, identity)
	return identity, nil
}

func (s *Server) sharedIdentityIssuer() string {
	if value := strings.TrimSpace(s.cfg.SharedJWTIssuer); value != "" {
		return value
	}
	return strings.TrimSpace(s.cfg.SharedIntrospectionURL)
}

func sharedIdentitySubject(issuer string, externalUserID uint) string {
	return strings.TrimSpace(issuer) + "|" + strconv.FormatUint(uint64(externalUserID), 10)
}

func tokenCacheKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Server) cachedIntrospection(key string) (pmauth.Identity, bool) {
	s.authCacheMu.Lock()
	defer s.authCacheMu.Unlock()
	row, ok := s.authCache[key]
	if !ok || time.Now().After(row.ExpiresAt) {
		delete(s.authCache, key)
		return pmauth.Identity{}, false
	}
	return row.Identity, true
}

func (s *Server) storeIntrospection(key string, identity pmauth.Identity) {
	ttl := s.cfg.SharedIntrospectionTTL
	if ttl <= 0 || ttl > 30*time.Second {
		ttl = 30 * time.Second
	}
	s.authCacheMu.Lock()
	s.authCache[key] = cachedIdentity{Identity: identity, ExpiresAt: time.Now().Add(ttl)}
	s.authCacheMu.Unlock()
}
