package workflow

import (
	"testing"
)

func guardTestDefinition() Definition {
	definition, err := ParseDefinition(`{"nodes":[
{"id":"start","type":"start","name":"开始"},
{"id":"frontend-dev","type":"human_task","name":"前端开发"},
{"id":"backend-dev","type":"human_task","name":"后端开发"},
{"id":"acceptance","type":"approval","name":"验收审批"},
{"id":"end","type":"end","name":"结束"}],
"edges":[
{"id":"e1","source":"start","target":"frontend-dev"},
{"id":"e2","source":"start","target":"backend-dev"},
{"id":"e3","source":"frontend-dev","target":"acceptance","kind":"complete"},
{"id":"e4","source":"backend-dev","target":"acceptance","kind":"complete"},
{"id":"e5","source":"acceptance","target":"end","kind":"approve"}]}`)
	if err != nil {
		panic(err)
	}
	return definition
}

func guardRun(nodeID, status string) GuardHistoryRun {
	return GuardHistoryRun{NodeID: nodeID, NodeType: nodeTypeForGuardTest(nodeID), Status: status}
}

func nodeTypeForGuardTest(nodeID string) string {
	switch nodeID {
	case "acceptance":
		return "approval"
	default:
		return "human_task"
	}
}

func guardAllOfGuard(mode string) *RequirementsGuard {
	return &RequirementsGuard{
		SchemaVersion: 1, Source: GuardSourceTemplateNodes, Scope: GuardScopeCurrentPlan,
		Bindings: []GuardBinding{{
			RequirementTemplateVersionID: 31,
			Condition: GuardCondition{AllOf: []GuardNodeRef{
				{NodeID: "frontend-dev", SuccessAction: "complete"},
				{NodeID: "backend-dev", SuccessAction: "complete"},
			}},
		}},
		Aggregate: GuardAggregate{Mode: mode}, OnSatisfied: GuardOnSatisfiedAction,
		EmptyScope: GuardPolicyBlock, MissingBinding: GuardPolicyBlock,
	}
}

func guardInput(versionID uint, instance bool, runs []GuardHistoryRun) GuardRequirementInput {
	definition := guardTestDefinition()
	input := GuardRequirementInput{RequirementID: 1, TemplateVersionID: versionID, Definition: &definition, Runs: runs}
	input.HasWorkflowInstance = instance
	return input
}

func TestRequirementsGuardAllOfFollowsEvidence(t *testing.T) {
	guard := guardAllOfGuard(GuardAggregateAll)
	completed := guard.Evaluate([]GuardRequirementInput{
		guardInput(31, true, []GuardHistoryRun{guardRun("frontend-dev", "completed"), guardRun("backend-dev", "completed")}),
	})
	if !completed.Satisfied() {
		t.Fatalf("all targets completed must satisfy, got %+v", completed)
	}
	partial := guard.Evaluate([]GuardRequirementInput{
		guardInput(31, true, []GuardHistoryRun{guardRun("frontend-dev", "completed")}),
	})
	if partial.Satisfied() || partial.State != GuardUnsatisfied {
		t.Fatalf("missing backend evidence must be unsatisfied, got %+v", partial)
	}
	for _, evidence := range partial.Requirements {
		for _, target := range evidence.Targets {
			if target.NodeID == "backend-dev" && target.State != GuardUnsatisfied {
				t.Fatalf("backend-dev target must be unsatisfied, got %+v", target)
			}
		}
	}
}

func TestRequirementsGuardRollbackInvalidatesEvidence(t *testing.T) {
	guard := guardAllOfGuard(GuardAggregateAll)
	// The frontend node was completed and then rolled back; the rollback run
	// resets the executed downstream, so the evidence must no longer count.
	rollbackOutput := `{"rollback_target_node_id":"frontend-dev"}`
	runs := []GuardHistoryRun{
		guardRun("frontend-dev", "completed"),
		{NodeID: "frontend-dev", NodeType: "human_task", Status: "rolled_back", OutputJSON: rollbackOutput},
		guardRun("backend-dev", "completed"),
	}
	evaluation := guard.Evaluate([]GuardRequirementInput{guardInput(31, true, runs)})
	if evaluation.Satisfied() {
		t.Fatalf("rolled-back evidence must not satisfy the guard, got %+v", evaluation)
	}
}

