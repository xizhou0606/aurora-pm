// Package releasenotes owns version release notes (发版说明): server-side
// generated and stored version announcements read by the public
// /release-notes page endpoint. Writes arrive from the deployment pipeline
// over the internal HMAC-authenticated /internal/release-notes route.
package releasenotes

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Note is one stored version announcement.
type Note struct {
	Version    string
	ReleaseSHA string
	ContentMd  string
	Source     string
	CreatedAt  time.Time
}

// Store persists release notes keyed by their version.
type Store interface {
	// UpsertByVersion stores one note. Writing the same version with the same
	// release SHA is idempotent and returns the stored row untouched; writing
	// the same version with a different release SHA fails with
	// ErrVersionConflict.
	UpsertByVersion(ctx context.Context, note Note) (Note, error)
	// Latest returns the newest note (created_at DESC, version DESC); found
	// is false when nothing has been published yet.
	Latest(ctx context.Context) (Note, bool, error)
	// List returns every note newest first (created_at DESC, version DESC).
	List(ctx context.Context) ([]Note, error)
}

var (
	// ErrVersionConflict is returned when an existing version is rewritten
	// with a different release SHA.
	ErrVersionConflict = errors.New("release note version already exists with a different release sha")
	// ErrInvalidInput is returned when version or content markdown is empty.
	ErrInvalidInput = errors.New("release note version and content are required")
)

// PublishInput is one deployment write.
type PublishInput struct {
	Version    string
	ReleaseSHA string
	ContentMd  string
	Source     string
}

// Service validates release-note writes and reads through the store.
type Service struct {
	store Store
}

func NewService(store Store) *Service { return &Service{store: store} }

// Publish validates and stores one release note.
func (s *Service) Publish(ctx context.Context, input PublishInput) (Note, error) {
	version := strings.TrimSpace(input.Version)
	content := strings.TrimSpace(input.ContentMd)
	if version == "" || content == "" {
		return Note{}, ErrInvalidInput
	}
	return s.store.UpsertByVersion(ctx, Note{
		Version:    version,
		ReleaseSHA: strings.TrimSpace(input.ReleaseSHA),
		ContentMd:  input.ContentMd,
		Source:     strings.TrimSpace(input.Source),
	})
}

// Latest returns the newest release note; found is false when nothing has
// been published yet. The deployment launcher reads this to anchor the next
// announcement on what pm-api already stores.
func (s *Service) Latest(ctx context.Context) (Note, bool, error) {
	return s.store.Latest(ctx)
}

// List returns every release note newest first.
func (s *Service) List(ctx context.Context) ([]Note, error) {
	return s.store.List(ctx)
}
