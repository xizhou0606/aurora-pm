package httpapi

import (
	"fmt"

	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

// validateWorkflowRequirementsGuardBindings checks requirements_guard
// bindings against the database at template save/validate/publish time: each
// bound version must be a published requirement workflow version in the same
// workspace, and every target node must exist there with a success action
// that matches its node type. Structural checks already ran in
// validateWorkflowGraph; this is the referential half.
func (s *Server) validateWorkflowRequirementsGuardBindings(workspaceID uint, def workflowDefinition) []string {
	issues := make([]string, 0)
	versions := map[uint]*workflowgorm.WorkflowTemplateVersion{}
	definitions := map[uint]*workflow.Definition{}
	for _, node := range def.Nodes {
		guard, present, err := workflow.ParseRequirementsGuard(node.Config)
		if !present {
			continue
		}
		if err != nil || guard == nil {
			continue
		}
		for _, binding := range guard.Bindings {
			issues = append(issues, s.validateGuardBindingRefs(workspaceID, "节点 "+node.ID, binding, versions, definitions)...)
		}
	}
	if def.DeliveryProgressConfig != nil {
		if config, present, err := workflow.ParseDeliveryProgress(def.DeliveryProgressConfig); err == nil && present {
			for _, milestone := range config.Milestones {
				for _, binding := range milestone.Bindings {
					issues = append(issues, s.validateGuardBindingRefs(workspaceID, "交付里程碑 "+milestone.Key, binding, versions, definitions)...)
				}
			}
		}
	}
	return uniqueStrings(issues)
}

// validateGuardBindingRefs checks one template-node binding against the
// workspace's published requirement workflow versions: the version must exist,
// and every target node must exist there with a success action that matches
// its node type (审批→approve，其余→complete).
func (s *Server) validateGuardBindingRefs(
	workspaceID uint,
	label string,
	binding workflow.GuardBinding,
	versions map[uint]*workflowgorm.WorkflowTemplateVersion,
	definitions map[uint]*workflow.Definition,
) []string {
	issues := make([]string, 0)
	versionID := binding.RequirementTemplateVersionID
	if versionID == 0 {
		return issues
	}
	version, definition, ok := s.workflowGuardRequirementVersion(workspaceID, versionID, versions, definitions)
	if !ok {
		return []string{fmt.Sprintf("%s 绑定的需求模板版本 %d 不存在或不属于当前工作区", label, versionID)}
	}
	if version != nil && definition == nil {
		return []string{fmt.Sprintf("%s 绑定的需求模板版本 %d 定义不可解析", label, versionID)}
	}
	refs := make([]workflow.GuardNodeRef, 0, len(binding.Condition.AllOf)+len(binding.Condition.AnyOf))
	refs = append(refs, binding.Condition.AllOf...)
	refs = append(refs, binding.Condition.AnyOf...)
	for _, ref := range refs {
		target, found := definition.Node(ref.NodeID)
		if !found {
			issues = append(issues, fmt.Sprintf("%s 绑定的需求模板版本 %d 中不存在目标节点 %s", label, versionID, ref.NodeID))
			continue
		}
		expected := "complete"
		if target.Type == "approval" {
			expected = "approve"
		}
		if ref.SuccessAction != expected {
			issues = append(issues, fmt.Sprintf(
				"%s 绑定的需求模板版本 %d 中目标节点 %s 的成功动作应为 %s",
				label, versionID, ref.NodeID, expected,
			))
		}
	}
	return issues
}

func (s *Server) workflowGuardRequirementVersion(
	workspaceID uint,
	versionID uint,
	versions map[uint]*workflowgorm.WorkflowTemplateVersion,
	definitions map[uint]*workflow.Definition,
) (*workflowgorm.WorkflowTemplateVersion, *workflow.Definition, bool) {
	version, cached := versions[versionID]
	if !cached {
		var row workflowgorm.WorkflowTemplateVersion
		if s.db.Where("id = ? AND workspace_id = ?", versionID, workspaceID).First(&row).Error != nil {
			versions[versionID] = nil
			return nil, nil, false
		}
		var template workflowgorm.WorkflowTemplate
		if s.db.Select("subject_type").Where("id = ? AND workspace_id = ?", row.TemplateID, workspaceID).
			First(&template).Error != nil || template.SubjectType != "requirement" {
			versions[versionID] = nil
			return nil, nil, false
		}
		rowCopy := row
		version = &rowCopy
		versions[versionID] = version
	}
	if version == nil {
		return nil, nil, false
	}
	definition, cachedDefinition := definitions[version.ID]
	if !cachedDefinition {
		parsed, err := workflow.ParseDefinition(version.Definition)
		if err != nil {
			definitions[version.ID] = nil
			return version, nil, true
		}
		definition = &parsed
		definitions[version.ID] = definition
	}
	return version, definition, definition != nil
}
