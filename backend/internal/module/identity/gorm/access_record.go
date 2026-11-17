package identitygorm

import "time"

// AccessRole, AccessUserRole, and the grant records are global RBAC data. No
// row carries a workspace, team, project, ownership, or assignee dimension.
type AccessRole struct {
	ID            uint   `gorm:"primaryKey"`
	Code          string `gorm:"size:96;not null;uniqueIndex:ux_access_role_code"`
	Name          string `gorm:"size:128;not null"`
	Description   string `gorm:"size:1000"`
	Enabled       bool   `gorm:"not null;default:true;index"`
	SystemManaged bool   `gorm:"not null;default:false"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AccessUserRole struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"not null;index;uniqueIndex:ux_access_user_role,priority:1"`
	RoleID    uint `gorm:"not null;index;uniqueIndex:ux_access_user_role,priority:2"`
	CreatedAt time.Time
}

type AccessServiceInterface struct {
	ID            uint   `gorm:"primaryKey"`
	Code          string `gorm:"size:96;not null;uniqueIndex:ux_access_service_interface_code"`
	ActionCode    string `gorm:"size:96;not null;default:'';index"`
	HTTPMethod    string `gorm:"size:10;not null;uniqueIndex:ux_access_service_interface_route,priority:1"`
	RouteTemplate string `gorm:"size:512;not null;uniqueIndex:ux_access_service_interface_route,priority:2"`
	Name          string `gorm:"size:128;not null"`
	Description   string `gorm:"size:1000"`
	Enabled       bool   `gorm:"not null;default:true;index"`
	SystemManaged bool   `gorm:"not null;default:false"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AccessRoleServiceInterface struct {
	ID                 uint `gorm:"primaryKey"`
	RoleID             uint `gorm:"not null;index;uniqueIndex:ux_access_role_service_interface,priority:1"`
	ServiceInterfaceID uint `gorm:"not null;index;uniqueIndex:ux_access_role_service_interface,priority:2"`
	CreatedAt          time.Time
}

type AccessGroupServiceInterface struct {
	ID                 uint `gorm:"primaryKey"`
	GroupID            uint `gorm:"not null;index;uniqueIndex:ux_access_group_service_interface,priority:1"`
	ServiceInterfaceID uint `gorm:"not null;index;uniqueIndex:ux_access_group_service_interface,priority:2"`
	CreatedAt          time.Time
}

// AccessRoleMenu belongs to identity because it is a role grant. menu_id is a
// stable reference owned and validated by the workbench module.
type AccessRoleMenu struct {
	ID        uint `gorm:"primaryKey"`
	RoleID    uint `gorm:"not null;index;uniqueIndex:ux_access_role_menu,priority:1"`
	MenuID    uint `gorm:"not null;index;uniqueIndex:ux_access_role_menu,priority:2"`
	CreatedAt time.Time
}

type AccessGroupMenu struct {
	ID        uint `gorm:"primaryKey"`
	GroupID   uint `gorm:"not null;index;uniqueIndex:ux_access_group_menu,priority:1"`
	MenuID    uint `gorm:"not null;index;uniqueIndex:ux_access_group_menu,priority:2"`
	CreatedAt time.Time
}
