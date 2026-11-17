package native

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (r *Repository) ResourcePreference(ctx context.Context, p bitable.Principal, id string) (bitable.ResourcePreference, error) {
	var v ResourcePreferenceRecord
	e := r.db.WithContext(ctx).Where("workspace_id = ? AND actor_id = ? AND resource_id = ?", p.WorkspaceID, p.UserID, id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return bitable.ResourcePreference{}, nil
	}
	return bitable.ResourcePreference{Revision: v.Revision, Pinned: v.Pinned, Favorited: v.Favorited}, e
}
func lockResource(tx *gorm.DB, p bitable.Principal, id string) (ResourceRecord, error) {
	var v ResourceRecord
	e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", p.WorkspaceID, id).First(&v).Error
	return v, dbError(e)
}
func resourceScope(p bitable.Principal, id string) bitable.Scope {
	return bitable.Scope{Principal: p, ResourceID: id, TableID: id}
}
func (r *Repository) MutateResource(ctx context.Context, p bitable.Principal, id, kind string, in bitable.ResourceMutation) (bitable.Resource, error) {
	var result bitable.Resource
	h, e := hash(in)
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		v, err := lockResource(tx, p, id)
		if err != nil {
			return err
		}
		s := resourceScope(p, id)
		replay, err := receipt(tx, s, in.OperationID, "res."+kind, h, &result)
		if err != nil || replay {
			return err
		}
		if v.Deleted {
			return bitable.ErrNotFound
		}
		if kind == "pin" || kind == "favorite" {
			err = r.mutatePreference(tx, p, v, kind, in)
			if err != nil {
				return err
			}
		} else {
			if v.Revision != in.ExpectedRevision || v.Revision >= bitable.MaxSafeRevision {
				return bitable.ErrConflict
			}
			switch kind {
			case "name":
				v.Name = in.Name
			case "location":
				v.ParentFolderID = in.ParentFolderID
			case "owner":
				v.OwnerUserID = in.OwnerUserID
			case "delete":
				v.Deleted = true
			case "duplicate":
				result, err = r.duplicateResource(ctx, tx, p, v, in)
				if err != nil {
					return err
				}
			default:
				return bitable.ErrInvalidInput
			}
			if kind != "duplicate" {
				v.Revision++
				v.UpdatedAt = time.Now().UTC()
				if err = tx.Save(&v).Error; err != nil {
					return err
				}
			}
		}
		if kind != "duplicate" {
			result = bitable.Resource{ID: v.ID, Name: v.Name, WorkspaceID: v.WorkspaceID, Revision: v.Revision, OwnerUserID: v.OwnerUserID, ParentFolderID: v.ParentFolderID, Tables: []bitable.Table{}, AllowedActions: []string{}}
			var tables []TableRecord
			if err = tx.Where("workspace_id = ? AND resource_id = ?", p.WorkspaceID, id).Order("created_at,id").Find(&tables).Error; err != nil {
				return err
			}
			for _, t := range tables {
				result.Tables = append(result.Tables, bitable.Table{ID: t.ID, Name: t.Name})
			}
			var pref ResourcePreferenceRecord
			err = tx.Where("workspace_id = ? AND actor_id = ? AND resource_id = ?", p.WorkspaceID, p.UserID, id).First(&pref).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			result.Pinned = pref.Pinned
			result.Favorited = pref.Favorited
			result.PreferenceRevision = pref.Revision
		}
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "resource."+kind, []string{}); err != nil {
			return err
		}
		return saveReceipt(tx, s, in.OperationID, "res."+kind, h, result)
	})
	return result, e
}
func (r *Repository) mutatePreference(tx *gorm.DB, p bitable.Principal, v ResourceRecord, kind string, in bitable.ResourceMutation) error {
	var pref ResourcePreferenceRecord
	e := tx.Where("workspace_id = ? AND actor_id = ? AND resource_id = ?", p.WorkspaceID, p.UserID, v.ID).First(&pref).Error
	if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return e
	}
	if pref.Revision != in.ExpectedRevision || pref.Revision >= bitable.MaxSafeRevision {
		return bitable.ErrConflict
	}
	pref.WorkspaceID = p.WorkspaceID
	pref.ResourceID = v.ID
	pref.ActorID = p.UserID
	pref.Revision++
	pref.UpdatedAt = time.Now().UTC()
	if kind == "pin" {
		pref.Pinned = *in.Pinned
	} else {
		pref.Favorited = *in.Favorited
	}
	return tx.Save(&pref).Error
}
