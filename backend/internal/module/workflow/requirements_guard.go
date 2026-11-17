package workflow

import (
	"context"
	"fmt"
)

// RequirementsGuard binds a plan-unit workflow node to requirement workflow
// evidence (可配置工作流联动, stage A). Source template_nodes maps each
// requirement template version to concrete target nodes on the requirement's
// own workflow; the guard never reads project-side state to decide success.
type RequirementsGuard struct {
	SchemaVersion  int            `json:"schema_version"`
	Source         string         `json:"source"`
	Scope          string         `json:"scope"`
	Stage          string         `json:"stage,omitempty"`
	Bindings       []GuardBinding `json:"bindings"`
	Aggregate      GuardAggregate `json:"aggregate"`
	OnSatisfied    string         `json:"on_satisfied"`
	EmptyScope     string         `json:"empty_scope"`
	MissingBinding string         `json:"missing_binding"`
}

type GuardBinding struct {
	RequirementTemplateVersionID uint           `json:"requirement_template_version_id"`
	Condition                    GuardCondition `json:"condition"`
}

type GuardCondition struct {
	AllOf []GuardNodeRef `json:"all_of,omitempty"`
	AnyOf []GuardNodeRef `json:"any_of,omitempty"`
}

type GuardNodeRef struct {
	NodeID        string `json:"node_id"`
	SuccessAction string `json:"success_action"`
}

type GuardAggregate struct {
	Mode     string  `json:"mode"`
	MinCount int     `json:"min_count,omitempty"`
	MinRatio float64 `json:"min_ratio,omitempty"`
}

// Evaluation states.
const (
	GuardSatisfied   = "satisfied"
	GuardUnsatisfied = "unsatisfied"
	GuardUnknown     = "unknown"
	GuardEmptyScope  = "empty_scope"
)

const (
	GuardSourceTemplateNodes = "template_nodes"
	// GuardSourceStageStatus drives a node from the requirement-set's synced
	// business stages (需求阶段驱动) instead of template-version node bindings.
	GuardSourceStageStatus = "stage_status"
	GuardScopeCurrentPlan  = "current_plan_requirements"
	GuardOnSatisfiedAction = "enable_action"
	// GuardOnSatisfiedAutoComplete is only valid on requirement_gate nodes
	// (enforced at template authoring time where the node type is known).
	GuardOnSatisfiedAutoComplete = "auto_complete"
	GuardPolicyBlock             = "block"
	GuardAggregateAll            = "all"
	GuardAggregateAny            = "any"
	GuardAggregateCount          = "count"
	GuardAggregateRatio          = "ratio"
)

// ParseRequirementsGuard extracts the requirements_guard config from a node
// config. present=false means the node carries no guard at all.
func ParseRequirementsGuard(config map[string]any) (*RequirementsGuard, bool, error) {
	raw, present := config["requirements_guard"]
	if !present || raw == nil {
		return nil, false, nil
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("requirements_guard 必须是对象")
	}
	guard := &RequirementsGuard{
		SchemaVersion:  intFloat(value["schema_version"]),
		Source:         stringField(value["source"]),
		Scope:          stringField(value["scope"]),
		Stage:          stringField(value["stage"]),
		OnSatisfied:    stringField(value["on_satisfied"]),
		EmptyScope:     stringField(value["empty_scope"]),
		MissingBinding: stringField(value["missing_binding"]),
	}
	// Defaults keep hand-written configs small; explicit values win.
	if guard.OnSatisfied == "" {
		guard.OnSatisfied = GuardOnSatisfiedAction
	}
	if guard.EmptyScope == "" {
		guard.EmptyScope = GuardPolicyBlock
	}
	if guard.MissingBinding == "" {
		guard.MissingBinding = GuardPolicyBlock
	}
	if rawBindings, exists := value["bindings"]; exists {
		items, _ := rawBindings.([]any)
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return guard, true, fmt.Errorf("requirements_guard.bindings 项必须是对象")
			}
			binding := GuardBinding{RequirementTemplateVersionID: uintFloat(entry["requirement_template_version_id"])}
			condition, ok := entry["condition"].(map[string]any)
			if !ok {
				return guard, true, fmt.Errorf("绑定 %d 缺少 condition 对象", binding.RequirementTemplateVersionID)
			}
			binding.Condition = parseGuardCondition(condition)
			guard.Bindings = append(guard.Bindings, binding)
		}
	}
	if rawAggregate, exists := value["aggregate"]; exists {
		aggregate, ok := rawAggregate.(map[string]any)
		if !ok {
			return guard, true, fmt.Errorf("requirements_guard.aggregate 必须是对象")
		}
		guard.Aggregate = GuardAggregate{
			Mode:     stringField(aggregate["mode"]),
			MinCount: intFloat(aggregate["min_count"]),
			MinRatio: floatField(aggregate["min_ratio"]),
		}
	}
	if guard.Aggregate.Mode == "" {
		guard.Aggregate.Mode = GuardAggregateAll
	}
	return guard, true, nil
}

