package projectgorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *MutationRepository) FindProjectDeletion(
	ctx context.Context,
	applyKey string,
) (project.DeletionSaga, bool, error) {
	if repository == nil || repository.db == nil || applyKey == "" {
		return project.DeletionSaga{}, false, project.ErrMutationInvalid
	}
	var row ProjectDeletionSaga
	err := repository.db.WithContext(ctx).Where("apply_key = ?", applyKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.DeletionSaga{}, false, nil
	}
	if err != nil {
		return project.DeletionSaga{}, false, err
	}
	saga, err := projectDeletionSagaSnapshot(row)
	return saga, true, err
}

func (repository *MutationRepository) BeginProjectDeletion(
	ctx context.Context,
	command project.SetMutation,
	impact project.DeletionImpact,
) (project.DeletionSaga, error) {
	if repository == nil || repository.db == nil {
		return project.DeletionSaga{}, project.ErrMutationInvalid
	}
	planIDs := canonicalDeletionIDs(impact.Local.PlanUnitIDs)
	requirementIDs := canonicalDeletionIDs(impact.RequirementIDs)
	planJSON, _ := json.Marshal(planIDs)
	requirementJSON, _ := json.Marshal(requirementIDs)
	digest := sha256.Sum256(append(append([]byte(nil), planJSON...), requirementJSON...))
	row := ProjectDeletionSaga{
		WorkspaceID: command.WorkspaceID, ProjectSetID: command.ID, ApplyKey: command.ApplyKey,
		InputDigest: command.InputDigest, SubjectDigest: hex.EncodeToString(digest[:]),
		PlanUnitIDsJSON: string(planJSON), RequirementIDsJSON: string(requirementJSON),
		NextStep: int(project.DeletionPreflight), Status: "in_progress", ResultJSON: "",
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projectSet, err := lockProjectSetForDeletion(tx, command.WorkspaceID, command.ID)
		if err != nil {
			return err
		}
		if projectSet.MutationState != "" {
			return project.ErrMutationConflict
		}
		var currentPlanIDs []uint
		if err := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND project_new_id = ?", command.WorkspaceID, command.ID).
			Order("id ASC").Pluck("id", &currentPlanIDs).Error; err != nil {
			return err
		}
		if !sameDeletionIDs(currentPlanIDs, planIDs) {
			return project.ErrMutationConflict
		}
		if err := tx.Create(&ToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey, Operation: string(project.SetDelete),
			InputDigest: command.InputDigest, ResultJSON: "",
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		updated := tx.Model(&ProjectSet{}).Where("workspace_id = ? AND id = ? AND mutation_state = ''", command.WorkspaceID, command.ID).
			Update("mutation_state", "deleting")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	if err == nil {
		return projectDeletionSagaSnapshot(row)
	}
	if existing, found, findErr := repository.FindProjectDeletion(ctx, command.ApplyKey); findErr != nil {
		return project.DeletionSaga{}, findErr
	} else if found {
		if existing.WorkspaceID != command.WorkspaceID || existing.ProjectSetID != command.ID ||
			existing.InputDigest != command.InputDigest || existing.SubjectDigest != row.SubjectDigest {
			return project.DeletionSaga{}, project.ErrMutationConflict
		}
		return existing, nil
	}
	return project.DeletionSaga{}, normalizeProjectMutationError(err)
}

func (repository *MutationRepository) RefreshProjectDeletionImpact(
	ctx context.Context,
	applyKey string,
	impact project.DeletionImpact,
) (project.DeletionSaga, error) {
	if repository == nil || repository.db == nil || applyKey == "" {
		return project.DeletionSaga{}, project.ErrMutationInvalid
	}
	planIDs := canonicalDeletionIDs(impact.Local.PlanUnitIDs)
	requirementIDs := canonicalDeletionIDs(impact.RequirementIDs)
	planJSON, _ := json.Marshal(planIDs)
	requirementJSON, _ := json.Marshal(requirementIDs)
	digest := sha256.Sum256(append(append([]byte(nil), planJSON...), requirementJSON...))
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saga ProjectDeletionSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ?", applyKey).First(&saga).Error; err != nil {
			return err
		}
		if saga.Status != "in_progress" || saga.NextStep != int(project.DeletionPreflight) {
			return project.ErrMutationConflict
		}
		projectSet, err := lockProjectSetForDeletion(tx, saga.WorkspaceID, saga.ProjectSetID)
		if err != nil {
			return err
		}
		if projectSet.MutationState != "deleting" {
			return project.ErrMutationConflict
		}
		updated := tx.Model(&ProjectDeletionSaga{}).Where("id = ? AND status = ? AND next_step = ?", saga.ID, "in_progress", int(project.DeletionPreflight)).Updates(map[string]any{
			"plan_unit_ids_json": string(planJSON), "requirement_ids_json": string(requirementJSON), "subject_digest": hex.EncodeToString(digest[:]),
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	if err != nil {
		return project.DeletionSaga{}, normalizeProjectMutationError(err)
	}
	saga, found, err := repository.FindProjectDeletion(ctx, applyKey)
	if err != nil {
		return project.DeletionSaga{}, err
	}
	if !found {
		return project.DeletionSaga{}, project.ErrMutationConflict
	}
	return saga, nil
}

func (repository *MutationRepository) AdvanceProjectDeletion(
	ctx context.Context,
	applyKey string,
	completed project.DeletionStep,
) (project.DeletionSaga, error) {
	if repository == nil || repository.db == nil || applyKey == "" || completed < project.DeletionPreflight || completed >= project.DeletionProject {
		return project.DeletionSaga{}, project.ErrMutationInvalid
	}
	updated := repository.db.WithContext(ctx).Model(&ProjectDeletionSaga{}).Where(
		"apply_key = ? AND status = ? AND next_step = ?", applyKey, "in_progress", int(completed),
	).Update("next_step", int(completed+1))
	if updated.Error != nil {
		return project.DeletionSaga{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return project.DeletionSaga{}, project.ErrMutationConflict
	}
	saga, found, err := repository.FindProjectDeletion(ctx, applyKey)
	if err != nil || !found {
		if err == nil {
			err = project.ErrMutationConflict
		}
		return project.DeletionSaga{}, err
	}
	return saga, nil
}

func (repository *MutationRepository) CompleteProjectDeletion(
	ctx context.Context,
	applyKey string,
) (project.SetMutationResult, error) {
	if repository == nil || repository.db == nil || applyKey == "" {
		return project.SetMutationResult{}, project.ErrMutationInvalid
	}
	var result project.SetMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var saga ProjectDeletionSaga
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("apply_key = ?", applyKey).First(&saga).Error; err != nil {
			return err
		}
		if saga.Status == "succeeded" {
			return json.Unmarshal([]byte(saga.ResultJSON), &result)
		}
		if saga.Status != "in_progress" || saga.NextStep != int(project.DeletionProject) {
			return project.ErrMutationConflict
		}
		projectSet, err := lockProjectSetForDeletion(tx, saga.WorkspaceID, saga.ProjectSetID)
		if err != nil {
			return err
		}
		if projectSet.MutationState != "deleting" {
			return project.ErrMutationConflict
		}
		var expectedPlanIDs []uint
		if json.Unmarshal([]byte(saga.PlanUnitIDsJSON), &expectedPlanIDs) != nil {
			return project.ErrMutationConflict
		}
		var currentPlanIDs []uint
		if err := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND project_new_id = ?", saga.WorkspaceID, saga.ProjectSetID).
			Order("id ASC").Pluck("id", &currentPlanIDs).Error; err != nil {
			return err
		}
		if !sameDeletionIDs(currentPlanIDs, expectedPlanIDs) {
			return project.ErrMutationConflict
		}
		if len(expectedPlanIDs) > 0 {
			if err := tx.Where("workspace_id = ? AND plan_unit_id IN ?", saga.WorkspaceID, expectedPlanIDs).Delete(&DeliveryNode{}).Error; err != nil {
				return err
			}
		}
		var permissionIDs []uint
		if err := tx.Model(&ProjectPermission{}).Where("workspace_id = ? AND project_new_id = ?", saga.WorkspaceID, saga.ProjectSetID).
			Pluck("id", &permissionIDs).Error; err != nil {
			return err
		}
		if len(permissionIDs) > 0 {
			if err := tx.Where("workspace_id = ? AND project_permission_id IN ?", saga.WorkspaceID, permissionIDs).
				Delete(&ProjectMemberRole{}).Error; err != nil {
				return err
			}
		}
		for _, deletion := range []struct {
			model any
			where string
		}{
			{&DeliveryNodeLabel{}, "project_set_id = ?"}, {&PlanUnit{}, "project_new_id = ?"},
			{&ProjectPermission{}, "project_new_id = ?"}, {&ProjectNews{}, "project_set_id = ?"},
		} {
			if err := tx.Where("workspace_id = ?", saga.WorkspaceID).Delete(deletion.model, deletion.where, saga.ProjectSetID).Error; err != nil {
				return err
			}
		}
		deleted := tx.Where("workspace_id = ? AND id = ? AND mutation_state = ?", saga.WorkspaceID, saga.ProjectSetID, "deleting").
			Delete(&ProjectSet{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		result = project.SetMutationResult{ID: saga.ProjectSetID, Deleted: true}
		encoded, _ := json.Marshal(result)
		ledger := tx.Model(&ToolMutationApply{}).Where(
			"apply_key = ? AND workspace_id = ? AND operation = ? AND result_json = ''",
			applyKey, saga.WorkspaceID, string(project.SetDelete),
		).Update("result_json", string(encoded))
		if ledger.Error != nil {
			return ledger.Error
		}
		if ledger.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		finished := tx.Model(&ProjectDeletionSaga{}).Where(
			"id = ? AND status = ? AND next_step = ?", saga.ID, "in_progress", int(project.DeletionProject),
		).Updates(map[string]any{"status": "succeeded", "next_step": int(project.DeletionDone), "result_json": string(encoded)})
		if finished.Error != nil {
			return finished.Error
		}
		if finished.RowsAffected != 1 {
			return project.ErrMutationConflict
		}
		return nil
	})
	return result, normalizeProjectMutationError(err)
}

func lockProjectSetForDeletion(tx *gorm.DB, workspaceID, id uint) (ProjectSet, error) {
	var row ProjectSet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&row).Error; err != nil {
		return ProjectSet{}, normalizeProjectMutationError(err)
	}
	return row, nil
}

func projectDeletionSagaSnapshot(row ProjectDeletionSaga) (project.DeletionSaga, error) {
	var planIDs, requirementIDs []uint
	if json.Unmarshal([]byte(row.PlanUnitIDsJSON), &planIDs) != nil ||
		json.Unmarshal([]byte(row.RequirementIDsJSON), &requirementIDs) != nil {
		return project.DeletionSaga{}, project.ErrMutationConflict
	}
	var result project.SetMutationResult
	if row.ResultJSON != "" && json.Unmarshal([]byte(row.ResultJSON), &result) != nil {
		return project.DeletionSaga{}, project.ErrMutationConflict
	}
	return project.DeletionSaga{
		WorkspaceID: row.WorkspaceID, ProjectSetID: row.ProjectSetID, ApplyKey: row.ApplyKey,
		InputDigest: row.InputDigest, SubjectDigest: row.SubjectDigest, PlanUnitIDs: planIDs,
		RequirementIDs: requirementIDs, NextStep: project.DeletionStep(row.NextStep), Status: row.Status, Result: result,
	}, nil
}

func canonicalDeletionIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func sameDeletionIDs(left, right []uint) bool {
	left, right = canonicalDeletionIDs(left), canonicalDeletionIDs(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

var _ project.DeletionSagaStore = (*MutationRepository)(nil)
