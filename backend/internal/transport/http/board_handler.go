package httpapi

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) board(c *gin.Context) {
	projectID := queryUint(c, "project_new_id")
	project, allowed := s.projectByQuery(c, projectID)
	if !allowed {
		return
	}
	included := includeSet(c.Query("include"))
	payload := gin.H{"project": s.projectDTO(c, project)}
	var units []projectgorm.PlanUnit
	var requirements []requirementgorm.Requirement
	var tasks []taskgorm.WorkTask
	// Version-level visibility: within an accessible set the creator (and
	// admins, via the empty fast-path filter) see everything; other members
	// only the units they own or participate in, plus those units'
	// requirements and tasks.
	if included("version_tasks") {
		unitsQuery := s.db.Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
			Order("id DESC")
		if !s.applyProjectDataFilter(c, unitsQuery, "plan_units", "id") {
			return
		}
		unitsQuery.Find(&units)
	}
	if included("requirements") {
		requirementsQuery := s.db.Where("workspace_id = ? AND project_new_id = ?", project.WorkspaceID, project.ID).
			Order("id DESC")
		if !s.applyProjectDataFilter(c, requirementsQuery, "requirements", "project_new_id") {
			return
		}
		requirementsQuery.Find(&requirements)
	}
	if included("tasks") {
		tasksQuery := pmTaskQuery(s.db).
			Where("workspace_id = ? AND project_set_id = ?", project.WorkspaceID, project.ID).
			Order("id DESC")
		if !s.applyProjectDataFilter(c, tasksQuery, "work_tasks", "project_set_id") {
			return
		}
		tasksQuery.Find(&tasks)
	}
	if included("version_tasks") {
		payload["version_tasks"] = s.planUnitDTORows(c, units)
	}
	if included("requirements") {
		requirementStats := loadTaskStatsByRequirement(s.db, project.WorkspaceID, requirementIDs(requirements))
		rows := make([]gin.H, 0, len(requirements))
		for _, requirement := range requirements {
			rows = append(rows, s.requirementDTOWithTaskStats(requirement, requirementStats[requirement.ID]))
		}
		payload["requirements"] = rows
	}
	if included("tasks") {
		rows := make([]gin.H, 0, len(tasks))
		for _, task := range tasks {
			rows = append(rows, taskDTO(task))
		}
		payload["tasks"] = rows
	}
	ok(c, payload)
}

func includeSet(raw string) func(string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return func(string) bool { return true }
	}
	values := map[string]struct{}{}
	for _, value := range strings.Split(raw, ",") {
		values[strings.TrimSpace(value)] = struct{}{}
	}
	return func(key string) bool {
		_, exists := values[key]
		return exists
	}
}

// scopeOptions feeds the workspace-wide project-set / plan-unit /
// requirement filter cascader. It is project data: the project_sets query is
// restricted to the caller's accessible projects, and plan units and
// requirements inherit that scope because they are only queried for the
// accessible project IDs.
func (s *Server) scopeOptions(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query := s.db.Select("id", "team_id", "name", "code").Where("workspace_id = ?", workspace.ID)
	if !s.applyProjectDataFilter(c, query, "project_sets", "id") {
		return
	}
	var projects []projectgorm.ProjectSet
	if query.Order("updated_at DESC, id DESC").Find(&projects).Error != nil {
		dbFail(c)
		return
	}
	projectRows := make([]gin.H, 0, len(projects))
	projectIDs := make([]uint, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
		projectRows = append(projectRows, gin.H{
			"id": project.ID, "team_id": project.TeamID, "name": project.Name, "code": project.Code,
		})
	}
	var unitRows []struct {
		ID                   uint       `json:"id"`
		TeamID               uint       `json:"team_id"`
		ProjectSetID         uint       `gorm:"column:project_new_id" json:"project_new_id"`
		Title                string     `json:"title"`
		RequirementsLockedAt *time.Time `gorm:"column:requirements_locked_at" json:"requirements_locked_at"`
	}
	var requirementRows []struct {
		ID           uint   `json:"id"`
		TeamID       uint   `json:"team_id"`
		ProjectSetID uint   `gorm:"column:project_new_id" json:"project_new_id"`
		PlanUnitID   uint   `gorm:"column:version_task_new_id" json:"version_task_new_id"`
		Title        string `json:"title"`
	}
	if len(projectIDs) > 0 {
		// The cascade mirrors the version-level visibility rule: the creator
		// (and admins, via the empty fast-path filter) see every unit and
		// requirement; other members only the units they own or participate
		// in and those units' requirements.
		unitsQuery := s.db.Table("plan_units").
			Select("id", "team_id", "project_new_id", "title", "requirements_locked_at").
			Where("workspace_id = ? AND project_new_id IN ?", workspace.ID, projectIDs)
		if !s.applyProjectDataFilter(c, unitsQuery, "plan_units", "id") {
			return
		}
		if err := unitsQuery.
			Order("project_new_id ASC, sort_order ASC, id DESC").Find(&unitRows).Error; err != nil {
			dbFail(c)
			return
		}
		requirementsQuery := s.db.Table("requirements").
			Select("requirements.id", "requirements.team_id", "requirements.project_new_id", "requirements.version_task_new_id", "requirements.title").
			Where("requirements.workspace_id = ? AND requirements.project_new_id IN ?", workspace.ID, projectIDs).
			Where(`EXISTS (
				SELECT 1
				FROM plan_units pu
				JOIN project_sets ps ON ps.id = pu.project_new_id
					AND ps.workspace_id = pu.workspace_id
					AND ps.team_id = pu.team_id
				JOIN teams tm ON tm.id = pu.team_id
					AND tm.workspace_id = pu.workspace_id
				WHERE pu.id = requirements.version_task_new_id
					AND pu.workspace_id = requirements.workspace_id
					AND pu.project_new_id = requirements.project_new_id
					AND pu.team_id = requirements.team_id
			)`)
		if !s.applyProjectDataFilter(c, requirementsQuery, "requirements", "project_new_id") {
			return
		}
		if err := requirementsQuery.
			Order("requirements.project_new_id ASC, requirements.version_task_new_id ASC, requirements.id DESC").
			Find(&requirementRows).Error; err != nil {
			dbFail(c)
			return
		}
	}
	// Empty (not null) arrays keep the cascade contract stable for callers
	// with no accessible project.
	if unitRows == nil {
		unitRows = []struct {
			ID                   uint       `json:"id"`
			TeamID               uint       `json:"team_id"`
			ProjectSetID         uint       `gorm:"column:project_new_id" json:"project_new_id"`
			Title                string     `json:"title"`
			RequirementsLockedAt *time.Time `gorm:"column:requirements_locked_at" json:"requirements_locked_at"`
		}{}
	}
	if requirementRows == nil {
		requirementRows = []struct {
			ID           uint   `json:"id"`
			TeamID       uint   `json:"team_id"`
			ProjectSetID uint   `gorm:"column:project_new_id" json:"project_new_id"`
			PlanUnitID   uint   `gorm:"column:version_task_new_id" json:"version_task_new_id"`
			Title        string `json:"title"`
		}{}
	}
	ok(c, gin.H{"project_sets": projectRows, "projects": unitRows, "requirements": requirementRows})
}