func parseGuardCondition(condition map[string]any) GuardCondition {
	result := GuardCondition{}
	for _, ref := range listField(condition["all_of"]) {
		result.AllOf = append(result.AllOf, parseGuardNodeRef(ref))
	}
	for _, ref := range listField(condition["any_of"]) {
		result.AnyOf = append(result.AnyOf, parseGuardNodeRef(ref))
	}
	return result
}

func parseGuardNodeRef(raw any) GuardNodeRef {
	ref, _ := raw.(map[string]any)
	return GuardNodeRef{NodeID: stringField(ref["node_id"]), SuccessAction: stringField(ref["success_action"])}
}

// ValidateStructural checks the guard shape without any database access. DB
// referential checks (binding versions, target nodes) live at template
// save/publish time where the workspace context is available.
func (guard *RequirementsGuard) ValidateStructural() []string {
	issues := make([]string, 0)
	if guard.SchemaVersion != 1 {
		issues = append(issues, "requirements_guard.schema_version 必须是 1")
	}
	switch guard.Source {
	case GuardSourceStageStatus:
		if StageIndex(guard.Stage) < 0 {
			issues = append(issues, "stage_status 来源必须声明有效的需求阶段（stage）")
		}
		if len(guard.Bindings) > 0 {
			issues = append(issues, "stage_status 来源不接受模板版本绑定（bindings）")
		}
	case GuardSourceTemplateNodes:
		if len(guard.Bindings) == 0 {
			issues = append(issues, "requirements_guard.bindings 至少需要一个需求模板版本映射")
		}
		seenVersions := map[uint]bool{}
		for _, binding := range guard.Bindings {
			versionID := binding.RequirementTemplateVersionID
			if versionID == 0 {
				issues = append(issues, "绑定缺少 requirement_template_version_id")
				continue
			}
			if seenVersions[versionID] {
				issues = append(issues, fmt.Sprintf("需求模板版本 %d 只能绑定一次", versionID))
			}
			seenVersions[versionID] = true
			issues = append(issues, validateGuardCondition(binding.Condition)...)
		}
	default:
		issues = append(issues, "requirements_guard.source 只支持 template_nodes/stage_status")
	}
	if guard.Scope != GuardScopeCurrentPlan {
		issues = append(issues, "requirements_guard.scope 只支持 current_plan_requirements")
	}
	if guard.OnSatisfied != GuardOnSatisfiedAction && guard.OnSatisfied != GuardOnSatisfiedAutoComplete {
		issues = append(issues, "requirements_guard.on_satisfied 只支持 enable_action/auto_complete")
	}
	if guard.EmptyScope != GuardPolicyBlock {
		issues = append(issues, "requirements_guard.empty_scope 只支持 block（空范围不按通过处理）")
	}
	if guard.MissingBinding != GuardPolicyBlock {
		issues = append(issues, "requirements_guard.missing_binding 只支持 block（缺少映射不自动排除）")
	}
	issues = append(issues, guard.Aggregate.validate()...)
	return issues
}

func (guard *RequirementsGuard) validateCondition(condition GuardCondition) []string {
	return validateGuardCondition(condition)
}

