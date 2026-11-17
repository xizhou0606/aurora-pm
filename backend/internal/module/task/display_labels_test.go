package task

import "testing"

func TestWorkTaskDisplayLabels(t *testing.T) {
	tests := []struct {
		name  string
		label func(string) string
		value string
		want  string
	}{
		{name: "subtask kind", label: WorkTaskKindLabel, value: " subtask ", want: "子任务"},
		{name: "normal priority", label: WorkTaskPriorityLabel, value: "normal", want: "普通"},
		{name: "canonical draft status", label: WorkTaskStatusLabel, value: "draft", want: "待处理"},
		{name: "legacy todo status", label: WorkTaskStatusLabel, value: "todo", want: "待处理"},
		{name: "unknown enum remains visible", label: WorkTaskStatusLabel, value: "Custom_State", want: "custom_state"},
		{name: "empty enum", label: WorkTaskPriorityLabel, value: " ", want: "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.label(tt.value); got != tt.want {
				t.Fatalf("label(%q)=%q want=%q", tt.value, got, tt.want)
			}
		})
	}
}
