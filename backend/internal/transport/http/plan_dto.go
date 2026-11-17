package httpapi

import (
	"math"

	"github.com/gin-gonic/gin"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

// planUnitDTOPrefetch carries the per-unit data that planUnitDTO used to load
// with four queries plus access checks per row. Batch loaders fill it once for
// the whole list so request cost stays flat as the project grows.
type planUnitDTOPrefetch struct {
	people             workspaceContext
	nodes              map[uint][]projectgorm.DeliveryNode
	requirementCounts  map[uint]int64
	stats              map[uint]taskStats
	instances          map[uint]workflowgorm.WorkflowInstance
	manageFacts        map[uint]bool
	delivery           planDeliveryProgressPrefetch
	intakePendingNodes map[uint]string
}

func planUnitIDs(units []projectgorm.PlanUnit) []uint {
	ids := make([]uint, 0, len(units))
	for _, unit := range units {
		ids = append(ids, unit.ID)
	}
	return ids
}

func (s *Server) loadPlanUnitPrefetch(c *gin.Context, units []projectgorm.PlanUnit) planUnitDTOPrefetch {
	ids := planUnitIDs(units)
	prefetch := planUnitDTOPrefetch{
		nodes:              map[uint][]projectgorm.DeliveryNode{},
		requirementCounts:  map[uint]int64{},
		stats:              map[uint]taskStats{},
		instances:          map[uint]workflowgorm.WorkflowInstance{},
		manageFacts:        map[uint]bool{},
		intakePendingNodes: map[uint]string{},
	}
	if len(units) == 0 {
		return prefetch
	}
	workspaceID := units[0].WorkspaceID
	refs := workspaceContextReferences{}
	for _, unit := range units {
		refs.UserIDs = appendOptionalID(refs.UserIDs, unit.OwnerUserID)
	}
	prefetch.people = s.loadWorkspaceContext(workspaceID, refs)
	var nodes []projectgorm.DeliveryNode
	if s.db.Where("workspace_id = ? AND plan_unit_id IN ?", workspaceID, ids).
		Order("id ASC").Find(&nodes).Error == nil {
		for _, node := range nodes {
			prefetch.nodes[node.PlanUnitID] = append(prefetch.nodes[node.PlanUnitID], node)
		}
	}
	var requirementCounts []struct {
		PlanUnitID uint
		Count      int64
	}
	if s.db.Model(&requirementgorm.Requirement{}).
		Select("version_task_new_id AS plan_unit_id, COUNT(*) AS count").
		Where("workspace_id = ? AND version_task_new_id IN ?", workspaceID, ids).
		Group("version_task_new_id").Scan(&requirementCounts).Error == nil {
		for _, row := range requirementCounts {
			prefetch.requirementCounts[row.PlanUnitID] = row.Count
		}
	}
	prefetch.stats = loadTaskStatsByPlanUnit(s.db, workspaceID, ids)
	// Ordered ascending so the last write per unit keeps the latest instance,
	// matching the previous per-unit "ORDER BY id DESC LIMIT 1" lookup.
	var instances []workflowgorm.WorkflowInstance
	if s.db.Where("workspace_id = ? AND subject_type = ? AND subject_id IN ?", workspaceID, "plan_unit", ids).
		Order("id ASC").Find(&instances).Error == nil {
		for _, instance := range instances {
			prefetch.instances[instance.SubjectID] = instance
		}
	}
	prefetch.manageFacts = s.planUnitManageFacts(c, units)
	prefetch.delivery = s.loadPlanDeliveryProgressPrefetch(workspaceID, ids, prefetch.instances)
	prefetch.intakePendingNodes = s.loadPlanUnitIntakePendingNodes(workspaceID, units, prefetch.instances)
	return prefetch
}

// loadPlanUnitIntakePendingNodes 找出流程仍停在需求录入节点且未锁版的计划
// 单元，给列表行的锁版快捷操作提供实例与节点定位。模板版本定义加载失败或
// 编译失败的行不标记待录入，前端就不显示快捷锁版按钮。
func (s *Server) loadPlanUnitIntakePendingNodes(
	workspaceID uint,
	units []projectgorm.PlanUnit,
	instances map[uint]workflowgorm.WorkflowInstance,
) map[uint]string {
	pending := map[uint]string{}
	versionIDs := make([]uint, 0, len(instances))
	seen := map[uint]bool{}
	for _, unit := range units {
		if unit.RequirementsLockedAt != nil {
			continue
		}
		instance, found := instances[unit.ID]
		if !found || instance.Status != "running" {
			continue
		}
		if !seen[instance.TemplateVersionID] {
			seen[instance.TemplateVersionID] = true
			versionIDs = append(versionIDs, instance.TemplateVersionID)
		}
	}
	if len(versionIDs) == 0 {
		return pending
	}
	var versions []workflowgorm.WorkflowTemplateVersion
	if s.db.Where("workspace_id = ? AND id IN ?", workspaceID, versionIDs).Find(&versions).Error != nil {
		return pending
	}
	definitions := map[uint]workflowDefinition{}
	for _, version := range versions {
		if definition, issues := compileWorkflowDefinition(version.Definition); len(issues) == 0 {
			definitions[version.ID] = definition
		}
	}
	for _, unit := range units {
		if unit.RequirementsLockedAt != nil {
			continue
		}
		instance, found := instances[unit.ID]
		if !found || instance.Status != "running" {
			continue
		}
		definition, found := definitions[instance.TemplateVersionID]
		if !found {
			continue
		}
		for _, nodeID := range currentWorkflowNodes(instance.CurrentNodeIDs) {
			node, ok := workflowNodeByID(definition, nodeID)
			if !ok {
				continue
			}
			if workflowNodeRequiresRequirementIntakeLock(instance, node) {
				pending[unit.ID] = node.ID
				break
			}
		}
	}
	return pending
}

// planUnitManageFacts evaluates the member-management fact for every unit with
// the admin/active checks hoisted out of the loop. It fails closed: units
// missing from the result map are not manageable.
func (s *Server) planUnitManageFacts(c *gin.Context, units []projectgorm.PlanUnit) map[uint]bool {
	actor, ok := currentActor(c)
	if !ok {
		return map[uint]bool{}
	}
	actorID := factUserID(actor.User)
	if batcher, ok := s.projectAccess.(project.PlanUnitManageFactBatcher); ok {
		facts, err := batcher.CanManagePlanUnitMembersBatch(c.Request.Context(), units[0].WorkspaceID, planUnitIDs(units), actorID)
		if err == nil {
			return facts
		}
		return map[uint]bool{}
	}
	facts := make(map[uint]bool, len(units))
	for _, unit := range units {
		facts[unit.ID] = s.canManagePlanUnitMembers(c, unit)
	}
	return facts
}

func (s *Server) planUnitDTORows(c *gin.Context, units []projectgorm.PlanUnit) []gin.H {
	prefetch := s.loadPlanUnitPrefetch(c, units)
	rows := make([]gin.H, 0, len(units))
	for _, unit := range units {
		rows = append(rows, s.planUnitDTOWithPrefetch(c, unit, prefetch))
	}
	return rows
}

func (s *Server) planUnitDTO(c *gin.Context, unit projectgorm.PlanUnit) gin.H {
	return s.planUnitDTOWithPrefetch(c, unit, s.loadPlanUnitPrefetch(c, []projectgorm.PlanUnit{unit}))
}

func (s *Server) planUnitDTOWithPrefetch(
	c *gin.Context,
	unit projectgorm.PlanUnit,
	prefetch planUnitDTOPrefetch,
) gin.H {
	nodes := prefetch.nodes[unit.ID]
	requirementCount := prefetch.requirementCounts[unit.ID]
	taskStats := prefetch.stats[unit.ID]
	progress := 0
	if taskStats.Total > 0 {
		progress = int(math.Round(float64(taskStats.Completed) * 100 / float64(taskStats.Total)))
	}
	dto := gin.H{
		"id": unit.ID, "workspace_id": unit.WorkspaceID, "team_id": unit.TeamID,
		"project_new_id": unit.ProjectSetID, "title": unit.Title, "type": unit.Type,
		"goal": unit.Goal, "owner_user_id": unit.OwnerUserID, "owner_name": userDisplayName(prefetch.people, unit.OwnerUserID),
		"starts_at": unit.PlannedStartAt, "due_at": unit.PlannedEndAt,
		"design_freeze_due_at":        unit.DesignFreezeDueAt,
		"development_complete_due_at": unit.DevelopmentCompleteAt,
		"test_acceptance_due_at":      unit.TestAcceptanceDueAt,
		"actual_start_at":             unit.ActualStartAt, "actual_completed_at": unit.ActualCompletedAt,
		"risk_level": unit.RiskLevel, "status": unit.Status, "delivery_nodes": nodes,
		"requirement_count": requirementCount, "scrum_task_count": taskStats.Total,
		"total_task_count": taskStats.Total, "completed_task_count": taskStats.Completed, "progress": progress,
		"requirements_locked":            unit.RequirementsLockedAt != nil,
		"requirements_locked_at":         unit.RequirementsLockedAt,
		"requirements_locked_by_user_id": unit.RequirementsLockedByUserID,
		"can_manage_participants":        prefetch.manageFacts[unit.ID],
		"created_at":                     unit.CreatedAt, "updated_at": unit.UpdatedAt,
	}
	if instance, found := prefetch.instances[unit.ID]; found {
		dto["workflow_instance_id"] = instance.ID
		dto["workflow_template_id"] = instance.TemplateID
		if nodeID, ok := prefetch.intakePendingNodes[unit.ID]; ok {
			dto["workflow_intake_pending"] = true
			dto["workflow_intake_node_id"] = nodeID
		}
	}
	for key, value := range evaluatePlanDeliveryProgress(prefetch.delivery, unit.ID, unit.DeliveryScopeRevision) {
		dto[key] = value
	}
	return dto
}

func (s *Server) planUnitByID(c *gin.Context, id uint) (projectgorm.PlanUnit, projectgorm.ProjectSet, bool) {
	workspace, _ := currentWorkspace(c)
	var unit projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&unit).Error != nil {
		fail(c, 404, "plan unit not found")
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", unit.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, 404, "project set not found")
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	if !s.requirePlanUnitAccess(c, workspace.ID, unit.ID) {
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	return unit, project, true
}

// planUnitByParam resolves the :id path parameter to an accessible plan unit.
func (s *Server) planUnitByParam(c *gin.Context) (projectgorm.PlanUnit, projectgorm.ProjectSet, bool) {
	id, valid := paramID(c, "id")
	if !valid {
		return projectgorm.PlanUnit{}, projectgorm.ProjectSet{}, false
	}
	return s.planUnitByID(c, id)
}

// requirePlanUnitMemberManage fails with 403 when the caller lacks the
// version-level member-management fact (set creator, unit owner, or access
// administrator).
func (s *Server) requirePlanUnitMemberManage(c *gin.Context, unit projectgorm.PlanUnit) bool {
	if !s.canManagePlanUnitMembers(c, unit) {
		fail(c, 403, "plan unit member modification is not permitted")
		return false
	}
	return true
}

// requirePlanUnitAccess fails with 404 when the caller lacks the
// version-level data-access fact (set creator, unit owner, unit participant,
// or access administrator). An inaccessible unit is indistinguishable from a
// missing one.
func (s *Server) requirePlanUnitAccess(c *gin.Context, workspaceID, planUnitID uint) bool {
	checker, ok := s.projectAccess.(project.PlanUnitAccessChecker)
	if !ok || s.projectAccess == nil {
		fail(c, 404, "plan unit not found")
		return false
	}
	userID := currentProjectAccessUserID(c)
	allowed, err := checker.CanAccessPlanUnit(c.Request.Context(), workspaceID, planUnitID, userID)
	if err != nil || !allowed {
		fail(c, 404, "plan unit not found")
		return false
	}
	return true
}

// canManagePlanUnitMembers evaluates the member-management fact: set creator
// or unit owner (admin handled by the service). Fails closed.
func (s *Server) canManagePlanUnitMembers(c *gin.Context, unit projectgorm.PlanUnit) bool {
	checker, ok := s.projectAccess.(project.PlanUnitAccessChecker)
	if !ok || s.projectAccess == nil {
		return false
	}
	actor, _ := currentActor(c)
	actorID := factUserID(actor.User)
	allowed, err := checker.CanManagePlanUnitMembers(c.Request.Context(), unit.WorkspaceID, unit.ID, actorID)
	return err == nil && allowed
}
