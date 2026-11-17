package requirementgorm

import "gorm.io/gorm"

// PlanRequirementWorkflowRef is one requirement scheduled to a plan unit with
// its workflow binding. Nil pointers mean "no workflow instance bound yet".
type PlanRequirementWorkflowRef struct {
	PlanUnitID         uint `gorm:"column:version_task_new_id"`
	RequirementID      uint
	Status             string
	WorkflowInstanceID *uint
	TemplateVersionID  *uint
}

// PlanUnitRequirementWorkflowRefs lists the current requirement scope of a
// plan unit. Requirements moved away disappear from the scope; deleted rows
// are hard-deleted. The scope is re-read live on every guard evaluation so
// moved-in requirements change the denominator on the next evaluation.
func PlanUnitRequirementWorkflowRefs(tx *gorm.DB, workspaceID, planUnitID uint) ([]PlanRequirementWorkflowRef, error) {
	refs := make([]PlanRequirementWorkflowRef, 0)
	err := tx.Table("requirements r").
		Select("r.id AS requirement_id, r.status, wi.id AS workflow_instance_id, wi.template_version_id AS template_version_id").
		Joins("LEFT JOIN workflow_instances wi ON wi.id = r.workflow_instance_id AND wi.workspace_id = r.workspace_id").
		Where("r.workspace_id = ? AND r.version_task_new_id = ?", workspaceID, planUnitID).
		Order("r.id").
		Scan(&refs).Error
	return refs, err
}

// PlanUnitsRequirementWorkflowRefs is the batch form over many plan units so
// list endpoints keep request cost flat as the project grows.
func PlanUnitsRequirementWorkflowRefs(tx *gorm.DB, workspaceID uint, planUnitIDs []uint) (map[uint][]PlanRequirementWorkflowRef, error) {
	if len(planUnitIDs) == 0 {
		return map[uint][]PlanRequirementWorkflowRef{}, nil
	}
	refs := make([]PlanRequirementWorkflowRef, 0)
	err := tx.Table("requirements r").
		Select("r.version_task_new_id, r.id AS requirement_id, r.status, wi.id AS workflow_instance_id, wi.template_version_id AS template_version_id").
		Joins("LEFT JOIN workflow_instances wi ON wi.id = r.workflow_instance_id AND wi.workspace_id = r.workspace_id").
		Where("r.workspace_id = ? AND r.version_task_new_id IN ?", workspaceID, planUnitIDs).
		Order("r.id").
		Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	grouped := make(map[uint][]PlanRequirementWorkflowRef, len(planUnitIDs))
	for _, ref := range refs {
		grouped[ref.PlanUnitID] = append(grouped[ref.PlanUnitID], ref)
	}
	return grouped, nil
}
