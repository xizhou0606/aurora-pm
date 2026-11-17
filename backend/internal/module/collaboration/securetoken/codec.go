// Package securetoken seals short-lived collaboration proofs into opaque tokens.
package securetoken

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	KeySize             = 32
	MaxPurposeBytes     = 128
	MaxPayloadBytes     = 16 * 1024
	MaxTokenBytes       = 24 * 1024
	MinTTL              = time.Millisecond
	MaxTTL              = 5 * time.Minute
	AllowedFutureSkew   = 30 * time.Second
	maxEnvelopeBytes    = MaxPayloadBytes + 512
	currentTokenVersion = 1
	aadPrefix           = "pm.collaboration.securetoken.v1\x00"
)

var (
	ErrInvalidKey      = errors.New("securetoken: key must contain exactly 32 bytes")
	ErrInvalidClock    = errors.New("securetoken: clock is invalid")
	ErrInvalidPurpose  = errors.New("securetoken: purpose is invalid")
	ErrInvalidPayload  = errors.New("securetoken: payload must be valid JSON")
	ErrPayloadTooLarge = errors.New("securetoken: payload exceeds the size limit")
	ErrInvalidTTL      = errors.New("securetoken: ttl is outside the allowed range")
	ErrInvalidTarget   = errors.New("securetoken: target must be a non-nil pointer")
	ErrInvalidToken    = errors.New("securetoken: token is invalid or expired")
)

type tokenEnvelope struct {
	Version   int             `json:"version"`
	Purpose   string          `json:"purpose"`
	IssuedAt  int64           `json:"issued_at"`
	ExpiresAt int64           `json:"expires_at"`
	Payload   json.RawMessage `json:"payload"`
}

type options struct {
	now func() time.Time
}

// Option configures a Codec. Callers should use the provided option constructors.
type Option func(*options) error

// WithClock supplies a clock for deterministic tests and controlled runtimes.
// The function must be safe for concurrent use.
func WithClock(now func() time.Time) Option {
	return func(settings *options) error {
		if now == nil {
			return ErrInvalidClock
		}
		settings.now = now
		return nil
	}
}

// Codec is safe for concurrent use. It retains a private copy of the key.
type Codec struct {
	key [KeySize]byte
	now func() time.Time
}

// New constructs an AES-256-GCM collaboration proof codec.
func New(key []byte, configured ...Option) (*Codec, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	settings := options{now: time.Now}
	for _, option := range configured {
		if option == nil {
			return nil, ErrInvalidClock
		}
		if err := option(&settings); err != nil {
			return nil, err
		}
	}
	if settings.now == nil {
		return nil, ErrInvalidClock
	}
	codec := &Codec{now: settings.now}
	copy(codec.key[:], key)
	return codec, nil
}

// Seal encrypts a JSON payload into a purpose-bound, short-lived base64url token.
// []byte and json.RawMessage payloads are interpreted as raw JSON.
func (codec *Codec) Seal(ctx context.Context, purpose string, payload any, ttl time.Duration) (string, error) {
	if err := contextError(ctx); err != nil {
		return "", err
	}
	if codec == nil || codec.now == nil {
		return "", ErrInvalidKey
	}
	if !validPurpose(purpose) {
		return "", ErrInvalidPurpose
	}
	if ttl < MinTTL || ttl > MaxTTL {
		return "", ErrInvalidTTL
	}
	rawPayload, err := encodePayload(payload)
	if err != nil {
		return "", err
	}
	now := codec.now().UTC()
	issuedAt := now.UnixMilli()
	expiresAt := now.Add(ttl).UnixMilli()
	if issuedAt <= 0 || expiresAt <= issuedAt {
		return "", ErrInvalidClock
	}
	envelope, err := json.Marshal(tokenEnvelope{
		Version: currentTokenVersion, Purpose: purpose, IssuedAt: issuedAt,
		ExpiresAt: expiresAt, Payload: rawPayload,
	})
	if err != nil || len(envelope) > maxEnvelopeBytes {
		return "", ErrPayloadTooLarge
	}
	aead, err := codec.newAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("securetoken: generate nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, envelope, associatedData(purpose))
	token := base64.RawURLEncoding.EncodeToString(sealed)
	if len(token) > MaxTokenBytes {
		return "", ErrPayloadTooLarge
	}
	return token, nil
}

// Open authenticates and decrypts a token into target. All token validation
// failures intentionally collapse to ErrInvalidToken. A *[]byte target receives
// a private copy of the raw JSON payload.
func (codec *Codec) Open(ctx context.Context, purpose, token string, target any) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if !validTarget(target) {
		return ErrInvalidTarget
	}
	if codec == nil || codec.now == nil || !validPurpose(purpose) || len(token) == 0 || len(token) > MaxTokenBytes {
		return ErrInvalidToken
	}
	sealed, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || base64.RawURLEncoding.EncodeToString(sealed) != token {
		return ErrInvalidToken
	}
	aead, err := codec.newAEAD()
	if err != nil || len(sealed) < aead.NonceSize()+aead.Overhead() {
		return ErrInvalidToken
	}
	nonce := sealed[:aead.NonceSize()]
	plaintext, err := aead.Open(nil, nonce, sealed[aead.NonceSize():], associatedData(purpose))
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxEnvelopeBytes {
		return ErrInvalidToken
	}
	envelope, err := decodeEnvelope(plaintext)
	if err != nil || !codec.validEnvelope(envelope, purpose) {
		return ErrInvalidToken
	}
	if err = decodePayload(envelope.Payload, target); err != nil {
		return ErrInvalidToken
	}
	return nil
}

