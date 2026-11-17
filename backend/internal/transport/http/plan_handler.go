package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

var errPlanUnitHasWorkflow = errors.New("plan unit has workflow history")
var errPlanUnitHasRequirements = errors.New("plan unit has requirements")

type planUnitBody struct {
	ProjectSetID                  uint                  `json:"project_new_id"`
	WorkflowTemplateID            *uint                 `json:"workflow_template_id"`
	Title                         *string               `json:"title"`
	Type                          *string               `json:"type"`
	Goal                          *string               `json:"goal"`
	OwnerUserID                   optionalValue[uint]   `json:"owner_user_id"`
	PlannedStartAt                optionalValue[string] `json:"planned_start_at"`
	PlannedEndAt                  optionalValue[string] `json:"planned_end_at"`
	DesignFreezeDueAt             optionalValue[string] `json:"design_freeze_due_at"`
	DevelopmentCompleteAt         optionalValue[string] `json:"development_complete_due_at"`
	TestAcceptanceDueAt           optionalValue[string] `json:"test_acceptance_due_at"`
	ActualStartAt                 optionalValue[string] `json:"actual_start_at"`
	ActualCompletedAt             optionalValue[string] `json:"actual_completed_at"`
	ExpectedPlannedStartAt        optionalValue[string] `json:"expected_planned_start_at"`
	ExpectedPlannedEndAt          optionalValue[string] `json:"expected_planned_end_at"`
	ExpectedDesignFreezeDueAt     optionalValue[string] `json:"expected_design_freeze_due_at"`
	ExpectedDevelopmentCompleteAt optionalValue[string] `json:"expected_development_complete_due_at"`
	ExpectedTestAcceptanceDueAt   optionalValue[string] `json:"expected_test_acceptance_due_at"`
	ExpectedActualStartAt         optionalValue[string] `json:"expected_actual_start_at"`
	ExpectedActualCompletedAt     optionalValue[string] `json:"expected_actual_completed_at"`
	RiskLevel                     *string               `json:"risk_level"`
	Status                        optionalValue[string] `json:"status"`
	DeliveryNodes                 []deliveryNodeBody    `json:"delivery_nodes"`
	DeliveryLabels                []deliveryLabelInput  `json:"delivery_node_labels"`
}

type deliveryNodeBody struct {
	LabelID           *uint                 `json:"label_id"`
	LabelClientKey    string                `json:"label_client_key"`
	LabelName         string                `json:"label_name"`
	Type              string                `json:"type"`
	DueAt             optionalValue[string] `json:"due_at"`
	ActualCompletedAt optionalValue[string] `json:"actual_completed_at"`
	OwnerUserID       optionalValue[uint]   `json:"owner_user_id"`
}

func (s *Server) listPlanUnits(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	projectID := queryUint(c, "project_new_id")
	if projectID == 0 {
		bad(c, "project_new_id required")
		return
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", projectID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return
	}
	var units []projectgorm.PlanUnit
	unitsQuery := s.db.Where("workspace_id = ? AND project_new_id = ?", workspace.ID, project.ID).
		Order("id DESC")
	// Version-level visibility: the set creator (and admins, via the empty
	// fast-path filter) see every unit; other members only the units they own
	// or participate in — a visible set never exposes siblings.
	if !s.applyProjectDataFilter(c, unitsQuery, "plan_units", "id") {
		return
	}
	if err := unitsQuery.Find(&units).Error; err != nil {
		dbFail(c)
		return
	}
	ok(c, s.planUnitDTORows(c, units))
}

func (s *Server) createPlanUnit(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body planUnitBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.ProjectSetID == 0 ||
		body.Title == nil || strings.TrimSpace(*body.Title) == "" {
		bad(c, "project_new_id and title required")
		return
	}
	if body.WorkflowTemplateID == nil || *body.WorkflowTemplateID == 0 {
		bad(c, "workflow_template_id required")
		return
	}
	if err := s.validateActivePMUserReferences(c, planBodyOwnerUserFactIDs(body)); err != nil {
		respondPMUserRegistryValidationError(c, err)
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
	unit, err := s.planUnitFromBody(project, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	workflowTemplate, workflowVersion, workflowDefinition, workflowOK := s.requirePublishedWorkflow(
		c, workspace.ID, body.WorkflowTemplateID, "plan_unit",
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
		conflicts := validatePlanUnitScheduleBoundary(
			planUnitScheduleBoundary(unit), projectSetScheduleBoundary(project),
		)
		if unit.PlannedStartAt != nil || unit.PlannedEndAt != nil {
			projectConflicts := validateProjectSetScheduleBoundary(projectSetScheduleBoundary(project))
			for index := range projectConflicts {
				projectConflicts[index].SubjectType, projectConflicts[index].SubjectID = "project_set", project.ID
			}
			conflicts = append(conflicts, projectConflicts...)
		}
		if len(conflicts) > 0 {
			for index := range conflicts {
				if conflicts[index].SubjectType == "" {
					conflicts[index].SubjectType = "plan_unit"
				}
			}
			return workflowScheduleScopeConflictError(workflowScheduleScopeForPlanUnit(unit, project), nil, conflicts)
		}
		if err := tx.Create(&unit).Error; err != nil {
			return err
		}
		if err := s.createInitialDeliveryLabels(tx, project, unit, body.DeliveryLabels); err != nil {
			return err
		}
		if err := s.replaceDeliveryNodes(tx, project, unit, body.DeliveryNodes); err != nil {
			return err
		}
		if _, err := s.createWorkflowInstanceTx(
			tx, c, workspace.ID, workflowTemplate, workflowVersion,
			workflowDefinition, "plan_unit", unit.ID,
		); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).
			First(&unit).Error; err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "plan_unit", unit.ID, "created", unit.Title, unit)
	})
	if err != nil {
		if respondWorkflowScheduleError(c, err) {
			return
		}
		bad(c, "plan unit could not be created")
		return
	}
	if err := s.db.Where("id = ? AND workspace_id = ?", unit.ID, workspace.ID).First(&unit).Error; err != nil {
		dbFail(c)
		return
	}
	created(c, s.planUnitDTO(c, unit))
}

