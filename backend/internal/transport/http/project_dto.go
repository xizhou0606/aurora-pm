package httpapi

import (
	"math"
	"time"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
)

func (s *Server) projectDTO(c *gin.Context, projectRow projectgorm.ProjectSet) gin.H {
	return s.projectDTORows(c, []projectgorm.ProjectSet{projectRow})[0]
}

// projectDTORows builds list rows with batch-loaded counts so a workspace with
// many project sets pays three aggregate queries instead of three per row.
func (s *Server) projectDTORows(c *gin.Context, projects []projectgorm.ProjectSet) []gin.H {
	if len(projects) == 0 {
		return []gin.H{}
	}
	ids := make([]uint, 0, len(projects))
	refs := workspaceContextReferences{}
	for _, project := range projects {
		ids = append(ids, project.ID)
		refs.UserIDs = appendOptionalID(refs.UserIDs, project.OwnerUserID)
		refs.UserIDs = appendOptionalID(refs.UserIDs, project.CreatedByUserID)
	}
	people := s.loadWorkspaceContext(projects[0].WorkspaceID, refs)
	counts := loadProjectScopeCounts(s.db, projects[0].WorkspaceID, ids)
	rows := make([]gin.H, 0, len(projects))
	for _, project := range projects {
		row := s.projectDTOWithCounts(c, project, counts)
		row["owner_name"] = userDisplayName(people, project.OwnerUserID)
		row["created_by_name"] = userDisplayName(people, project.CreatedByUserID)
		rows = append(rows, row)
	}
	return rows
}

type projectScopeCounts struct {
	planUnits      map[uint]int64
	requirements   map[uint]int64
	taskStatsBySet map[uint]taskStats
}

func loadProjectScopeCounts(db *gorm.DB, workspaceID uint, projectIDs []uint) projectScopeCounts {
	counts := projectScopeCounts{
		planUnits:      map[uint]int64{},
		requirements:   map[uint]int64{},
		taskStatsBySet: map[uint]taskStats{},
	}
	if len(projectIDs) == 0 {
		return counts
	}
	var planCounts []struct {
		ProjectSetID uint
		Count        int64
	}
	if db.Model(&projectgorm.PlanUnit{}).
		Select("project_new_id AS project_set_id, COUNT(*) AS count").
		Where("workspace_id = ? AND project_new_id IN ?", workspaceID, projectIDs).
		Group("project_new_id").Scan(&planCounts).Error == nil {
		for _, row := range planCounts {
			counts.planUnits[row.ProjectSetID] = row.Count
		}
	}
	var requirementCounts []struct {
		ProjectSetID uint
		Count        int64
	}
	if db.Model(&requirementgorm.Requirement{}).
		Select("project_new_id AS project_set_id, COUNT(*) AS count").
		Where("workspace_id = ? AND project_new_id IN ?", workspaceID, projectIDs).
		Group("project_new_id").Scan(&requirementCounts).Error == nil {
		for _, row := range requirementCounts {
			counts.requirements[row.ProjectSetID] = row.Count
		}
	}
	counts.taskStatsBySet = loadTaskStatsByProjectSet(db, workspaceID, projectIDs)
	return counts
}

func (s *Server) projectDTOWithCounts(
	c *gin.Context,
	projectRow projectgorm.ProjectSet,
	counts projectScopeCounts,
) gin.H {
	planCount := counts.planUnits[projectRow.ID]
	requirementCount := counts.requirements[projectRow.ID]
	taskStats := counts.taskStatsBySet[projectRow.ID]
	progress := 0
	if taskStats.Total > 0 {
		progress = int(math.Round(float64(taskStats.Completed) * 100 / float64(taskStats.Total)))
	}
	health, reason := projectHealth(projectRow, progress)

	// The project set has a single membership fact — its creator. Owner and
	// participant management live at the plan unit level.
	return gin.H{
		"id": projectRow.ID, "workspace_id": projectRow.WorkspaceID, "team_id": projectRow.TeamID,
		"name": projectRow.Name, "code": projectRow.Code, "description": projectRow.Description,
		"owner_user_id": projectRow.OwnerUserID, "created_by_user_id": projectRow.CreatedByUserID,
		"starts_at": projectRow.PlannedStartAt,
		"due_at":    projectRow.PlannedEndAt, "status": projectRow.Status,
		"version_task_count": planCount, "requirement_count": requirementCount,
		"scrum_task_count": taskStats.Total, "open_scrum_task_count": taskStats.Open,
		"total_task_count": taskStats.Total, "completed_task_count": taskStats.Completed, "progress": progress,
		"health_status": health, "health_reason": reason,
		"created_at": projectRow.CreatedAt, "updated_at": projectRow.UpdatedAt,
	}
}

func projectHealth(project projectgorm.ProjectSet, progress int) (string, string) {
	if project.Status == "archived" || project.Status == "released" {
		return "on_track", "项目已归档"
	}
	if project.PlannedEndAt != nil && project.PlannedEndAt.Before(time.Now()) && progress < 100 {
		return "off_track", "计划结束日期已过"
	}
	if project.PlannedEndAt != nil && project.PlannedEndAt.Before(time.Now().Add(7*24*time.Hour)) && progress < 80 {
		return "at_risk", "临近计划结束日期"
	}
	return "on_track", "按计划推进"
}