func validateGuardCondition(condition GuardCondition) []string {
	refs := make([]GuardNodeRef, 0, len(condition.AllOf)+len(condition.AnyOf))
	if len(condition.AllOf) > 0 && len(condition.AnyOf) > 0 {
		return []string{"绑定条件不能同时声明 all_of 与 any_of"}
	}
	refs = append(refs, condition.AllOf...)
	refs = append(refs, condition.AnyOf...)
	if len(refs) == 0 {
		return []string{"绑定条件必须声明 all_of 或 any_of 的至少一个目标节点"}
	}
	issues := make([]string, 0)
	for _, ref := range refs {
		if ref.NodeID == "" {
			issues = append(issues, "绑定条件的目标节点缺少 node_id")
		}
		if ref.SuccessAction != "complete" && ref.SuccessAction != "approve" {
			issues = append(issues, fmt.Sprintf("节点 %s 的 success_action 只支持 complete/approve", ref.NodeID))
		}
	}
	return issues
}

func (aggregate GuardAggregate) validate() []string {
	switch aggregate.Mode {
	case GuardAggregateAll, GuardAggregateAny:
		return nil
	case GuardAggregateCount:
		if aggregate.MinCount < 1 {
			return []string{"aggregate.mode=count 需要 min_count >= 1"}
		}
		return nil
	case GuardAggregateRatio:
		if aggregate.MinRatio <= 0 || aggregate.MinRatio > 1 {
			return []string{"aggregate.mode=ratio 需要 0 < min_ratio <= 1"}
		}
		return nil
	default:
		return []string{"aggregate.mode 只支持 all/any/count/ratio"}
	}
}

// PlanRequirementRef locates one requirement currently scheduled to a plan
// unit together with its workflow binding. WorkflowInstanceID is 0 when the
// requirement has no workflow instance yet; such requirements stay in the
// denominator as undeterminable evidence.
type PlanRequirementRef struct {
	RequirementID      uint
	Status             string
	WorkflowInstanceID uint
	TemplateVersionID  uint
}

// PlanRequirementScopeReader exposes the current requirement scope of a plan
// unit. Implemented by the application composition root over the requirement
// module; the caller's transaction decides isolation.
type PlanRequirementScopeReader interface {
	PlanUnitRequirementRefs(ctx context.Context, workspaceID, planUnitID uint) ([]PlanRequirementRef, error)
}

// GuardBlockedError is returned by the action stores when a guarded node's
// success action was attempted while the aggregate condition is unmet. It
// carries the evaluation so surfaces with richer protocols can render the
// blocking detail.
type GuardBlockedError struct {
	Evaluation GuardEvaluation
}

func (blocked *GuardBlockedError) Error() string {
	return "workflow requirements guard blocked the action"
}

// Is makes every *GuardBlockedError match the ErrRequirementsGuardBlocked
// sentinel for errors.Is-based classification.
func (blocked *GuardBlockedError) Is(target error) bool {
	_, ok := target.(*GuardBlockedError)
	return ok
}

// ErrRequirementsGuardBlocked is the classification sentinel.
var ErrRequirementsGuardBlocked error = &GuardBlockedError{}

// GuardRequirementInput carries one scoped requirement with its workflow
// evidence. Definition/Runs come from the requirement's own workflow instance;
// Status is the requirement's business stage synced by that workflow (used by
// the stage_status source).
type GuardRequirementInput struct {
	RequirementID       uint
	TemplateVersionID   uint
	Status              string
	HasWorkflowInstance bool
	Definition          *Definition
	Runs                []GuardHistoryRun
}

type GuardNodeEvidence struct {
	NodeID string `json:"node_id"`
	Name   string `json:"name,omitempty"`
	State  string `json:"state"`
}

type GuardRequirementEvidence struct {
	RequirementID     uint                `json:"requirement_id"`
	TemplateVersionID uint                `json:"template_version_id,omitempty"`
	Bound             bool                `json:"bound"`
	State             string              `json:"state"`
	Reason            string              `json:"reason,omitempty"`
	Targets           []GuardNodeEvidence `json:"targets,omitempty"`
}

