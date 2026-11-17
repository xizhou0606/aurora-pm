package notification

import (
	"fmt"
	"strings"
)

// PublicURL joins a configured public origin with path segments. Handlers pass
// the notification public base so every scene reuses the same URL rules.
func PublicURL(base string, parts ...string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	segments := make([]string, 0, len(parts)+1)
	segments = append(segments, base)
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "/")
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return strings.Join(segments, "/")
}

func WorkItemURL(base string, taskID uint) string {
	return PublicURL(base, "workspace", "work-items", fmt.Sprintf("%d", taskID))
}

func DocumentCommentURL(base string, documentID, threadID uint) string {
	value := PublicURL(base, "workspace", "documents", fmt.Sprintf("%d", documentID))
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s#comment-%d", value, threadID)
}

func QualityBugURL(base string, bugID uint) string {
	return PublicURL(base, "workspace", "quality-management", "bugs", fmt.Sprintf("%d", bugID))
}

func RequirementURL(base string, requirementID uint) string {
	return PublicURL(base, "workspace", "requirements", fmt.Sprintf("%d", requirementID))
}

func WorkflowListURL(base string) string {
	return PublicURL(base, "workspace", "pm", "workflows")
}

// PlanUnitURL links to the project/version item page whose member management
// lives there.
func PlanUnitURL(base string, projectSetID, planUnitID uint) string {
	return PublicURL(base, "workspace", "projects", fmt.Sprintf("%d", projectSetID),
		"items", fmt.Sprintf("%d", planUnitID))
}
