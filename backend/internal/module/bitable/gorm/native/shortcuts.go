package native

import (
	"context"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (r *Repository) Shortcuts(ctx context.Context, p bitable.Principal) ([]bitable.Shortcut, error) {
	result := []bitable.Shortcut{}
	var rows []ShortcutRecord
	e := r.db.WithContext(ctx).Where("workspace_id = ? AND created_by = ? AND deleted = ?", p.WorkspaceID, p.UserID, false).Order("created_at,id").Limit(bitable.MaxExportRows + 1).Find(&rows).Error
	if e != nil {
		return result, e
	}
	if len(rows) > bitable.MaxExportRows {
		return result, bitable.ErrBudgetExceeded
	}
	for _, v := range rows {
		target, e := r.Get(ctx, p.WorkspaceID, v.TargetResourceID)
		if e == bitable.ErrNotFound {
			continue
		}
		if e != nil {
			return result, e
		}
		result = append(result, bitable.Shortcut{ID: v.ID, TargetResourceID: v.TargetResourceID, ParentFolderID: v.ParentFolderID, Revision: v.Revision, Name: target.Name})
	}
	return result, nil
}

func (r *Repository) GetShortcut(ctx context.Context, p bitable.Principal, id string) (bitable.Shortcut, error) {
	var row ShortcutRecord
	if err := r.db.WithContext(ctx).Where(
		"id = ? AND workspace_id = ? AND created_by = ? AND deleted = ?", id, p.WorkspaceID, p.UserID, false,
	).First(&row).Error; err != nil {
		return bitable.Shortcut{}, dbError(err)
	}
	return bitable.Shortcut{
		ID: row.ID, TargetResourceID: row.TargetResourceID, ParentFolderID: row.ParentFolderID, Revision: row.Revision,
	}, nil
}
func (r *Repository) MutateShortcut(ctx context.Context, p bitable.Principal, id, kind string, in bitable.ResourceMutation) (bitable.Shortcut, error) {
	var result bitable.Shortcut
	h, e := hash(in)
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		targetID := id
		v := ShortcutRecord{}
		if kind != "shortcut" {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND created_by = ? AND id = ?", p.WorkspaceID, p.UserID, id).First(&v).Error; err != nil {
				return dbError(err)
			}
			targetID = v.TargetResourceID
		}
		target, err := lockResource(tx, p, targetID)
		if err != nil {
			return err
		}
		s := resourceScope(p, id)
		s.ResourceID = targetID
		replay, err := receipt(tx, s, in.OperationID, "link."+kind, h, &result)
		if err != nil || replay {
			return err
		}
		if target.Deleted || v.Deleted {
			return bitable.ErrNotFound
		}
		now := time.Now().UTC()
		if kind == "shortcut" {
			if target.Revision != in.ExpectedRevision {
				return bitable.ErrConflict
			}
			v = ShortcutRecord{ID: uuid.NewString(), WorkspaceID: p.WorkspaceID, TargetResourceID: id, ParentFolderID: in.ParentFolderID, Revision: 1, CreatedBy: p.UserID, CreatedAt: now, UpdatedAt: now}
		} else {
			if v.Revision != in.ExpectedRevision || v.Revision >= bitable.MaxSafeRevision {
				return bitable.ErrConflict
			}
			v.Revision++
			v.UpdatedAt = now
			switch kind {
			case "location":
				v.ParentFolderID = in.ParentFolderID
			case "delete":
				v.Deleted = true
			default:
				return bitable.ErrInvalidInput
			}
		}
		if err = tx.Save(&v).Error; err != nil {
			return err
		}
		result = bitable.Shortcut{ID: v.ID, TargetResourceID: v.TargetResourceID, ParentFolderID: v.ParentFolderID, Revision: v.Revision, Name: target.Name}
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "shortcut."+kind, []string{v.ID}); err != nil {
			return err
		}
		return saveReceipt(tx, s, in.OperationID, "link."+kind, h, result)
	})
	return result, e
}
