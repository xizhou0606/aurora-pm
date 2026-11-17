package native

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// A duplicate owns fresh scoped attachment metadata but references immutable
// object bytes. Soft deletion never garbage-collects those shared objects.
func (r *Repository) duplicateResource(ctx context.Context, tx *gorm.DB, p bitable.Principal, source ResourceRecord, in bitable.ResourceMutation) (bitable.Resource, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	name := in.Name
	if name == "" {
		name = source.Name
	}
	v := ResourceRecord{ID: id, WorkspaceID: p.WorkspaceID, Name: name, OwnerUserID: p.UserID, Revision: 1, ParentFolderID: in.ParentFolderID, CreatedBy: p.UserID, CreatedAt: now, UpdatedAt: now}
	result := bitable.Resource{ID: id, Name: name, WorkspaceID: p.WorkspaceID, OwnerUserID: p.UserID, Revision: 1, ParentFolderID: in.ParentFolderID, Tables: []bitable.Table{}, AllowedActions: []string{}}
	if e := tx.Create(&v).Error; e != nil {
		return result, e
	}
	var tables []TableRecord
	if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ?", p.WorkspaceID, source.ID).Order("id").Find(&tables).Error; e != nil {
		return result, e
	}
	for _, old := range tables {
		oldScope := bitable.Scope{Principal: p, ResourceID: source.ID, TableID: old.ID}
		fields := []bitable.Field{}
		views := []bitable.View{}
		if e := json.Unmarshal([]byte(old.Fields), &fields); e != nil {
			return result, e
		}
		if e := json.Unmarshal([]byte(old.Views), &views); e != nil {
			return result, e
		}
		if e := tableBudget(tx, oldScope, len(fields)); e != nil {
			return result, e
		}
		fieldMap := map[string]string{}
		for i := range fields {
			fieldMap[fields[i].ID] = uuid.NewString()
			fields[i].ID = fieldMap[fields[i].ID]
		}
		// View configs contain field UUIDs only; recursive replacement preserves all
		// typed filters, orders, widths and future non-ID configuration values.
		for i := range views {
			views[i].ID = uuid.NewString()
			views[i].Revision = 1
			raw, _ := json.Marshal(views[i].Config)
			var config any
			if e := json.Unmarshal(raw, &config); e != nil {
				return result, e
			}
			raw, e := json.Marshal(remapIDs(config, fieldMap))
			if e != nil {
				return result, e
			}
			if e = json.Unmarshal(raw, &views[i].Config); e != nil {
				return result, e
			}
		}
		table := old
		table.ID = uuid.NewString()
		table.ResourceID = id
		table.SchemaRevision = 1
		table.DataRevision = 1
		table.CreatedAt = now
		table.UpdatedAt = now
		var e error
		table.Fields, e = encode(fields)
		if e != nil {
			return result, e
		}
		table.Views, e = encode(views)
		if e != nil {
			return result, e
		}
		if e = tx.Create(&table).Error; e != nil {
			return result, e
		}
		var rows []RowRecord
		if e = scopeDB(tx, oldScope).Where("deleted = ?", false).Order("sort_order,id").Find(&rows).Error; e != nil {
			return result, e
		}
		for _, row := range rows {
			oldID := row.ID
			row.ID = uuid.NewString()
			row.ResourceID = id
			row.TableID = table.ID
			row.Revision = 1
			row.CreatedBy = p.UserID
			row.UpdatedBy = p.UserID
			row.CreatedAt = now
			row.UpdatedAt = now
			values := map[string]any{}
			if e = json.Unmarshal([]byte(row.Values), &values); e != nil {
				return result, e
			}
			mapped := map[string]any{}
			for key, value := range values {
				if target, ok := fieldMap[key]; ok {
					mapped[target] = value
				}
			}
			mapped, e = r.normalizeValues(ctx, p, fields, mapped)
			if e != nil {
				return result, e
			}
			// Asset metadata has a record foreign key. Create the scoped record
			// first; its complete rewritten references remain in this transaction.
			row.Values = "{}"
			if e = tx.Create(&row).Error; e != nil {
				return result, e
			}
			for _, f := range fields {
				if f.Type != bitable.FieldAttachment {
					continue
				}
				var refs []bitable.AttachmentRef
				raw, _ := json.Marshal(mapped[f.ID])
				if e = json.Unmarshal(raw, &refs); e != nil {
					return result, e
				}
				for j, ref := range refs {
					var asset AssetRecord
					if e = scopeDB(tx, oldScope).Where("id = ? AND record_id = ?", ref.ID, oldID).First(&asset).Error; e != nil {
						return result, dbError(e)
					}
					var meta bitable.Asset
					if e = json.Unmarshal([]byte(asset.Metadata), &meta); e != nil {
						return result, e
					}
					meta.ID = uuid.NewString()
					meta.RecordID = row.ID
					meta.FieldID = f.ID
					meta.AttachmentRef = bitable.AttachmentPaths(id, table.ID, row.ID, f.ID, meta.AttachmentRef)
					asset.ID = meta.ID
					asset.ResourceID = id
					asset.TableID = table.ID
					asset.RecordID = row.ID
					asset.FieldID = f.ID
					asset.CreatedAt = now
					asset.Metadata, e = encode(meta)
					if e != nil {
						return result, e
					}
					if e = tx.Create(&asset).Error; e != nil {
						return result, e
					}
					refs[j] = meta.AttachmentRef
				}
				mapped[f.ID] = refs
			}
			row.Values, e = encode(mapped)
			if e != nil {
				return result, e
			}
			if e = tx.Model(&RowRecord{}).Where("id = ? AND resource_id = ? AND workspace_id = ?", row.ID, id, p.WorkspaceID).Update("values", row.Values).Error; e != nil {
				return result, e
			}
		}
		result.Tables = append(result.Tables, bitable.Table{ID: table.ID, Name: table.Name})
	}
	return result, nil
}
func remapIDs(v any, ids map[string]string) any {
	switch x := v.(type) {
	case string:
		if id, ok := ids[x]; ok {
			return id
		}
	case []any:
		for i := range x {
			x[i] = remapIDs(x[i], ids)
		}
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			key := k
			if id, ok := ids[k]; ok {
				key = id
			}
			out[key] = remapIDs(v, ids)
		}
		return out
	}
	return v
}
