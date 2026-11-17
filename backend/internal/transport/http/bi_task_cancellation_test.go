package httpapi

import (
	"testing"
	"time"

	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestBITeamMemberCancelledTaskIsNotOverdue(t *testing.T) {
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	item := biTeamTaskItem(
		taskgorm.WorkTask{ID: 1, Status: "cancelled", DueAt: &past},
		biTeamTaskDueFact{dueAt: &past, source: biTaskDueSourceSelf, sourceTaskID: 1},
		biTeamTaskFacts{},
		now,
	)
	if item.Overdue == nil || *item.Overdue {
		t.Fatalf("cancelled task member item overdue=%v", item.Overdue)
	}
}
