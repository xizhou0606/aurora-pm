package projectgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func (r *PlanUnitMemberRepository) ListPlanUnitMemberTasks(ctx context.Context, ws, unit, user uint) ([]task.MemberTask, error) {
	if r.taskHandover == nil {
		return nil, task.ErrBatchMutationUnavailable
	}
	return r.taskHandover(r.db).ListMemberTasks(ctx, ws, unit, []uint{user})
}

func (r *PlanUnitMemberRepository) resolveMemberRemoval(ctx context.Context, tx *gorm.DB, ws, unit uint, current, next project.MemberState, meta project.MemberMutationMeta) error {
	removed := project.RemovedMemberIDs(current, next)
	if meta.TaskResolution != nil {
		if r.taskHandover == nil {
			return task.ErrBatchMutationUnavailable
		}
		if id := meta.TaskResolution.AssigneeID; id != nil {
			retained := *id == *next.OwnerUserID
			for _, participant := range next.ParticipantUserIDs {
				retained = retained || participant == *id
			}
			if !retained {
				return project.ErrParticipantInvalid
			}
		}
		if err := r.taskHandover(tx).ResolveMemberTasks(ctx, ws, unit, meta.ActorUserID, removed, *meta.TaskResolution); err != nil {
			return err
		}
	}
	inspector := NewRemovalImpactRepository(tx)
	for _, user := range removed {
		impact, err := inspector.InspectPlanUnitRemovalImpact(ctx, ws, unit, user)
		if err != nil {
			return err
		}
		if impact.HasOutstanding() {
			return &project.MemberRemovalBlocked{UserID: user, Impact: impact}
		}
	}
	return nil
}
