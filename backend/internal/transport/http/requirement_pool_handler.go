package httpapi

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
)

type requirementPoolBody struct {
	requirementBody
	ProjectSetID    uint  `json:"project_new_id"`
	ExpectedVersion *uint `json:"expected_version"`
}

const requirementRoleOwnersRoute = "/api/v1/platform/requirements/:id/role-owners"

type requirementRoleOwnersBody struct {
	ProductOwnerUserID   optionalValue[uint] `json:"product_owner_user_id"`
	ProductOwnerUserIDs  *[]uint             `json:"product_owner_user_ids"`
	DesignOwnerUserID    optionalValue[uint] `json:"design_owner_user_id"`
	DesignOwnerUserIDs   *[]uint             `json:"design_owner_user_ids"`
	FrontendOwnerUserID  optionalValue[uint] `json:"frontend_owner_user_id"`
	FrontendOwnerUserIDs *[]uint             `json:"frontend_owner_user_ids"`
	BackendOwnerUserID   optionalValue[uint] `json:"backend_owner_user_id"`
	BackendOwnerUserIDs  *[]uint             `json:"backend_owner_user_ids"`
	TesterUserIDs        *[]uint             `json:"tester_user_ids"`
	DeveloperUserIDs     *[]uint             `json:"developer_user_ids"`
	ExpectedVersion      *uint               `json:"expected_version"`
}

func (body requirementRoleOwnersBody) requirementPoolBody() requirementPoolBody {
	return requirementPoolBody{
		requirementBody: requirementBody{
			ProductOwnerUserID: body.ProductOwnerUserID, ProductOwnerUserIDs: body.ProductOwnerUserIDs,
			DesignOwnerUserID: body.DesignOwnerUserID, DesignOwnerUserIDs: body.DesignOwnerUserIDs,
			FrontendOwnerUserID: body.FrontendOwnerUserID, FrontendOwnerUserIDs: body.FrontendOwnerUserIDs,
			BackendOwnerUserID: body.BackendOwnerUserID, BackendOwnerUserIDs: body.BackendOwnerUserIDs,
			TesterUserIDs: body.TesterUserIDs, DeveloperUserIDs: body.DeveloperUserIDs,
		},
		ExpectedVersion: body.ExpectedVersion,
	}
}

func requirementBodyAssignsRoleOwners(body requirementBody) bool {
	return body.ProductOwnerUserID.Set || body.ProductOwnerUserIDs != nil ||
		body.DesignOwnerUserID.Set || body.DesignOwnerUserIDs != nil ||
		body.FrontendOwnerUserID.Set || body.FrontendOwnerUserIDs != nil ||
		body.BackendOwnerUserID.Set || body.BackendOwnerUserIDs != nil ||
		body.TesterUserIDs != nil || body.DeveloperUserIDs != nil
}

func (s *Server) listRequirementPool(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query, valid := s.requirementPoolQuery(c, workspace.ID)
	if !valid {
		return
	}
	limit, offset, all := requirementhttp.PoolPage(c)
	var total int64
	if query.Session(&gorm.Session{}).Count(&total).Error != nil {
		dbFail(c)
		return
	}
	summary, err := loadRequirementPoolSummary(query, total)
	if err != nil {
		dbFail(c)
		return
	}
	var rows []requirementgorm.Requirement
	finder := query.Session(&gorm.Session{}).Order("created_at DESC, id DESC")
	if !all {
		finder = finder.Limit(limit).Offset(offset)
	}
	if finder.Find(&rows).Error != nil {
		dbFail(c)
		return
	}
	overviews, err := s.loadRequirementPoolOverviews(workspace.ID, rows, requirementPoolBusinessDate(c))
	if err != nil {
		dbFail(c)
		return
	}
	data := s.requirementOverviewRows(rows, overviews)
	users, err := s.listUserProfiles(requirementListUserIDs(rows))
	if err != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "summary": summary, "users": users})
}

