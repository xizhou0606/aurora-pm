package projectgorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PlanScheduleGuardFactory func(*gorm.DB) (project.PlanScheduleGuard, error)

type MutationRepository struct {
	db            *gorm.DB
	planSchedules PlanScheduleGuardFactory
}

func NewMutationRepository(db *gorm.DB) *MutationRepository { return &MutationRepository{db: db} }

func NewPlanMutationRepository(db *gorm.DB, schedules PlanScheduleGuardFactory) (*MutationRepository, error) {
	if db == nil || schedules == nil {
		return nil, project.ErrMutationInvalid
	}
	return &MutationRepository{db: db, planSchedules: schedules}, nil
}

func (repository *MutationRepository) ApplyDeliveryLabel(
	ctx context.Context,
	command project.DeliveryLabelMutation,
) (project.DeliveryLabelMutationResult, error) {
	if repository == nil || repository.db == nil {
		return project.DeliveryLabelMutationResult{}, project.ErrMutationInvalid
	}
	if replay, found, err := repository.replay(ctx, command); err != nil || found {
		return replay, err
	}
	var result project.DeliveryLabelMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey,
			Operation: string(command.Operation), InputDigest: command.InputDigest, ResultJSON: "",
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		var err error
		switch command.Operation {
		case project.DeliveryLabelCreate:
			result, err = applyDeliveryLabelCreate(tx, command)
		case project.DeliveryLabelUpdate:
			result, err = applyDeliveryLabelUpdate(tx, command)
		case project.DeliveryLabelDelete:
			result, err = applyDeliveryLabelDelete(tx, command)
		default:
			err = project.ErrMutationInvalid
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Model(&ToolMutationApply{}).Where("id = ?", ledger.ID).Update("result_json", string(encoded)).Error
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replay(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return project.DeliveryLabelMutationResult{}, normalizeProjectMutationError(err)
}

func (repository *MutationRepository) ApplyProjectSet(
	ctx context.Context,
	command project.SetMutation,
) (project.SetMutationResult, error) {
	if repository == nil || repository.db == nil {
		return project.SetMutationResult{}, project.ErrMutationInvalid
	}
	if replay, found, err := repository.replayProjectSet(ctx, command); err != nil || found {
		return replay, err
	}
	var result project.SetMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{
			WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey,
			Operation: string(command.Operation), InputDigest: command.InputDigest, ResultJSON: "",
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		var err error
		switch command.Operation {
		case project.SetCreate:
			result, err = applyProjectSetCreate(tx, command)
		case project.SetUpdate:
			result, err = applyProjectSetUpdate(tx, command)
		case project.SetTransferOwner:
			result, err = applyProjectSetTransferOwner(tx, command)
		default:
			err = project.ErrMutationInvalid
		}
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Model(&ToolMutationApply{}).Where("id = ?", ledger.ID).Update("result_json", string(encoded)).Error
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replayProjectSet(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return project.SetMutationResult{}, normalizeProjectMutationError(err)
}

func (repository *MutationRepository) replayProjectSet(
	ctx context.Context,
	command project.SetMutation,
) (project.SetMutationResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.SetMutationResult{}, false, nil
	}
	if err != nil {
		return project.SetMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.Operation != string(command.Operation) ||
		ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return project.SetMutationResult{}, true, project.ErrMutationConflict
	}
	var result project.SetMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return project.SetMutationResult{}, true, project.ErrMutationConflict
	}
	return result, true, nil
}

func applyProjectSetCreate(tx *gorm.DB, command project.SetMutation) (project.SetMutationResult, error) {
	status := command.Status
	if !command.StatusSet {
		status = "active"
	}
	row := ProjectSet{
		WorkspaceID: command.WorkspaceID, TeamID: command.TeamID, Name: command.Name, Code: command.Code,
		Description: command.Description, OwnerUserID: command.OwnerUserID,
		PlannedStartAt: command.PlannedStartAt, PlannedEndAt: command.PlannedEndAt, Status: status,
	}
	if err := tx.Create(&row).Error; err != nil {
		return project.SetMutationResult{}, err
	}
	if row.Code == "" {
		row.Code = fmt.Sprintf("PM-%d", row.ID)
		if err := tx.Model(&ProjectSet{}).Where("id = ? AND workspace_id = ?", row.ID, row.WorkspaceID).
			Update("code", row.Code).Error; err != nil {
			return project.SetMutationResult{}, err
		}
	}
	return projectSetMutationResult(row), nil
}

func applyProjectSetUpdate(tx *gorm.DB, command project.SetMutation) (project.SetMutationResult, error) {
	row, err := lockProjectSet(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return project.SetMutationResult{}, err
	}
	updates := map[string]any{}
	if command.TeamIDSet {
		updates["team_id"], row.TeamID = command.TeamID, command.TeamID
	}
	if command.NameSet {
		updates["name"], row.Name = command.Name, command.Name
	}
	if command.DescriptionSet {
		updates["description"], row.Description = command.Description, command.Description
	}
	if command.OwnerSet {
		updates["owner_user_id"], row.OwnerUserID = command.OwnerUserID, command.OwnerUserID
	}
	dateChanged := command.PlannedStartSet && !sameMutationTime(row.PlannedStartAt, command.PlannedStartAt) ||
		command.PlannedEndSet && !sameMutationTime(row.PlannedEndAt, command.PlannedEndAt)
	if dateChanged {
		var planUnits int64
		if err := tx.Model(&PlanUnit{}).Where("workspace_id = ? AND project_new_id = ?", row.WorkspaceID, row.ID).Count(&planUnits).Error; err != nil {
			return project.SetMutationResult{}, err
		}
		if planUnits > 0 {
			return project.SetMutationResult{}, project.ErrMutationConflict
		}
	}
	if command.PlannedStartSet {
		updates["planned_start_at"], row.PlannedStartAt = command.PlannedStartAt, command.PlannedStartAt
	}
	if command.PlannedEndSet {
		updates["planned_end_at"], row.PlannedEndAt = command.PlannedEndAt, command.PlannedEndAt
	}
	if projectScheduleInvalid(row) {
		return project.SetMutationResult{}, project.ErrMutationInvalid
	}
	if command.StatusSet {
		updates["status"], row.Status = command.Status, command.Status
	}
	if len(updates) > 0 {
		updated := tx.Model(&ProjectSet{}).Where("id = ? AND workspace_id = ?", row.ID, row.WorkspaceID).Updates(updates)
		if updated.Error != nil {
			return project.SetMutationResult{}, updated.Error
		}
		if updated.RowsAffected != 1 {
			return project.SetMutationResult{}, project.ErrMutationConflict
		}
	}
	return projectSetMutationResult(row), nil
}

func applyProjectSetTransferOwner(tx *gorm.DB, command project.SetMutation) (project.SetMutationResult, error) {
	row, err := lockProjectSet(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return project.SetMutationResult{}, err
	}
	updated := tx.Model(&ProjectSet{}).Where("id = ? AND workspace_id = ?", row.ID, row.WorkspaceID).
		Update("owner_user_id", command.NewOwnerUserID)
	if updated.Error != nil {
		return project.SetMutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return project.SetMutationResult{}, project.ErrMutationConflict
	}
	row.OwnerUserID = &command.NewOwnerUserID
	return projectSetMutationResult(row), nil
}

func lockProjectSet(tx *gorm.DB, workspaceID, id uint) (ProjectSet, error) {
	var row ProjectSet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&row).Error; err != nil {
		return ProjectSet{}, normalizeProjectMutationError(err)
	}
	if row.MutationState != "" {
		return ProjectSet{}, project.ErrMutationConflict
	}
	return row, nil
}

func projectSetMutationResult(row ProjectSet) project.SetMutationResult {
	return project.SetMutationResult{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, Name: row.Name, Code: row.Code,
		Description: row.Description, OwnerUserID: row.OwnerUserID,
		PlannedStartAt: row.PlannedStartAt, PlannedEndAt: row.PlannedEndAt, Status: row.Status,
	}
}

func projectScheduleInvalid(row ProjectSet) bool {
	return row.PlannedStartAt != nil && row.PlannedEndAt != nil && row.PlannedStartAt.After(*row.PlannedEndAt)
}

func sameMutationTime(left, right *time.Time) bool {
	return left == nil && right == nil || left != nil && right != nil && left.Equal(*right)
}

func (repository *MutationRepository) replay(
	ctx context.Context,
	command project.DeliveryLabelMutation,
) (project.DeliveryLabelMutationResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.DeliveryLabelMutationResult{}, false, nil
	}
	if err != nil {
		return project.DeliveryLabelMutationResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.Operation != string(command.Operation) ||
		ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return project.DeliveryLabelMutationResult{}, true, project.ErrMutationConflict
	}
	var result project.DeliveryLabelMutationResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil {
		return project.DeliveryLabelMutationResult{}, true, project.ErrMutationConflict
	}
	return result, true, nil
}

