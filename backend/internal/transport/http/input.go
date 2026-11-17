package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type optionalValue[T any] struct {
	Set   bool
	Value *T
}

const workflowManagedStatusMessage = "status is managed by workflow"

func (o *optionalValue[T]) UnmarshalJSON(raw []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func parseOptionalTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, strings.TrimSpace(*value))
		if err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}
	return nil, fmt.Errorf("invalid date %q", *value)
}

func normalizeStatus(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validProjectStatus(value string) bool {
	switch normalizeStatus(value) {
	case "active", "archived":
		return true
	default:
		return false
	}
}

func validPlanUnitStatus(value string) bool {
	switch normalizeStatus(value) {
	case "planning", "grooming", "development", "acceptance", "released", "archived":
		return true
	default:
		return false
	}
}

func validRequirementStatus(value string) bool {
	switch normalizeStatus(value) {
	case "backlog", "grooming", "development", "acceptance", "done":
		return true
	default:
		return false
	}
}

// validPriority 校验需求优先级档位 P0–P3。
func validPriority(value string) bool {
	switch normalizePriority(value) {
	case "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

func normalizePriority(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// validTaskPriority 校验任务优先级档位（沿用 low/normal/medium/high/urgent，medium 归一为 normal）。
func validTaskPriority(value string) bool {
	switch normalizeTaskPriority(value) {
	case "low", "normal", "medium", "high", "urgent":
		return true
	default:
		return false
	}
}

func normalizeTaskPriority(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "medium" {
		return "normal"
	}
	return value
}

func validTaskStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft", "in_dev", "pending_test", "completed", "cancelled":
		return true
	default:
		return false
	}
}

func completedTaskStatuses() []string {
	return []string{"completed", "done", "released", "archived", "closed"}
}

func terminalTaskStatuses() []string {
	return append(completedTaskStatuses(), "cancelled", "canceled")
}

func inProgressTaskStatuses() []string {
	return []string{"in_dev", "pending_test", "in_progress", "doing", "development", "review", "acceptance"}
}

func todoTaskStatuses() []string {
	return []string{"draft", "todo", "backlog"}
}
