package biscreen

import (
	"sort"
	"strings"
)

type workspaceScope struct {
	ProjectSetIDs  []uint
	ProjectIDs     []uint
	RequirementIDs []uint
	VisibleSets    []uint
}

func normalizeStatus(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func completedTaskStatuses() []string {
	return []string{"completed", "done", "released", "archived", "closed"}
}

func normalizeQualityBugStatus(status string) string {
	switch value := normalizeStatus(status); value {
	case "open", "draft":
		return "triage"
	case "doing", "in_dev", "in-dev":
		return "in_progress"
	case "reopen":
		return "reopened"
	case "resolved":
		return "review"
	case "closed", "completed":
		return "done"
	case "cancelled":
		return "canceled"
	default:
		return value
	}
}

func mergeSortedUserIDs(ids []uint, extra ...uint) []uint {
	seen := map[uint]struct{}{}
	for _, id := range append(ids, extra...) {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	result := make([]uint, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
