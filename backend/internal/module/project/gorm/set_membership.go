package projectgorm

import (
	"time"
)

// ProjectSetMembership is retained for schema compatibility only; it no longer grants access.
type ProjectSetMembership struct {
	ID              uint      `gorm:"primaryKey"`
	WorkspaceID     uint      `gorm:"not null;uniqueIndex:ux_project_set_memberships_scope,priority:1"`
	ProjectSetID    uint      `gorm:"not null;uniqueIndex:ux_project_set_memberships_scope,priority:2"`
	UserID          uint      `gorm:"not null;uniqueIndex:ux_project_set_memberships_scope,priority:3"`
	CreatedByUserID uint      `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (ProjectSetMembership) TableName() string { return "project_set_memberships" }
