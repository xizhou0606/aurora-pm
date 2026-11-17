package projectgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

func (r *AccessGuideRepository) LockGuideUnit(ctx context.Context, w, id uint) error {
	var unit PlanUnit
	return r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", w, id).First(&unit).Error
}

// Caller holds the unit lock and owns the request + membership transaction.
func (r *AccessGuideRepository) AddApprovedParticipant(ctx context.Context, s project.JoinScope, user, actor uint, reason string) error {
	if err := NewMemberRepository(r.db).VerifyMemberUsers(ctx, s.WorkspaceID, []uint{user}); err != nil {
		return project.ErrJoinWorkspaceRequired
	}
	now := time.Now().UTC()
	row := PlanUnitParticipant{WorkspaceID: s.WorkspaceID, PlanUnitID: s.PlanUnitID, UserID: user, CreatedByUserID: actor, CreatedAt: now}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Model(&PlanUnit{}).Where("workspace_id = ? AND id = ?", s.WorkspaceID, s.PlanUnitID).Update("updated_at", now).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&PlanUnitMemberMutationAudit{WorkspaceID: s.WorkspaceID, PlanUnitID: s.PlanUnitID, ActorUserID: actor, Operation: "access_request_approved", ParticipantIDs: "[" + strconv.FormatUint(uint64(user), 10) + "]", Reason: string([]rune(reason)[:min(len([]rune(reason)), 512)]), CreatedAt: now}).Error
}
