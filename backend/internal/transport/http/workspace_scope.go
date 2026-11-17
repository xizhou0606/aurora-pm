package httpapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
)

type workspaceScope struct {
	ProjectSetIDs  []uint
	ProjectIDs     []uint
	RequirementIDs []uint
}

func parseUintListQuery(c *gin.Context, key string) ([]uint, error) {
	rawValues := c.Request.URL.Query()[key]
	if len(rawValues) == 0 {
		return nil, nil
	}
	seen := map[uint]struct{}{}
	values := make([]uint, 0)
	for _, raw := range rawValues {
		for _, part := range strings.Split(raw, ",") {
			value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
			if err != nil || value == 0 {
				return nil, fmt.Errorf("%s contains an invalid id", key)
			}
			id := uint(value)
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			values = append(values, id)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values, nil
}

func (s *Server) parseWorkspaceScope(c *gin.Context) (workspaceScope, error) {
	workspace, _ := currentWorkspace(c)
	scope := workspaceScope{}
	var err error
	if scope.ProjectSetIDs, err = parseUintListQuery(c, "project_set_ids"); err != nil {
		return scope, err
	}
	if scope.ProjectIDs, err = parseUintListQuery(c, "project_ids"); err != nil {
		return scope, err
	}
	if scope.RequirementIDs, err = parseUintListQuery(c, "requirement_ids"); err != nil {
		return scope, err
	}

	workspaceProjects := map[uint]struct{}{}
	if len(scope.ProjectSetIDs) > 0 {
		var projects []projectgorm.ProjectSet
		if err := s.db.Select("id").Where(
			"workspace_id = ? AND id IN ?", workspace.ID, scope.ProjectSetIDs,
		).Find(&projects).Error; err != nil {
			return scope, err
		}
		for _, project := range projects {
			workspaceProjects[project.ID] = struct{}{}
		}
		if len(workspaceProjects) != len(scope.ProjectSetIDs) {
			return scope, fmt.Errorf("project_set_ids contains a project set outside the workspace")
		}
	}
	if err := s.validatePlanUnitScope(workspace.ID, scope.ProjectSetIDs, scope.ProjectIDs); err != nil {
		return scope, err
	}
	if err := s.validateRequirementScope(workspace.ID, scope.ProjectSetIDs, scope.RequirementIDs); err != nil {
		return scope, err
	}
	return scope, nil
}

func (s *Server) validatePlanUnitScope(workspaceID uint, projectSetIDs, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var rows []projectgorm.PlanUnit
	if err := s.db.Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return fmt.Errorf("project_ids contains a missing project")
	}
	if len(projectSetIDs) > 0 {
		allowed := uintSet(projectSetIDs)
		for _, row := range rows {
			if _, ok := allowed[row.ProjectSetID]; ok {
				continue
			}
			return fmt.Errorf("project_ids contains a project outside the workspace")
		}
	}
	return nil
}

func (s *Server) validateRequirementScope(workspaceID uint, projectSetIDs, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var rows []requirementgorm.Requirement
	if err := s.db.Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return fmt.Errorf("requirement_ids contains a missing requirement")
	}
	if len(projectSetIDs) > 0 {
		allowed := uintSet(projectSetIDs)
		for _, row := range rows {
			if _, ok := allowed[row.ProjectSetID]; ok {
				continue
			}
			return fmt.Errorf("requirement_ids contains a requirement outside the workspace")
		}
	}
	return nil
}

func applyWorkspaceScope(query *gorm.DB, scope workspaceScope) *gorm.DB {
	if len(scope.ProjectSetIDs) > 0 {
		query = query.Where("project_set_id IN ?", scope.ProjectSetIDs)
	}
	if len(scope.ProjectIDs) > 0 {
		query = query.Where("project_id IN ?", scope.ProjectIDs)
	}
	if len(scope.RequirementIDs) > 0 {
		query = query.Where("requirement_id IN ?", scope.RequirementIDs)
	}
	return query
}

// applyWorkspaceHierarchyScope rejects stale or corrupt rows whose denormalized
// PM scope does not resolve to one consistent workspace-owned hierarchy.
func applyWorkspaceHierarchyScope(query *gorm.DB, table string, workspaceID uint) *gorm.DB {
	return query.Where(fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM project_sets ps
		JOIN teams tm ON tm.id = ps.team_id AND tm.workspace_id = ps.workspace_id
		JOIN plan_units pu ON pu.id = %[1]s.project_id
		 AND pu.workspace_id = ps.workspace_id
		 AND pu.project_new_id = ps.id
		 AND pu.team_id = ps.team_id
		JOIN requirements req ON req.id = %[1]s.requirement_id
		 AND req.workspace_id = ps.workspace_id
		 AND req.project_new_id = ps.id
		 AND req.version_task_new_id = pu.id
		 AND req.team_id = ps.team_id
		WHERE ps.id = %[1]s.project_set_id
		 AND ps.workspace_id = ?
		 AND %[1]s.workspace_id = ps.workspace_id
		 AND %[1]s.team_id = ps.team_id
	)`, table), workspaceID)
}

func (s *Server) validateWorkspaceHierarchy(
	workspaceID uint,
	project projectgorm.ProjectSet,
	requirement requirementgorm.Requirement,
) error {
	if project.WorkspaceID != workspaceID || requirement.WorkspaceID != workspaceID ||
		requirement.ProjectSetID != project.ID || requirement.TeamID != project.TeamID {
		return fmt.Errorf("requirement does not belong to project workspace")
	}
	var count int64
	if err := s.db.Model(&identitygorm.Team{}).
		Where("id = ? AND workspace_id = ?", project.TeamID, workspaceID).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("project team does not belong to workspace")
	}
	count = 0
	if err := s.db.Model(&projectgorm.PlanUnit{}).
		Where("id = ? AND workspace_id = ? AND project_new_id = ? AND team_id = ?",
			requirement.PlanUnitID, workspaceID, project.ID, project.TeamID).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("requirement plan unit does not belong to project workspace")
	}
	return nil
}

func escapedLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(strings.ToLower(strings.TrimSpace(value))) + "%"
}

func parseCSVStrings(raw string) []string {
	values := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(part))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