func (s *Server) createRequirementPool(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body requirementPoolBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.ProjectSetID == 0 || body.Title == nil || strings.TrimSpace(*body.Title) == "" {
		bad(c, "project_new_id and title required")
		return
	}
	if body.WorkflowTemplateID == nil || *body.WorkflowTemplateID == 0 {
		bad(c, "workflow_template_id required")
		return
	}
	if body.PlanUnitID == nil || *body.PlanUnitID == 0 {
		bad(c, "version_task_new_id required")
		return
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", body.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return
	}
	var unit projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ? AND project_new_id = ?", *body.PlanUnitID, workspace.ID, project.ID).First(&unit).Error != nil {
		fail(c, http.StatusNotFound, "plan unit not found")
		return
	}
	requirement, err := s.requirementFromBody(project, unit, body.requirementBody)
	if err != nil {
		bad(c, err.Error())
		return
	}
	applyRequirementOwnerDefaults(&requirement, currentFactUserID(c))
	requirement.Version = 1
	workflowTemplate, workflowVersion, workflowDefinition, workflowOK := s.requirePublishedWorkflow(
		c, workspace.ID, body.WorkflowTemplateID, "requirement",
	)
	if !workflowOK {
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, workspace.ID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if err := projectgorm.EnsurePlanUnitAcceptsNewRequirementsTx(tx, workspace.ID, unit.ID); err != nil {
			return err
		}
		if err := tx.Create(&requirement).Error; err != nil {
			return err
		}
		if err := s.attachInitialRequirementDocument(tx, c, &requirement, body.InternalDocument); err != nil {
			return err
		}
		if _, err := s.createWorkflowInstanceTx(
			tx, c, workspace.ID, workflowTemplate, workflowVersion,
			workflowDefinition, "requirement", requirement.ID,
		); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND workspace_id = ?", requirement.ID, workspace.ID).
			First(&requirement).Error; err != nil {
			return err
		}
		if body.ProductDocURL != nil {
			if err := requirementcover.EnqueueTx(tx, s.cfg, requirement); err != nil {
				return err
			}
		}
		if err := s.recordMutation(tx, c, project.ID, "requirement", requirement.ID, "created", requirement.Title, requirement); err != nil {
			return err
		}
		if err := projectgorm.BumpPlanUnitDeliveryScopeRevisionTx(tx, workspace.ID, unit.ID); err != nil {
			return err
		}
		return recordDeliveryReconcileTx(tx, workspace.ID, unit.ID, "requirement_created")
	}); err != nil {
		if errors.Is(err, projectgorm.ErrPlanUnitRequirementsLocked) {
			failCode(c, http.StatusConflict, apiErrorProjectRequirementsLocked,
				"项目已锁版，不能新建需求；已有需求仍可移入或排期")
			return
		}
		if respondInitialDocumentError(c, err) {
			return
		}
		dbFail(c)
		return
	}
	if err := s.db.Where("id = ? AND workspace_id = ?", requirement.ID, workspace.ID).
		First(&requirement).Error; err != nil {
		dbFail(c)
		return
	}
	created(c, s.requirementDTO(requirement))
}

func (s *Server) getRequirementPool(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirement, _, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	ok(c, s.requirementDTO(requirement))
}

func (s *Server) patchRequirementPool(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	var body requirementPoolBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if requirementBodyAssignsRoleOwners(body.requirementBody) &&
		!s.requireAccessRoute(c, http.MethodPatch, requirementRoleOwnersRoute) {
		return
	}
	if body.InternalDocument != nil {
		bad(c, "internal_document is only supported at creation")
		return
	}
	s.patchRequirementPoolBody(c, id, body)
}

func (s *Server) patchRequirementRoleOwners(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	var body requirementRoleOwnersBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if !requirementBodyAssignsRoleOwners(body.requirementPoolBody().requirementBody) {
		bad(c, "at least one role owner field is required")
		return
	}
	s.patchRequirementPoolBody(c, id, body.requirementPoolBody())
}

func (s *Server) patchRequirementPoolBody(c *gin.Context, id uint, body requirementPoolBody) {
	requirement, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.ExpectedVersion == nil {
		bad(c, "expected_version required")
		return
	}
	if body.WorkflowTemplateID != nil {
		bad(c, "workflow_template_id can only be set at creation")
		return
	}
	updates, err := s.requirementUpdates(project, body.requirementBody)
	if err != nil {
		bad(c, err.Error())
		return
	}
	delete(updates, "version_task_new_id")
	updates["version"] = gorm.Expr("version + 1")
	err = s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&requirementgorm.Requirement{}).
			Where("id = ? AND workspace_id = ? AND version = ?", id, requirement.WorkspaceID, *body.ExpectedVersion).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errRequirementConflict
		}
		if err := tx.Where("id = ? AND workspace_id = ?", id, requirement.WorkspaceID).First(&requirement).Error; err != nil {
			return err
		}
		if body.ProductDocURL != nil {
			if err := requirementcover.EnqueueTx(tx, s.cfg, requirement); err != nil {
				return err
			}
		}
		return s.recordMutation(tx, c, project.ID, "requirement", id, "updated", requirement.Title, requirement)
	})
	if errors.Is(err, errRequirementConflict) {
		fail(c, http.StatusConflict, "requirement changed; refresh and retry")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, s.requirementDTO(requirement))
}

