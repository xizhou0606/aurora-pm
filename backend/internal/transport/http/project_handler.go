package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

type projectBody struct {
	TeamID                 *uint                 `json:"team_id"`
	Name                   *string               `json:"name"`
	Code                   *string               `json:"code"`
	Description            *string               `json:"description"`
	OwnerUserID            optionalValue[uint]   `json:"owner_user_id"`
	PlannedStartAt         optionalValue[string] `json:"planned_start_at"`
	PlannedEndAt           optionalValue[string] `json:"planned_end_at"`
	ExpectedPlannedStartAt optionalValue[string] `json:"expected_planned_start_at"`
	ExpectedPlannedEndAt   optionalValue[string] `json:"expected_planned_end_at"`
	Status                 *string               `json:"status"`
}

func (s *Server) listProjectSets(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query := s.db.Where("workspace_id = ?", workspace.ID)
	if teamID := queryUint(c, "team_id"); teamID > 0 {
		if !s.teamInWorkspace(teamID, workspace.ID) {
			bad(c, "team_id does not belong to workspace")
			return
		}
		query = query.Where("team_id = ?", teamID)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	if !s.applyProjectDataFilter(c, query, "project_sets", "id") {
		return
	}
	var projects []projectgorm.ProjectSet
	if query.Order("updated_at DESC, id DESC").Find(&projects).Error != nil {
		dbFail(c)
		return
	}
	limit, offset := pagination(c, 20)
	end := offset + limit
	if offset > len(projects) {
		offset = len(projects)
	}
	if end > len(projects) {
		end = len(projects)
	}
	rows := s.projectDTORows(c, projects[offset:end])
	total := len(projects)
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total})
}

func (s *Server) createProjectSet(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body projectBody
	if c.ShouldBindJSON(&body) != nil || body.Name == nil || strings.TrimSpace(*body.Name) == "" {
		bad(c, "name required")
		return
	}
	teamID := uint(0)
	if body.TeamID != nil {
		teamID = *body.TeamID
	}
	if !s.teamInWorkspace(teamID, workspace.ID) {
		bad(c, "team_id does not belong to workspace")
		return
	}
	start, errStart := parseOptionalTime(body.PlannedStartAt.Value)
	end, errEnd := parseOptionalTime(body.PlannedEndAt.Value)
	if errStart != nil || errEnd != nil {
		bad(c, "planned dates are invalid")
		return
	}
	// The project owner is always the authenticated user. The caller cannot
	// specify an arbitrary owner_user_id.
	a, _ := currentActor(c)
	ownerID := factUserID(a.User)
	if ownerID == 0 {
		bad(c, "owner identity is required")
		return
	}
	if err := s.validateActivePMUserReferences(c, []uint{ownerID}); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	status := "active"
	if body.Status != nil {
		if !validProjectStatus(*body.Status) {
			bad(c, "status is invalid")
			return
		}
		status = normalizeStatus(*body.Status)
	}
	project := projectgorm.ProjectSet{
		WorkspaceID: workspace.ID, TeamID: teamID, Name: strings.TrimSpace(*body.Name),
		Description: stringValue(body.Description), OwnerUserID: &ownerID, CreatedByUserID: &ownerID,
		PlannedStartAt: start, PlannedEndAt: end, Status: status,
	}
	if body.Code != nil {
		project.Code = strings.TrimSpace(*body.Code)
	}
	if conflicts := validateProjectSetScheduleBoundary(projectSetScheduleBoundary(project)); len(conflicts) > 0 {
		for index := range conflicts {
			conflicts[index].SubjectType = "project_set"
		}
		_ = respondWorkflowScheduleError(c, workflowScheduleConflictError(projectSetScheduleBoundary(project), nil, conflicts))
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		if project.Code == "" {
			project.Code = fmt.Sprintf("PM-%d", project.ID)
			if err := tx.Model(&project).Update("code", project.Code).Error; err != nil {
				return err
			}
		}
		return s.recordMutation(tx, c, project.ID, "project_set", project.ID, "created", project.Name, project)
	})
	if err != nil {
		bad(c, "project set could not be created")
		return
	}
	created(c, s.projectDTO(c, project))
}

