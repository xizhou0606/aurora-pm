package toolruntime

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var (
	opaqueSecretPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{43,256}$`)
	pkceVerifierPattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	pkceChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

func newOpaqueSecret(source io.Reader) (string, error) {
	if source == nil {
		source = rand.Reader
	}
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(source, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func credentialDigest(raw string) (string, bool) {
	if raw != strings.TrimSpace(raw) || !opaqueSecretPattern.MatchString(raw) {
		return "", false
	}
	value := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(value[:]), true
}

func pkceS256Challenge(verifier string) (string, bool) {
	if !pkceVerifierPattern.MatchString(verifier) {
		return "", false
	}
	value := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(value[:]), true
}

func constantTimeTextEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func normalizeRedirectURI(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", false
	}
	if parsed.Scheme == "http" {
		host := strings.ToLower(parsed.Hostname())
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return "", false
		}
	}
	return parsed.String(), true
}
