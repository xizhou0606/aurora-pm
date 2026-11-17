package taskplan

import (
	"encoding/json"
	"testing"
)

func TestParseForSubjectNormalizesLanes(t *testing.T) {
	parsed, err := ParseForSubject(Capability, "requirement", 7, json.RawMessage(`{
		"taskPlan":{"chains":[
			{"lane":" frontend ","summary":"页面","tasks":[{"title":"前端","description":"实现页面","acceptanceCriteria":["可用"]}]},
			{"lane":"backend","summary":"接口","tasks":[{"title":"后端","description":"实现接口","acceptanceCriteria":["通过"]}]}
		]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Plan.Chains[0].Lane != "frontend" {
		t.Fatalf("lane=%q", parsed.Plan.Chains[0].Lane)
	}
}

func TestParseForSubjectRejectsBrokenCreatedTaskHierarchy(t *testing.T) {
	parentID := uint(10)
	refs := []CreatedTaskRef{
		{ID: 10, TaskKey: "PM-10", Title: "前端", Lane: "frontend", Kind: "task"},
		{ID: 11, TaskKey: "PM-11", Title: "前端子任务", Lane: "frontend", Kind: "subtask", ParentID: &parentID},
		{ID: 20, TaskKey: "PM-20", Title: "后端", Lane: "backend", Kind: "task"},
		{ID: 21, TaskKey: "PM-21", Title: "后端子任务", Lane: "backend", Kind: "subtask", ParentID: &parentID},
	}
	output := map[string]any{
		"taskPlan": map[string]any{"chains": []any{
			map[string]any{"lane": "frontend", "summary": "页面", "tasks": []any{map[string]any{"title": "前端", "description": "实现页面", "acceptanceCriteria": []string{"可用"}}}},
			map[string]any{"lane": "backend", "summary": "接口", "tasks": []any{map[string]any{"title": "后端", "description": "实现接口", "acceptanceCriteria": []string{"通过"}}}},
		}},
		"createdTasks": refs,
	}
	raw, _ := json.Marshal(output)
	if _, err := ParseForSubject(Capability, "requirement", 7, raw); err == nil {
		t.Fatal("broken backend parent hierarchy was accepted")
	}
}
