package native

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func (r *Repository) MutateField(ctx context.Context, s bitable.Scope, id string, in bitable.FieldMutation) (bitable.Field, error) {
	result := bitable.Field{}
	if e := bitable.ValidateFieldMutation(in); e != nil {
		return result, e
	}
	requestHash, e := hash(struct {
		ID    string
		Input bitable.FieldMutation
	}{id, in})
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		table, fields, _, err := loadTable(tx, s, true)
		if err != nil {
			return err
		}
		replay, err := receipt(tx, s, in.OperationID, "field", requestHash, &result)
		if err != nil || replay {
			return err
		}
		if table.SchemaRevision != in.ExpectedSchemaRevision || table.SchemaRevision >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		index := -1
		for i, f := range fields {
			if f.ID == id {
				index = i
			}
			if f.ID != id && f.Name == in.Name {
				return bitable.ErrConflict
			}
		}
		if id != "" && index < 0 {
			return bitable.ErrNotFound
		}
		if index >= 0 && fields[index].Type != in.Type {
			return bitable.ErrInvalidInput
		}
		cellAffecting := index >= 0 && !reflect.DeepEqual(fields[index].Options, in.Options)
		if index < 0 {
			if len(fields) >= 100 {
				return bitable.ErrBudgetExceeded
			}
			id = uuid.NewString()
		}
		result = bitable.Field{ID: id, Name: in.Name, Type: in.Type, Options: in.Options, ReadOnly: bitable.AuditField(in.Type)}
		if index >= 0 {
			fields[index] = result
		} else {
			fields = append(fields, result)
		}
		var rows []RowRecord
		if err = tableBudget(tx, s, len(fields)); err != nil {
			return err
		}
		if err = scopeDB(tx, s).Where("deleted = ?", false).Limit(bitable.MaxExportRows + 1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > bitable.MaxExportRows || len(rows)*len(fields) > bitable.MaxExportCells {
			return bitable.ErrBudgetExceeded
		}
		if cellAffecting {
			if len(in.CellContracts) == 0 {
				recordIDs := make([]string, 0, len(rows))
				for _, row := range rows {
					recordIDs = append(recordIDs, row.ID)
				}
				if err = r.requireNoConflictingCellLeases(ctx, tx, s, []bitable.CellLeaseConflictScope{{RecordIDs: recordIDs, FieldIDs: []string{id}}}); err != nil {
					return err
				}
			} else {
				byRecord := map[string]bitable.CellContract{}
				for _, contract := range in.CellContracts {
					if contract.FieldID != id || byRecord[contract.RecordID].RecordID != "" {
						return bitable.ErrInvalidInput
					}
					byRecord[contract.RecordID] = contract
				}
				if len(byRecord) != len(rows) {
					return bitable.ErrInvalidInput
				}
				if len(rows) > 0 {
					if err = r.verifyCellContracts(ctx, tx, s, in.CellContracts); err != nil {
						return err
					}
				}
				for _, row := range rows {
					contract, ok := byRecord[row.ID]
					if !ok {
						return bitable.ErrInvalidInput
					}
					_, revisions, decodeErr := decodeRowState(row, fields)
					if decodeErr != nil {
						return decodeErr
					}
					if revisions[id] != contract.BaseRevision {
						return bitable.ErrConflict
					}
				}
			}
		} else if len(in.CellContracts) != 0 {
			return bitable.ErrInvalidInput
		}
		if index >= 0 && !result.ReadOnly && result.Type != bitable.FieldAttachment {
			for _, row := range rows {
				values := map[string]any{}
				if err = json.Unmarshal([]byte(row.Values), &values); err != nil {
					return err
				}
				if err = bitable.ValidateValue(result, values[id]); err != nil {
					return err
				}
			}
		}
		data, err := encode(fields)
		if err != nil {
			return err
		}
		if err = tableDB(tx.Model(&TableRecord{}), s).Updates(map[string]any{"fields": data, "schema_revision": table.SchemaRevision + 1, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "field.write", []string{}); err != nil {
			return err
		}
		if err = saveReceipt(tx, s, in.OperationID, "field", requestHash, result); err != nil {
			return err
		}
		return r.appendCommittedEvent(ctx, tx, s, in.OperationID, "bitable.field.committed", result)
	})
	return result, e
}

