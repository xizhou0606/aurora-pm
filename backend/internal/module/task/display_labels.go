package task

import "strings"

// WorkTaskKindLabel returns the user-facing label for a persisted task kind.
// Unknown values are preserved so notifications remain diagnosable when a new
// enum reaches an older service version.
func WorkTaskKindLabel(value string) string {
	key := normalizeWorkTaskDisplayKey(value)
	for _, kind := range StandardWorkflowKinds() {
		if kind.KindKey == key {
			return kind.Label
		}
	}
	return fallbackWorkTaskDisplayLabel(key)
}

// WorkTaskPriorityLabel is the single backend source for task-priority copy.
func WorkTaskPriorityLabel(value string) string {
	key := normalizeWorkTaskDisplayKey(value)
	switch key {
	case "low":
		return "低"
	case "normal", "medium":
		return "普通"
	case "high":
		return "高"
	case "urgent":
		return "紧急"
	default:
		return fallbackWorkTaskDisplayLabel(key)
	}
}

// WorkTaskStatusLabel returns canonical workflow copy and keeps compatibility
// with legacy task states that may still exist in historical records.
func WorkTaskStatusLabel(value string) string {
	key := normalizeWorkTaskDisplayKey(value)
	for _, status := range StandardWorkflowStatuses() {
		if status.StatusKey == key {
			return status.Label
		}
	}
	switch key {
	case "backlog":
		return "待规划"
	case "todo":
		return "待处理"
	case "in_progress":
		return "进行中"
	case "pending", "review":
		return "待验证"
	case "blocked":
		return "已阻塞"
	case "done":
		return "已完成"
	case "canceled":
		return "已取消"
	case "reopened":
		return "重新打开"
	default:
		return fallbackWorkTaskDisplayLabel(key)
	}
}

func normalizeWorkTaskDisplayKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func fallbackWorkTaskDisplayLabel(key string) string {
	if key == "" {
		return "-"
	}
	return key
}
