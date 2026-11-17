package projectgorm

import "time"

// PlanUnitParticipant is the version-level access grant beside the unit owner
// fact on plan_units. workspace_id is a mandatory tenant fact: every query
// matches both the unit and its workspace so participants can never leak
// across workspaces.
type PlanUnitParticipant struct {
	ID              uint      `gorm:"primaryKey"`
	WorkspaceID     uint      `gorm:"not null;index:idx_plan_unit_participants_workspace"`
	PlanUnitID      uint      `gorm:"not null;index:idx_plan_unit_participants_unit;uniqueIndex:ux_plan_unit_participants_unit_user,priority:1"`
	UserID          uint      `gorm:"not null;index:idx_plan_unit_participants_user;uniqueIndex:ux_plan_unit_participants_unit_user,priority:2"`
	CreatedByUserID uint      `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (PlanUnitParticipant) TableName() string { return "plan_unit_participants" }

// PlanUnitMemberMutationAudit records every atomic plan unit owner or
// participant change. Written in the same transaction as the member change
// for consistent auditability. Owned by the project module (no cross-module
// import).
type PlanUnitMemberMutationAudit struct {
	ID             uint   `gorm:"primaryKey"`
	WorkspaceID    uint   `gorm:"not null;index:idx_plan_unit_member_audit_workspace"`
	PlanUnitID     uint   `gorm:"not null;index:idx_plan_unit_member_audit_unit"`
	ActorUserID    uint   `gorm:"not null"`
	Operation      string `gorm:"size:32;not null"`
	OwnerUserID    *uint
	ParticipantIDs string    `gorm:"type:text;not null;default:''"`
	Reason         string    `gorm:"size:512;not null;default:''"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (PlanUnitMemberMutationAudit) TableName() string { return "plan_unit_member_mutation_audits" }

// PlanUnitMemberMigrationRecords keeps SQLite tests and first-time empty
// Postgres installs aligned with the versioned SQL schema.
func PlanUnitMemberMigrationRecords() []any {
	return []any{&ProjectSetMembership{}, &PlanUnitParticipant{}, &PlanUnitMemberMutationAudit{}}
}