type GuardEvaluation struct {
	State           string                     `json:"state"`
	SatisfiedCount  int                        `json:"satisfied_count"`
	TotalCount      int                        `json:"total_count"`
	UnknownCount    int                        `json:"unknown_count"`
	Requirements    []GuardRequirementEvidence `json:"requirements"`
	MissingBindings []uint                     `json:"missing_binding_version_ids,omitempty"`
}

// Satisfied reports whether the guard lets the guarded action proceed.
func (evaluation GuardEvaluation) Satisfied() bool {
	return evaluation.State == GuardSatisfied
}

// Evaluate computes the aggregate result over the scoped requirement
// evidence. Unknown facts never count as success: with any unknown evidence
// the evaluation stays undeterminable, even when a ratio threshold is met.
func (guard *RequirementsGuard) Evaluate(inputs []GuardRequirementInput) GuardEvaluation {
	evaluation := GuardEvaluation{State: GuardSatisfied, TotalCount: len(inputs), Requirements: make([]GuardRequirementEvidence, 0, len(inputs))}
	if len(inputs) == 0 {
		evaluation.State = GuardEmptyScope
		if guard.EmptyScope != GuardPolicyBlock {
			evaluation.State = GuardSatisfied
		}
		return evaluation
	}
	if guard.Source == GuardSourceStageStatus {
		return guard.evaluateStageStatus(inputs)
	}
	bindings := make(map[uint]GuardBinding, len(guard.Bindings))
	for _, binding := range guard.Bindings {
		bindings[binding.RequirementTemplateVersionID] = binding
	}
	for _, input := range inputs {
		evidence := guard.evaluateRequirement(input, bindings)
		evaluation.Requirements = append(evaluation.Requirements, evidence)
		switch evidence.State {
		case GuardSatisfied:
			evaluation.SatisfiedCount++
		case GuardUnknown:
			evaluation.UnknownCount++
			if evidence.TemplateVersionID != 0 && !evidence.Bound {
				evaluation.MissingBindings = append(evaluation.MissingBindings, evidence.TemplateVersionID)
			}
		}
	}
	evaluation.State = guard.aggregateState(evaluation.SatisfiedCount, evaluation.UnknownCount, len(inputs))
	return evaluation
}

// evaluateStageStatus drives the guard from each requirement's synced
// business stage (需求阶段驱动)：达标 = status 达到目标阶段。没有模板版本、
// 没有绑定概念——任何需求工作流自动纳入。
func (guard *RequirementsGuard) evaluateStageStatus(inputs []GuardRequirementInput) GuardEvaluation {
	evaluation := GuardEvaluation{State: GuardSatisfied, TotalCount: len(inputs), Requirements: make([]GuardRequirementEvidence, 0, len(inputs))}
	for _, input := range inputs {
		state := StageEvidence(input.Status, guard.Stage)
		evidence := GuardRequirementEvidence{
			RequirementID: input.RequirementID,
			Bound:         true,
			State:         state,
			Targets: []GuardNodeEvidence{{
				NodeID: guard.Stage,
				Name:   stageDisplayName(guard.Stage),
				State:  state,
			}},
		}
		if state == GuardUnknown {
			evidence.Reason = "requirement_status_unknown"
		}
		evaluation.Requirements = append(evaluation.Requirements, evidence)
		switch state {
		case GuardSatisfied:
			evaluation.SatisfiedCount++
		case GuardUnknown:
			evaluation.UnknownCount++
		}
	}
	evaluation.State = guard.aggregateState(evaluation.SatisfiedCount, evaluation.UnknownCount, len(inputs))
	return evaluation
}

var stageDisplayNames = map[string]string{
	StageBacklog:     "待梳理",
	StageGrooming:    "梳理中",
	StageDevelopment: "研发中",
	StageAcceptance:  "验收中",
	StageDone:        "已完成",
}

func stageDisplayName(stage string) string {
	if name, ok := stageDisplayNames[stage]; ok {
		return name
	}
	return stage
}

