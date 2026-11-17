package securetoken_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/securetoken"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

type claims struct {
	ResourceID     string `json:"resource_id"`
	PolicyRevision uint64 `json:"policy_revision"`
}

func TestCodecRoundTripGenericAndRawJSON(t *testing.T) {
	clock := fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC))
	codec := mustCodec(t, clock)

	token, err := codec.Seal(context.Background(), "validation-receipt", claims{
		ResourceID: "doc-secret-resource", PolicyRevision: 42,
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, "doc-secret-resource") || strings.Contains(token, "validation-receipt") {
		t.Fatalf("token disclosed claims: %q", token)
	}
	sealed, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("doc-secret-resource")) || bytes.Contains(sealed, []byte("validation-receipt")) {
		t.Fatal("decoded token bytes disclosed encrypted claims")
	}
	var decoded claims
	if err = codec.Open(context.Background(), "validation-receipt", token, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ResourceID != "doc-secret-resource" || decoded.PolicyRevision != 42 {
		t.Fatalf("decoded = %#v", decoded)
	}

	rawInput := []byte(" { \"policy_revision\" : 43, \"resource_id\" : \"raw-doc\" } ")
	rawToken, err := codec.Seal(context.Background(), "validation-receipt", rawInput, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var rawOutput []byte
	if err = codec.Open(context.Background(), "validation-receipt", rawToken, &rawOutput); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(rawOutput) || bytes.Contains(rawOutput, []byte(" ")) {
		t.Fatalf("raw output was not compact JSON: %q", rawOutput)
	}
}

func TestCodecRejectsTamperingAndWrongPurposeWithoutMutatingTarget(t *testing.T) {
	codec := mustCodec(t, fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
	token, err := codec.Seal(context.Background(), "connection-grant", claims{ResourceID: "doc-1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 0x01
	tampered := base64.RawURLEncoding.EncodeToString(sealed)
	for name, candidate := range map[string]struct {
		purpose string
		token   string
	}{
		"tampered":      {purpose: "connection-grant", token: tampered},
		"wrong purpose": {purpose: "validation-receipt", token: token},
	} {
		t.Run(name, func(t *testing.T) {
			target := claims{ResourceID: "unchanged", PolicyRevision: 99}
			err := codec.Open(context.Background(), candidate.purpose, candidate.token, &target)
			if !errors.Is(err, securetoken.ErrInvalidToken) {
				t.Fatalf("error = %v", err)
			}
			if target.ResourceID != "unchanged" || target.PolicyRevision != 99 {
				t.Fatalf("target was mutated: %#v", target)
			}
		})
	}
}

func TestCodecRejectsExpiredAndFutureIssuedTokens(t *testing.T) {
	issuedAt := time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)
	issuer := mustCodec(t, fixedClock(issuedAt))
	token, err := issuer.Seal(context.Background(), "connection-grant", claims{ResourceID: "doc-1"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	expiredReader := mustCodec(t, fixedClock(issuedAt.Add(time.Second)))
	if err = expiredReader.Open(context.Background(), "connection-grant", token, &claims{}); !errors.Is(err, securetoken.ErrInvalidToken) {
		t.Fatalf("expired error = %v", err)
	}

	futureIssuer := mustCodec(t, fixedClock(issuedAt.Add(securetoken.AllowedFutureSkew+time.Millisecond)))
	futureToken, err := futureIssuer.Seal(context.Background(), "connection-grant", claims{ResourceID: "doc-1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	reader := mustCodec(t, fixedClock(issuedAt))
	if err = reader.Open(context.Background(), "connection-grant", futureToken, &claims{}); !errors.Is(err, securetoken.ErrInvalidToken) {
		t.Fatalf("future-issued error = %v", err)
	}
}

func TestCodecEnforcesKeyTTLJSONAndSizeBounds(t *testing.T) {
	if _, err := securetoken.New(make([]byte, securetoken.KeySize-1)); !errors.Is(err, securetoken.ErrInvalidKey) {
		t.Fatalf("short key error = %v", err)
	}
	codec := mustCodec(t, fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
	if _, err := codec.Seal(context.Background(), "proof", claims{}, securetoken.MinTTL-time.Nanosecond); !errors.Is(err, securetoken.ErrInvalidTTL) {
		t.Fatalf("minimum ttl error = %v", err)
	}
	if _, err := codec.Seal(context.Background(), "proof", claims{}, securetoken.MaxTTL+time.Millisecond); !errors.Is(err, securetoken.ErrInvalidTTL) {
		t.Fatalf("ttl error = %v", err)
	}
	if _, err := codec.Seal(context.Background(), "proof", []byte(`{"broken":`), time.Minute); !errors.Is(err, securetoken.ErrInvalidPayload) {
		t.Fatalf("json error = %v", err)
	}
	oversized := []byte(`"` + strings.Repeat("x", securetoken.MaxPayloadBytes) + `"`)
	if _, err := codec.Seal(context.Background(), "proof", oversized, time.Minute); !errors.Is(err, securetoken.ErrPayloadTooLarge) {
		t.Fatalf("payload size error = %v", err)
	}
	if err := codec.Open(context.Background(), "proof", strings.Repeat("a", securetoken.MaxTokenBytes+1), &claims{}); !errors.Is(err, securetoken.ErrInvalidToken) {
		t.Fatalf("token size error = %v", err)
	}
	if err := codec.Open(context.Background(), "proof", "invalid=padding", &claims{}); !errors.Is(err, securetoken.ErrInvalidToken) {
		t.Fatalf("base64 error = %v", err)
	}
}

func TestCodecCopiesKeyMaterial(t *testing.T) {
	key := append([]byte(nil), testKey...)
	codec, err := securetoken.New(key, securetoken.WithClock(fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC))))
	if err != nil {
		t.Fatal(err)
	}
	token, err := codec.Seal(context.Background(), "proof", claims{ResourceID: "doc-1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for index := range key {
		key[index] = 0
	}
	var decoded claims
	if err = codec.Open(context.Background(), "proof", token, &decoded); err != nil || decoded.ResourceID != "doc-1" {
		t.Fatalf("caller key mutation affected codec: %#v, %v", decoded, err)
	}
}

func TestCodecUsesRandomNonces(t *testing.T) {
	codec := mustCodec(t, fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
	first, err := codec.Seal(context.Background(), "proof", claims{ResourceID: "same"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codec.Seal(context.Background(), "proof", claims{ResourceID: "same"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("identical claims reused a nonce")
	}
}

func TestCodecIsConcurrentSafe(t *testing.T) {
	codec := mustCodec(t, fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
	var group sync.WaitGroup
	var failures atomic.Int64
	for worker := 0; worker < 32; worker++ {
		worker := worker
		group.Add(1)
		go func() {
			defer group.Done()
			for sequence := 0; sequence < 40; sequence++ {
				expected := claims{ResourceID: fmt.Sprintf("doc-%d-%d", worker, sequence), PolicyRevision: uint64(sequence + 1)}
				token, err := codec.Seal(context.Background(), "concurrent-proof", expected, time.Minute)
				if err != nil {
					failures.Add(1)
					continue
				}
				var actual claims
				if err = codec.Open(context.Background(), "concurrent-proof", token, &actual); err != nil || actual != expected {
					failures.Add(1)
				}
			}
		}()
	}
	group.Wait()
	if count := failures.Load(); count != 0 {
		t.Fatalf("concurrent failures = %d", count)
	}
}

func TestCodecRejectsInvalidTargetAndCanceledContext(t *testing.T) {
	codec := mustCodec(t, fixedClock(time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)))
	if err := codec.Open(context.Background(), "proof", "token", claims{}); !errors.Is(err, securetoken.ErrInvalidTarget) {
		t.Fatalf("target error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := codec.Seal(ctx, "proof", claims{}, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("seal context error = %v", err)
	}
	if err := codec.Open(ctx, "proof", "token", &claims{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("open context error = %v", err)
	}
}

func mustCodec(t *testing.T, clock func() time.Time) *securetoken.Codec {
	t.Helper()
	codec, err := securetoken.New(testKey, securetoken.WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}
