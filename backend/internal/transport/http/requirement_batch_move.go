package httpapi

import (
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

const maxRequirementBatchMoveItems = 50

type requirementBatchMoveItem struct {
	RequirementID   uint `json:"requirement_id"`
	ExpectedVersion uint `json:"expected_version"`
}

type requirementBatchMoveBody struct {
	Items                  []requirementBatchMoveItem `json:"items"`
	PlanUnitID             uint                       `json:"version_task_new_id"`
	ConfirmSubjectSchedule bool                       `json:"confirm_subject_schedule"`
}

type requirementBatchMoveResult struct {
	RequirementID        uint   `json:"requirement_id"`
	Title                string `json:"title"`
	FromProjectSetID     uint   `json:"from_project_new_id"`
	FromPlanUnitID       uint   `json:"from_version_task_new_id"`
	ToProjectSetID       uint   `json:"to_project_new_id"`
	ToPlanUnitID         uint   `json:"to_version_task_new_id"`
	OldVersion           uint   `json:"old_version"`
	NewVersion           uint   `json:"new_version"`
	WorkflowInstanceID   *uint  `json:"workflow_instance_id,omitempty"`
	AffectedTaskCount    int64  `json:"affected_task_count"`
	AffectedQualityCount int64  `json:"affected_quality_bug_count"`
}

func (s *Server) batchMoveRequirements(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body requirementBatchMoveBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	items, err := normalizeRequirementBatchMoveItems(body.Items)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if body.PlanUnitID == 0 {
		bad(c, "version_task_new_id required")
		return
	}

	var targetUnit projectgorm.PlanUnit
	if err := s.db.Where("id = ? AND workspace_id = ?", body.PlanUnitID, workspace.ID).
		First(&targetUnit).Error; err != nil {
		fail(c, http.StatusNotFound, "target version not found")
		return
	}
	if targetUnit.Type != "version" {
		bad(c, "target plan unit must be a version")
		return
	}
	var targetProject projectgorm.ProjectSet
	if err := s.db.Where("id = ? AND workspace_id = ?", targetUnit.ProjectSetID, workspace.ID).
		First(&targetProject).Error; err != nil {
		fail(c, http.StatusNotFound, "target project not found")
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, targetProject.ID) {
		return
	}

	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.RequirementID)
	}
	var observed []requirementgorm.Requirement
	if err := s.db.Where("workspace_id = ? AND id IN ?", workspace.ID, ids).
		Order("id ASC").Find(&observed).Error; err != nil {
		dbFail(c)
		return
	}
	if len(observed) != len(items) {
		fail(c, http.StatusNotFound, "one or more requirements were not found")
		return
	}
	// Gate source project access for every requirement.
	sourceProjects := map[uint]bool{}
	for _, requirement := range observed {
		if !sourceProjects[requirement.ProjectSetID] {
			if !s.requireProjectAccess(c, workspace.ID, requirement.ProjectSetID) {
				return
			}
			sourceProjects[requirement.ProjectSetID] = true
		}
	}

	batchID := uuid.NewString()
	results := make([]requirementBatchMoveResult, 0, len(items))
	movedCount := 0
	movedSourceUnits := map[uint]bool{}
	err = workflowScheduleTransaction(s.db, func(tx *gorm.DB) error {
		observedByID := make(map[uint]requirementgorm.Requirement, len(observed))
		projectIDs := []uint{targetProject.ID}
		expectedProjects := make(map[uint]uint, len(observed))
		for _, requirement := range observed {
			observedByID[requirement.ID] = requirement
			projectIDs = append(projectIDs, requirement.ProjectSetID)
			expectedProjects[requirement.ID] = requirement.ProjectSetID
		}
		lockedProjects, lockErr := lockProjectSetsTx(tx, workspace.ID, projectIDs)
		if lockErr != nil {
			return errProjectChildScopeChanged
		}
		lockedTargetProject, exists := lockedProjects[targetProject.ID]
		if !exists {
			return errProjectChildScopeChanged
		}
		targetProject = lockedTargetProject

		instancesByRequirement := make(map[uint][]workflowgorm.WorkflowInstance, len(items))
		for _, item := range items {
			instances, lockErr := lockRequirementScheduleInstancesTx(
				tx, workspace.ID, item.RequirementID, targetUnit.ID,
			)
			if lockErr != nil {
				return lockErr
			}
			instancesByRequirement[item.RequirementID] = instances
		}
		lockedRequirements, lockErr := lockProjectRequirementsTx(tx, workspace.ID, expectedProjects)
		if lockErr != nil {
			return lockErr
		}
		expectedUnits := map[uint]uint{targetUnit.ID: targetProject.ID}
		for _, requirement := range lockedRequirements {
			if requirement.PlanUnitID > 0 {
				expectedUnits[requirement.PlanUnitID] = requirement.ProjectSetID
			}
		}
		lockedUnits, lockErr := lockProjectPlanUnitsTx(tx, workspace.ID, expectedUnits)
		if lockErr != nil {
			return lockErr
		}
		lockedTargetUnit, exists := lockedUnits[targetUnit.ID]
		if !exists || !planUnitMatchesProject(lockedTargetUnit, targetProject) || lockedTargetUnit.Type != "version" {
			return errProjectChildScopeChanged
		}
		targetUnit = lockedTargetUnit

		for _, item := range items {
			requirement := lockedRequirements[item.RequirementID]
			observedRequirement := observedByID[item.RequirementID]
			if requirement.Version != item.ExpectedVersion {
				return errRequirementConflict
			}
			if requirement.TeamID != observedRequirement.TeamID ||
				requirement.ProjectSetID != observedRequirement.ProjectSetID ||
				requirement.PlanUnitID != observedRequirement.PlanUnitID {
				return errProjectChildScopeChanged
			}
			sourceProject, exists := lockedProjects[requirement.ProjectSetID]
			if !exists {
				return errProjectChildScopeChanged
			}
			sourceScope := workflowScheduleScopeForProject(sourceProject)
			var sourceUnit projectgorm.PlanUnit
			if requirement.PlanUnitID > 0 {
				sourceUnit, exists = lockedUnits[requirement.PlanUnitID]
				if !exists || !planUnitMatchesProject(sourceUnit, sourceProject) {
					return errProjectChildScopeChanged
				}
				sourceScope = workflowScheduleScopeForPlanUnit(sourceUnit, sourceProject)
			}
			if requirement.PlanUnitID != targetUnit.ID {
				if validateErr := s.validateRequirementTargetScheduleTx(
					tx, c, requirement, &targetUnit, targetProject,
					instancesByRequirement[requirement.ID], sourceScope, body.ConfirmSubjectSchedule,
				); validateErr != nil {
					return validateErr
				}
			}
		}

		for _, item := range items {
			requirement := lockedRequirements[item.RequirementID]
			var taskCount, qualityCount int64
			if err := pmTaskQuery(tx).Model(&taskgorm.WorkTask{}).
				Where("workspace_id = ? AND requirement_id = ?", workspace.ID, requirement.ID).
				Count(&taskCount).Error; err != nil {
				return err
			}
			if err := tx.Model(&qualitygorm.QualityBug{}).
				Where("workspace_id = ? AND requirement_id = ?", workspace.ID, requirement.ID).
				Count(&qualityCount).Error; err != nil {
				return err
			}
			oldProjectID, oldPlanUnitID, oldVersion := requirement.ProjectSetID, requirement.PlanUnitID, requirement.Version
			if oldPlanUnitID != targetUnit.ID {
				movedCount++
				movedSourceUnits[oldPlanUnitID] = true
				updated := tx.Model(&requirementgorm.Requirement{}).
					Where("id = ? AND workspace_id = ? AND version = ?", requirement.ID, workspace.ID, item.ExpectedVersion).
					Updates(map[string]any{
						"team_id": targetUnit.TeamID, "project_new_id": targetProject.ID,
						"version_task_new_id": targetUnit.ID, "version": gorm.Expr("version + 1"),
					})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return errRequirementConflict
				}
				if err := tx.Where("id = ? AND workspace_id = ?", requirement.ID, workspace.ID).
					First(&requirement).Error; err != nil {
					return err
				}
				if err := moveRequirementTasks(tx, requirement); err != nil {
					return err
				}
				sourceProject := lockedProjects[oldProjectID]
				movePlan := requirementMovePlan{sourceUnit: lockedUnits[oldPlanUnitID], targetUnit: targetUnit, targetProject: targetProject}
				if err := s.recordMutationWithDetail(
					tx, c, sourceProject.ID, "requirement", requirement.ID, "moved", requirement.Title,
					movePlan.auditDetail(requirement.Title), gin.H{
						"batch_id": batchID, "from_project_new_id": oldProjectID,
						"from_version_task_new_id": oldPlanUnitID, "to_project_new_id": targetProject.ID,
						"to_version_task_new_id": targetUnit.ID,
					},
				); err != nil {
					return err
				}
				if err := s.recordAuditWithDetail(
					tx, c, targetProject.ID, "requirement", requirement.ID, "moved_in",
					requirement.Title, movePlan.targetAuditDetail(requirement.Title),
				); err != nil {
					return err
				}
			}
			results = append(results, requirementBatchMoveResult{
				RequirementID: requirement.ID, Title: requirement.Title,
				FromProjectSetID: oldProjectID, FromPlanUnitID: oldPlanUnitID,
				ToProjectSetID: targetProject.ID, ToPlanUnitID: targetUnit.ID,
				OldVersion: oldVersion, NewVersion: requirement.Version,
				WorkflowInstanceID: requirement.WorkflowInstanceID,
				AffectedTaskCount:  taskCount, AffectedQualityCount: qualityCount,
			})
		}
		// The current requirement set changed on both ends: bump the scope
		// revisions and ask the delivery consumer to re-evaluate gates and
		// progress for every affected plan unit.
		if movedCount > 0 {
			affectedUnits := map[uint]bool{targetUnit.ID: true}
			for sourceUnitID := range movedSourceUnits {
				affectedUnits[sourceUnitID] = true
			}
			unitIDs := make([]uint, 0, len(affectedUnits))
			for unitID := range affectedUnits {
				unitIDs = append(unitIDs, unitID)
			}
			sort.Slice(unitIDs, func(left, right int) bool { return unitIDs[left] < unitIDs[right] })
			for _, unitID := range unitIDs {
				if err := projectgorm.BumpPlanUnitDeliveryScopeRevisionTx(tx, workspace.ID, unitID); err != nil {
					return err
				}
				if err := recordDeliveryReconcileTx(tx, workspace.ID, unitID, "requirement_moved"); err != nil {
					return err
				}
			}
		}
		return s.recordMutation(tx, c, targetProject.ID, "requirement_batch", 0, "moved", targetUnit.Title, gin.H{
			"batch_id": batchID, "requirement_ids": ids, "version_task_new_id": targetUnit.ID,
			"moved_count": movedCount,
		})
	})
	if respondWorkflowScheduleError(c, err) {
		return
	}
	if errors.Is(err, errRequirementConflict) {
		failCode(c, http.StatusConflict, apiErrorRequirementStale, "one or more requirements changed; refresh and retry")
		return
	}
	if respondProjectChildScopeConflict(c, err) {
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	var affectedTasks, affectedQuality int64
	for _, item := range results {
		affectedTasks += item.AffectedTaskCount
		affectedQuality += item.AffectedQualityCount
	}
	ok(c, gin.H{
		"batch_id": batchID, "moved_count": movedCount, "items": results,
		"target": gin.H{
			"project_new_id": targetProject.ID, "project_name": targetProject.Name,
			"version_task_new_id": targetUnit.ID, "version_title": targetUnit.Title,
			"team_id": targetUnit.TeamID,
		},
		"affected_task_count": affectedTasks, "affected_quality_bug_count": affectedQuality,
	})
}

func normalizeRequirementBatchMoveItems(items []requirementBatchMoveItem) ([]requirementBatchMoveItem, error) {
	if len(items) == 0 || len(items) > maxRequirementBatchMoveItems {
		return nil, errors.New("items must contain 1-50 requirements")
	}
	seen := make(map[uint]struct{}, len(items))
	result := append([]requirementBatchMoveItem(nil), items...)
	for _, item := range result {
		if item.RequirementID == 0 || item.ExpectedVersion == 0 {
			return nil, errors.New("requirement_id and expected_version must be positive integers")
		}
		if _, exists := seen[item.RequirementID]; exists {
			return nil, errors.New("requirement_id is duplicated")
		}
		seen[item.RequirementID] = struct{}{}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RequirementID < result[j].RequirementID })
	return result, nil
}
