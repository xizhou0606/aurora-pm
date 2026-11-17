package httpapi

import (
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errProjectChildScopeChanged = errors.New("project child scope changed")

func lockProjectRequirementTx(
	tx *gorm.DB, workspaceID, requirementID, expectedProjectID uint,
) (requirementgorm.Requirement, error) {
	rows, err := lockProjectRequirementsTx(tx, workspaceID, map[uint]uint{
		requirementID: expectedProjectID,
	})
	if err != nil {
		return requirementgorm.Requirement{}, err
	}
	row, exists := rows[requirementID]
	if !exists {
		return requirementgorm.Requirement{}, errProjectChildScopeChanged
	}
	return row, nil
}

func lockProjectRequirementsTx(
	tx *gorm.DB, workspaceID uint, expectedProjectIDs map[uint]uint,
) (map[uint]requirementgorm.Requirement, error) {
	ids := make([]uint, 0, len(expectedProjectIDs))
	for requirementID := range expectedProjectIDs {
		if requirementID == 0 || expectedProjectIDs[requirementID] == 0 {
			return nil, errProjectChildScopeChanged
		}
		ids = append(ids, requirementID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	rows := make(map[uint]requirementgorm.Requirement, len(ids))
	for _, requirementID := range ids {
		var requirement requirementgorm.Requirement
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", requirementID, workspaceID).
			First(&requirement).Error
		if err != nil {
			return nil, projectChildLockError(err)
		}
		if requirement.ProjectSetID != expectedProjectIDs[requirementID] {
			return nil, errProjectChildScopeChanged
		}
		rows[requirementID] = requirement
	}
	return rows, nil
}

func lockProjectWorkTaskTx(
	tx *gorm.DB, workspaceID, taskID, expectedProjectID uint,
) (taskgorm.WorkTask, error) {
	rows, err := lockProjectWorkTasksTx(tx, workspaceID, map[uint]uint{taskID: expectedProjectID})
	if err != nil {
		return taskgorm.WorkTask{}, err
	}
	row, exists := rows[taskID]
	if !exists {
		return taskgorm.WorkTask{}, errProjectChildScopeChanged
	}
	return row, nil
}

func lockProjectWorkTasksTx(
	tx *gorm.DB, workspaceID uint, expectedProjectIDs map[uint]uint,
) (map[uint]taskgorm.WorkTask, error) {
	ids := make([]uint, 0, len(expectedProjectIDs))
	for taskID := range expectedProjectIDs {
		if taskID == 0 || expectedProjectIDs[taskID] == 0 {
			return nil, errProjectChildScopeChanged
		}
		ids = append(ids, taskID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	rows := make(map[uint]taskgorm.WorkTask, len(ids))
	for _, taskID := range ids {
		var task taskgorm.WorkTask
		err := pmTaskQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("id = ? AND workspace_id = ?", taskID, workspaceID).
			First(&task).Error
		if err != nil {
			return nil, projectChildLockError(err)
		}
		if task.ProjectSetID != expectedProjectIDs[taskID] {
			return nil, errProjectChildScopeChanged
		}
		rows[taskID] = task
	}
	return rows, nil
}

func lockProjectQualityBugTx(
	tx *gorm.DB, workspaceID, bugID, expectedProjectID uint,
) (qualitygorm.QualityBug, error) {
	var bug qualitygorm.QualityBug
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND workspace_id = ?", bugID, workspaceID).
		First(&bug).Error
	if err != nil {
		return qualitygorm.QualityBug{}, projectChildLockError(err)
	}
	if bug.ProjectSetID != expectedProjectID {
		return qualitygorm.QualityBug{}, errProjectChildScopeChanged
	}
	return bug, nil
}

func lockProjectPlanUnitsTx(
	tx *gorm.DB, workspaceID uint, expectedProjectIDs map[uint]uint,
) (map[uint]projectgorm.PlanUnit, error) {
	ids := make([]uint, 0, len(expectedProjectIDs))
	for planUnitID := range expectedProjectIDs {
		if planUnitID == 0 || expectedProjectIDs[planUnitID] == 0 {
			return nil, errProjectChildScopeChanged
		}
		ids = append(ids, planUnitID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	rows := make(map[uint]projectgorm.PlanUnit, len(ids))
	for _, planUnitID := range ids {
		var unit projectgorm.PlanUnit
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", planUnitID, workspaceID).
			First(&unit).Error
		if err != nil {
			return nil, projectChildLockError(err)
		}
		if unit.ProjectSetID != expectedProjectIDs[planUnitID] {
			return nil, errProjectChildScopeChanged
		}
		// A plan-unit deletion saga owns the aggregate while it is in progress.
		// Every legacy HTTP mutation that re-parents or schedules a child goes
		// through this lock, so fail closed instead of racing the post-gate
		// deletion preflight.
		if unit.MutationState != "" {
			return nil, errProjectChildScopeChanged
		}
		rows[planUnitID] = unit
	}
	return rows, nil
}

func projectChildLockError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errProjectChildScopeChanged
	}
	return err
}

func respondProjectChildScopeConflict(c *gin.Context, err error) bool {
	if !errors.Is(err, errProjectChildScopeChanged) {
		return false
	}
	fail(c, http.StatusConflict, "parent scope changed; refresh and retry")
	return true
}

func applyTaskRequirementScope(task *taskgorm.WorkTask, requirement requirementgorm.Requirement) {
	task.TeamID = requirement.TeamID
	task.ProjectSetID = requirement.ProjectSetID
	task.ProjectID = requirement.PlanUnitID
	task.RequirementID = requirement.ID
}

func requirementMatchesProject(requirement requirementgorm.Requirement, project projectgorm.ProjectSet) bool {
	return requirement.WorkspaceID == project.WorkspaceID &&
		requirement.TeamID == project.TeamID &&
		requirement.ProjectSetID == project.ID
}

func planUnitMatchesProject(unit projectgorm.PlanUnit, project projectgorm.ProjectSet) bool {
	return unit.WorkspaceID == project.WorkspaceID &&
		unit.TeamID == project.TeamID &&
		unit.ProjectSetID == project.ID
}

func sameTaskScope(left, right taskgorm.WorkTask) bool {
	return left.TeamID == right.TeamID &&
		left.ProjectSetID == right.ProjectSetID &&
		left.ProjectID == right.ProjectID &&
		left.RequirementID == right.RequirementID
}

func sameQualityBugScope(left, right qualitygorm.QualityBug) bool {
	return left.TeamID == right.TeamID &&
		left.ProjectSetID == right.ProjectSetID &&
		left.ProjectID == right.ProjectID &&
		left.RequirementID == right.RequirementID
}
