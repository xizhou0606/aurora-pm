package native

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

func (r *Repository) AppendAttachment(ctx context.Context, s bitable.Scope, in bitable.AssetAppend) (bitable.Operation, error) {
	result := bitable.Operation{ID: in.OperationID, State: "committed", Records: []bitable.Record{}, DeletedRecordIDs: []string{}}
	h, e := hash(struct {
		Record, Field    string
		Schema           uint64
		Contract         bitable.CellContract
		Name, Hash, Mime string
		Size             int64
	}{in.RecordID, in.FieldID, in.ExpectedSchemaRevision, in.CellContract, in.Asset.Name, in.Asset.ContentHash, in.Asset.MimeType, in.Asset.Size})
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		table, fields, _, err := loadTable(tx, s, true)
		if err != nil {
			return err
		}
		replay, err := receipt(tx, s, in.OperationID, "attachment", h, &result)
		if err != nil || replay {
			return err
		}
		if table.SchemaRevision != in.ExpectedSchemaRevision || table.DataRevision >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		if bitable.ValidateCellContract(in.CellContract) != nil || in.CellContract.RecordID != in.RecordID || in.CellContract.FieldID != in.FieldID {
			return bitable.ErrInvalidInput
		}
		if err = r.verifyCellContracts(ctx, tx, s, []bitable.CellContract{in.CellContract}); err != nil {
			return err
		}
		found := false
		for _, f := range fields {
			if f.ID == in.FieldID && f.Type == bitable.FieldAttachment && !f.ReadOnly {
				found = true
			}
		}
		if !found {
			return bitable.ErrForbidden
		}
		var row RowRecord
		if err = scopeDB(tx, s).Where("id = ? AND deleted = ?", in.RecordID, false).First(&row).Error; err != nil {
			return dbError(err)
		}
		if row.Revision >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		values, fieldRevisions, err := decodeRowState(row, fields)
		if err != nil {
			return err
		}
		if fieldRevisions[in.FieldID] != in.CellContract.BaseRevision || fieldRevisions[in.FieldID] >= bitable.MaxSafeRevision {
			return bitable.ErrConflict
		}
		refs := []bitable.AttachmentRef{}
		if value := values[in.FieldID]; value != nil {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(data, &refs); err != nil {
				return err
			}
		}
		if len(refs) >= 100 {
			return bitable.ErrBudgetExceeded
		}
		asset := in.Asset
		asset.AttachmentRef = bitable.AttachmentPaths(s.ResourceID, s.TableID, in.RecordID, in.FieldID, asset.AttachmentRef)
		metadata, err := encode(asset)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err = tx.Create(&AssetRecord{ID: asset.ID, WorkspaceID: s.Principal.WorkspaceID, ResourceID: s.ResourceID, TableID: s.TableID, RecordID: in.RecordID, FieldID: in.FieldID, Metadata: metadata, CreatedAt: now}).Error; err != nil {
			return err
		}
		refs = append(refs, asset.AttachmentRef)
		values[in.FieldID] = refs
		fieldRevisions[in.FieldID]++
		data, err := encodeRowState(values, fieldRevisions)
		if err != nil {
			return err
		}
		row.Revision++
		row.Values = data
		row.UpdatedBy = s.Principal.UserID
		row.UpdatedAt = now
		if err = scopeDB(tx, s).Model(&RowRecord{}).Where("id = ?", row.ID).Select("revision", "values", "updated_by", "updated_at").Updates(&row).Error; err != nil {
			return err
		}
		if err = tableBudget(tx, s, len(fields)); err != nil {
			return err
		}
		if err = tableDB(tx.Model(&TableRecord{}), s).Updates(map[string]any{"data_revision": table.DataRevision + 1, "updated_at": now}).Error; err != nil {
			return err
		}
		dto, err := rowDTO(row, fields)
		if err != nil {
			return err
		}
		result.Records = append(result.Records, dto)
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "attachment.append", []string{row.ID}); err != nil {
			return err
		}
		if err = saveReceipt(tx, s, in.OperationID, "attachment", h, result); err != nil {
			return err
		}
		return r.appendCommittedEvent(ctx, tx, s, in.OperationID, "bitable.attachment.committed", result)
	})
	return result, e
}

func (r *Repository) GetAttachment(ctx context.Context, s bitable.Scope, record, field, id string) (bitable.Asset, error) {
	result := bitable.Asset{}
	e := r.transaction(ctx, true, func(tx *gorm.DB) error {
		_, fields, _, err := loadTable(tx, s, false)
		if err != nil {
			return err
		}
		found := false
		for _, f := range fields {
			if f.ID == field && f.Type == bitable.FieldAttachment {
				found = true
			}
		}
		if !found {
			return bitable.ErrNotFound
		}
		var row RowRecord
		if err = scopeDB(tx, s).Where("id = ? AND deleted = ?", record, false).First(&row).Error; err != nil {
			return dbError(err)
		}
		values := map[string]json.RawMessage{}
		if err = json.Unmarshal([]byte(row.Values), &values); err != nil {
			return err
		}
		refs := []bitable.AttachmentRef{}
		if err = json.Unmarshal(values[field], &refs); err != nil {
			return bitable.ErrNotFound
		}
		found = false
		for _, ref := range refs {
			if ref.ID == id {
				found = true
			}
		}
		if !found {
			return bitable.ErrNotFound
		}
		var asset AssetRecord
		if err = scopeDB(tx, s).Where("id = ? AND record_id = ? AND field_id = ?", id, record, field).First(&asset).Error; err != nil {
			return dbError(err)
		}
		return json.Unmarshal([]byte(asset.Metadata), &result)
	})
	return result, e
}
