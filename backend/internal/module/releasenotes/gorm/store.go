// Package gorm implements the releasenotes store on GORM.
package gorm

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	releasenotes "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes"
)

// ReleaseNote is the persistent release-notes row (release_notes table,
// backend/migrations/20260909100000_add_release_notes).
type ReleaseNote struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	Version    string    `gorm:"column:version"`
	ReleaseSHA string    `gorm:"column:release_sha"`
	ContentMd  string    `gorm:"column:content_md"`
	Source     string    `gorm:"column:source"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (ReleaseNote) TableName() string { return "release_notes" }

// Store implements releasenotes.Store on GORM.
type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// UpsertByVersion is idempotent for the same version + release SHA and fails
// with ErrVersionConflict when the version is rewritten with another SHA.
func (store *Store) UpsertByVersion(ctx context.Context, note releasenotes.Note) (releasenotes.Note, error) {
	var existing ReleaseNote
	err := store.db.WithContext(ctx).Where("version = ?", note.Version).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		source := note.Source
		if strings.TrimSpace(source) == "" {
			// Keep the migration column default ('autogen') meaningful even
			// though GORM inserts the column explicitly.
			source = "autogen"
		}
		row := ReleaseNote{
			Version: note.Version, ReleaseSHA: note.ReleaseSHA,
			ContentMd: note.ContentMd, Source: source, CreatedAt: time.Now().UTC(),
		}
		if err := store.db.WithContext(ctx).Create(&row).Error; err != nil {
			return releasenotes.Note{}, err
		}
		return noteFromRow(row), nil
	}
	if err != nil {
		return releasenotes.Note{}, err
	}
	if existing.ReleaseSHA != note.ReleaseSHA {
		return releasenotes.Note{}, releasenotes.ErrVersionConflict
	}
	return noteFromRow(existing), nil
}

// Latest returns the newest note (created_at DESC, version DESC); found is
// false when the table has no rows yet.
func (store *Store) Latest(ctx context.Context) (releasenotes.Note, bool, error) {
	var row ReleaseNote
	err := store.db.WithContext(ctx).
		Order("created_at DESC, version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return releasenotes.Note{}, false, nil
	}
	if err != nil {
		return releasenotes.Note{}, false, err
	}
	return noteFromRow(row), true, nil
}

// List returns every note newest first (created_at DESC, version DESC).
func (store *Store) List(ctx context.Context) ([]releasenotes.Note, error) {
	var rows []ReleaseNote
	if err := store.db.WithContext(ctx).
		Order("created_at DESC, version DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]releasenotes.Note, 0, len(rows))
	for _, row := range rows {
		result = append(result, noteFromRow(row))
	}
	return result, nil
}

func noteFromRow(row ReleaseNote) releasenotes.Note {
	return releasenotes.Note{
		Version: row.Version, ReleaseSHA: row.ReleaseSHA,
		ContentMd: row.ContentMd, Source: row.Source, CreatedAt: row.CreatedAt,
	}
}