func (s *Server) scheduleRequirement(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirement, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	var body struct {
		PlanUnitID             uint                  `json:"version_task_new_id"`
		ExpectedVersion        uint                  `json:"expected_version"`
		ConfirmSubjectSchedule bool                  `json:"confirm_subject_schedule"`
		Status                 optionalValue[string] `json:"status"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.PlanUnitID == 0 || body.ExpectedVersion == 0 {
		bad(c, "version_task_new_id and expected_version required")
		return
	}
	var unit projectgorm.PlanUnit
	unitLookup := s.db.Where(
		"id = ? AND workspace_id = ? AND project_new_id = ?",
		body.PlanUnitID, requirement.WorkspaceID, project.ID,
	).First(&unit)
	if errors.Is(unitLookup.Error, gorm.ErrRecordNotFound) {
		s.respondRequirementScheduleStale(c, requirement.WorkspaceID, requirement.ID, "version_task_new_id")
		return
	}
	if unitLookup.Error != nil {
		dbFail(c)
		return
	}
	err := workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, requirement.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		scheduleInstances, err := lockRequirementScheduleInstancesTx(
			tx, requirement.WorkspaceID, requirement.ID, unit.ID,
		)
		if err != nil {
			return err
		}
		lockedRequirement, err := lockProjectRequirementTx(
			tx, requirement.WorkspaceID, requirement.ID, project.ID,
		)
		if err != nil {
			return err
		}
		if lockedRequirement.Version != body.ExpectedVersion {
			return errRequirementConflict
		}
		if lockedRequirement.TeamID != requirement.TeamID ||
			lockedRequirement.PlanUnitID != requirement.PlanUnitID {
			return errProjectChildScopeChanged
		}
		requirement = lockedRequirement
		expectedUnits := map[uint]uint{unit.ID: project.ID}
		if requirement.PlanUnitID > 0 {
			expectedUnits[requirement.PlanUnitID] = project.ID
		}
		lockedUnits, err := lockProjectPlanUnitsTx(tx, requirement.WorkspaceID, expectedUnits)
		if err != nil {
			return err
		}
		lockedUnit, exists := lockedUnits[unit.ID]
		if !exists || !planUnitMatchesProject(lockedUnit, project) {
			return errProjectChildScopeChanged
		}
		unit = lockedUnit
		sourceScope := workflowScheduleScopeForProject(project)
		if requirement.PlanUnitID > 0 {
			sourceUnit, exists := lockedUnits[requirement.PlanUnitID]
			if !exists || !planUnitMatchesProject(sourceUnit, project) {
				return errProjectChildScopeChanged
			}
			sourceScope = workflowScheduleScopeForPlanUnit(sourceUnit, project)
		}
		if err := s.validateRequirementTargetScheduleTx(
			tx, c, requirement, &unit, project, scheduleInstances, sourceScope,
			body.ConfirmSubjectSchedule,
		); err != nil {
			return err
		}
		oldPlanUnitID := requirement.PlanUnitID
		result := tx.Model(&requirementgorm.Requirement{}).
			Where("id = ? AND workspace_id = ? AND version = ?", id, requirement.WorkspaceID, body.ExpectedVersion).
			Updates(map[string]any{"version_task_new_id": unit.ID, "team_id": unit.TeamID, "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errRequirementConflict
		}
		if err := tx.Where("id = ? AND workspace_id = ?", id, requirement.WorkspaceID).First(&requirement).Error; err != nil {
			return err
		}
		if oldPlanUnitID != unit.ID {
			affectedUnits := map[uint]bool{unit.ID: true}
			if oldPlanUnitID > 0 {
				affectedUnits[oldPlanUnitID] = true
			}
			unitIDs := make([]uint, 0, len(affectedUnits))
			for affectedUnitID := range affectedUnits {
				unitIDs = append(unitIDs, affectedUnitID)
			}
			sort.Slice(unitIDs, func(left, right int) bool { return unitIDs[left] < unitIDs[right] })
			for _, affectedUnitID := range unitIDs {
				if err := projectgorm.BumpPlanUnitDeliveryScopeRevisionTx(tx, requirement.WorkspaceID, affectedUnitID); err != nil {
					return err
				}
				if err := recordDeliveryReconcileTx(tx, requirement.WorkspaceID, affectedUnitID, "requirement_scheduled"); err != nil {
					return err
				}
			}
		}
		return s.recordMutation(tx, c, project.ID, "requirement", id, "scheduled", requirement.Title, gin.H{"version_task_new_id": unit.ID, "version": requirement.Version})
	})
	if respondWorkflowScheduleError(c, err) {
		return
	}
	if errors.Is(err, errRequirementConflict) {
		s.respondRequirementScheduleStale(c, requirement.WorkspaceID, requirement.ID, "version")
		return
	}
	if errors.Is(err, errProjectChildScopeChanged) {
		s.respondRequirementScheduleStale(c, requirement.WorkspaceID, requirement.ID, "version_task_new_id")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, s.requirementDTO(requirement))
}

func (s *Server) validateRequirementFeishuDocument(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	if _, _, allowed := s.requirementByID(c, id); !allowed {
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	normalized, err := feishu.NormalizeDocumentURL(body.URL, s.cfg.Feishu.DomainAllowlist)
	if err != nil {
		bad(c, err.Error())
		return
	}
	ok(c, gin.H{"valid": true, "fetch_enabled": s.cfg.Feishu.Enabled, "url": normalized})
}

var errRequirementConflict = errors.New("requirement version conflict")
