package httpapi

import "testing"

func TestFirstRequirementEntryNodeID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		definition workflowDefinition
		want       string
	}{
		{
			name: "named requirement entry",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "input", Type: "form", Name: "需求录入"},
					{ID: "review", Type: "approval", Name: "产品评审"},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "input"},
					{ID: "e2", Source: "input", Target: "review"},
					{ID: "e3", Source: "review", Target: "end", Kind: "approve"},
				},
			},
			want: "input",
		},
		{
			name: "requirement_input config without matching name",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "collect", Type: "human_task", Name: "业务信息", Config: map[string]any{"requirement_input": true}},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "collect"},
					{ID: "e2", Source: "collect", Target: "end"},
				},
			},
			want: "collect",
		},
		{
			name: "skips non-entry form and later review",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "info", Type: "form", Name: "信息录入"},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "info"},
					{ID: "e2", Source: "info", Target: "end"},
				},
			},
			want: "",
		},
		{
			name: "first of two entry nodes along default path",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "first", Type: "form", Name: "需求收集"},
					{ID: "second", Type: "form", Name: "需求录入"},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "first"},
					{ID: "e2", Source: "first", Target: "second"},
					{ID: "e3", Source: "second", Target: "end"},
				},
			},
			want: "first",
		},
		{
			name: "walks through parallel split",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "split", Type: "parallel_split", Name: "分流"},
					{ID: "input", Type: "form", Name: "需求录入"},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "split"},
					{ID: "e2", Source: "split", Target: "input"},
					{ID: "e3", Source: "input", Target: "end"},
				},
			},
			want: "input",
		},
		{
			name: "approval named 需求录入 is not an entry node",
			definition: workflowDefinition{
				Nodes: []workflowNode{
					{ID: "start", Type: "start", Name: "开始"},
					{ID: "review", Type: "approval", Name: "需求录入审批"},
					{ID: "end", Type: "end", Name: "结束"},
				},
				Edges: []workflowEdge{
					{ID: "e1", Source: "start", Target: "review"},
					{ID: "e2", Source: "review", Target: "end", Kind: "approve"},
				},
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := firstRequirementEntryNodeID(tc.definition); got != tc.want {
				t.Fatalf("firstRequirementEntryNodeID()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestFirstAssignableWorkflowNodeID(t *testing.T) {
	t.Parallel()
	definition := workflowDefinition{
		Nodes: []workflowNode{
			{ID: "start", Type: "start", Name: "开始"},
			{ID: "info", Type: "form", Name: "信息录入"},
			{ID: "review", Type: "approval", Name: "产品评审"},
			{ID: "end", Type: "end", Name: "结束"},
		},
		Edges: []workflowEdge{
			{ID: "e1", Source: "start", Target: "info"},
			{ID: "e2", Source: "info", Target: "review"},
			{ID: "e3", Source: "review", Target: "end", Kind: "approve"},
		},
	}
	if got := firstAssignableWorkflowNodeID(definition); got != "info" {
		t.Fatalf("firstAssignableWorkflowNodeID()=%q want info", got)
	}
}
