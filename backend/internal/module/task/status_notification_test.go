package task

import "testing"

func TestStatusNotificationRecipientsAndNoOp(t *testing.T) {
	actor, owner, reporter := uint(1), uint(2), uint(3)
	row := Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, PlanUnitID: 10, Title: "任务标题", Status: "completed", AssigneeID: &owner, ReporterUserID: &reporter}
	intent, ok := StatusNotification(row, "in_dev", actor, "操作人", "https://pm.test", false)
	if !ok || !intent.SkipExternal || len(intent.RecipientUserIDs) != 2 || intent.ResourceType != "work_task" || intent.ResourceID != "9" || intent.Vars["fromStatusKey"] != "in_dev" || intent.Vars["toStatusKey"] != "completed" {
		t.Fatalf("intent=%+v send=%v", intent, ok)
	}
	row.ReporterUserID = &owner
	intent, ok = StatusNotification(row, "in_dev", actor, "操作人", "", true)
	if !ok || len(intent.RecipientUserIDs) != 1 || intent.SkipExternal {
		t.Fatalf("dedupe=%+v", intent)
	}
	row.AssigneeID = &actor
	intent, ok = StatusNotification(row, "in_dev", actor, "操作人", "", false)
	if !ok || len(intent.RecipientUserIDs) != 1 || intent.RecipientUserIDs[0] != owner {
		t.Fatalf("actor exclusion=%+v", intent)
	}
	for _, previous := range []string{"", "completed"} {
		if _, send := StatusNotification(row, previous, actor, "", "", false); send {
			t.Fatalf("no-op emitted for %q", previous)
		}
	}
	row.ReporterUserID = &actor
	if _, send := StatusNotification(row, "in_dev", actor, "", "", false); send {
		t.Fatal("self-only change emitted")
	}
}
