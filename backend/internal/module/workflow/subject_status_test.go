package workflow

import "testing"

func TestSubjectStageStatusPrefersIntakeOverBareRequirement(t *testing.T) {
	got := SubjectStageStatus("plan_unit", DefinitionNode{Name: "需求录入"}, "running")
	if got != "planning" {
		t.Fatalf("intake status=%q, want planning", got)
	}
	got = SubjectStageStatus("plan_unit", DefinitionNode{Name: "产品评审"}, "running")
	if got != "grooming" {
		t.Fatalf("review status=%q, want grooming", got)
	}
}

func TestSubjectStatusForCurrentIgnoresParallelAINodes(t *testing.T) {
	status := SubjectStatusForCurrent("plan_unit", "running", []DefinitionNode{
		{ID: "inspect", Type: "ai_check", Name: "AI 风险智能巡检"},
		{ID: "dev", Type: "human_task", Name: "开发与实验"},
	})
	if status != "development" {
		t.Fatalf("parallel status=%q, want development", status)
	}
}
