package auth

import (
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestPMSessionDoesNotCarryAuthorizationClaims(t *testing.T) {
	tokens := NewTokens(config.Config{LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters", TokenTTL: time.Hour})
	raw, err := tokens.SignSession(9, "alice", "Alice", "alice@example.test")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := tokens.VerifySession(raw)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Source != "pm_session" || identity.Issuer != PMSessionIssuer || identity.Subject != "user:9" || identity.UserID != 9 ||
		identity.Username != "alice" || identity.Role != "" || identity.WorkspaceID != 0 {
		t.Fatalf("identity=%+v", identity)
	}
}

func TestPMSessionIsAcceptedWhenLegacyLocalModeIsDisabled(t *testing.T) {
	tokens := NewTokens(config.Config{AuthMode: "shared_jwt", LocalJWTSecret: "test-secret-that-is-at-least-thirty-two-characters", TokenTTL: time.Hour})
	raw, err := tokens.SignSession(9, "alice", "Alice", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.VerifySession(raw); err != nil {
		t.Fatalf("PM session verification=%v", err)
	}
	if _, err := tokens.VerifyLocal(raw); err == nil {
		t.Fatal("PM session was accepted as legacy local token")
	}
}
