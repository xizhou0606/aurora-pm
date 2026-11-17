package workflowgorm

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

// LoadGuardRequirementInputs assembles the evidence inputs for a guard
// evaluation: the requirement template definitions and the effective node-run
// history of every scoped requirement's workflow instance. Definitions are
// loaded once per template version so a single request reuses them across all
// requirements (第一版批量即时计算，不建物化投影).
func LoadGuardRequirementInputs(tx *gorm.DB, workspaceID uint, refs []workflow.PlanRequirementRef) ([]workflow.GuardRequirementInput, error) {
	inputs := make([]workflow.GuardRequirementInput, 0, len(refs))
	if len(refs) == 0 {
		return inputs, nil
	}
	instanceIDs := make([]uint, 0, len(refs))
	versionIDs := make([]uint, 0, len(refs))
	seenInstances, seenVersions := map[uint]bool{}, map[uint]bool{}
	for _, ref := range refs {
		if ref.WorkflowInstanceID > 0 && !seenInstances[ref.WorkflowInstanceID] {
			instanceIDs = append(instanceIDs, ref.WorkflowInstanceID)
			seenInstances[ref.WorkflowInstanceID] = true
		}
		if ref.TemplateVersionID > 0 && !seenVersions[ref.TemplateVersionID] {
			versionIDs = append(versionIDs, ref.TemplateVersionID)
			seenVersions[ref.TemplateVersionID] = true
		}
	}
	definitions := make(map[uint]*workflow.Definition, len(versionIDs))
	if len(versionIDs) > 0 {
		var versions []WorkflowTemplateVersion
		if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, versionIDs).Find(&versions).Error; err != nil {
			return nil, err
		}
		for _, version := range versions {
			definition, err := workflow.ParseDefinition(version.Definition)
			if err != nil {
				// A version that no longer parses is a data anomaly: the
				// requirement stays in the denominator as undeterminable.
				continue
			}
			copied := definition
			definitions[version.ID] = &copied
		}
	}
	runsByInstance := make(map[uint][]workflow.GuardHistoryRun, len(instanceIDs))
	if len(instanceIDs) > 0 {
		var runs []WorkflowNodeRun
		if err := tx.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
			Order("id").Find(&runs).Error; err != nil {
			return nil, err
		}
		for _, run := range runs {
			runsByInstance[run.InstanceID] = append(runsByInstance[run.InstanceID], workflow.GuardHistoryRun{
				NodeID: run.NodeID, NodeType: run.NodeType, Status: run.Status, OutputJSON: run.OutputJSON,
			})
		}
	}
	for _, ref := range refs {
		input := workflow.GuardRequirementInput{
			RequirementID:       ref.RequirementID,
			Status:              ref.Status,
			TemplateVersionID:   ref.TemplateVersionID,
			HasWorkflowInstance: ref.WorkflowInstanceID > 0,
			Definition:          definitions[ref.TemplateVersionID],
			Runs:                runsByInstance[ref.WorkflowInstanceID],
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}
