package native

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

type CommentRecord struct {
	ID          string    `gorm:"type:uuid;primaryKey;index:idx_pm_bitable_comments_scope,priority:6"`
	WorkspaceID uint      `gorm:"not null;index:idx_pm_bitable_comments_scope,priority:1"`
	ResourceID  string    `gorm:"type:uuid;not null;index:idx_pm_bitable_comments_scope,priority:2"`
	TableID     string    `gorm:"type:uuid;not null;index:idx_pm_bitable_comments_scope,priority:3"`
	RecordID    string    `gorm:"type:uuid;not null;index:idx_pm_bitable_comments_scope,priority:4"`
	Body        string    `gorm:"type:text;not null"`
	AuthorID    uint      `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null;index:idx_pm_bitable_comments_scope,priority:5"`
}

func (CommentRecord) TableName() string { return "pm_bitable_comments" }
func commentDTO(c CommentRecord) bitable.Comment {
	return bitable.Comment{ID: c.ID, RecordID: c.RecordID, Body: c.Body, Author: bitable.UserRef{ID: c.AuthorID}, CreatedAt: c.CreatedAt}
}
func liveCommentRow(tx *gorm.DB, s bitable.Scope, record string) error {
	var row RowRecord
	return dbError(scopeDB(tx, s).Select("id").Where("id = ? AND deleted = ?", record, false).First(&row).Error)
}
func (r *Repository) ListComments(ctx context.Context, s bitable.Scope, record string, offset, limit int) (bitable.CommentPage, error) {
	result := bitable.CommentPage{Items: []bitable.Comment{}, Offset: offset}
	if offset < 0 || offset > bitable.MaxExportRows || limit < 1 || limit > bitable.MaxPageRows {
		return result, bitable.ErrInvalidInput
	}
	err := r.transaction(ctx, true, func(tx *gorm.DB) error {
		if _, _, _, e := loadTable(tx, s, false); e != nil {
			return e
		}
		if e := liveCommentRow(tx, s, record); e != nil {
			return e
		}
		query := scopeDB(tx, s).Model(&CommentRecord{}).Where("record_id = ?", record)
		if e := query.Count(&result.Total).Error; e != nil {
			return e
		}
		var rows []CommentRecord
		if e := query.Order("created_at ASC, id ASC").Offset(offset).Limit(limit).Find(&rows).Error; e != nil {
			return e
		}
		for _, row := range rows {
			result.Items = append(result.Items, commentDTO(row))
		}
		return nil
	})
	return result, err
}
func (r *Repository) CreateComment(ctx context.Context, s bitable.Scope, record string, in bitable.CommentInput) (bitable.Comment, error) {
	result := bitable.Comment{}
	if e := bitable.ValidateComment(in); e != nil {
		return result, e
	}
	requestHash, e := hash(struct{ Record, Body string }{record, in.Body})
	if e != nil {
		return result, e
	}
	e = r.transaction(ctx, false, func(tx *gorm.DB) error {
		if _, _, _, err := loadTable(tx, s, true); err != nil {
			return err
		}
		// A deleted row remains inaccessible even when this key previously committed.
		if err := liveCommentRow(tx, s, record); err != nil {
			return err
		}
		replay, err := receipt(tx, s, in.OperationID, "comment", requestHash, &result)
		if err != nil || replay {
			return err
		}
		var count int64
		if err = scopeDB(tx, s).Model(&CommentRecord{}).Where("record_id = ?", record).Count(&count).Error; err != nil {
			return err
		}
		if count >= bitable.MaxExportRows {
			return bitable.ErrBudgetExceeded
		}
		row := CommentRecord{ID: uuid.NewString(), WorkspaceID: s.Principal.WorkspaceID, ResourceID: s.ResourceID, TableID: s.TableID, RecordID: record, Body: in.Body, AuthorID: s.Principal.UserID, CreatedAt: time.Now().UTC()}
		if err = tx.Create(&row).Error; err != nil {
			return err
		}
		result = commentDTO(row)
		if err = r.recordMutation(ctx, tx, s, in.OperationID, "comment.create", []string{record}); err != nil {
			return err
		}
		if err = saveReceipt(tx, s, in.OperationID, "comment", requestHash, result); err != nil {
			return err
		}
		return r.appendCommittedEvent(ctx, tx, s, in.OperationID, "bitable.comment.committed", result)
	})
	return result, e
}