func (s *Server) patchPlanUnit(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	unit, project, allowed := s.planUnitByID(c, id)
	if !allowed {
		return
	}
	var body planUnitBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	if body.Status.Set {
		bad(c, workflowManagedStatusMessage)
		return
	}
	if body.WorkflowTemplateID != nil {
		bad(c, "workflow_template_id can only be set at creation")
		return
	}
	if err := s.validateActivePMUserReferences(c, planBodyOwnerUserFactIDs(body)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	updates, err := s.planUnitUpdates(project, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	expectations, err := prepareScheduleDateExpectations(
		scheduleDateCASInput{Field: "planned_start_at", ActualSet: body.PlannedStartAt.Set, ExpectedRaw: body.ExpectedPlannedStartAt},
		scheduleDateCASInput{Field: "planned_end_at", ActualSet: body.PlannedEndAt.Set, ExpectedRaw: body.ExpectedPlannedEndAt},
		scheduleDateCASInput{Field: "design_freeze_due_at", ActualSet: body.DesignFreezeDueAt.Set, ExpectedRaw: body.ExpectedDesignFreezeDueAt},
		scheduleDateCASInput{Field: "development_complete_due_at", ActualSet: body.DevelopmentCompleteAt.Set, ExpectedRaw: body.ExpectedDevelopmentCompleteAt},
		scheduleDateCASInput{Field: "test_acceptance_due_at", ActualSet: body.TestAcceptanceDueAt.Set, ExpectedRaw: body.ExpectedTestAcceptanceDueAt},
		scheduleDateCASInput{Field: "actual_start_at", ActualSet: body.ActualStartAt.Set, ExpectedRaw: body.ExpectedActualStartAt},
		scheduleDateCASInput{Field: "actual_completed_at", ActualSet: body.ActualCompletedAt.Set, ExpectedRaw: body.ExpectedActualCompletedAt},
	)
	if err != nil {
		bad(c, err.Error())
		return
	}
	scheduleTouched := planUnitScheduleTouched(body)
	dateTouched := len(expectations) > 0
	err = workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		if dateTouched {
			var currentUnit projectgorm.PlanUnit
			if err := tx.Where("id = ? AND workspace_id = ?", unit.ID, unit.WorkspaceID).
				First(&currentUnit).Error; err != nil {
				return err
			}
			if !planUnitMatchesProject(currentUnit, project) {
				return errProjectChildScopeChanged
			}
			proposed := currentUnit
			applyPlanUnitUpdates(&proposed, updates)
			boundaryChanged := scheduleTouched && workflowScheduleBoundaryChanged(
				planUnitScheduleBoundary(currentUnit), planUnitScheduleBoundary(proposed),
			)
			if boundaryChanged {
				instances, err := workflowInstancesForPlanUnitTx(tx, unit.WorkspaceID, unit.ID, true)
				if err != nil {
					return err
				}
				lockedUnits, err := lockProjectPlanUnitsTx(tx, unit.WorkspaceID, map[uint]uint{unit.ID: project.ID})
				if err != nil {
					return err
				}
				lockedUnit, exists := lockedUnits[unit.ID]
				if !exists {
					return errProjectChildScopeChanged
				}
				unit = lockedUnit
				proposed = unit
				applyPlanUnitUpdates(&proposed, updates)
				if conflicts := planUnitScheduleDateCASConflicts(expectations, unit); len(conflicts) > 0 {
					return workflowScheduleStaleError(workflowScheduleScopeForPlanUnit(unit, project), conflicts)
				}
				conflicts := validatePlanUnitScheduleMutation(unit, proposed, project)
				for index := range conflicts {
					if conflicts[index].SubjectType == "" {
						conflicts[index].SubjectType, conflicts[index].SubjectID = "plan_unit", unit.ID
					}
				}
				if len(conflicts) > 0 {
					return workflowScheduleScopeConflictError(workflowScheduleScopeForPlanUnit(proposed, project), nil, conflicts)
				}
				if err := validateWorkflowInstancesAgainstPlanUnit(tx, instances, unit, proposed, project); err != nil {
					return err
				}
			} else {
				lockedUnits, err := lockProjectPlanUnitsTx(tx, unit.WorkspaceID, map[uint]uint{unit.ID: project.ID})
				if err != nil {
					return err
				}
				lockedUnit, exists := lockedUnits[unit.ID]
				if !exists {
					return errProjectChildScopeChanged
				}
				unit = lockedUnit
				if conflicts := planUnitScheduleDateCASConflicts(expectations, unit); len(conflicts) > 0 {
					return workflowScheduleStaleError(workflowScheduleScopeForPlanUnit(unit, project), conflicts)
				}
			}
		}
		if len(updates) > 0 && tx.Model(&unit).Updates(updates).Error != nil {
			return fmt.Errorf("update plan unit")
		}
		if body.DeliveryNodes != nil {
			if err := s.replaceDeliveryNodes(tx, project, unit, body.DeliveryNodes); err != nil {
				return err
			}
		}
		return s.recordMutation(tx, c, project.ID, "plan_unit", unit.ID, "updated", unit.Title, updates)
	})
	if respondWorkflowScheduleError(c, err) {
		return
	}
	if respondProjectChildScopeConflict(c, err) {
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	var refreshed projectgorm.PlanUnit
	if err := s.db.Where("id = ? AND workspace_id = ?", unit.ID, unit.WorkspaceID).
		First(&refreshed).Error; err != nil {
		dbFail(c)
		return
	}
	unit = refreshed
	ok(c, s.planUnitDTO(c, unit))
}

func planBodyOwnerUserFactIDs(body planUnitBody) []uint {
	ids := make([]uint, 0, len(body.DeliveryNodes)+1)
	if body.OwnerUserID.Set && body.OwnerUserID.Value != nil && *body.OwnerUserID.Value > 0 {
		ids = append(ids, *body.OwnerUserID.Value)
	}
	for _, node := range body.DeliveryNodes {
		if node.OwnerUserID.Set && node.OwnerUserID.Value != nil && *node.OwnerUserID.Value > 0 {
			ids = append(ids, *node.OwnerUserID.Value)
		}
	}
	return ids
}

func (s *Server) deletePlanUnit(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	unit, project, allowed := s.planUnitByID(c, id)
	if !allowed {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		lockedUnits, err := lockProjectPlanUnitsTx(tx, unit.WorkspaceID, map[uint]uint{unit.ID: project.ID})
		if err != nil {
			return err
		}
		lockedUnit, exists := lockedUnits[unit.ID]
		if !exists || !planUnitMatchesProject(lockedUnit, project) {
			return errProjectChildScopeChanged
		}
		unit = lockedUnit
		instanceIDs, err := workflowgorm.SubjectInstanceIDsTx(tx, unit.WorkspaceID, "plan_unit", unit.ID)
		if err != nil {
			return err
		}
		if err := workflowgorm.GateInstancesTx(tx, unit.WorkspaceID, instanceIDs); err != nil {
			return err
		}
		hasWorkflowHistory, err := workflowgorm.InstancesHaveExecutionHistoryTx(tx, unit.WorkspaceID, instanceIDs)
		if err != nil {
			return err
		}
		if !hasWorkflowHistory && len(instanceIDs) > 0 {
			hasWorkflowHistory, err = aigorm.SubjectHasRunsTx(tx, unit.WorkspaceID, "plan_unit", unit.ID)
			if err != nil {
				return err
			}
		}
		if hasWorkflowHistory {
			return errPlanUnitHasWorkflow
		}
		if err := workflowgorm.DeleteInstanceRowsAfterGateTx(tx, unit.WorkspaceID, instanceIDs); err != nil {
			return err
		}
		var childCount int64
		if err := tx.Model(&requirementgorm.Requirement{}).Where(
			"workspace_id = ? AND version_task_new_id = ?", project.WorkspaceID, unit.ID,
		).Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return errPlanUnitHasRequirements
		}
		if err := tx.Where("workspace_id = ? AND plan_unit_id = ?", project.WorkspaceID, unit.ID).
			Delete(&projectgorm.DeliveryNode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND plan_unit_id = ?", project.WorkspaceID, unit.ID).
			Delete(&projectgorm.DeliveryNodeLabel{}).Error; err != nil {
			return err
		}
		if err := s.recordMutation(tx, c, project.ID, "plan_unit", unit.ID, "deleted", unit.Title, unit); err != nil {
			return err
		}
		deleted := tx.Where("id = ? AND workspace_id = ? AND project_new_id = ?", unit.ID, unit.WorkspaceID, project.ID).
			Delete(&projectgorm.PlanUnit{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return errProjectChildScopeChanged
		}
		return nil
	})
	if errors.Is(err, errPlanUnitHasWorkflow) {
		fail(c, http.StatusConflict, "plan unit with workflow history cannot be deleted")
		return
	}
	if errors.Is(err, errPlanUnitHasRequirements) {
		fail(c, http.StatusConflict, "plan unit with requirements cannot be deleted")
		return
	}
	if respondProjectChildScopeConflict(c, err) {
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true})
}
