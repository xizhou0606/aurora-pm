package workflow

import (
	"fmt"
)

// DeliveryProgressConfig is the template-level 需求交付进度 configuration
// (可配置工作流联动 §0.6/§7): milestones reference the same template-node
// binding conditions as requirements_guard, weighted per milestone. Progress
// comes from requirement workflow evidence only; unknown evidence contributes
// zero weight and is surfaced separately instead of being treated as done.
type DeliveryProgressConfig struct {
	SchemaVersion int                 `json:"schema_version"`
	Milestones    []DeliveryMilestone `json:"milestones"`
}

type DeliveryMilestone struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Weight       float64        `json:"weight"`
	IsAcceptance bool           `json:"is_acceptance,omitempty"`
	Bindings     []GuardBinding `json:"bindings"`
}

const deliveryProgressSchemaVersion = 1

// ParseDeliveryProgress parses the top-level delivery_progress definition
// value. present=false means the template does not configure delivery progress.
func ParseDeliveryProgress(raw any) (*DeliveryProgressConfig, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	value, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("delivery_progress 必须是对象")
	}
	configResult := &DeliveryProgressConfig{
		SchemaVersion: intFloat(value["schema_version"]),
	}
	milestones, _ := value["milestones"].([]any)
	for _, item := range milestones {
		entry, ok := item.(map[string]any)
		if !ok {
			return configResult, true, fmt.Errorf("delivery_progress.milestones 项必须是对象")
		}
		milestone := DeliveryMilestone{
			Key:    stringField(entry["key"]),
			Name:   stringField(entry["name"]),
			Weight: floatField(entry["weight"]),
		}
		if isAcceptance, exists := entry["is_acceptance"]; exists {
			milestone.IsAcceptance, _ = isAcceptance.(bool)
		}
		bindings, _ := entry["bindings"].([]any)
		for _, rawBinding := range bindings {
			bindingEntry, ok := rawBinding.(map[string]any)
			if !ok {
				return configResult, true, fmt.Errorf("里程碑 %s 的绑定项必须是对象", milestone.Key)
			}
			binding := GuardBinding{RequirementTemplateVersionID: uintFloat(bindingEntry["requirement_template_version_id"])}
			condition, ok := bindingEntry["condition"].(map[string]any)
			if !ok {
				return configResult, true, fmt.Errorf("里程碑 %s 绑定缺少 condition 对象", milestone.Key)
			}
			binding.Condition = parseGuardCondition(condition)
			milestone.Bindings = append(milestone.Bindings, binding)
		}
		configResult.Milestones = append(configResult.Milestones, milestone)
	}
	return configResult, true, nil
}

// ValidateStructural checks the delivery progress shape without database
// access; referential binding checks run at template save/publish time.
func (config *DeliveryProgressConfig) ValidateStructural() []string {
	issues := make([]string, 0)
	if config.SchemaVersion != deliveryProgressSchemaVersion {
		issues = append(issues, "delivery_progress.schema_version 必须是 1")
	}
	if len(config.Milestones) == 0 {
		issues = append(issues, "delivery_progress.milestones 至少需要一个交付里程碑")
	}
	totalWeight := 0.0
	acceptanceCount := 0
	seenKeys := map[string]bool{}
	for _, milestone := range config.Milestones {
		if milestone.Key == "" || len(milestone.Key) > 64 {
			issues = append(issues, "里程碑 key 必须是非空且不超过 64 字符")
		}
		if seenKeys[milestone.Key] {
			issues = append(issues, "里程碑 key 重复："+milestone.Key)
		}
		seenKeys[milestone.Key] = true
		if milestone.Weight < 0 {
			issues = append(issues, "里程碑权重不能为负："+milestone.Key)
		}
		totalWeight += milestone.Weight
		if milestone.IsAcceptance {
			acceptanceCount++
		}
		if len(milestone.Bindings) == 0 {
			issues = append(issues, "里程碑至少需要一个需求模板版本映射："+milestone.Key)
		}
		seenVersions := map[uint]bool{}
		for _, binding := range milestone.Bindings {
			if binding.RequirementTemplateVersionID == 0 {
				issues = append(issues, "里程碑绑定缺少 requirement_template_version_id："+milestone.Key)
				continue
			}
			if seenVersions[binding.RequirementTemplateVersionID] {
				issues = append(issues, fmt.Sprintf("里程碑 %s 中需求模板版本 %d 只能绑定一次", milestone.Key, binding.RequirementTemplateVersionID))
			}
			seenVersions[binding.RequirementTemplateVersionID] = true
			issues = append(issues, prefixIssues(validateGuardCondition(binding.Condition), "里程碑 "+milestone.Key+" ")...)
		}
	}
	if totalWeight <= 0 {
		issues = append(issues, "里程碑权重之和必须大于 0")
	}
	if acceptanceCount > 1 {
		issues = append(issues, "最多一个里程碑可以标记 is_acceptance")
	}
	return issues
}