func (s *Server) patchProjectSet(c *gin.Context) {
	project, allowed := s.projectFromParam(c)
	if !allowed {
		return
	}
	var body projectBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	// Owner changes must go through the transactional members/transfer
	// endpoints (removal-impact checks, participant consistency, audit rows).
	// A plain PATCH that flips owner_user_id would bypass all of them.
	if body.OwnerUserID.Set {
		bad(c, "owner_user_id cannot be changed on this endpoint; use the project members transfer endpoint")
		return
	}
	updates, err := s.projectUpdates(c, project, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	expectations, err := prepareScheduleDateExpectations(
		scheduleDateCASInput{Field: "planned_start_at", ActualSet: body.PlannedStartAt.Set, ExpectedRaw: body.ExpectedPlannedStartAt},
		scheduleDateCASInput{Field: "planned_end_at", ActualSet: body.PlannedEndAt.Set, ExpectedRaw: body.ExpectedPlannedEndAt},
	)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if len(updates) == 0 {
		ok(c, s.projectDTO(c, project))
		return
	}
	scheduleTouched := projectSetScheduleTouched(body)
	err = workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if scheduleTouched {
			if conflicts := scheduleDateStaleConflicts(expectations, map[string]*time.Time{
				"planned_start_at": project.PlannedStartAt,
				"planned_end_at":   project.PlannedEndAt,
			}, "project_set", project.ID, ""); len(conflicts) > 0 {
				return workflowScheduleStaleError(workflowScheduleScopeForProject(project), conflicts)
			}
			proposed := project
			if value, exists := updates["planned_start_at"]; exists {
				proposed.PlannedStartAt, _ = value.(*time.Time)
			}
			if value, exists := updates["planned_end_at"]; exists {
				proposed.PlannedEndAt, _ = value.(*time.Time)
			}
			if workflowScheduleBoundaryChanged(
				projectSetScheduleBoundary(project), projectSetScheduleBoundary(proposed),
			) {
				if err := validateProjectSetPlanUnitsTx(tx, project, proposed); err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&project).Updates(updates).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "project_set", project.ID, "updated", project.Name, updates)
	})
	if respondWorkflowScheduleError(c, err) {
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	var refreshed projectgorm.ProjectSet
	if err := s.db.Where("id = ? AND workspace_id = ?", project.ID, project.WorkspaceID).
		First(&refreshed).Error; err != nil {
		dbFail(c)
		return
	}
	project = refreshed
	ok(c, s.projectDTO(c, project))
}

func (s *Server) projectUpdates(c *gin.Context, project projectgorm.ProjectSet, body projectBody) (map[string]any, error) {
	updates := map[string]any{}
	if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	if body.Description != nil {
		updates["description"] = strings.TrimSpace(*body.Description)
	}
	// owner_user_id is intentionally absent: owner changes only go through the
	// transactional members/transfer endpoints. patchProjectSet rejects the
	// field outright; this defense-in-depth drop keeps a stale caller from
	// reintroducing the bypass.
	if body.Status != nil {
		if !validProjectStatus(*body.Status) {
			return nil, fmt.Errorf("status is invalid")
		}
		updates["status"] = normalizeStatus(*body.Status)
	}
	if body.PlannedStartAt.Set {
		value, err := parseOptionalTime(body.PlannedStartAt.Value)
		if err != nil {
			return nil, err
		}
		updates["planned_start_at"] = value
	}
	if body.PlannedEndAt.Set {
		value, err := parseOptionalTime(body.PlannedEndAt.Value)
		if err != nil {
			return nil, err
		}
		updates["planned_end_at"] = value
	}
	return updates, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
