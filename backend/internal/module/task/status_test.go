package task

import "testing"

func TestTaskTerminalAndCancellationStatuses(t *testing.T) {
	for _, status := range []string{"cancelled", " CANCELED "} {
		if !IsCancelledStatus(status) || !IsTerminalStatus(status) {
			t.Fatalf("cancelled status %q was not terminal", status)
		}
	}
	for _, status := range []string{"completed", "done", "released", "archived", "closed"} {
		if IsCancelledStatus(status) || !IsTerminalStatus(status) {
			t.Fatalf("completed status %q has wrong classification", status)
		}
	}
	for _, status := range []string{"draft", "in_dev", "pending_test", "blocked", ""} {
		if IsCancelledStatus(status) || IsTerminalStatus(status) {
			t.Fatalf("active status %q was classified as terminal", status)
		}
	}
}
