package httpapi

import (
	"time"

	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

const (
	biTaskDueSourceSelf   = "self"
	biTaskDueSourceParent = "parent"
)

type biTeamTaskDueFact struct {
	dueAt        *time.Time
	source       string
	sourceTaskID uint
}

// biTeamTaskDueFacts resolves the deadline used by BI without mutating task data.
// A task's own deadline wins; otherwise the nearest ancestor with a deadline is used.
func biTeamTaskDueFacts(tasks []task.Snapshot) map[uint]biTeamTaskDueFact {
	byID := make(map[uint]task.Snapshot, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}

	result := make(map[uint]biTeamTaskDueFact, len(tasks))
	state := make(map[uint]uint8, len(tasks))
	var resolve func(uint) biTeamTaskDueFact
	resolve = func(taskID uint) biTeamTaskDueFact {
		if state[taskID] == 2 {
			return result[taskID]
		}
		if state[taskID] == 1 {
			return biTeamTaskDueFact{}
		}
		task, found := byID[taskID]
		if !found {
			return biTeamTaskDueFact{}
		}
		state[taskID] = 1
		fact := biTeamTaskDueFact{}
		if task.DueAt != nil {
			dueAt := *task.DueAt
			fact = biTeamTaskDueFact{dueAt: &dueAt, source: biTaskDueSourceSelf, sourceTaskID: task.ID}
		} else if task.ParentID != nil && *task.ParentID > 0 && *task.ParentID != task.ID {
			parentFact := resolve(*task.ParentID)
			if parentFact.dueAt != nil {
				dueAt := *parentFact.dueAt
				fact = biTeamTaskDueFact{
					dueAt: &dueAt, source: biTaskDueSourceParent, sourceTaskID: parentFact.sourceTaskID,
				}
			}
		}
		state[taskID] = 2
		result[taskID] = fact
		return fact
	}

	for taskID := range byID {
		resolve(taskID)
	}
	return result
}

func biTeamTaskOverdue(dueAt, deliveryAt *time.Time, now time.Time) bool {
	if dueAt == nil {
		return false
	}
	if deliveryAt != nil {
		return deliveryAt.After(*dueAt)
	}
	return now.After(*dueAt)
}