func (guard *RequirementsGuard) evaluateRequirement(input GuardRequirementInput, bindings map[uint]GuardBinding) GuardRequirementEvidence {
	evidence := GuardRequirementEvidence{
		RequirementID:     input.RequirementID,
		TemplateVersionID: input.TemplateVersionID,
	}
	binding, bound := bindings[input.TemplateVersionID]
	if !bound {
		evidence.State = GuardUnknown
		evidence.Reason = "missing_binding"
		if !input.HasWorkflowInstance {
			evidence.TemplateVersionID = 0
		}
		return evidence
	}
	evidence.Bound = true
	target := evaluateGuardBinding(binding, input)
	evidence.State = target.State
	evidence.Reason = target.Reason
	evidence.Targets = target.Targets
	return evidence
}

// evaluateGuardBinding resolves one template-node binding against the
// requirement's effective workflow evidence. Shared by requirements_guard and
// the delivery-progress milestones so both interpret evidence identically.
func evaluateGuardBinding(binding GuardBinding, input GuardRequirementInput) GuardRequirementEvidence {
	evidence := GuardRequirementEvidence{Bound: true}
	if !input.HasWorkflowInstance {
		evidence.State = GuardUnknown
		evidence.Reason = "no_workflow_instance"
		return evidence
	}
	if input.Definition == nil {
		evidence.State = GuardUnknown
		evidence.Reason = "template_version_unavailable"
		return evidence
	}
	statuses := EffectiveNodeStatuses(*input.Definition, input.Runs)
	refs := binding.Condition.AllOf
	combinators := "all_of"
	if len(refs) == 0 {
		refs, combinators = binding.Condition.AnyOf, "any_of"
	}
	evidence.Targets = make([]GuardNodeEvidence, 0, len(refs))
	satisfiedTargets := 0
	for _, ref := range refs {
		target := GuardNodeEvidence{NodeID: ref.NodeID, State: GuardUnknown}
		node, found := input.Definition.Node(ref.NodeID)
		if !found {
			evidence.State = GuardUnknown
			evidence.Reason = "binding_node_missing_from_template"
			evidence.Targets = append(evidence.Targets, target)
			continue
		}
		target.Name = node.Name
		if statuses[ref.NodeID] == "completed" {
			target.State = GuardSatisfied
			satisfiedTargets++
		} else {
			target.State = GuardUnsatisfied
		}
		evidence.Targets = append(evidence.Targets, target)
	}
	if evidence.State == GuardUnknown {
		return evidence
	}
	if (combinators == "all_of" && satisfiedTargets == len(refs)) ||
		(combinators == "any_of" && satisfiedTargets > 0) {
		evidence.State = GuardSatisfied
		return evidence
	}
	evidence.State = GuardUnsatisfied
	evidence.Reason = "binding_targets_not_completed"
	return evidence
}

func (guard *RequirementsGuard) aggregateState(satisfied, unknown, total int) string {
	if unknown > 0 {
		return GuardUnknown
	}
	switch guard.Aggregate.Mode {
	case GuardAggregateAny:
		if satisfied >= 1 {
			return GuardSatisfied
		}
		return GuardUnsatisfied
	case GuardAggregateCount:
		if satisfied >= guard.Aggregate.MinCount {
			return GuardSatisfied
		}
		return GuardUnsatisfied
	case GuardAggregateRatio:
		ratio := float64(satisfied) / float64(total)
		if total > 0 && ratio >= guard.Aggregate.MinRatio {
			return GuardSatisfied
		}
		return GuardUnsatisfied
	default:
		if satisfied == total {
			return GuardSatisfied
		}
		return GuardUnsatisfied
	}
}

func intFloat(raw any) int {
	if value, ok := raw.(float64); ok {
		return int(value)
	}
	return 0
}

func uintFloat(raw any) uint {
	if value, ok := raw.(float64); ok && value > 0 {
		return uint(value)
	}
	return 0
}

func floatField(raw any) float64 {
	if value, ok := raw.(float64); ok {
		return value
	}
	return 0
}

func stringField(raw any) string {
	if value, ok := raw.(string); ok {
		return value
	}
	return ""
}

func listField(raw any) []any {
	if value, ok := raw.([]any); ok {
		return value
	}
	return nil
}
