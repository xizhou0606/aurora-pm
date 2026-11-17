package database

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
)

type legacyQualityBug struct {
	ID          uint `gorm:"primaryKey"`
	WorkspaceID uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (legacyQualityBug) TableName() string {
	return "quality_bugs"
}

func TestPrepareQualityStatusChangedAtBackfillsLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:quality-status-migration?mode=memory&cache=shared"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&legacyQualityBug{}, &qualitygorm.QualityBugActivity{}); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.August, 1, 8, 30, 0, 0, time.UTC)
	bug := legacyQualityBug{WorkspaceID: 7, CreatedAt: createdAt}
	if err := db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Transaction(prepareQualityStatusChangedAt); err != nil {
		t.Fatal(err)
	}

	var changedAt time.Time
	if err := db.Table("quality_bugs").Select("status_changed_at").
		Where("id = ?", bug.ID).Scan(&changedAt).Error; err != nil {
		t.Fatal(err)
	}
	if !changedAt.Equal(createdAt) {
		t.Fatalf("status_changed_at=%s, want %s", changedAt, createdAt)
	}
}
