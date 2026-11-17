package httpapi

import (
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

func workflowSubjectProjectIDForGate(
	tx *gorm.DB,
	workspaceID uint,
	subjectType string,
	subjectID uint,
) (uint, error) {
	projectID := uint(0)
	var err error
	switch subjectType {
	case "requirement":
		var requirement requirementgorm.Requirement
		err = tx.Select("project_new_id").
			Where("id = ? AND workspace_id = ?", subjectID, workspaceID).
			First(&requirement).Error
		projectID = requirement.ProjectSetID
	case "plan_unit":
		var unit projectgorm.PlanUnit
		err = tx.Select("project_new_id").
			Where("id = ? AND workspace_id = ?", subjectID, workspaceID).
			First(&unit).Error
		projectID = unit.ProjectSetID
	default:
		return 0, errProjectChildScopeChanged
	}
	if err != nil {
		return 0, projectChildLockError(err)
	}
	if projectID == 0 {
		return 0, errProjectChildScopeChanged
	}
	return projectID, nil
}

func (s *Server) lockWorkflowSubjectProjectTx(
	tx *gorm.DB,
	instance workflowgorm.WorkflowInstance,
	expectedProjectID uint,
) error {
	if expectedProjectID == 0 {
		return errProjectChildScopeChanged
	}
	if _, err := lockProjectSetTx(tx, instance.WorkspaceID, expectedProjectID); err != nil {
		return projectChildLockError(err)
	}
	// Subject moves are serialized by the project lock. Keep this as a plain
	// reread because normal workflow actions lock their token before syncing the
	// subject row.
	observedProjectID, err := workflowSubjectProjectIDForGate(
		tx, instance.WorkspaceID, instance.SubjectType, instance.SubjectID,
	)
	if err != nil {
		return err
	}
	if observedProjectID != expectedProjectID {
		return errProjectChildScopeChanged
	}
	return nil
}
