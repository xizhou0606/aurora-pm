package httpapi

func normalizeQualityBugStatus(status string) string {
	value := normalizeStatus(status)
	switch value {
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

func qualityBugStatusStorageAliases(status string) []string {
	switch normalizeQualityBugStatus(status) {
	case "triage":
		return []string{"triage", "open", "draft"}
	case "in_progress":
		return []string{"in_progress", "doing", "in_dev", "in-dev"}
	case "reopened":
		return []string{"reopened", "reopen"}
	case "review":
		return []string{"review", "resolved"}
	case "done":
		return []string{"done", "closed", "completed"}
	case "canceled":
		return []string{"canceled", "cancelled"}
	default:
		return []string{normalizeQualityBugStatus(status)}
	}
}

func qualityBugStatusesStorageAliases(statuses ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(statuses))
	for _, status := range statuses {
		for _, alias := range qualityBugStatusStorageAliases(status) {
			if alias == "" {
				continue
			}
			if _, exists := seen[alias]; exists {
				continue
			}
			seen[alias] = struct{}{}
			result = append(result, alias)
		}
	}
	return result
}

func qualityReviewerOwnsStatus(status string) bool {
	switch normalizeQualityBugStatus(status) {
	case "review", "done":
		return true
	default:
		return false
	}
}

func qualityListReviewerOwnsStatus(status string) bool {
	switch normalizeQualityBugStatus(status) {
	case "review", "done", "canceled":
		return true
	default:
		return false
	}
}
