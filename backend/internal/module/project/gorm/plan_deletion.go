package projectgorm

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *MutationRepository) FindPlanDeletion(ctx context.Context, key string) (project.PlanDeletionSaga, bool, error) {
	var row PlanDeletionSaga
	err := r.db.WithContext(ctx).Where("apply_key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.PlanDeletionSaga{}, false, nil
	}
	if err != nil {
		return project.PlanDeletionSaga{}, false, err
	}
	s, err := planDeletionSnapshot(row)
	return s, true, err
}
func (r *MutationRepository) BeginPlanDeletion(ctx context.Context, c project.PlanMutation) (project.PlanDeletionSaga, error) {
	row := PlanDeletionSaga{WorkspaceID: c.WorkspaceID, PlanUnitID: c.ID, ApplyKey: c.ApplyKey, InputDigest: c.InputDigest, NextStep: 0, Status: "in_progress", ResultJSON: ""}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var unit PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", c.WorkspaceID, c.ID).First(&unit).Error; err != nil {
			return normalizeProjectMutationError(err)
		}
		if unit.MutationState != "" {
			return project.ErrMutationConflict
		}
		if _, err := lockProjectSet(tx, c.WorkspaceID, unit.ProjectSetID); err != nil {
			return err
		}
		row.ProjectSetID = unit.ProjectSetID
		if err := tx.Create(&ToolMutationApply{WorkspaceID: c.WorkspaceID, ApplyKey: c.ApplyKey, Operation: string(project.PlanDelete), InputDigest: c.InputDigest, ResultJSON: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		u := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND id = ? AND mutation_state = ''", c.WorkspaceID, c.ID).Update("mutation_state", "deleting")
		if u.Error != nil {
			return u.Error
		}
		if u.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	if err != nil {
		return project.PlanDeletionSaga{}, normalizeProjectMutationError(err)
	}
	return planDeletionSnapshot(row)
}
func (r *MutationRepository) RejectPlanDeletion(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var s PlanDeletionSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ? AND status = ?", key, "in_progress").First(&s).Error; err != nil {
			return err
		}
		if err := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND id = ? AND mutation_state = ?", s.WorkspaceID, s.PlanUnitID, "deleting").Update("mutation_state", "").Error; err != nil {
			return err
		}
		return tx.Model(&PlanDeletionSaga{}).Where("id = ?", s.ID).Update("status", "rejected").Error
	})
}
func (r *MutationRepository) AdvancePlanDeletion(ctx context.Context, key string, step int) (project.PlanDeletionSaga, error) {
	u := r.db.WithContext(ctx).Model(&PlanDeletionSaga{}).Where("apply_key = ? AND status = ? AND next_step = ?", key, "in_progress", step).Update("next_step", step+1)
	if u.Error != nil {
		return project.PlanDeletionSaga{}, u.Error
	}
	if u.RowsAffected != 1 {
		return project.PlanDeletionSaga{}, project.ErrMutationConflict
	}
	s, _, err := r.FindPlanDeletion(ctx, key)
	return s, err
}
func (r *MutationRepository) CompletePlanDeletion(ctx context.Context, key string) (project.PlanMutationResult, error) {
	var result project.PlanMutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var s PlanDeletionSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ?", key).First(&s).Error; err != nil {
			return err
		}
		if s.Status == "succeeded" {
			return json.Unmarshal([]byte(s.ResultJSON), &result)
		}
		if s.Status != "in_progress" || s.NextStep != 2 {
			return project.ErrMutationConflict
		}
		var unit PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND project_new_id = ?", s.WorkspaceID, s.PlanUnitID, s.ProjectSetID).First(&unit).Error; err != nil {
			return normalizeProjectMutationError(err)
		}
		if unit.MutationState != "deleting" {
			return project.ErrMutationConflict
		}
		for _, row := range []any{&DeliveryNode{}, &DeliveryNodeLabel{}} {
			if err := tx.Where("workspace_id = ? AND plan_unit_id = ?", s.WorkspaceID, s.PlanUnitID).Delete(row).Error; err != nil {
				return err
			}
		}
		d := tx.Where("workspace_id = ? AND id = ? AND mutation_state = ?", s.WorkspaceID, s.PlanUnitID, "deleting").Delete(&PlanUnit{})
		if d.Error != nil {
			return d.Error
		}
		if d.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		result = project.PlanMutationResult{ID: s.PlanUnitID, Deleted: true}
		b, _ := json.Marshal(result)
		if u := tx.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json = ''", key).Update("result_json", string(b)); u.Error != nil || u.RowsAffected != 1 {
			if u.Error != nil {
				return u.Error
			}
			return project.ErrMutationConflict
		}
		updated := tx.Model(&PlanDeletionSaga{}).
			Where("id = ? AND status = ? AND next_step = ?", s.ID, "in_progress", 2).
			Updates(map[string]any{"status": "succeeded", "next_step": 3, "result_json": string(b)})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	return result, normalizeProjectMutationError(err)
}
func planDeletionSnapshot(row PlanDeletionSaga) (project.PlanDeletionSaga, error) {
	var result project.PlanMutationResult
	if row.ResultJSON != "" && json.Unmarshal([]byte(row.ResultJSON), &result) != nil {
		return project.PlanDeletionSaga{}, project.ErrMutationConflict
	}
	return project.PlanDeletionSaga{WorkspaceID: row.WorkspaceID, ProjectSetID: row.ProjectSetID, PlanUnitID: row.PlanUnitID, ApplyKey: row.ApplyKey, InputDigest: row.InputDigest, NextStep: row.NextStep, Status: row.Status, Result: result}, nil
}

var _ project.PlanDeletionStore = (*MutationRepository)(nil)
