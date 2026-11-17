package native

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func (r *Repository) Execute(ctx context.Context, s bitable.Scope, b bitable.CommandBatch) (bitable.Operation, error) {
	result := bitable.Operation{ID: b.OperationID, State: "committed", Records: []bitable.Record{}, DeletedRecordIDs: []string{}}
	kind := "commands"
	eventType := "bitable.records.committed"
	if b.Importing {
		kind = "import"
		eventType = "bitable.import.committed"
	}
	if e := bitable.ValidateBatch(b); e != nil {
		return result, e
	}
	requestHash, e := commandHash(b)
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		table, fields, _, err := loadTable(tx, s, true)
		if err != nil {
			return err
		}
		replay, err := receipt(tx, s, b.OperationID, kind, requestHash, &result)
		if err != nil || replay {
			return err
		}
		if table.SchemaRevision != b.ExpectedSchemaRevision || table.DataRevision >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		if err = bitable.ValidateBatchFields(b, fields); err != nil {
			return err
		}
		contracts := []bitable.CellContract{}
		leaseProofs := []bitable.CellLeaseProof{}
		uncoveredRows := []string{}
		fieldIDs := make([]string, 0, len(fields))
		for _, field := range fields {
			fieldIDs = append(fieldIDs, field.ID)
		}
		for _, command := range b.Commands {
			contracts = append(contracts, command.CellContracts...)
			if command.Kind != "delete" && command.Kind != "restore" && command.Kind != "move" {
				continue
			}
			if len(command.CellLeaseProofs) == 0 {
				uncoveredRows = append(uncoveredRows, command.RecordID)
				continue
			}
			if len(command.CellLeaseProofs) != len(fields) {
				return bitable.ErrInvalidInput
			}
			covered := map[string]bool{}
			for _, proof := range command.CellLeaseProofs {
				if proof.RecordID != command.RecordID || covered[proof.FieldID] {
					return bitable.ErrInvalidInput
				}
				covered[proof.FieldID] = true
			}
			for _, fieldID := range fieldIDs {
				if !covered[fieldID] {
					return bitable.ErrInvalidInput
				}
			}
			leaseProofs = append(leaseProofs, command.CellLeaseProofs...)
		}
		// A mutation batch uses exactly one global lease-locking strategy. Mixing
		// owned proofs with uncovered row scans would acquire two independently
		// sorted lock groups and permits opposite batches to deadlock on Postgres.
		if len(leaseProofs) > 0 && len(uncoveredRows) > 0 {
			return bitable.ErrInvalidInput
		}
		if len(contracts) > 0 {
			if err = r.verifyCellContracts(ctx, tx, s, contracts); err != nil {
				return err
			}
		}
		if len(leaseProofs) > 0 {
			if err = r.verifyCellLeaseProofs(ctx, tx, s, leaseProofs); err != nil {
				return err
			}
		}
		if len(uncoveredRows) > 0 && len(fieldIDs) > 0 {
			if err = r.requireNoConflictingCellLeases(ctx, tx, s, []bitable.CellLeaseConflictScope{{RecordIDs: uncoveredRows, FieldIDs: fieldIDs}}); err != nil {
				return err
			}
		}
		ids := []string{}
		after := map[string]string{}
		writtenCells := 0
		now := time.Now().UTC()
		for _, command := range b.Commands {
			row := RowRecord{}
			values := map[string]any{}
			fieldRevisions := map[string]uint64{}
			if command.Kind == "create" {
				order, e := createSortOrder(tx, s, command.Position, after)
				if e != nil {
					return e
				}
				row = RowRecord{ID: uuid.NewString(), TableID: s.TableID, ResourceID: s.ResourceID, WorkspaceID: s.Principal.WorkspaceID, Revision: 1, SortOrder: order, CreatedBy: s.Principal.UserID, UpdatedBy: s.Principal.UserID, CreatedAt: now, UpdatedAt: now}
				if command.Position != nil && command.Position.Direction == "after" {
					after[command.Position.RecordID] = row.ID
				}
			} else {
				if err = scopeDB(tx, s).Where("id = ?", command.RecordID).First(&row).Error; err != nil {
					return dbError(err)
				}
				if row.Revision >= bitable.MaxSafeRevision {
					return bitable.ErrConflict
				}
				if command.Kind == "update" {
					if row.Deleted {
						return bitable.ErrConflict
					}
				} else if command.ExpectedRevision == nil || row.Revision != *command.ExpectedRevision {
					return bitable.ErrConflict
				}
				if (command.Kind == "restore") != row.Deleted {
					return bitable.ErrConflict
				}
				values, fieldRevisions, err = decodeRowState(row, fields)
				if err != nil {
					return err
				}
				if command.Kind == "update" {
					for fieldID, contract := range contractsByField(command.CellContracts) {
						if fieldRevisions[fieldID] != contract.BaseRevision || fieldRevisions[fieldID] >= bitable.MaxSafeRevision {
							return bitable.ErrConflict
						}
					}
				}
				row.Revision++
				row.UpdatedBy = s.Principal.UserID
				row.UpdatedAt = now
			}
			if command.Kind == "move" {
				before, e := moveSortOrder(tx, s, &row, command.Position, after)
				if e != nil {
					return e
				}
				result.Moves = append(result.Moves, bitable.MoveBefore{RecordID: row.ID, Before: before})
			}
			for id, value := range command.Values {
				values[id] = value
			}
			writtenCells += len(command.Values)
			if command.Kind == "create" && !b.Importing {
				values, err = bitable.ApplyCreateDefaults(fields, values)
				if err != nil {
					return err
				}
				writtenCells += len(values) - len(command.Values)
			}
			if command.Kind == "create" {
				for _, field := range fields {
					fieldRevisions[field.ID] = row.Revision
				}
			} else if command.Kind == "update" {
				for _, contract := range command.CellContracts {
					fieldRevisions[contract.FieldID]++
				}
			}
			if writtenCells > bitable.MaxCommandCells {
				return bitable.ErrBudgetExceeded
			}
			if command.Kind == "delete" {
				row.Deleted = true
				result.DeletedRecordIDs = append(result.DeletedRecordIDs, row.ID)
			}
			if command.Kind == "restore" {
				for _, field := range fields {
					if field.ReadOnly || field.Type == bitable.FieldAttachment {
						continue
					}
					if err = bitable.ValidateValue(field, values[field.ID]); err != nil {
						return err
					}
				}
				row.Deleted = false
			}
			if command.Kind != "delete" && command.Kind != "move" {
				values, err = r.normalizeValues(ctx, s.Principal, fields, values)
				if err != nil {
					return err
				}
			}
			row.Values, err = encodeRowState(values, fieldRevisions)
			if err != nil {
				return err
			}
			if command.Kind == "create" {
				err = tx.Create(&row).Error
			} else {
				updated := scopeDB(tx, s).Where("id = ? AND revision = ?", row.ID, row.Revision-1).Select("revision", "values", "deleted", "updated_by", "updated_at", "sort_order").Updates(&row)
				err = updated.Error
				if err == nil && updated.RowsAffected != 1 {
					err = bitable.ErrConflict
				}
			}
			if err != nil {
				return err
			}
			dto, err := rowDTO(row, fields)
			if err != nil {
				return err
			}
			result.Records = append(result.Records, dto)
			ids = append(ids, row.ID)
		}
		for i := range result.Moves {
			var anchor RowRecord
			if err = scopeDB(tx, s).Where("id = ? AND deleted = ?", result.Moves[i].Before.RecordID, false).First(&anchor).Error; err != nil {
				return dbError(err)
			}
			revision := anchor.Revision
			result.Moves[i].Before.ExpectedRevision = &revision
		}
		if err = tableBudget(tx, s, len(fields)); err != nil {
			return err
		}
		if err = tableDB(tx.Model(&TableRecord{}), s).Updates(map[string]any{"data_revision": table.DataRevision + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		if err = r.recordMutation(ctx, tx, s, b.OperationID, "records.batch", ids); err != nil {
			return err
		}
		if err = saveReceipt(tx, s, b.OperationID, kind, requestHash, result); err != nil {
			return err
		}
		return r.appendCommittedEvent(ctx, tx, s, b.OperationID, eventType, result)
	})
	return result, e
}

