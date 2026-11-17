package task

import "strings"

// IsCancelledStatus reports whether a task was explicitly cancelled. Cancelled
// tasks remain visible as historical work, but must not enter active or overdue
// samples.
func IsCancelledStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "cancelled", "canceled":
		return true
	default:
		return false
	}
}

// IsTerminalStatus reports whether a task can no longer become overdue.
func IsTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "done", "released", "archived", "closed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}