func applyDeliveryLabelCreate(tx *gorm.DB, command project.DeliveryLabelMutation) (project.DeliveryLabelMutationResult, error) {
	unit, projectSet, err := lockLabelHierarchy(tx, command.WorkspaceID, command.ID, 0)
	if err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	row := DeliveryNodeLabel{
		WorkspaceID: command.WorkspaceID, ProjectSetID: projectSet.ID, PlanUnitID: unit.ID,
		ClientKey: command.ClientKey, Name: command.Name, SystemKey: command.SystemKey, SortOrder: command.SortOrder,
	}
	if err := tx.Create(&row).Error; err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	return deliveryLabelMutationResult(row), nil
}

func applyDeliveryLabelUpdate(tx *gorm.DB, command project.DeliveryLabelMutation) (project.DeliveryLabelMutationResult, error) {
	_, _, row, err := lockDeliveryLabelHierarchy(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	if row.Deleted {
		return project.DeliveryLabelMutationResult{}, project.ErrMutationConflict
	}
	if err := tx.Model(&DeliveryNodeLabel{}).Where("id = ? AND workspace_id = ? AND deleted = ?", row.ID, command.WorkspaceID, false).
		Update("name", command.Name).Error; err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	row.Name = command.Name
	return deliveryLabelMutationResult(row), nil
}

func applyDeliveryLabelDelete(tx *gorm.DB, command project.DeliveryLabelMutation) (project.DeliveryLabelMutationResult, error) {
	_, _, row, err := lockDeliveryLabelHierarchy(tx, command.WorkspaceID, command.ID)
	if err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	if row.Deleted {
		return deliveryLabelDeleteResult(row.ID), nil
	}
	var references int64
	if err := tx.Model(&DeliveryNode{}).Where("workspace_id = ? AND label_id = ?", command.WorkspaceID, row.ID).Count(&references).Error; err != nil {
		return project.DeliveryLabelMutationResult{}, err
	}
	if references > 0 {
		return project.DeliveryLabelMutationResult{}, project.ErrMutationConflict
	}
	updated := tx.Model(&DeliveryNodeLabel{}).Where("id = ? AND workspace_id = ? AND deleted = ?", row.ID, command.WorkspaceID, false).
		Update("deleted", true)
	if updated.Error != nil {
		return project.DeliveryLabelMutationResult{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return project.DeliveryLabelMutationResult{}, project.ErrMutationConflict
	}
	return deliveryLabelDeleteResult(row.ID), nil
}

func lockLabelHierarchy(tx *gorm.DB, workspaceID, planUnitID, projectSetID uint) (PlanUnit, ProjectSet, error) {
	var unit PlanUnit
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, planUnitID)
	if projectSetID > 0 {
		query = query.Where("project_new_id = ?", projectSetID)
	}
	if err := query.First(&unit).Error; err != nil {
		return PlanUnit{}, ProjectSet{}, normalizeProjectMutationError(err)
	}
	var projectSet ProjectSet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, unit.ProjectSetID).
		First(&projectSet).Error; err != nil {
		return PlanUnit{}, ProjectSet{}, normalizeProjectMutationError(err)
	}
	if projectSet.MutationState != "" {
		return PlanUnit{}, ProjectSet{}, project.ErrMutationConflict
	}
	return unit, projectSet, nil
}

