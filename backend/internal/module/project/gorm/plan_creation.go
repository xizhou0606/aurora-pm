package projectgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *MutationRepository) FindPlanCreation(ctx context.Context, key string) (project.PlanCreationSaga, bool, error) {
	var row PlanCreationSaga
	err := repository.db.WithContext(ctx).Where("apply_key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.PlanCreationSaga{}, false, nil
	}
	if err != nil {
		return project.PlanCreationSaga{}, false, err
	}
	saga, err := planCreationSnapshot(row)
	return saga, true, err
}

func (repository *MutationRepository) BeginPlanCreation(ctx context.Context, command project.PlanMutation) (project.PlanCreationSaga, error) {
	row := PlanCreationSaga{WorkspaceID: command.WorkspaceID, ProjectSetID: command.ProjectSetID,
		WorkflowTemplateID: command.WorkflowTemplateID, ApplyKey: command.ApplyKey, InputDigest: command.InputDigest,
		Status: "in_progress", SubjectStatus: "", ResultJSON: ""}
	var provisional project.PlanMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projectSet, err := lockProjectSet(tx, command.WorkspaceID, command.ProjectSetID)
		if err != nil {
			return err
		}
		if command.PlannedStartAt != nil && projectSet.PlannedStartAt != nil && command.PlannedStartAt.Before(*projectSet.PlannedStartAt) ||
			command.PlannedStartAt != nil && projectSet.PlannedEndAt != nil && command.PlannedStartAt.After(*projectSet.PlannedEndAt) ||
			command.PlannedEndAt != nil && projectSet.PlannedStartAt != nil && command.PlannedEndAt.Before(*projectSet.PlannedStartAt) ||
			command.PlannedEndAt != nil && projectSet.PlannedEndAt != nil && command.PlannedEndAt.After(*projectSet.PlannedEndAt) {
			return project.ErrMutationConflict
		}
		unit := PlanUnit{WorkspaceID: command.WorkspaceID, TeamID: projectSet.TeamID, ProjectSetID: projectSet.ID,
			Title: command.Title, Type: command.Type, Goal: command.Goal, OwnerUserID: command.OwnerUserID,
			PlannedStartAt: command.PlannedStartAt, PlannedEndAt: command.PlannedEndAt,
			RiskLevel: command.RiskLevel, Status: "planning", MutationState: "creating"}
		if err := tx.Create(&unit).Error; err != nil {
			return err
		}
		row.PlanUnitID = unit.ID
		provisional = planMutationResult(unit)
		raw, _ := json.Marshal(provisional)
		row.ResultJSON = string(raw)
		if err := tx.Create(&ToolMutationApply{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey,
			Operation: string(project.PlanCreate), InputDigest: command.InputDigest, ResultJSON: ""}).Error; err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return project.PlanCreationSaga{}, normalizeProjectMutationError(err)
	}
	return planCreationSnapshot(row)
}

func (repository *MutationRepository) RecordPlanCreationWorkflow(ctx context.Context, key string, instanceID uint, subjectStatus string) (project.PlanCreationSaga, error) {
	updated := repository.db.WithContext(ctx).Model(&PlanCreationSaga{}).
		Where("apply_key = ? AND status = ? AND next_step = ? AND workflow_instance_id = 0", key, "in_progress", 0).
		Updates(map[string]any{"workflow_instance_id": instanceID, "subject_status": subjectStatus, "next_step": 1})
	if updated.Error != nil {
		return project.PlanCreationSaga{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return project.PlanCreationSaga{}, project.ErrMutationConflict
	}
	saga, _, err := repository.FindPlanCreation(ctx, key)
	return saga, err
}

func (repository *MutationRepository) CompletePlanCreation(ctx context.Context, key string) (project.PlanMutationResult, error) {
	var result project.PlanMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saga PlanCreationSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ?", key).First(&saga).Error; err != nil {
			return err
		}
		if saga.Status == "succeeded" {
			return json.Unmarshal([]byte(saga.ResultJSON), &result)
		}
		if saga.Status != "in_progress" || saga.NextStep != 1 || saga.WorkflowInstanceID == 0 || saga.SubjectStatus == "" {
			return project.ErrMutationConflict
		}
		var unit PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND workspace_id = ? AND project_new_id = ?", saga.PlanUnitID, saga.WorkspaceID, saga.ProjectSetID,
		).First(&unit).Error; err != nil {
			return normalizeProjectMutationError(err)
		}
		if unit.MutationState != "creating" {
			return project.ErrMutationConflict
		}
		changed := tx.Model(&PlanUnit{}).Where("id = ? AND workspace_id = ? AND mutation_state = ?", unit.ID, unit.WorkspaceID, "creating").
			Updates(map[string]any{"mutation_state": "", "status": saga.SubjectStatus})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		unit.MutationState, unit.Status = "", saga.SubjectStatus
		result = planMutationResult(unit)
		raw, _ := json.Marshal(result)
		ledger := tx.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json = ''", key).Update("result_json", string(raw))
		if ledger.Error != nil {
			return ledger.Error
		}
		if ledger.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		completed := tx.Model(&PlanCreationSaga{}).Where("id = ? AND status = ? AND next_step = ?", saga.ID, "in_progress", 1).
			Updates(map[string]any{"status": "succeeded", "next_step": 2, "result_json": string(raw)})
		if completed.Error != nil {
			return completed.Error
		}
		if completed.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	return result, normalizeProjectMutationError(err)
}

func (repository *MutationRepository) RejectPlanCreation(ctx context.Context, key string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saga PlanCreationSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"apply_key = ? AND status = ? AND next_step = ? AND workflow_instance_id = 0", key, "in_progress", 0,
		).First(&saga).Error; err != nil {
			return normalizeProjectMutationError(err)
		}
		deleted := tx.Where("id = ? AND workspace_id = ? AND project_new_id = ? AND mutation_state = ?",
			saga.PlanUnitID, saga.WorkspaceID, saga.ProjectSetID, "creating").Delete(&PlanUnit{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		updated := tx.Model(&PlanCreationSaga{}).Where("id = ? AND status = ?", saga.ID, "in_progress").Update("status", "rejected")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
}

func planCreationSnapshot(row PlanCreationSaga) (project.PlanCreationSaga, error) {
	var result project.PlanMutationResult
	if row.ResultJSON != "" && json.Unmarshal([]byte(row.ResultJSON), &result) != nil {
		return project.PlanCreationSaga{}, project.ErrMutationConflict
	}
	return project.PlanCreationSaga{WorkspaceID: row.WorkspaceID, ProjectSetID: row.ProjectSetID, PlanUnitID: row.PlanUnitID,
		WorkflowTemplateID: row.WorkflowTemplateID, WorkflowInstanceID: row.WorkflowInstanceID,
		ApplyKey: row.ApplyKey, InputDigest: row.InputDigest, SubjectStatus: row.SubjectStatus,
		Status: row.Status, NextStep: row.NextStep, Result: result}, nil
}

var _ project.PlanCreationStore = (*MutationRepository)(nil)
