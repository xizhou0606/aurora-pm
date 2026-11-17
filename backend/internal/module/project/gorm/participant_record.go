package projectgorm

import "time"

// ProjectMemberMutationAudit records historical project-level owner or
// participant changes (retired member model). Kept for audit history only;
// new member mutations write plan_unit_member_mutation_audits.
type ProjectMemberMutationAudit struct {
	ID             uint   `gorm:"primaryKey"`
	WorkspaceID    uint   `gorm:"not null;index:idx_member_audit_workspace"`
	ProjectSetID   uint   `gorm:"not null;index:idx_member_audit_project"`
	ActorUserID    uint   `gorm:"not null"`
	Operation      string `gorm:"size:32;not null"`
	OwnerUserID    *uint
	ParticipantIDs string    `gorm:"type:text;not null;default:''"`
	Reason         string    `gorm:"size:512;not null;default:''"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (ProjectMemberMutationAudit) TableName() string { return "project_member_mutation_audits" }

// MemberMutationAuditMigrationRecords keeps SQLite tests aligned.
func MemberMutationAuditMigrationRecords() []any {
	return []any{&ProjectMemberMutationAudit{}}
}