func lockDeliveryLabelHierarchy(tx *gorm.DB, workspaceID, labelID uint) (PlanUnit, ProjectSet, DeliveryNodeLabel, error) {
	var row DeliveryNodeLabel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, labelID).
		First(&row).Error; err != nil {
		return PlanUnit{}, ProjectSet{}, DeliveryNodeLabel{}, normalizeProjectMutationError(err)
	}
	unit, projectSet, err := lockLabelHierarchy(tx, workspaceID, row.PlanUnitID, row.ProjectSetID)
	if err != nil || row.ProjectSetID != projectSet.ID {
		if err == nil {
			err = project.ErrMutationConflict
		}
		return PlanUnit{}, ProjectSet{}, DeliveryNodeLabel{}, err
	}
	return unit, projectSet, row, nil
}

func deliveryLabelMutationResult(row DeliveryNodeLabel) project.DeliveryLabelMutationResult {
	return project.DeliveryLabelMutationResult{
		ID: row.ID, WorkspaceID: row.WorkspaceID, ProjectSetID: row.ProjectSetID, PlanUnitID: row.PlanUnitID,
		ClientKey: row.ClientKey, Name: row.Name, SystemKey: row.SystemKey, SortOrder: row.SortOrder, Deleted: row.Deleted,
	}
}

func deliveryLabelDeleteResult(id uint) project.DeliveryLabelMutationResult {
	return project.DeliveryLabelMutationResult{ID: id, Deleted: true}
}

func normalizeProjectMutationError(err error) error {
	if err == nil || errors.Is(err, project.ErrMutationInvalid) || errors.Is(err, project.ErrMutationConflict) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.ErrMutationNotFound
	}
	return fmt.Errorf("project mutation store: %w", err)
}

var _ project.MutationStore = (*MutationRepository)(nil)
