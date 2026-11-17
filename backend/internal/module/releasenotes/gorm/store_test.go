package gorm

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	releasenotes "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes"
	"gorm.io/gorm"
)

func newReleaseNotesDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "release_notes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ReleaseNote{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestUpsertByVersionIsIdempotentForSameSHA(t *testing.T) {
	store := NewStore(newReleaseNotesDB(t))
	ctx := context.Background()
	first, err := store.UpsertByVersion(ctx, releasenotes.Note{
		Version: "1.2.3", ReleaseSHA: "abc123", ContentMd: "# notes", Source: "deploy",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.UpsertByVersion(ctx, releasenotes.Note{
		Version: "1.2.3", ReleaseSHA: "abc123", ContentMd: "# notes rewritten", Source: "deploy",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 幂等返回原始行：重写内容不生效，created_at 保持不变。
	if second.Version != first.Version || second.ContentMd != first.ContentMd ||
		second.CreatedAt != first.CreatedAt {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	var count int64
	if err := store.db.Model(&ReleaseNote{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count=%d, want 1", count)
	}
}

func TestUpsertByVersionConflictsOnDifferentSHA(t *testing.T) {
	store := NewStore(newReleaseNotesDB(t))
	ctx := context.Background()
	if _, err := store.UpsertByVersion(ctx, releasenotes.Note{
		Version: "1.2.3", ReleaseSHA: "abc123", ContentMd: "# notes",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := store.UpsertByVersion(ctx, releasenotes.Note{
		Version: "1.2.3", ReleaseSHA: "def456", ContentMd: "# notes",
	})
	if err != releasenotes.ErrVersionConflict {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
	var row ReleaseNote
	if err := store.db.Where("version = ?", "1.2.3").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReleaseSHA != "abc123" {
		t.Fatalf("conflicting write must not change the row, got %+v", row)
	}
}

func TestUpsertByVersionDefaultsSourceToAutogen(t *testing.T) {
	store := NewStore(newReleaseNotesDB(t))
	note, err := store.UpsertByVersion(context.Background(), releasenotes.Note{
		Version: "2.0.0", ContentMd: "# notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.Source != "autogen" {
		t.Fatalf("source=%q, want autogen", note.Source)
	}
}

func TestLatestReturnsNewestRowOrNone(t *testing.T) {
	db := newReleaseNotesDB(t)
	store := NewStore(db)
	ctx := context.Background()
	if _, found, err := store.Latest(ctx); err != nil || found {
		t.Fatalf("empty store: found=%v err=%v, want found=false", found, err)
	}
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	rows := []ReleaseNote{
		{Version: "1.0.0", CreatedAt: base},
		{Version: "1.0.1", CreatedAt: base.Add(time.Hour)},
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	note, found, err := store.Latest(ctx)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if note.Version != "1.0.1" {
		t.Fatalf("latest=%+v, want 1.0.1", note)
	}
}

func TestListOrdersNewestFirst(t *testing.T) {
	db := newReleaseNotesDB(t)
	store := NewStore(db)
	ctx := context.Background()
	base := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	rows := []ReleaseNote{
		{Version: "1.0.0", CreatedAt: base},
		{Version: "1.1.0", CreatedAt: base.Add(time.Hour)},
		{Version: "1.0.1", CreatedAt: base.Add(30 * time.Minute)},
		// 与 1.0.1 同一时刻:并列时按 version 倒序,应排在 1.0.1 之前。
		{Version: "1.0.2", CreatedAt: base.Add(30 * time.Minute)},
	}
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	notes, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(notes))
	for _, note := range notes {
		got = append(got, note.Version)
	}
	want := []string{"1.1.0", "1.0.2", "1.0.1", "1.0.0"}
	for index, version := range want {
		if got[index] != version {
			t.Fatalf("order=%v, want %v", got, want)
		}
	}
}
