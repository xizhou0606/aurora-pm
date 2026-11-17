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

	"golang.org/x/crypto/bcrypt"
)

const defaultPasswordResetTTL = 30 * time.Minute

var (
	ErrPasswordResetInvalid                 = errors.New("password reset link is invalid or expired")
	ErrPasswordResetScope                   = errors.New("password reset notification requires a workspace")
	ErrPasswordResetNotificationUnavailable = errors.New("password reset notification is unavailable")
)

type PasswordResetRequest struct {
	TokenHash    string
	ActorUserID  uint
	TargetUserID uint
	WorkspaceID  uint
	Link         string
	Notify       bool
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type PasswordResetLink struct {
	URL                string    `json:"url"`
	ExpiresAt          time.Time `json:"expires_at"`
	NotificationQueued bool      `json:"notification_queued"`
}

type PasswordResetRepository interface {
	CreatePasswordReset(context.Context, PasswordResetRequest) error
	CompletePasswordReset(context.Context, string, string, time.Time) error
}

type PasswordResetOptions struct {
	PublicBaseURL string
	TTL           time.Duration
	Now           func() time.Time
	Random        io.Reader
}

type PasswordResetService struct {
	repo          PasswordResetRepository
	publicBaseURL string
	ttl           time.Duration
	now           func() time.Time
	random        io.Reader
}

func NewPasswordResetService(repo PasswordResetRepository, options PasswordResetOptions) *PasswordResetService {
	if options.TTL <= 0 {
		options.TTL = defaultPasswordResetTTL
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	return &PasswordResetService{
		repo: repo, publicBaseURL: strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/"),
		ttl: options.TTL, now: options.Now, random: options.Random,
	}
}

func (service *PasswordResetService) Create(
	ctx context.Context,
	actorUserID, targetUserID, workspaceID uint,
	notify bool,
) (PasswordResetLink, error) {
	if service == nil || service.repo == nil || actorUserID == 0 || targetUserID == 0 || service.publicBaseURL == "" {
		return PasswordResetLink{}, ErrPasswordResetInvalid
	}
	if notify && workspaceID == 0 {
		return PasswordResetLink{}, ErrPasswordResetScope
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(service.random, raw); err != nil {
		return PasswordResetLink{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	now := service.now().UTC()
	link := service.publicBaseURL + "/reset-password#token=" + url.QueryEscape(token)
	request := PasswordResetRequest{
		TokenHash: hex.EncodeToString(sum[:]), ActorUserID: actorUserID, TargetUserID: targetUserID,
		WorkspaceID: workspaceID, Link: link, Notify: notify, CreatedAt: now, ExpiresAt: now.Add(service.ttl),
	}
	if err := service.repo.CreatePasswordReset(ctx, request); err != nil {
		return PasswordResetLink{}, err
	}
	return PasswordResetLink{URL: link, ExpiresAt: request.ExpiresAt, NotificationQueued: notify}, nil
}

func (service *PasswordResetService) Complete(ctx context.Context, token, password, confirmation string) error {
	if service == nil || service.repo == nil {
		return ErrPasswordResetInvalid
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 256 {
		return ErrPasswordResetInvalid
	}
	if password != confirmation {
		return ErrIdentityPasswordMismatch
	}
	if len([]byte(password)) < 12 || len([]byte(password)) > 72 {
		return ErrIdentityPasswordInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(token))
	return service.repo.CompletePasswordReset(ctx, hex.EncodeToString(sum[:]), string(hash), service.now().UTC())
}