func commandHash(b bitable.CommandBatch) (string, error) {
	data, e := json.Marshal(b)
	if e != nil {
		return "", e
	}
	var value any
	if e = json.Unmarshal(data, &value); e != nil {
		return "", e
	}
	var normalize func(any)
	normalize = func(v any) {
		switch item := v.(type) {
		case map[string]any:
			if _, ok := item["id"].(float64); ok {
				delete(item, "name")
				delete(item, "username")
			}
			for _, child := range item {
				normalize(child)
			}
		case []any:
			for _, child := range item {
				normalize(child)
			}
		}
	}
	normalize(value)
	return hash(value)
}
func (r *Repository) Operation(ctx context.Context, s bitable.Scope, key string) (bitable.Operation, error) {
	result := bitable.Operation{}
	if _, _, _, e := loadTable(r.db.WithContext(ctx), s, false); e != nil {
		return result, e
	}
	var item ReceiptRecord
	if e := r.db.WithContext(ctx).Where("scope_key = ? AND operation_id = ? AND workspace_id = ? AND resource_id = ? AND actor_id = ?", s.TableID, key, s.Principal.WorkspaceID, s.ResourceID, s.Principal.UserID).First(&item).Error; e != nil {
		return result, dbError(e)
	}
	if item.Kind != "commands" && item.Kind != "import" && item.Kind != "attachment" {
		return result, bitable.ErrNotFound
	}
	e := json.Unmarshal([]byte(item.Result), &result)
	result.RequiredActions = []accesscatalog.ActionCode{accesscatalog.ActionBitablesRecordsWrite}
	if item.Kind == "import" {
		result.RequiredActions = append(result.RequiredActions, accesscatalog.ActionBitablesTransferImport)
	}
	if item.Kind == "attachment" {
		result.RequiredActions = append(result.RequiredActions, accesscatalog.ActionBitablesAssetsUpload)
	}
	return result, e
}
