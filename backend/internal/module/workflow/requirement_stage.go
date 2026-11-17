package workflow

import (
	"strings"
)

// 需求阶段（Requirement.Status）按工作流推进顺序排列；阶段驱动联动比较的是
// "达到至少该阶段"，需求 status 由每条需求自己的工作流自动同步。
const (
	StageBacklog     = "backlog"
	StageGrooming    = "grooming"
	StageDevelopment = "development"
	StageAcceptance  = "acceptance"
	StageDone        = "done"
)

var stageOrder = map[string]int{
	StageBacklog:     0,
	StageGrooming:    1,
	StageDevelopment: 2,
	StageAcceptance:  3,
	StageDone:        4,
}

func StageIndex(stage string) int { return stageOrder[stage] }

// StageIndexMap exposes the valid stage set for config validation.
func StageIndexMap() map[string]int { return stageOrder }

// stageNameHints map node-name keywords to requirement stages. Order matters:
// more specific phrases (验收准备) must match before generic ones (验收).
var stageNameHints = []struct {
	keyword string
	stage   string
}{
	{"验收准备", StageAcceptance},
	{"提测", StageAcceptance},
	{"联调", StageAcceptance},
	{"验收", StageDone},
	{"测试", StageAcceptance},
	{"开发", StageAcceptance},
	{"评审", StageDevelopment},
	{"发布", StageDone},
	{"交付", StageDone},
	{"上线", StageDone},
}

// StageDrivenNode reports the requirement stage a plan-unit node is driven by
// (需求阶段驱动联动). Tri-state like the intake lock:
//   - explicit requirement_stage config wins ("none" opts out, a valid stage
//     opts in on any node type that can carry it);
//   - when unset, node-name inference applies to human task/form nodes only
//     (评审→所有需求进入研发中、开发/测试→验收中、验收/发布/交付/上线→已完成).
//
// Intake nodes (需求录入/需求收集) never stage-drive: their completion is the
// human intake action with the lock confirmation.
func StageDrivenNode(node DefinitionNode) (string, bool) {
	if raw, exists := node.Config["requirement_stage"]; exists {
		stage, _ := raw.(string)
		if stage == "" || stage == "none" {
			return "", false
		}
		if _, valid := stageOrder[stage]; valid {
			return stage, true
		}
		return "", false
	}
	if node.Type != "human_task" && node.Type != "form" {
		return "", false
	}
	if strings.Contains(node.Name, "需求录入") || strings.Contains(node.Name, "需求收集") {
		return "", false
	}
	for _, hint := range stageNameHints {
		if strings.Contains(node.Name, hint.keyword) {
			return hint.stage, true
		}
	}
	return "", false
}

// StageGuard builds the stage-driven guard for a node: all requirements in the
// current scope must have reached the stage (aggregate all). The caller picks
// on_satisfied: auto_complete for全自动推进, enable_action for blocking-only.
func StageGuard(stage, onSatisfied string) *RequirementsGuard {
	return &RequirementsGuard{
		SchemaVersion:  1,
		Source:         GuardSourceStageStatus,
		Scope:          GuardScopeCurrentPlan,
		Stage:          stage,
		Aggregate:      GuardAggregate{Mode: GuardAggregateAll},
		OnSatisfied:    onSatisfied,
		EmptyScope:     GuardPolicyBlock,
		MissingBinding: GuardPolicyBlock,
	}
}

// StageEvidence evaluates one requirement against a target stage from its
// workflow-synced status.
func StageEvidence(status, stage string) string {
	if StageIndex(stage) < 0 {
		return GuardUnknown
	}
	if strings.TrimSpace(status) == "" {
		return GuardUnknown
	}
	current, ok := stageOrder[status]
	if !ok {
		return GuardUnknown
	}
	if current >= stageOrder[stage] {
		return GuardSatisfied
	}
	return GuardUnsatisfied
}

// RequirementStageDriven evaluates the requirement stage scope on the
// caller's behalf; it is the stage_status twin of guard binding evaluation.
func RequirementStageDriven(stage string, inputs []GuardRequirementInput) (satisfied, unknown, total int) {
	total = len(inputs)
	for _, input := range inputs {
		switch StageEvidence(input.Status, stage) {
		case GuardSatisfied:
			satisfied++
		case GuardUnknown:
			unknown++
		}
	}
	return satisfied, unknown, total
}

// NodeStageGuard returns the stage-driven guard for a node and whether the
// node auto-completes when the stage is reached (人工任务/表单节点，需求阶段
// 驱动 + 全自动推进)。Approvals and intake nodes are never stage-driven.
func NodeStageGuard(node DefinitionNode) (*RequirementsGuard, bool, bool) {
	stage, ok := StageDrivenNode(node)
	if !ok {
		return nil, false, false
	}
	auto := node.Type == "human_task" || node.Type == "form"
	return StageGuard(stage, GuardOnSatisfiedAutoComplete), true, auto
}