func validateViewFields(in bitable.ViewMutation, fields []bitable.Field) error {
	byID := map[string]bitable.Field{}
	for _, f := range fields {
		byID[f.ID] = f
	}
	ids := append(append([]string{}, in.Config.HiddenFieldIDs...), in.Config.ColumnOrder...)
	for id := range in.Config.ColumnWidths {
		ids = append(ids, id)
	}
	for _, f := range in.Config.Filters {
		ids = append(ids, f.FieldID)
	}
	for _, s := range in.Config.Sorts {
		ids = append(ids, s.FieldID)
	}
	if in.Config.GroupFieldID != "" {
		ids = append(ids, in.Config.GroupFieldID)
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return bitable.ErrInvalidInput
		}
	}
	if in.Kind == "kanban" && in.Config.GroupFieldID != "" && byID[in.Config.GroupFieldID].Type != bitable.FieldSingleSelect {
		return bitable.ErrInvalidInput
	}
	return nil
}
func (r *Repository) MutateView(ctx context.Context, s bitable.Scope, id string, in bitable.ViewMutation) (bitable.View, error) {
	result := bitable.View{}
	if e := bitable.ValidateViewMutation(in, id != ""); e != nil {
		return result, e
	}
	requestHash, e := hash(struct {
		ID    string
		Input bitable.ViewMutation
	}{id, in})
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		table, fields, views, err := loadTable(tx, s, true)
		if err != nil {
			return err
		}
		replay, err := receipt(tx, s, in.OperationID, "view", requestHash, &result)
		if err != nil || replay {
			return err
		}
		if err = validateViewFields(in, fields); err != nil {
			return err
		}
		if table.DataRevision >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		index := -1
		for i, v := range views {
			if v.ID == id {
				index = i
			}
		}
		revision := uint64(1)
		if id != "" {
			if index < 0 {
				return bitable.ErrNotFound
			}
			if in.ExpectedRevision == nil || views[index].Revision != *in.ExpectedRevision || views[index].Revision >= bitable.MaxSafeRevision {
				return bitable.ErrConflict
			}
			revision = views[index].Revision + 1
		} else {
			if len(views) >= 100 {
				return bitable.ErrBudgetExceeded
			}
			id = uuid.NewString()
		}
		result = bitable.View{ID: id, Name: in.Name, Kind: in.Kind, Config: in.Config, Revision: revision}
		if index >= 0 {
			views[index] = result
		} else {
			views = append(views, result)
		}
		if result.Config.HiddenFieldIDs == nil {
			result.Config.HiddenFieldIDs = []string{}
		}
		if result.Config.ColumnOrder == nil {
			result.Config.ColumnOrder = []string{}
		}
		if result.Config.ColumnWidths == nil {
			result.Config.ColumnWidths = map[string]int{}
		}
		if result.Config.Filters == nil {
			result.Config.Filters = []bitable.Filter{}
		}
		if result.Config.Sorts == nil {
			result.Config.Sorts = []bitable.Sort{}
		}
		if index >= 0 {
			views[index] = result
		} else {
			views[len(views)-1] = result
		}
		data, err := encode(views)
		if err != nil {
			return err
		}
		if err = tableDB(tx.Model(&TableRecord{}), s).Updates(map[string]any{"views": data, "data_revision": table.DataRevision + 1, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "view.write", []string{}); err != nil {
			return err
		}
		if err = saveReceipt(tx, s, in.OperationID, "view", requestHash, result); err != nil {
			return err
		}
		return r.appendCommittedEvent(ctx, tx, s, in.OperationID, "bitable.view.committed", result)
	})
	return result, e
}