func prefixIssues(issues []string, prefix string) []string {
	result := make([]string, 0, len(issues))
	for _, issue := range issues {
		result = append(result, prefix+issue)
	}
	return result
}

// DeliveryRequirementMilestone records one requirement's per-milestone outcome.
type DeliveryRequirementMilestone struct {
	Key string `json:"key"`
	Met bool   `json:"met"`
}

type DeliveryRequirementResult struct {
	RequirementID uint                           `json:"requirement_id"`
	State         string                         `json:"state"`
	Progress      float64                        `json:"progress"`
	Milestones    []DeliveryRequirementMilestone `json:"milestones"`
	Reason        string                         `json:"reason,omitempty"`
}

type DeliveryMilestoneSummary struct {
	Key            string  `json:"key"`
	Name           string  `json:"name"`
	Weight         float64 `json:"weight"`
	IsAcceptance   bool    `json:"is_acceptance"`
	SatisfiedCount int     `json:"satisfied_count"`
}

type DeliveryProgressResult struct {
	ScopeCount           int                         `json:"scope_count"`
	Progress             float64                     `json:"progress"`
	AcceptedCount        int                         `json:"accepted_count"`
	AcceptanceConfigured bool                        `json:"acceptance_configured"`
	UnknownCount         int                         `json:"unknown_count"`
	Milestones           []DeliveryMilestoneSummary  `json:"milestones"`
	Requirements         []DeliveryRequirementResult `json:"requirements"`
}

// Evaluate computes per-requirement and aggregate delivery progress from the
// scoped requirement evidence. Milestones are weighted per requirement
// identically (任务拆分不影响需求权重)；size weights stay uniform in v1. A
// requirement whose template version has no mapping for a milestone stays in
// the denominator as undeterminable and contributes no confirmed progress.
func (config *DeliveryProgressConfig) Evaluate(inputs []GuardRequirementInput) DeliveryProgressResult {
	result := DeliveryProgressResult{
		ScopeCount:   len(inputs),
		Milestones:   make([]DeliveryMilestoneSummary, 0, len(config.Milestones)),
		Requirements: make([]DeliveryRequirementResult, 0, len(inputs)),
	}
	for _, milestone := range config.Milestones {
		result.Milestones = append(result.Milestones, DeliveryMilestoneSummary{
			Key: milestone.Key, Name: milestone.Name, Weight: milestone.Weight, IsAcceptance: milestone.IsAcceptance,
		})
		if milestone.IsAcceptance {
			result.AcceptanceConfigured = true
		}
	}
	totalWeight := 0.0
	for _, milestone := range config.Milestones {
		totalWeight += milestone.Weight
	}
	if totalWeight <= 0 || len(inputs) == 0 {
		return result
	}
	for _, input := range inputs {
		requirement := DeliveryRequirementResult{RequirementID: input.RequirementID, State: GuardUnsatisfied}
		earned := 0.0
		unknown := false
		acceptanceMet := false
		for index, milestone := range config.Milestones {
			met := false
			milestoneState := GuardUnsatisfied
			milestoneReason := ""
			matched := false
			for _, binding := range milestone.Bindings {
				if binding.RequirementTemplateVersionID != input.TemplateVersionID {
					continue
				}
				matched = true
				evidence := evaluateGuardBinding(binding, input)
				if evidence.State == GuardSatisfied {
					met, milestoneState = true, GuardSatisfied
					break
				}
				if evidence.State == GuardUnknown {
					milestoneState, milestoneReason = GuardUnknown, evidence.Reason
				}
			}
			if !matched {
				milestoneState, milestoneReason = GuardUnknown, "missing_binding"
			}
			if milestoneState == GuardUnknown {
				requirement.State = GuardUnknown
				if requirement.Reason == "" {
					requirement.Reason = milestoneReason
				}
				unknown = true
			}
			if met {
				earned += milestone.Weight
				result.Milestones[index].SatisfiedCount++
				if milestone.IsAcceptance {
					acceptanceMet = true
				}
			}
			requirement.Milestones = append(requirement.Milestones, DeliveryRequirementMilestone{Key: milestone.Key, Met: met})
		}
		requirement.Progress = earned / totalWeight
		switch {
		case unknown:
			result.UnknownCount++
		case requirement.Progress >= 1:
			requirement.State = GuardSatisfied
		}
		if acceptanceMet && !unknown {
			result.AcceptedCount++
		}
		result.Requirements = append(result.Requirements, requirement)
		result.Progress += requirement.Progress
	}
	result.Progress = result.Progress / float64(len(inputs))
	return result
}
