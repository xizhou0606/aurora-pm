package httpapi

import (
	"sort"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

// planDeliveryProgressPrefetch carries per-unit delivery progress facts that
// planUnitDTO used to compute per row. Batch loaders fill it once for the
// whole list so the 交付进度 display stays flat as the project grows.
type planDeliveryProgressPrefetch struct {
	configs map[uint]*workflow.DeliveryProgressConfig
	inputs  map[uint][]workflow.GuardRequirementInput
}

func newPlanDeliveryProgressPrefetch() planDeliveryProgressPrefetch {
	return planDeliveryProgressPrefetch{
		configs: map[uint]*workflow.DeliveryProgressConfig{},
		inputs:  map[uint][]workflow.GuardRequirementInput{},
	}
}

func mapKeysBool(values map[uint]bool) []uint {
	keys := make([]uint, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool { return keys[left] < keys[right] })
	return keys
}

// loadPlanDeliveryProgressPrefetch batch-loads the current requirement scope
// of every plan unit, the pinned template definitions (with their delivery
// progress config) and the effective run evidence, then keeps per-unit
// evaluation inputs ready for planUnitDTOWithPrefetch.
func (s *Server) loadPlanDeliveryProgressPrefetch(
	workspaceID uint,
	unitIDs []uint,
	instances map[uint]workflowgorm.WorkflowInstance,
) planDeliveryProgressPrefetch {
	prefetch := newPlanDeliveryProgressPrefetch()
	if len(unitIDs) == 0 {
		return prefetch
	}
	refsByUnit, err := requirementgorm.PlanUnitsRequirementWorkflowRefs(s.db, workspaceID, unitIDs)
	if err != nil {
		return prefetch
	}
	versionIDs := map[uint]bool{}
	instanceIDs := map[uint]bool{}
	for _, refs := range refsByUnit {
		for _, ref := range refs {
			if ref.WorkflowInstanceID != nil && *ref.WorkflowInstanceID > 0 {
				instanceIDs[*ref.WorkflowInstanceID] = true
			}
			if ref.TemplateVersionID != nil && *ref.TemplateVersionID > 0 {
				versionIDs[*ref.TemplateVersionID] = true
			}
		}
	}
	for _, instance := range instances {
		if instance.TemplateVersionID > 0 {
			versionIDs[instance.TemplateVersionID] = true
		}
	}
	definitions, err := loadWorkflowDefinitionsByVersion(s.db, workspaceID, mapKeysBool(versionIDs))
	if err != nil || len(definitions) == 0 {
		return prefetch
	}
	configsByVersion := map[uint]*workflow.DeliveryProgressConfig{}
	for versionID, parsed := range definitions {
		if config, present, _ := workflow.ParseDeliveryProgress(parsed.DeliveryProgressConfig); present && config != nil {
			configsByVersion[versionID] = config
		}
	}
	if len(configsByVersion) == 0 {
		return prefetch
	}
	runsByInstance, err := loadWorkflowRunsByInstance(s.db, workspaceID, mapKeysBool(instanceIDs))
	if err != nil {
		return prefetch
	}
	for _, unitID := range unitIDs {
		instance, hasInstance := instances[unitID]
		config := configsByVersion[instance.TemplateVersionID]
		if !hasInstance || config == nil {
			continue
		}
		inputs := make([]workflow.GuardRequirementInput, 0, len(refsByUnit[unitID]))
		for _, ref := range refsByUnit[unitID] {
			input := workflow.GuardRequirementInput{RequirementID: ref.RequirementID}
			if ref.TemplateVersionID != nil {
				input.TemplateVersionID = *ref.TemplateVersionID
			}
			if ref.WorkflowInstanceID != nil && *ref.WorkflowInstanceID > 0 {
				input.HasWorkflowInstance = true
				input.Definition = definitions[input.TemplateVersionID]
				input.Runs = runsByInstance[*ref.WorkflowInstanceID]
			}
			inputs = append(inputs, input)
		}
		prefetch.configs[unitID] = config
		prefetch.inputs[unitID] = inputs
	}
	return prefetch
}

// loadWorkflowDefinitionsByVersion parses the pinned template definitions once
// per version; definitions that no longer parse are skipped so a single data
// anomaly cannot break the whole list response.
func loadWorkflowDefinitionsByVersion(db *gorm.DB, workspaceID uint, versionIDs []uint) (map[uint]*workflow.Definition, error) {
	if len(versionIDs) == 0 {
		return map[uint]*workflow.Definition{}, nil
	}
	var versions []workflowgorm.WorkflowTemplateVersion
	if err := db.Where("workspace_id = ? AND id IN ?", workspaceID, versionIDs).Find(&versions).Error; err != nil {
		return nil, err
	}
	definitions := make(map[uint]*workflow.Definition, len(versions))
	for _, version := range versions {
		parsed, err := workflow.ParseDefinition(version.Definition)
		if err != nil {
			continue
		}
		copied := parsed
		definitions[version.ID] = &copied
	}
	return definitions, nil
}

func loadWorkflowRunsByInstance(db *gorm.DB, workspaceID uint, instanceIDs []uint) (map[uint][]workflow.GuardHistoryRun, error) {
	runsByInstance := map[uint][]workflow.GuardHistoryRun{}
	if len(instanceIDs) == 0 {
		return runsByInstance, nil
	}
	var runs []workflowgorm.WorkflowNodeRun
	if err := db.Where("workspace_id = ? AND instance_id IN ?", workspaceID, instanceIDs).
		Order("id").Find(&runs).Error; err != nil {
		return nil, err
	}
	for _, run := range runs {
		runsByInstance[run.InstanceID] = append(runsByInstance[run.InstanceID], workflow.GuardHistoryRun{
			NodeID: run.NodeID, NodeType: run.NodeType, Status: run.Status, OutputJSON: run.OutputJSON,
		})
	}
	return runsByInstance, nil
}

// evaluatePlanDeliveryProgress computes the DTO projection for one unit.
func evaluatePlanDeliveryProgress(
	prefetch planDeliveryProgressPrefetch,
	unitID uint,
	scopeRevision int64,
) gin.H {
	config := prefetch.configs[unitID]
	if config == nil {
		return gin.H{"delivery_progress_configured": false, "delivery_scope_revision": scopeRevision}
	}
	result := config.Evaluate(prefetch.inputs[unitID])
	// 展示口径：向下截断到 0.1%，未全部完成时绝不因舍入显示 100%。
	display := floorToTenthPercent(result.Progress)
	return gin.H{
		"delivery_progress_configured":   true,
		"delivery_progress":              display,
		"delivery_scope_count":           result.ScopeCount,
		"delivery_accepted_count":        result.AcceptedCount,
		"delivery_acceptance_configured": result.AcceptanceConfigured,
		"delivery_unknown_count":         result.UnknownCount,
		"delivery_milestones":            result.Milestones,
		"delivery_scope_revision":        scopeRevision,
	}
}

func floorToTenthPercent(progress float64) float64 {
	if progress >= 1 {
		return 100
	}
	return float64(int(progress*1000)) / 10
}
