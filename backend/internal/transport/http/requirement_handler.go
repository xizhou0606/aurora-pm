package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskmodule "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type initialRequirementDocument struct {
	ResourceID             string `json:"resource_id"`
	ExpectedPolicyRevision uint64 `json:"expected_policy_revision"`
	OperationID            string `json:"operation_id"`
}

type requirementBody struct {
	InternalDocument       *initialRequirementDocument `json:"internal_document"`
	PlanUnitID             *uint                       `json:"version_task_new_id"`
	WorkflowTemplateID     *uint                       `json:"workflow_template_id"`
	ConfirmSubjectSchedule bool                        `json:"confirm_subject_schedule"`
	Title                  *string                     `json:"title"`
	Description            *string                     `json:"description"`
	ProductDocURL          *string                     `json:"product_doc_url"`
	ProductOwnerUserID     optionalValue[uint]         `json:"product_owner_user_id"`
	ProductOwnerUserIDs    *[]uint                     `json:"product_owner_user_ids"`
	DesignURL              *string                     `json:"design_url"`
	DesignOwnerUserID      optionalValue[uint]         `json:"design_owner_user_id"`
	DesignOwnerUserIDs     *[]uint                     `json:"design_owner_user_ids"`
	FrontendOwnerUserID    optionalValue[uint]         `json:"frontend_owner_user_id"`
	FrontendOwnerUserIDs   *[]uint                     `json:"frontend_owner_user_ids"`
	BackendOwnerUserID     optionalValue[uint]         `json:"backend_owner_user_id"`
	BackendOwnerUserIDs    *[]uint                     `json:"backend_owner_user_ids"`
	TesterUserIDs          *[]uint                     `json:"tester_user_ids"`
	DeveloperUserIDs       *[]uint                     `json:"developer_user_ids"`
	AcceptanceCriteria     *string                     `json:"acceptance_criteria"`
	Priority               *string                     `json:"priority"`
	Status                 optionalValue[string]       `json:"status"`
}

type requirementPatchBody struct {
	requirementBody
	ExpectedVersion *uint `json:"expected_version"`
}

func (s *Server) listRequirements(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	projectID := queryUint(c, "project_new_id")
	planUnitID := queryUint(c, "version_task_new_id")
	keyword := strings.TrimSpace(c.Query("q"))
	if projectID == 0 && planUnitID == 0 && keyword == "" {
		bad(c, "project_new_id, version_task_new_id or q required")
		return
	}
	query := s.db.Where("workspace_id = ?", workspace.ID)
	if keyword != "" {
		query = query.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	if projectID > 0 {
		query = query.Where("project_new_id = ?", projectID)
	}
	if planUnitID > 0 {
		var unit projectgorm.PlanUnit
		if s.db.Where("id = ? AND workspace_id = ?", planUnitID, workspace.ID).First(&unit).Error != nil {
			fail(c, http.StatusNotFound, "plan unit not found")
			return
		}
		if projectID > 0 && unit.ProjectSetID != projectID {
			bad(c, "plan unit does not belong to project")
			return
		}
		projectID = unit.ProjectSetID
		query = query.Where("version_task_new_id = ?", planUnitID)
	}
	if projectID > 0 {
		var project projectgorm.ProjectSet
		if s.db.Where("id = ? AND workspace_id = ?", projectID, workspace.ID).First(&project).Error != nil {
			fail(c, http.StatusNotFound, "project set not found")
			return
		}
		if !s.requireProjectAccess(c, workspace.ID, project.ID) {
			return
		}
	}
	// Project data access is enforced even for keyword-only searches: the
	// filter is applied in SQL on requirements.project_new_id, so a workspace
	// member without access to a project never sees its requirements in global
	// search results.
	if !s.applyProjectDataFilter(c, query, "requirements", "project_new_id") {
		return
	}
	limit, offset := pagination(c, 50)
	var total int64
	query.Model(&requirementgorm.Requirement{}).Count(&total)
	var requirements []requirementgorm.Requirement
	if query.Order("id DESC").Limit(limit).Offset(offset).Find(&requirements).Error != nil {
		dbFail(c)
		return
	}
	overviews, err := s.loadRequirementPoolOverviews(workspace.ID, requirements, requirementPoolBusinessDate(c))
	if err != nil {
		dbFail(c)
		return
	}
	rows := s.requirementOverviewRows(requirements, overviews)
	users, err := s.listUserProfiles(requirementListUserIDs(requirements))
	if err != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "users": users})
}

