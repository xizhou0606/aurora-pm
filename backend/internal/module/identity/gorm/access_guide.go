package identitygorm

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"strings"
)

type AccessGuideDirectory struct{ db *gorm.DB }

func NewAccessGuideDirectory(db *gorm.DB) *AccessGuideDirectory { return &AccessGuideDirectory{db: db} }
func (r *AccessGuideDirectory) GuideWorkspaceName(ctx context.Context, id uint) (string, error) {
	var row Workspace
	err := r.db.WithContext(ctx).Select("name").Where("id = ? AND enabled = ?", id, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return row.Name, err
}
func (r *AccessGuideDirectory) GuideOwnerName(ctx context.Context, id uint) (string, error) {
	if id == 0 {
		return "", nil
	}
	var row User
	err := r.db.WithContext(ctx).Select("display_name", "username").Where("active = ? AND id = ?", true, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	name := strings.TrimSpace(row.DisplayName)
	if name == "" {
		name = row.Username
	}
	return name, err
}
