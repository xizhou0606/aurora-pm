package projectgorm

import "time"

type ProjectPermission struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID  uint      `gorm:"not null;uniqueIndex:ux_project_user;index" json:"workspace_id"`
	TeamID       uint      `gorm:"not null;index" json:"team_id"`
	ProjectSetID uint      `gorm:"column:project_new_id;not null;uniqueIndex:ux_project_user;index" json:"project_new_id"`
	UserID       uint      `gorm:"not null;uniqueIndex:ux_project_user;index" json:"user_id"`
	AccessLevel  string    `gorm:"size:30;not null;default:member" json:"access_level"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ProjectMemberRole struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID         uint      `gorm:"not null;index" json:"workspace_id"`
	ProjectPermissionID uint      `gorm:"not null;index" json:"project_permission_id"`
	RoleKey             string    `gorm:"size:80;not null" json:"role_key"`
	ScopeType           string    `gorm:"size:40;not null;default:project" json:"scope_type"`
	ScopeID             uint      `gorm:"not null" json:"scope_id"`
	SourceType          string    `gorm:"size:60" json:"source_type,omitempty"`
	SourceID            uint      `json:"source_id,omitempty"`
	SourceField         string    `gorm:"size:80" json:"source_field,omitempty"`
	SourceLabel         string    `gorm:"size:160" json:"source_label,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
