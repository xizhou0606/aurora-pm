package workflow

import "testing"

func TestStageDrivenNodeInference(t *testing.T) {
	cases := []struct {
		name     string
		nodeType string
		want     string
		ok       bool
	}{
		{name: "产品评审", nodeType: "human_task", want: StageDevelopment, ok: true},
		{name: "开发与实施", nodeType: "human_task", want: StageAcceptance, ok: true},
		{name: "测试与验收准备", nodeType: "form", want: StageAcceptance, ok: true},
		{name: "最终验收", nodeType: "human_task", want: StageDone, ok: true},
		{name: "发布执行", nodeType: "human_task", want: StageDone, ok: true},
		{name: "需求录入", nodeType: "form", want: "", ok: false},
		{name: "需求收集", nodeType: "human_task", want: "", ok: false},
		{name: "风险应对处理", nodeType: "human_task", want: "", ok: false},
		{name: "产品评审", nodeType: "approval", want: "", ok: false},
	}
	for _, tc := range cases {
		node := DefinitionNode{ID: "n", Type: tc.nodeType, Name: tc.name}
		stage, ok := StageDrivenNode(node)
		if ok != tc.ok || stage != tc.want {
			t.Errorf("%s(%s): got (%q,%v) want (%q,%v)", tc.name, tc.nodeType, stage, ok, tc.want, tc.ok)
		}
	}
}

func TestStageDrivenNodeExplicitConfig(t *testing.T) {
	// 显式阶段覆盖名称推断。
	node := DefinitionNode{ID: "n", Type: "human_task", Name: "风险应对处理", Config: map[string]any{"requirement_stage": "development"}}
	if stage, ok := StageDrivenNode(node); !ok || stage != StageDevelopment {
		t.Fatalf("explicit stage config must win, got (%q,%v)", stage, ok)
	}
	// none 显式关闭：即使名称匹配也不联动。
	optOut := DefinitionNode{ID: "n", Type: "human_task", Name: "开发与实施", Config: map[string]any{"requirement_stage": "none"}}
	if _, ok := StageDrivenNode(optOut); ok {
		t.Fatal("requirement_stage=none must opt out")
	}
	// 非法值不联动（fail-closed）。
	invalid := DefinitionNode{ID: "n", Type: "human_task", Name: "开发与实施", Config: map[string]any{"requirement_stage": "shipping"}}
	if _, ok := StageDrivenNode(invalid); ok {
		t.Fatal("invalid stage must not stage-drive")
	}
}

func TestStageEvidenceOrdering(t *testing.T) {
	if got := StageEvidence("development", "development"); got != GuardSatisfied {
		t.Fatalf("same stage must satisfy, got %v", got)
	}
	if got := StageEvidence("acceptance", "development"); got != GuardSatisfied {
		t.Fatalf("later stage must satisfy, got %v", got)
	}
	if got := StageEvidence("grooming", "development"); got != GuardUnsatisfied {
		t.Fatalf("earlier stage must be unsatisfied, got %v", got)
	}
	if got := StageEvidence("", "development"); got != GuardUnknown {
		t.Fatalf("empty status must be unknown, got %v", got)
	}
	if got := StageEvidence("weird", "development"); got != GuardUnknown {
		t.Fatalf("unknown status must be unknown, got %v", got)
	}
}

func TestStageStatusGuardEvaluate(t *testing.T) {
	guard := StageGuard(StageAcceptance, GuardOnSatisfiedAutoComplete)
	if issues := guard.ValidateStructural(); len(issues) != 0 {
		t.Fatalf("stage guard must validate, got %v", issues)
	}
	inputs := []GuardRequirementInput{
		{RequirementID: 1, Status: StageDone},
		{RequirementID: 2, Status: StageAcceptance},
		{RequirementID: 3, Status: StageDevelopment},
	}
	evaluation := guard.Evaluate(inputs)
	if evaluation.Satisfied() || evaluation.SatisfiedCount != 2 || evaluation.TotalCount != 3 {
		t.Fatalf("2/3 reached acceptance must be unsatisfied, got %+v", evaluation)
	}
	if evaluation.Requirements[2].State != GuardUnsatisfied {
		t.Fatalf("requirement below stage must be unsatisfied, got %+v", evaluation.Requirements[2])
	}
	// 汇聚到验收中后自动达标。
	inputs[2].Status = StageAcceptance
	if result := guard.Evaluate(inputs); !result.Satisfied() {
		t.Fatalf("all reached stage must satisfy, got %+v", result)
	}
}

func TestStageStatusGuardRejectsBindings(t *testing.T) {
	guard := StageGuard(StageDone, GuardOnSatisfiedAutoComplete)
	guard.Bindings = []GuardBinding{{RequirementTemplateVersionID: 4}}
	if issues := guard.ValidateStructural(); len(issues) == 0 {
		t.Fatal("stage_status guard with bindings must fail validation")
	}
}
