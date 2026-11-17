package workbenchgorm

import "time"

// HostNavigationPreference remains only to map the historical table during the
// rollback retention period. Online code must not read or write it; global
// access menus are the only navigation source.
type HostNavigationPreference struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"not null;uniqueIndex:ux_host_navigation_workspace_host_key;index" json:"workspace_id"`
	HostKey     string    `gorm:"size:64;not null;uniqueIndex:ux_host_navigation_workspace_host_key;index" json:"host_key"`
	MenuKey     string    `gorm:"size:64;not null;uniqueIndex:ux_host_navigation_workspace_host_key" json:"menu_key"`
	Visible     bool      `gorm:"not null" json:"visible"`
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
