package workflow

import "testing"

func deliveryConfigFixture() *DeliveryProgressConfig {
	return &DeliveryProgressConfig{
		SchemaVersion: 1,
		Milestones: []DeliveryMilestone{
			{Key: "review_passed", Name: "评审通过", Weight: 20, Bindings: []GuardBinding{{
				RequirementTemplateVersionID: 31,
				Condition:                    GuardCondition{AllOf: []GuardNodeRef{{NodeID: "frontend-dev", SuccessAction: "complete"}}},
			}}},
			{Key: "development_completed", Name: "开发完成", Weight: 50, Bindings: []GuardBinding{{
				RequirementTemplateVersionID: 31,
				Condition: GuardCondition{AllOf: []GuardNodeRef{
					{NodeID: "frontend-dev", SuccessAction: "complete"},
					{NodeID: "backend-dev", SuccessAction: "complete"},
				}},
			}}},
			{Key: "acceptance_passed", Name: "验收通过", Weight: 30, IsAcceptance: true, Bindings: []GuardBinding{{
				RequirementTemplateVersionID: 31,
				Condition:                    GuardCondition{AllOf: []GuardNodeRef{{NodeID: "acceptance", SuccessAction: "approve"}}},
			}}},
		},
	}
}

func TestDeliveryProgressWeightedExample(t *testing.T) {
	config := deliveryConfigFixture()
	// A: 全部达标 100%；B：开发完成 70%；C：仅评审 20% —— 文档 §7 的 63.3% 样例。
	progressFor := func(acceptance, backend, review bool) GuardRequirementInput {
		runs := []GuardHistoryRun{}
		if review {
			runs = append(runs, guardRun("frontend-dev", "completed"))
		}
		if backend {
			runs = append(runs, guardRun("backend-dev", "completed"))
		}
		if acceptance {
			runs = append(runs, guardRun("acceptance", "completed"))
		}
		return guardInput(31, true, runs)
	}
	result := config.Evaluate([]GuardRequirementInput{
		progressFor(true, true, true),
		progressFor(false, true, true),
		progressFor(false, false, true),
	})
	// B lacks acceptance (approvals) — 70%；C lacks backend and acceptance — 20%。
	if got := result.Progress; got < 0.633 || got > 0.634 {
		t.Fatalf("aggregate progress = %v, want ~0.633", got)
	}
	if result.AcceptedCount != 1 || !result.AcceptanceConfigured {
		t.Fatalf("accepted=%d configured=%v, want 1/true", result.AcceptedCount, result.AcceptanceConfigured)
	}
	if result.ScopeCount != 3 {
		t.Fatalf("scope count = %d", result.ScopeCount)
	}
	// 逐需求里程碑计数：开发完成里程碑应有 2 条需求满足。
	for _, milestone := range result.Milestones {
		if milestone.Key == "development_completed" && milestone.SatisfiedCount != 2 {
			t.Fatalf("development satisfied = %d, want 2", milestone.SatisfiedCount)
		}
	}
}

func TestDeliveryProgressUnknownBlocksCounting(t *testing.T) {
	config := deliveryConfigFixture()
	// 需求没有绑定版本：所有里程碑都无法判定，不贡献已确认完成值。
	unmapped := GuardRequirementInput{RequirementID: 9, TemplateVersionID: 99, HasWorkflowInstance: true}
	result := config.Evaluate([]GuardRequirementInput{unmapped})
	if result.UnknownCount != 1 {
		t.Fatalf("unmapped requirement must be unknown, got %+v", result)
	}
	if result.Progress != 0 {
		t.Fatalf("unknown evidence contributes no progress, got %v", result.Progress)
	}
	// 验收计数也以非 unknown 为前提。
	if result.AcceptedCount != 0 {
		t.Fatalf("unknown requirement must not count as accepted")
	}
}

func TestDeliveryProgressValidation(t *testing.T) {
	config := deliveryConfigFixture()
	if issues := config.ValidateStructural(); len(issues) != 0 {
		t.Fatalf("valid config must pass, got %v", issues)
	}
	config.Milestones[2].Weight = -1
	config.Milestones[0].Key = config.Milestones[1].Key
	config.Milestones[1].IsAcceptance = true
	if issues := config.ValidateStructural(); len(issues) == 0 {
		t.Fatal("negative weight, duplicate key and second acceptance must fail")
	}
}
