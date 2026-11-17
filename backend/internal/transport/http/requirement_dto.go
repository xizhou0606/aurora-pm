package httpapi

import (
	"math"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
)

func (s *Server) requirementDTO(requirement requirementgorm.Requirement) gin.H {
	return s.requirementDTOWithTaskStats(requirement, loadTaskStats(
		s.db,
		"workspace_id = ? AND requirement_id = ?",
		requirement.WorkspaceID,
		requirement.ID,
	))
}

// requirementDTOWithTaskStats lets list endpoints pass batch-loaded task
// counts instead of paying one aggregate query per row.
func (s *Server) requirementDTOWithTaskStats(requirement requirementgorm.Requirement, taskStats taskStats) gin.H {
	progress := 0
	if taskStats.Total > 0 {
		progress = int(math.Round(float64(taskStats.Completed) * 100 / float64(taskStats.Total)))
	}
	var planUnitID any
	if requirement.PlanUnitID > 0 {
		planUnitID = requirement.PlanUnitID
	}
	return gin.H{
		"id": requirement.ID, "workspace_id": requirement.WorkspaceID,
		"team_id": requirement.TeamID, "project_new_id": requirement.ProjectSetID,
		"version_task_new_id": planUnitID, "title": requirement.Title,
		"version": requirement.Version, "workflow_instance_id": requirement.WorkflowInstanceID,
		"description": requirement.Description, "product_doc_url": requirement.ProductDocURL,
		"product_owner_user_id":   requirement.ProductOwnerUserID,
		"product_owner_user_ids":  ownerListDTO(requirement.ProductOwnerUserIDs),
		"design_url":              requirement.DesignURL,
		"design_owner_user_id":    requirement.DesignOwnerUserID,
		"design_owner_user_ids":   ownerListDTO(requirement.DesignOwnerUserIDs),
		"frontend_owner_user_id":  requirement.FrontendOwnerUserID,
		"frontend_owner_user_ids": ownerListDTO(requirement.FrontendOwnerUserIDs),
		"backend_owner_user_id":   requirement.BackendOwnerUserID,
		"backend_owner_user_ids":  ownerListDTO(requirement.BackendOwnerUserIDs),
		"tester_user_ids":         ownerListDTO(requirement.TesterUserIDs),
		"developer_user_ids":      ownerListDTO(requirement.DeveloperUserIDs),
		"acceptance_criteria":     requirement.AcceptanceCriteria,
		"priority":                requirement.Priority, "status": requirement.Status,
		"task_count": taskStats.Total, "open_task_count": taskStats.Open, "total_task_count": taskStats.Total,
		"completed_task_count": taskStats.Completed, "in_progress_task_count": taskStats.InProgress,
		"todo_task_count": taskStats.Todo, "progress": progress,
		"created_at": requirement.CreatedAt, "updated_at": requirement.UpdatedAt,
		"allowed_actions": []string{"update", "schedule", "run_ai"},
	}
}

func (s *Server) requirementByID(
	c *gin.Context,
	id uint,
) (requirementgorm.Requirement, projectgorm.ProjectSet, bool) {
	workspace, _ := currentWorkspace(c)
	var requirement requirementgorm.Requirement
	if s.db.Where("id = ? AND workspace_id = ?", id, workspace.ID).First(&requirement).Error != nil {
		fail(c, 404, "requirement not found")
		return requirementgorm.Requirement{}, projectgorm.ProjectSet{}, false
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", requirement.ProjectSetID, workspace.ID).
		First(&project).Error != nil {
		fail(c, 404, "project set not found")
		return requirementgorm.Requirement{}, projectgorm.ProjectSet{}, false
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return requirementgorm.Requirement{}, projectgorm.ProjectSet{}, false
	}
	return requirement, project, true
}

func requirementIDs(requirements []requirementgorm.Requirement) []uint {
	ids := make([]uint, 0, len(requirements))
	for _, requirement := range requirements {
		ids = append(ids, requirement.ID)
	}
	return ids
}

// requirementOverviewRows builds list rows with batch-loaded task stats and
// per-requirement workflow/deliverable overviews. Without batching every row
// would pay its own aggregate query and the pool would degrade linearly with
// the workspace size.
func (s *Server) requirementOverviewRows(
	requirements []requirementgorm.Requirement,
	overviews map[uint]requirementPoolOverview,
) []gin.H {
	rows := make([]gin.H, 0, len(requirements))
	if len(requirements) == 0 {
		return rows
	}
	taskStatsByRequirement := loadTaskStatsByRequirement(s.db, requirements[0].WorkspaceID, requirementIDs(requirements))
	for _, requirement := range requirements {
		dto := s.requirementDTOWithTaskStats(requirement, taskStatsByRequirement[requirement.ID])
		overview := overviews[requirement.ID]
		if overview.Workflow != nil {
			dto["workflow_overview"] = overview.Workflow
		}
		dto["deliverable_overview"] = overview.Deliverables
		rows = append(rows, dto)
	}
	return rows
}
