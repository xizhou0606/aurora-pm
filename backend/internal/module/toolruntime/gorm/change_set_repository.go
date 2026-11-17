package toolruntimegorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChangeSetRepository struct{ db *gorm.DB }

func NewChangeSetRepository(db *gorm.DB) *ChangeSetRepository { return &ChangeSetRepository{db: db} }

var changeSetIdempotencyColumns = []clause.Column{
	{Name: "principal_id"}, {Name: "workspace_id"}, {Name: "grant_id"}, {Name: "client_id"},
	{Name: "channel"}, {Name: "audience"}, {Name: "idempotency_digest"}, {Name: "tool_name"},
}

func (repository *ChangeSetRepository) CreateOrFind(
	ctx context.Context,
	draft toolruntime.ChangeSetDraft,
) (toolruntime.ChangeSetCreateResult, error) {
	if repository == nil || repository.db == nil {
		return toolruntime.ChangeSetCreateResult{}, toolruntime.ErrChangeSetInvalid
	}
	row := changeSetRecord(draft)
	var result toolruntime.ChangeSetCreateResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created := tx.Clauses(clause.OnConflict{Columns: changeSetIdempotencyColumns, DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			if err := createChangeSetEvent(tx, row, toolruntime.ChangeSetPending, "", row.CreatedAt); err != nil {
				return err
			}
			result = toolruntime.ChangeSetCreateResult{ChangeSet: changeSetSnapshot(row), Created: true}
			return nil
		}
		var existing ToolChangeSet
		if err := idempotencyQuery(tx, row).First(&existing).Error; err != nil {
			return err
		}
		if existing.InputDigest != row.InputDigest || existing.ToolVersion != row.ToolVersion || existing.Risk != row.Risk {
			return toolruntime.ErrChangeSetConflict
		}
		result = toolruntime.ChangeSetCreateResult{ChangeSet: changeSetSnapshot(existing), Created: false}
		return nil
	})
	return result, normalizeChangeSetStoreError(err)
}

func (repository *ChangeSetRepository) FindBound(
	ctx context.Context,
	id string,
	authorization toolruntime.ExecutionAuthorization,
	now time.Time,
) (toolruntime.ChangeSet, error) {
	if repository == nil || repository.db == nil {
		return toolruntime.ChangeSet{}, toolruntime.ErrChangeSetNotFound
	}
	var row ToolChangeSet
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := boundChangeSetQuery(tx, id, authorization).First(&row).Error; err != nil {
			return err
		}
		if row.Status == string(toolruntime.ChangeSetPending) && !now.Before(row.ExpiresAt) {
			update := tx.Model(&ToolChangeSet{}).Where("id = ? AND status = ?", row.ID, string(toolruntime.ChangeSetPending)).Updates(map[string]any{
				"status": string(toolruntime.ChangeSetExpired), "error_code": "CHANGE_SET_EXPIRED",
				"input_json": "", "completed_at": now, "updated_at": now,
			})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 1 {
				row.Status, row.ErrorCode, row.InputJSON, row.CompletedAt, row.UpdatedAt =
					string(toolruntime.ChangeSetExpired), "CHANGE_SET_EXPIRED", "", &now, now
				return createChangeSetEvent(tx, row, toolruntime.ChangeSetExpired, "CHANGE_SET_EXPIRED", now)
			}
			return boundChangeSetQuery(tx, id, authorization).First(&row).Error
		}
		return nil
	})
	return changeSetSnapshot(row), normalizeChangeSetStoreError(err)
}

func changeSetRecord(draft toolruntime.ChangeSetDraft) ToolChangeSet {
	auth := draft.Authorization
	return ToolChangeSet{
		ID: draft.ID, Status: string(draft.Status), ToolName: draft.ToolName, ToolVersion: draft.ToolVersion,
		Risk: string(draft.Risk), AuditEvent: draft.AuditEvent,
		PrincipalID: auth.PrincipalID, WorkspaceID: auth.WorkspaceID, GrantID: auth.GrantID,
		ClientID: auth.ClientID, Channel: string(auth.Channel), Audience: string(auth.Audience),
		IdempotencyDigest: draft.IdempotencyDigest, InputDigest: draft.InputDigest, InputSize: draft.InputSize,
		InputJSON: string(draft.Input), ResultJSON: "", ErrorCode: "", ApplyClaimID: "",
		ExpiresAt: draft.ExpiresAt, CreatedAt: draft.CreatedAt, UpdatedAt: draft.UpdatedAt,
	}
}

func changeSetSnapshot(row ToolChangeSet) toolruntime.ChangeSet {
	var result json.RawMessage
	if row.ResultJSON != "" {
		result = json.RawMessage(row.ResultJSON)
	}
	return toolruntime.ChangeSet{
		ID: row.ID, Status: toolruntime.ChangeSetStatus(row.Status), ToolName: row.ToolName, ToolVersion: row.ToolVersion,
		Risk: toolruntime.RiskLevel(row.Risk), AuditEvent: row.AuditEvent,
		Authorization: toolruntime.ExecutionAuthorization{
			PrincipalID: row.PrincipalID, WorkspaceID: row.WorkspaceID, GrantID: row.GrantID, ClientID: row.ClientID,
			Channel: toolruntime.DeliveryChannel(row.Channel), Audience: toolruntime.TokenAudience(row.Audience),
		},
		InputDigest: row.InputDigest, InputSize: row.InputSize, Result: result, ErrorCode: row.ErrorCode,
		ExpiresAt: row.ExpiresAt, ConfirmedAt: row.ConfirmedAt, CompletedAt: row.CompletedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func boundChangeSetQuery(db *gorm.DB, id string, authorization toolruntime.ExecutionAuthorization) *gorm.DB {
	return db.Where(
		"id = ? AND principal_id = ? AND workspace_id = ? AND grant_id = ? AND client_id = ? AND channel = ? AND audience = ?",
		id, authorization.PrincipalID, authorization.WorkspaceID, authorization.GrantID, authorization.ClientID,
		string(authorization.Channel), string(authorization.Audience),
	)
}

func idempotencyQuery(db *gorm.DB, row ToolChangeSet) *gorm.DB {
	return db.Where(
		"principal_id = ? AND workspace_id = ? AND grant_id = ? AND client_id = ? AND channel = ? AND audience = ? AND idempotency_digest = ? AND tool_name = ?",
		row.PrincipalID, row.WorkspaceID, row.GrantID, row.ClientID, row.Channel, row.Audience, row.IdempotencyDigest, row.ToolName,
	)
}

func createChangeSetEvent(tx *gorm.DB, row ToolChangeSet, status toolruntime.ChangeSetStatus, errorCode string, at time.Time) error {
	return tx.Create(&ToolChangeSetEvent{
		ChangeSetID: row.ID, WorkspaceID: row.WorkspaceID, PrincipalID: row.PrincipalID,
		Status: string(status), AuditEvent: row.AuditEvent, ErrorCode: errorCode, OccurredAt: at,
	}).Error
}

func normalizeChangeSetStoreError(err error) error {
	if err == nil || errors.Is(err, toolruntime.ErrChangeSetConflict) || errors.Is(err, toolruntime.ErrChangeSetInProgress) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return toolruntime.ErrChangeSetNotFound
	}
	return err
}

var _ toolruntime.ChangeSetRepository = (*ChangeSetRepository)(nil)
