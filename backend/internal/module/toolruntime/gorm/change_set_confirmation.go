package toolruntimegorm

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
)

func (repository *ChangeSetRepository) ConfirmAndClaim(
	ctx context.Context,
	command toolruntime.ChangeSetClaimCommand,
) (toolruntime.ChangeSetClaim, error) {
	var row ToolChangeSet
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := boundChangeSetQuery(tx, command.ID, command.Authorization).First(&row).Error; err != nil {
			return err
		}
		if row.Status == string(toolruntime.ChangeSetPending) {
			if !command.Now.Before(row.ExpiresAt) {
				return toolruntime.ErrChangeSetConflict
			}
			update := tx.Model(&ToolChangeSet{}).Where("id = ? AND status = ?", row.ID, string(toolruntime.ChangeSetPending)).Updates(map[string]any{
				"status": string(toolruntime.ChangeSetConfirmed), "confirmed_at": command.Now,
				"apply_claim_id": command.ClaimID, "apply_claimed_at": command.Now, "updated_at": command.Now,
			})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return toolruntime.ErrChangeSetConflict
			}
			row.Status, row.ConfirmedAt, row.ApplyClaimID, row.ApplyClaimedAt, row.UpdatedAt =
				string(toolruntime.ChangeSetConfirmed), &command.Now, command.ClaimID, &command.Now, command.Now
			if err := createChangeSetEvent(tx, row, toolruntime.ChangeSetConfirmed, "", command.Now); err != nil {
				return err
			}
		} else if row.Status == string(toolruntime.ChangeSetConfirmed) {
			staleBefore := command.Now.Add(-command.ClaimLease)
			if row.ApplyClaimID != "" && row.ApplyClaimedAt != nil && row.ApplyClaimedAt.After(staleBefore) {
				return toolruntime.ErrChangeSetInProgress
			}
			update := tx.Model(&ToolChangeSet{}).Where(
				"id = ? AND status = ? AND (apply_claim_id = '' OR apply_claimed_at IS NULL OR apply_claimed_at <= ?)",
				row.ID, string(toolruntime.ChangeSetConfirmed), staleBefore,
			).Updates(map[string]any{"apply_claim_id": command.ClaimID, "apply_claimed_at": command.Now, "updated_at": command.Now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return toolruntime.ErrChangeSetInProgress
			}
			row.ApplyClaimID, row.ApplyClaimedAt, row.UpdatedAt = command.ClaimID, &command.Now, command.Now
		} else {
			return toolruntime.ErrChangeSetConflict
		}
		if row.InputJSON == "" {
			return toolruntime.ErrChangeSetConflict
		}
		return nil
	})
	return toolruntime.ChangeSetClaim{
		ChangeSet: changeSetSnapshot(row), ClaimID: command.ClaimID, Input: []byte(row.InputJSON),
	}, normalizeChangeSetStoreError(err)
}

func (repository *ChangeSetRepository) Succeed(
	ctx context.Context,
	command toolruntime.ChangeSetFinishCommand,
) (toolruntime.ChangeSet, error) {
	return repository.finish(ctx, command, toolruntime.ChangeSetSucceeded)
}

func (repository *ChangeSetRepository) Fail(
	ctx context.Context,
	command toolruntime.ChangeSetFinishCommand,
) (toolruntime.ChangeSet, error) {
	return repository.finish(ctx, command, toolruntime.ChangeSetFailed)
}

func (repository *ChangeSetRepository) RejectClaim(
	ctx context.Context,
	command toolruntime.ChangeSetFinishCommand,
) (toolruntime.ChangeSet, error) {
	return repository.finish(ctx, command, toolruntime.ChangeSetRejected)
}

func (repository *ChangeSetRepository) ReleaseClaim(
	ctx context.Context,
	command toolruntime.ChangeSetReleaseCommand,
) (toolruntime.ChangeSet, error) {
	var row ToolChangeSet
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := boundChangeSetQuery(tx, command.ID, command.Authorization).First(&row).Error; err != nil {
			return err
		}
		update := tx.Model(&ToolChangeSet{}).Where(
			"id = ? AND principal_id = ? AND workspace_id = ? AND grant_id = ? AND client_id = ? AND channel = ? AND audience = ? AND status = ? AND apply_claim_id = ?",
			row.ID, command.Authorization.PrincipalID, command.Authorization.WorkspaceID, command.Authorization.GrantID,
			command.Authorization.ClientID, command.Authorization.Channel, command.Authorization.Audience,
			string(toolruntime.ChangeSetConfirmed), command.ClaimID,
		).Updates(map[string]any{
			"apply_claim_id": "", "apply_claimed_at": nil, "updated_at": command.Now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return toolruntime.ErrChangeSetConflict
		}
		row.ApplyClaimID, row.ApplyClaimedAt, row.UpdatedAt = "", nil, command.Now
		return createChangeSetEvent(tx, row, toolruntime.ChangeSetConfirmed, command.ErrorCode, command.Now)
	})
	return changeSetSnapshot(row), normalizeChangeSetStoreError(err)
}

func (repository *ChangeSetRepository) finish(
	ctx context.Context,
	command toolruntime.ChangeSetFinishCommand,
	status toolruntime.ChangeSetStatus,
) (toolruntime.ChangeSet, error) {
	var row ToolChangeSet
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := boundChangeSetQuery(tx, command.ID, command.Authorization).First(&row).Error; err != nil {
			return err
		}
		result := ""
		if status == toolruntime.ChangeSetSucceeded {
			result = string(command.Result)
		}
		update := tx.Model(&ToolChangeSet{}).Where(
			"id = ? AND status = ? AND apply_claim_id = ?", row.ID, string(toolruntime.ChangeSetConfirmed), command.ClaimID,
		).Updates(map[string]any{
			"status": string(status), "result_json": result, "error_code": command.ErrorCode,
			"input_json": "", "apply_claim_id": "", "apply_claimed_at": nil,
			"completed_at": command.Now, "updated_at": command.Now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return toolruntime.ErrChangeSetConflict
		}
		row.Status, row.ResultJSON, row.ErrorCode, row.InputJSON = string(status), result, command.ErrorCode, ""
		row.ApplyClaimID, row.ApplyClaimedAt, row.CompletedAt, row.UpdatedAt = "", nil, &command.Now, command.Now
		return createChangeSetEvent(tx, row, status, command.ErrorCode, command.Now)
	})
	return changeSetSnapshot(row), normalizeChangeSetStoreError(err)
}

func (repository *ChangeSetRepository) RejectPending(
	ctx context.Context,
	id string,
	authorization toolruntime.ExecutionAuthorization,
	errorCode string,
	now time.Time,
) (toolruntime.ChangeSet, error) {
	var row ToolChangeSet
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := boundChangeSetQuery(tx, id, authorization).First(&row).Error; err != nil {
			return err
		}
		update := tx.Model(&ToolChangeSet{}).Where("id = ? AND status = ?", row.ID, string(toolruntime.ChangeSetPending)).Updates(map[string]any{
			"status": string(toolruntime.ChangeSetRejected), "error_code": errorCode, "input_json": "",
			"completed_at": now, "updated_at": now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return toolruntime.ErrChangeSetConflict
		}
		row.Status, row.ErrorCode, row.InputJSON, row.CompletedAt, row.UpdatedAt =
			string(toolruntime.ChangeSetRejected), errorCode, "", &now, now
		return createChangeSetEvent(tx, row, toolruntime.ChangeSetRejected, errorCode, now)
	})
	return changeSetSnapshot(row), normalizeChangeSetStoreError(err)
}
