package httpapi

import (
	"testing"
	"time"

	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func TestBITeamTaskDueFactsUseNearestConfiguredDeadline(t *testing.T) {
	rootDue := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	childDue := rootDue.Add(-24 * time.Hour)
	rootID, childID, missingID := uint(1), uint(2), uint(999)
	tasks := []task.Snapshot{
		{ID: rootID, DueAt: &rootDue},
		{ID: childID, ParentID: &rootID, DueAt: &childDue},
		{ID: 3, ParentID: &childID},
		{ID: 4, ParentID: &rootID},
		{ID: 5, ParentID: &missingID},
		{ID: 6},
	}

	facts := biTeamTaskDueFacts(tasks)
	assertDueFact := func(taskID uint, expected time.Time, source string, sourceTaskID uint) {
		t.Helper()
		fact := facts[taskID]
		if fact.dueAt == nil || !fact.dueAt.Equal(expected) || fact.source != source || fact.sourceTaskID != sourceTaskID {
			t.Fatalf("task %d due fact=%+v", taskID, fact)
		}
	}
	assertDueFact(rootID, rootDue, biTaskDueSourceSelf, rootID)
	assertDueFact(childID, childDue, biTaskDueSourceSelf, childID)
	assertDueFact(3, childDue, biTaskDueSourceParent, childID)
	assertDueFact(4, rootDue, biTaskDueSourceParent, rootID)
	if facts[5].dueAt != nil || facts[6].dueAt != nil {
		t.Fatalf("tasks without a reachable deadline must remain unsampled: %+v", facts)
	}
}

func TestBITeamTaskDueFactsFailClosedOnParentCycles(t *testing.T) {
	firstParent, secondParent := uint(2), uint(1)
	facts := biTeamTaskDueFacts([]task.Snapshot{
		{ID: 1, ParentID: &firstParent},
		{ID: 2, ParentID: &secondParent},
	})
	if facts[1].dueAt != nil || facts[2].dueAt != nil {
		t.Fatalf("cyclic hierarchy must not produce a deadline: %+v", facts)
	}
}