func (codec *Codec) newAEAD() (cipher.AEAD, error) {
	block, err := aes.NewCipher(codec.key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (codec *Codec) validEnvelope(envelope tokenEnvelope, purpose string) bool {
	now := codec.now().UTC().UnixMilli()
	maximumTTL := MaxTTL.Milliseconds()
	maximumFuture := AllowedFutureSkew.Milliseconds()
	return envelope.Version == currentTokenVersion && envelope.Purpose == purpose &&
		envelope.IssuedAt > 0 && envelope.ExpiresAt > envelope.IssuedAt &&
		envelope.ExpiresAt-envelope.IssuedAt <= maximumTTL &&
		envelope.IssuedAt <= now+maximumFuture && envelope.ExpiresAt > now &&
		len(envelope.Payload) > 0 && len(envelope.Payload) <= MaxPayloadBytes && json.Valid(envelope.Payload)
}

func decodeEnvelope(raw []byte) (tokenEnvelope, error) {
	var envelope tokenEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return tokenEnvelope{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return tokenEnvelope{}, ErrInvalidToken
	}
	return envelope, nil
}

func encodePayload(payload any) (json.RawMessage, error) {
	var raw []byte
	switch value := payload.(type) {
	case []byte:
		if len(value) > MaxPayloadBytes {
			return nil, ErrPayloadTooLarge
		}
		raw = append([]byte(nil), value...)
	case json.RawMessage:
		if len(value) > MaxPayloadBytes {
			return nil, ErrPayloadTooLarge
		}
		raw = append([]byte(nil), value...)
	default:
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
		}
		raw = encoded
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, ErrInvalidPayload
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, ErrInvalidPayload
	}
	if compact.Len() > MaxPayloadBytes {
		return nil, ErrPayloadTooLarge
	}
	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if bytesTarget, ok := target.(*[]byte); ok {
		*bytesTarget = append((*bytesTarget)[:0], raw...)
		return nil
	}
	value := reflect.ValueOf(target)
	temporary := reflect.New(value.Elem().Type())
	if err := json.Unmarshal(raw, temporary.Interface()); err != nil {
		return err
	}
	value.Elem().Set(temporary.Elem())
	return nil
}

func validTarget(target any) bool {
	if target == nil {
		return false
	}
	value := reflect.ValueOf(target)
	return value.Kind() == reflect.Pointer && !value.IsNil()
}

func validPurpose(purpose string) bool {
	if len(purpose) == 0 || len(purpose) > MaxPurposeBytes || !utf8.ValidString(purpose) || strings.TrimSpace(purpose) != purpose {
		return false
	}
	for _, character := range []byte(purpose) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func associatedData(purpose string) []byte {
	return []byte(aadPrefix + purpose)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return context.Canceled
	}
	return ctx.Err()
}
