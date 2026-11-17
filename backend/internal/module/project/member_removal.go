package project

import (
	"context"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

type MemberRemovalBlocked struct {
	UserID uint
	Impact RemovalImpact
}

func (e *MemberRemovalBlocked) Error() string {
	return fmt.Sprintf("participant %d has outstanding responsibilities: %s", e.UserID, e.Impact.Summary())
}

// RemovedMemberIDs compares effective membership, including ownership.
func RemovedMemberIDs(current, next MemberState) []uint {
	retained := map[uint]bool{}
	for _, id := range next.ParticipantUserIDs {
		retained[id] = true
	}
	if next.OwnerUserID != nil {
		retained[*next.OwnerUserID] = true
	}
	previous := append([]uint{}, current.ParticipantUserIDs...)
	if current.OwnerUserID != nil {
		previous = append(previous, *current.OwnerUserID)
	}
	removed := []uint{}
	seen := map[uint]bool{}
	for _, id := range previous {
		if !retained[id] && !seen[id] {
			removed = append(removed, id)
			seen[id] = true
		}
	}
	return removed
}

type MemberTaskReader interface {
	ListPlanUnitMemberTasks(context.Context, uint, uint, uint) ([]task.MemberTask, error)
}