func (s *Server) createRequirement(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body requirementBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.PlanUnitID == nil || *body.PlanUnitID == 0 ||
		body.Title == nil || strings.TrimSpace(*body.Title) == "" {
		bad(c, "version_task_new_id and title required")
		return
	}
	if body.WorkflowTemplateID == nil || *body.WorkflowTemplateID == 0 {
		bad(c, "workflow_template_id required")
		return
	}
	var unit projectgorm.PlanUnit
	if s.db.Where("id = ? AND workspace_id = ?", *body.PlanUnitID, workspace.ID).First(&unit).Error != nil {
		fail(c, http.StatusNotFound, "plan unit not found")
		return
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", unit.ProjectSetID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return
	}
	requirement, err := s.requirementFromBody(project, unit, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	applyRequirementOwnerDefaults(&requirement, currentFactUserID(c))
	workflowTemplate, workflowVersion, workflowDefinition, workflowOK := s.requirePublishedWorkflow(
		c, workspace.ID, body.WorkflowTemplateID, "requirement",
	)
	if !workflowOK {
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
		if err := s.recordMutation(tx, c, project.ID, "requirement", requirement.ID,
			"created", requirement.Title, requirement); err != nil {
			return err
		}
		if err := projectgorm.BumpPlanUnitDeliveryScopeRevisionTx(tx, workspace.ID, unit.ID); err != nil {
			return err
		}
		return recordDeliveryReconcileTx(tx, workspace.ID, unit.ID, "requirement_created")
	})
	if err != nil {
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

func (s *Server) patchRequirement(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirement, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	var patchBody requirementPatchBody
	if c.ShouldBindJSON(&patchBody) != nil {
		bad(c, "invalid body")
		return
	}
	if requirementBodyAssignsRoleOwners(patchBody.requirementBody) &&
		!s.requireAccessRoute(c, http.MethodPatch, requirementRoleOwnersRoute) {
		return
	}
	if patchBody.InternalDocument != nil {
		bad(c, "internal_document is only supported at creation")
		return
	}
	if patchBody.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if patchBody.ExpectedVersion == nil || *patchBody.ExpectedVersion == 0 {
		bad(c, "expected_version must be a positive integer")
		return
	}
	body := patchBody.requirementBody
	if body.WorkflowTemplateID != nil {
		bad(c, "workflow_template_id can only be set at creation")
		return
	}
	expectedVersion := *patchBody.ExpectedVersion
	if requirement.Version != expectedVersion {
		failCode(c, http.StatusConflict, apiErrorRequirementStale, "requirement changed; refresh and retry")
		return
	}
	movePlan, err := s.buildRequirementMovePlan(c, requirement, body)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			fail(c, http.StatusNotFound, "target project not found")
		default:
			bad(c, err.Error())
		}
		return
	}
	if body.ConfirmSubjectSchedule && movePlan == nil {
		scope := workflowScheduleScopeForProject(project)
		if requirement.PlanUnitID > 0 {
			var currentUnit projectgorm.PlanUnit
			if err := s.db.Where(
				"id = ? AND workspace_id = ? AND project_new_id = ?",
				requirement.PlanUnitID, requirement.WorkspaceID, project.ID,
			).First(&currentUnit).Error; err != nil {
				dbFail(c)
				return
			}
			scope = workflowScheduleScopeForPlanUnit(currentUnit, project)
		}
		_ = respondWorkflowScheduleError(c, workflowScheduleConfirmationNotAvailableError(scope))
		return
	}
	assignmentProject := project
	if movePlan != nil {
		assignmentProject = movePlan.targetProject
	}
	updates, err := s.requirementUpdates(assignmentProject, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	applyRequirementMoveUpdates(updates, movePlan)
	updates["version"] = gorm.Expr("version + 1")
	err = workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		if movePlan != nil {
			lockedProjects, err := lockProjectSetsTx(
				tx, requirement.WorkspaceID, []uint{project.ID, movePlan.targetProject.ID},
			)
			if err != nil {
				return err
			}
			lockedProject, exists := lockedProjects[project.ID]
			if !exists {
				return gorm.ErrRecordNotFound
			}
			project = lockedProject
			lockedTargetProject, exists := lockedProjects[movePlan.targetProject.ID]
			if !exists {
				return gorm.ErrRecordNotFound
			}
			movePlan.targetProject = lockedTargetProject
			scheduleInstances, err := lockRequirementScheduleInstancesTx(
				tx, requirement.WorkspaceID, requirement.ID, movePlan.targetUnit.ID,
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
			if lockedRequirement.Version != expectedVersion {
				return errRequirementConflict
			}
			if lockedRequirement.TeamID != requirement.TeamID ||
				lockedRequirement.PlanUnitID != requirement.PlanUnitID {
				return errProjectChildScopeChanged
			}
			requirement = lockedRequirement
			expectedUnits := map[uint]uint{
				movePlan.targetUnit.ID: movePlan.targetProject.ID,
			}
			if movePlan.sourceUnit.ID > 0 {
				expectedUnits[movePlan.sourceUnit.ID] = project.ID
			}
			lockedUnits, err := lockProjectPlanUnitsTx(tx, requirement.WorkspaceID, expectedUnits)
			if err != nil {
				return err
			}
			lockedTargetUnit, targetExists := lockedUnits[movePlan.targetUnit.ID]
			if !targetExists || !planUnitMatchesProject(lockedTargetUnit, movePlan.targetProject) {
				return errProjectChildScopeChanged
			}
			movePlan.targetUnit = lockedTargetUnit
			sourceScope := workflowScheduleScopeForProject(project)
			if movePlan.sourceUnit.ID > 0 {
				lockedSourceUnit, sourceExists := lockedUnits[movePlan.sourceUnit.ID]
				if !sourceExists || !planUnitMatchesProject(lockedSourceUnit, project) {
					return errProjectChildScopeChanged
				}
				movePlan.sourceUnit = lockedSourceUnit
				sourceScope = workflowScheduleScopeForPlanUnit(movePlan.sourceUnit, project)
			}
			if err := s.validateRequirementTargetScheduleTx(
				tx, c, requirement, &movePlan.targetUnit, movePlan.targetProject,
				scheduleInstances, sourceScope, body.ConfirmSubjectSchedule,
			); err != nil {
				return err
			}
			applyRequirementMoveUpdates(updates, movePlan)
		}
		result := tx.Model(&requirementgorm.Requirement{}).
			Where("id = ? AND workspace_id = ? AND version = ?", requirement.ID, requirement.WorkspaceID, expectedVersion).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errRequirementConflict
		}
		if movePlan != nil {
			if err := tx.First(&requirement, requirement.ID).Error; err != nil {
				return err
			}
			if err := moveRequirementTasks(tx, requirement); err != nil {
				return err
			}
		}
		if err := tx.Where("id = ? AND workspace_id = ?", requirement.ID, requirement.WorkspaceID).
			First(&requirement).Error; err != nil {
			return err
		}
		if body.ProductDocURL != nil {
			if err := requirementcover.EnqueueTx(tx, s.cfg, requirement); err != nil {
				return err
			}
		}
		if movePlan != nil {
			if err := s.recordMutationWithDetail(
				tx, c, project.ID, "requirement", requirement.ID,
				"moved", requirement.Title, movePlan.auditDetail(requirement.Title), gin.H{
					"from_project_new_id":      project.ID,
					"from_version_task_new_id": movePlan.sourceUnit.ID,
					"to_project_new_id":        movePlan.targetProject.ID,
					"to_version_task_new_id":   movePlan.targetUnit.ID,
				},
			); err != nil {
				return err
			}
			return s.recordAuditWithDetail(
				tx, c, movePlan.targetProject.ID, "requirement", requirement.ID,
				"moved_in", requirement.Title,
				movePlan.targetAuditDetail(requirement.Title),
			)
		}
		return s.recordMutation(tx, c, project.ID, "requirement", requirement.ID,
			"updated", requirement.Title, updates)
	})
	if respondWorkflowScheduleError(c, err) {
		return
	}
	if errors.Is(err, errRequirementConflict) {
		failCode(c, http.StatusConflict, apiErrorRequirementStale, "requirement changed; refresh and retry")
		return
	}
	if respondProjectChildScopeConflict(c, err) {
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	_ = s.db.First(&requirement, requirement.ID).Error
	ok(c, s.requirementDTO(requirement))
}

