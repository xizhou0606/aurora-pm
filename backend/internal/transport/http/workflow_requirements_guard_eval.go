package httpapi

import (
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

// workflowNodeGuard parses the requirements_guard config of a node. present
// is false when the node carries no guard configuration.
func workflowNodeGuard(node workflowNode) (*workflow.RequirementsGuard, bool) {
	guard, present, err := workflow.ParseRequirementsGuard(node.Config)
	if err != nil || !present {
		return nil, false
	}
	return guard, true
}

// workflowGuardInputsForInstance loads the current requirement scope of the
// plan unit together with each requirement's workflow evidence. Callers pass
// a transaction when the evaluation gates a mutation, so the decision reads
// committed data under the same coordination lock as the write.
func workflowGuardInputsForInstance(tx *gorm.DB, instance workflowgorm.WorkflowInstance) ([]workflow.GuardRequirementInput, error) {
	raw, err := requirementgorm.PlanUnitRequirementWorkflowRefs(tx, instance.WorkspaceID, instance.SubjectID)
	if err != nil {
		return nil, err
	}
	refs := make([]workflow.PlanRequirementRef, 0, len(raw))
	for _, ref := range raw {
		item := workflow.PlanRequirementRef{RequirementID: ref.RequirementID, Status: ref.Status}
		if ref.WorkflowInstanceID != nil {
			item.WorkflowInstanceID = *ref.WorkflowInstanceID
		}
		if ref.TemplateVersionID != nil {
			item.TemplateVersionID = *ref.TemplateVersionID
		}
		refs = append(refs, item)
	}
	return workflowgorm.LoadGuardRequirementInputs(tx, instance.WorkspaceID, refs)
}

// enforceWorkflowRequirementsGuardTx blocks success actions on guarded
// plan-unit nodes until the aggregate condition is satisfied. Reject,
// rollback, and every other recovery action stay available.
func enforceWorkflowRequirementsGuardTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	node workflowNode,
	status string,
) error {
	if status != "completed" || instance.SubjectType != "plan_unit" {
		return nil
	}
	guard, present := workflowNodeGuard(node)
	if !present {
		return nil
	}
	inputs, err := workflowGuardInputsForInstance(tx, instance)
	if err != nil {
		return err
	}
	evaluation := guard.Evaluate(inputs)
	if evaluation.Satisfied() {
		return nil
	}
	return &workflowRequirementsGuardBlocked{evaluation: evaluation}
}

type workflowRequirementsGuardBlocked struct {
	evaluation workflow.GuardEvaluation
}

func (blocked *workflowRequirementsGuardBlocked) Error() string {
	return "workflow requirements guard blocked the action"
}
