package identitygorm

import "time"

type AccessUserGroup struct {
	ID            uint   `gorm:"primaryKey"`
	Code          string `gorm:"size:96;not null;uniqueIndex:ux_access_user_group_code"`
	Name          string `gorm:"size:128;not null"`
	Description   string `gorm:"size:1000"`
	Enabled       bool   `gorm:"not null;default:true;index"`
	SystemManaged bool   `gorm:"not null;default:false"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AccessUserGroupBinding deliberately has a unique user_id: a PM user belongs
// to exactly one group while AccessUserRole remains many-to-many.
type AccessUserGroupBinding struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"not null;uniqueIndex:ux_access_user_group_binding_user"`
	GroupID   uint `gorm:"not null;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PersonnelAuditEvent is append-only. No repository update/delete method is
// exposed; each personnel mutation inserts its event in the same transaction.
type PersonnelAuditEvent struct {
	ID             uint      `gorm:"primaryKey"`
	EventType      string    `gorm:"size:64;not null;index"`
	ActorUserID    uint      `gorm:"not null;index"`
	SubjectUserID  *uint     `gorm:"index"`
	SubjectGroupID *uint     `gorm:"index"`
	SubjectRoleID  *uint     `gorm:"index"`
	BeforeJSON     string    `gorm:"type:text"`
	AfterJSON      string    `gorm:"type:text"`
	Reason         string    `gorm:"size:1000"`
	CreatedAt      time.Time `gorm:"index;not null"`
}