func TestRequirementsGuardAggregateModes(t *testing.T) {
	satisfied := guardInput(31, true, []GuardHistoryRun{guardRun("frontend-dev", "completed"), guardRun("backend-dev", "completed")})
	unsatisfied := guardInput(31, true, []GuardHistoryRun{guardRun("frontend-dev", "completed")})
	anyGuard := guardAllOfGuard(GuardAggregateAny)
	if !anyGuard.Evaluate([]GuardRequirementInput{satisfied, unsatisfied}).Satisfied() {
		t.Fatal("any requires at least one satisfied")
	}
	if anyGuard.Evaluate([]GuardRequirementInput{unsatisfied, unsatisfied}).Satisfied() {
		t.Fatal("any without satisfied evidence stays unsatisfied")
	}
	countGuard := guardAllOfGuard(GuardAggregateCount)
	countGuard.Aggregate.MinCount = 2
	if countGuard.Evaluate([]GuardRequirementInput{satisfied, unsatisfied}).Satisfied() {
		t.Fatal("count=2 with one satisfied must stay unsatisfied")
	}
	countGuard.Aggregate.MinCount = 1
	if !countGuard.Evaluate([]GuardRequirementInput{satisfied, unsatisfied}).Satisfied() {
		t.Fatal("count=1 with one satisfied must pass")
	}
	ratioGuard := guardAllOfGuard(GuardAggregateRatio)
	ratioGuard.Aggregate.MinRatio = 0.5
	if ratioGuard.Evaluate([]GuardRequirementInput{satisfied, unsatisfied, unsatisfied}).Satisfied() {
		t.Fatal("ratio 1/3 below 0.5 must stay unsatisfied")
	}
	if !ratioGuard.Evaluate([]GuardRequirementInput{satisfied, satisfied, unsatisfied}).Satisfied() {
		t.Fatal("ratio 2/3 above 0.5 must pass")
	}
}

func TestRequirementsGuardUnknownFactsBlock(t *testing.T) {
	guard := guardAllOfGuard(GuardAggregateAll)
	// Requirement without any workflow instance stays in the denominator.
	missing := guardInput(31, false, nil)
	evaluation := guard.Evaluate([]GuardRequirementInput{missing})
	if evaluation.State != GuardUnknown || evaluation.Satisfied() {
		t.Fatalf("no-workflow requirement must be unknown, got %+v", evaluation)
	}
	// Unmapped template version: missing binding blocks, version id surfaced.
	unmapped := guardInput(99, true, nil)
	evaluation = guard.Evaluate([]GuardRequirementInput{unmapped})
	if evaluation.State != GuardUnknown || len(evaluation.MissingBindings) != 1 || evaluation.MissingBindings[0] != 99 {
		t.Fatalf("unmapped version must surface missing binding, got %+v", evaluation)
	}
	// Empty scope is a blocking configuration error, not a pass.
	if empty := guard.Evaluate(nil); empty.Satisfied() || empty.State != GuardEmptyScope {
		t.Fatalf("empty scope must block, got %+v", empty)
	}
	// Unknown evidence poisons aggregate modes that would otherwise pass.
	satisfied := guardInput(31, true, []GuardHistoryRun{guardRun("frontend-dev", "completed"), guardRun("backend-dev", "completed")})
	anyGuard := guardAllOfGuard(GuardAggregateAny)
	if poisoned := anyGuard.Evaluate([]GuardRequirementInput{satisfied, unmapped}); poisoned.Satisfied() {
		t.Fatal("unknown evidence must block even when a ratio/any threshold is met")
	}
}

func TestRequirementsGuardStructuralValidation(t *testing.T) {
	guard := guardAllOfGuard(GuardAggregateAll)
	if issues := guard.ValidateStructural(); len(issues) != 0 {
		t.Fatalf("valid guard must pass, got %v", issues)
	}
	broken := &RequirementsGuard{SchemaVersion: 2, Source: "outcomes", Scope: "all", OnSatisfied: "auto_complete"}
	if issues := broken.ValidateStructural(); len(issues) == 0 {
		t.Fatal("broken guard must fail structural validation")
	}
}

func TestEffectiveNodeStatusesRollbackAndBranches(t *testing.T) {
	definition := guardTestDefinition()
	// Completed parallel branch pair counts as completed.
	statuses := EffectiveNodeStatuses(definition, []GuardHistoryRun{
		guardRun("frontend-dev", "completed"), guardRun("backend-dev", "completed"),
	})
	if statuses["frontend-dev"] != "completed" || statuses["backend-dev"] != "completed" {
		t.Fatalf("completed branches expected, got %v", statuses)
	}
	// A rollback from the frontend node resets the frontend evidence.
	rollbackOutput := `{"rollback_target_node_id":"frontend-dev"}`
	statuses = EffectiveNodeStatuses(definition, []GuardHistoryRun{
		guardRun("frontend-dev", "completed"),
		{NodeID: "frontend-dev", NodeType: "human_task", Status: "rolled_back", OutputJSON: rollbackOutput},
		guardRun("backend-dev", "completed"),
	})
	if statuses["frontend-dev"] != "pending" {
		t.Fatalf("rollback target must reset to pending, got %v", statuses["frontend-dev"])
	}
	// Supplemental runs never change completion.
	statuses = EffectiveNodeStatuses(definition, []GuardHistoryRun{
		guardRun("frontend-dev", "completed"),
		{NodeID: "frontend-dev", NodeType: "human_task", Status: "supplemented", OutputJSON: `{}`},
	})
	if statuses["frontend-dev"] != "completed" {
		t.Fatalf("supplemental run must not change status, got %v", statuses["frontend-dev"])
	}
}