func (s *Server) deleteRequirement(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	requirement, project, allowed := s.requirementByID(c, id)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteWorkflowRequirementRuntimeTx(tx, requirement.WorkspaceID, requirement.ID); err != nil {
			return err
		}
		var taskIDs []uint
		if err := tx.Model(&taskgorm.WorkTask{}).Where("workspace_id = ? AND requirement_id = ?",
			project.WorkspaceID, requirement.ID).Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if err := deleteWorkTaskDependents(tx, project.WorkspaceID, taskIDs); err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND requirement_id = ?", project.WorkspaceID, requirement.ID).
			Delete(&taskgorm.WorkTask{}).Error; err != nil {
			return err
		}
		if err := deleteQualityBugsForRequirement(tx, project.WorkspaceID, requirement.ID); err != nil {
			return err
		}
		if err := s.recordMutation(tx, c, project.ID, "requirement", requirement.ID,
			"deleted", requirement.Title, requirement); err != nil {
			return err
		}
		if err := requirementcover.MarkDeletedTx(tx, project.WorkspaceID, requirement.ID); err != nil {
			return err
		}
		return tx.Delete(&requirement).Error
	})
	if errors.Is(err, taskmodule.ErrCascadeDeletionConflict) {
		fail(c, http.StatusConflict, "remove task attachments before deleting the requirement")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true})
}
