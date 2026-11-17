package projectgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *MutationRepository) ApplyPlanUnit(ctx context.Context, command project.PlanMutation) (project.PlanMutationResult, error) {
	if replay, found, err := repository.replayPlan(ctx, command); found || err != nil {
		return replay, err
	}
	var result project.PlanMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var unit PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", command.WorkspaceID, command.ID).First(&unit).Error; err != nil {
			return normalizeProjectMutationError(err)
		}
		if unit.MutationState != "" {
			return project.ErrMutationConflict
		}
		projectSet, err := lockProjectSet(tx, command.WorkspaceID, unit.ProjectSetID)
		if err != nil {
			return err
		}
		if projectSet.TeamID != unit.TeamID {
			return project.ErrMutationConflict
		}
		proposedStart, proposedEnd := unit.PlannedStartAt, unit.PlannedEndAt
		if command.PlannedStartSet {
			proposedStart = command.PlannedStartAt
		}
		if command.PlannedEndSet {
			proposedEnd = command.PlannedEndAt
		}
		if proposedStart != nil && proposedEnd != nil && proposedStart.After(*proposedEnd) ||
			proposedStart != nil && projectSet.PlannedStartAt != nil && proposedStart.Before(*projectSet.PlannedStartAt) ||
			proposedStart != nil && projectSet.PlannedEndAt != nil && proposedStart.After(*projectSet.PlannedEndAt) ||
			proposedEnd != nil && projectSet.PlannedStartAt != nil && proposedEnd.Before(*projectSet.PlannedStartAt) ||
			proposedEnd != nil && projectSet.PlannedEndAt != nil && proposedEnd.After(*projectSet.PlannedEndAt) {
			return project.ErrMutationConflict
		}
		if command.PlannedStartSet || command.PlannedEndSet {
			if repository.planSchedules == nil {
				return project.ErrMutationConflict
			}
			guard, err := repository.planSchedules(tx)
			if err != nil || guard == nil || guard.ValidatePlanUnitSchedule(ctx, command.WorkspaceID, unit.ID, proposedStart, proposedEnd) != nil {
				return project.ErrMutationConflict
			}
		}
		updates := map[string]any{}
		if command.TitleSet {
			updates["title"], unit.Title = command.Title, command.Title
		}
		if command.TypeSet {
			updates["type"], unit.Type = command.Type, command.Type
		}
		if command.GoalSet {
			updates["goal"], unit.Goal = command.Goal, command.Goal
		}
		if command.OwnerSet {
			updates["owner_user_id"], unit.OwnerUserID = command.OwnerUserID, command.OwnerUserID
		}
		if command.RiskLevelSet {
			updates["risk_level"], unit.RiskLevel = command.RiskLevel, command.RiskLevel
		}
		if command.PlannedStartSet {
			updates["planned_start_at"], unit.PlannedStartAt = command.PlannedStartAt, command.PlannedStartAt
		}
		if command.PlannedEndSet {
			updates["planned_end_at"], unit.PlannedEndAt = command.PlannedEndAt, command.PlannedEndAt
		}
		updated := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND id = ? AND project_new_id = ? AND mutation_state = ''", command.WorkspaceID, unit.ID, unit.ProjectSetID).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		result = planMutationResult(unit)
		encoded, _ := json.Marshal(result)
		return tx.Create(&ToolMutationApply{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, Operation: string(command.Operation), InputDigest: command.InputDigest, ResultJSON: string(encoded)}).Error
	})
	return result, normalizeProjectMutationError(err)
}

func (repository *MutationRepository) replayPlan(ctx context.Context, command project.PlanMutation) (project.PlanMutationResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.PlanMutationResult{}, false, nil
	}
	if err != nil {
		return project.PlanMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.Operation != string(command.Operation) || ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return project.PlanMutationResult{}, true, project.ErrMutationConflict
	}
	var result project.PlanMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return project.PlanMutationResult{}, true, project.ErrMutationConflict
	}
	return result, true, nil
}

func planMutationResult(row PlanUnit) project.PlanMutationResult {
	return project.PlanMutationResult{ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		Title: row.Title, Type: row.Type, Goal: row.Goal, OwnerUserID: row.OwnerUserID, PlannedStartAt: row.PlannedStartAt,
		PlannedEndAt: row.PlannedEndAt, RiskLevel: row.RiskLevel, Status: row.Status}
}

var _ project.PlanMutationStore = (*MutationRepository)(nil)
