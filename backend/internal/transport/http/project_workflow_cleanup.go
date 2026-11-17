package httpapi

import (
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
)

func deleteProjectWorkflowDependents(tx *gorm.DB, workspaceID, projectID uint) error {
	instanceIDs, err := projectWorkflowInstanceIDs(tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	if err := workflowgorm.DeleteInstanceRowsTx(tx, workspaceID, instanceIDs); err != nil {
		return err
	}
	// Keep the shared write order compatible with workflow transitions,
	// AI application, and task-plan materialization:
	// WorkflowInstance -> subject -> KeySequence -> AIRun.
	if err := lockProjectWorkflowSubjectsTx(tx, workspaceID, projectID); err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND project_set_id = ?", workspaceID, projectID).
		Delete(&taskgorm.KeySequence{}).Error; err != nil {
		return err
	}

	// Query after workflow actions are gated so an AI run queued by an action
	// that was already in flight is included in this deletion.
	runIDs, err := projectAIRunIDs(tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	if len(runIDs) == 0 {
		return nil
	}
	// AIRun is the worker write gate. Deleting it first either waits for the
	// worker transaction or makes its final compare-and-swap roll back.
	if err := tx.Where("workspace_id = ? AND id IN ?", workspaceID, runIDs).
		Delete(&aigorm.AIRun{}).Error; err != nil {
		return err
	}
	for _, row := range []any{
		&aigorm.AIArtifactApplication{},
		&aigorm.AIFeedback{},
		&aigorm.AIArtifact{},
		&aigorm.AIRunSource{},
	} {
		if err := tx.Where("workspace_id = ? AND run_id IN ?", workspaceID, runIDs).
			Delete(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func lockProjectWorkflowSubjectsTx(tx *gorm.DB, workspaceID, projectID uint) error {
	var requirements []requirementgorm.Requirement
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectID).
		Order("id").
		Find(&requirements).Error; err != nil {
		return err
	}
	var planUnits []projectgorm.PlanUnit
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectID).
		Order("id").
		Find(&planUnits).Error
}

func projectWorkflowInstanceIDs(tx *gorm.DB, workspaceID, projectID uint) ([]uint, error) {
	var ids []uint
	err := projectWorkflowSubjectQuery(
		tx.Model(&workflowgorm.WorkflowInstance{}), &workflowgorm.WorkflowInstance{}, workspaceID, projectID,
	).Pluck("id", &ids).Error
	return ids, err
}

func projectAIRunIDs(tx *gorm.DB, workspaceID, projectID uint) ([]uint, error) {
	var ids []uint
	err := projectWorkflowSubjectQuery(
		tx.Model(&aigorm.AIRun{}), &aigorm.AIRun{}, workspaceID, projectID,
	).Pluck("id", &ids).Error
	return ids, err
}

func projectWorkflowSubjectQuery(
	query *gorm.DB,
	row any,
	workspaceID, projectID uint,
) *gorm.DB {
	requirementIDs := query.Session(&gorm.Session{NewDB: true}).
		Model(&requirementgorm.Requirement{}).
		Select("id").
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectID)
	planUnitIDs := query.Session(&gorm.Session{NewDB: true}).
		Model(&projectgorm.PlanUnit{}).
		Select("id").
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectID)
	return query.Model(row).Where(
		"workspace_id = ? AND ((subject_type = ? AND subject_id IN (?)) OR (subject_type = ? AND subject_id IN (?)))",
		workspaceID,
		"requirement",
		requirementIDs,
		"plan_unit",
		planUnitIDs,
	)
}
