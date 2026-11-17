package documentgorm

import (
	"context"
	"time"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MutationRecorderFactory func(*gorm.DB) (document.MutationRecorder, error)

type CommentMutationRecorderFactory = MutationRecorderFactory

const deletedCommentBodyJSON = `{"type":"doc","content":[]}`

func (repository *Repository) ConfigureCommentMutationRecorderFactory(factory CommentMutationRecorderFactory) {
	repository.mutationRecorderFactory = factory
}

func (repository *Repository) ConfigureMutationRecorderFactory(factory MutationRecorderFactory) {
	repository.mutationRecorderFactory = factory
}

func (repository *Repository) CreateCommentThread(
	ctx context.Context,
	command document.CreateCommentThreadCommand,
) (document.CommentThread, error) {
	var created document.CommentThread
	err := repository.withCommentMutation(ctx, command.Thread.WorkspaceID, command.Thread.DocumentID, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		threadRow, err := commentThreadRecord(command.Thread)
		if err != nil {
			return err
		}
		if err := tx.Create(&threadRow).Error; err != nil {
			return err
		}
		commentRow := commentRecord(command.RootComment)
		commentRow.ThreadID = threadRow.ID
		if err := tx.Create(&commentRow).Error; err != nil {
			return err
		}
		if err := createCommentMentions(tx, command.Thread.WorkspaceID, commentRow.ID, command.MentionedUserIDs); err != nil {
			return err
		}
		created, err = commentThreadFromRecord(threadRow)
		if err != nil {
			return err
		}
		root := commentFromRecord(commentRow, append([]uint(nil), command.MentionedUserIDs...))
		created.Comments = []document.Comment{root}
		command.Effect.ResourceID = threadRow.ID
		command.Effect.Payload["thread_id"] = threadRow.ID
		command.Effect.Payload["comment_id"] = commentRow.ID
		command.Effect.CommentMention = &document.CommentMentionNotification{DocumentID: commentRow.DocumentID, ThreadID: threadRow.ID, CommentID: commentRow.ID, RecipientUserIDs: command.MentionedUserIDs}
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return created, err
}

func (repository *Repository) ReplyToCommentThread(
	ctx context.Context,
	command document.ReplyCommentCommand,
) (document.Comment, error) {
	var created document.Comment
	err := repository.withCommentMutation(ctx, command.WorkspaceID, command.DocumentID, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		thread, err := lockCommentThread(tx, command.WorkspaceID, command.DocumentID, command.ThreadID)
		if err != nil {
			return err
		}
		if thread.Status != string(document.CommentThreadOpen) {
			return document.ErrCommentThreadResolved
		}
		parentID, err := resolveReplyParent(tx, command, thread.ID)
		if err != nil {
			return err
		}
		row := Comment{
			WorkspaceID: command.WorkspaceID, DocumentID: command.DocumentID, ThreadID: thread.ID,
			ParentCommentID: &parentID, AuthorUserID: command.ActorUserID, BodyJSON: command.Body,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := createCommentMentions(tx, command.WorkspaceID, row.ID, command.MentionedUserIDs); err != nil {
			return err
		}
		if err := tx.Model(&CommentThread{}).Where("id = ?", thread.ID).Update("updated_at", time.Now().UTC()).Error; err != nil {
			return err
		}
		created = commentFromRecord(row, append([]uint(nil), command.MentionedUserIDs...))
		command.Effect.ResourceID = row.ID
		command.Effect.Payload["comment_id"] = row.ID
		command.Effect.CommentMention = &document.CommentMentionNotification{DocumentID: row.DocumentID, ThreadID: row.ThreadID, CommentID: row.ID, RecipientUserIDs: command.MentionedUserIDs}
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return created, err
}

func (repository *Repository) EditOwnComment(
	ctx context.Context,
	command document.EditCommentCommand,
) (document.Comment, error) {
	var updated document.Comment
	err := repository.withCommentMutation(ctx, command.WorkspaceID, command.DocumentID, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		row, err := lockOwnedComment(tx, command.WorkspaceID, command.DocumentID, command.CommentID, command.ActorUserID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&Comment{}).Where("id = ? AND deleted_at IS NULL", row.ID).
			Updates(map[string]any{"body_json": command.Body, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return document.ErrCommentNotFound
		}
		var previous []uint
		if err := tx.Model(&CommentMention{}).Where("workspace_id = ? AND comment_id = ?", command.WorkspaceID, row.ID).Pluck("user_id", &previous).Error; err != nil {
			return err
		}
		seen := make(map[uint]bool, len(previous))
		for _, id := range previous {
			seen[id] = true
		}
		added := make([]uint, 0)
		for _, id := range command.MentionedUserIDs {
			if !seen[id] {
				added = append(added, id)
				seen[id] = true
			}
		}
		command.Effect.CommentMention = &document.CommentMentionNotification{DocumentID: row.DocumentID, ThreadID: row.ThreadID, CommentID: row.ID, RecipientUserIDs: added}
		if err := replaceCommentMentions(tx, command.WorkspaceID, row.ID, command.MentionedUserIDs); err != nil {
			return err
		}
		row.BodyJSON, row.UpdatedAt = command.Body, now
		updated = commentFromRecord(row, append([]uint(nil), command.MentionedUserIDs...))
		command.Effect.ResourceID = row.ID
		command.Effect.Payload["comment_id"] = row.ID
		command.Effect.Payload["thread_id"] = row.ThreadID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return updated, err
}

func (repository *Repository) DeleteOwnComment(ctx context.Context, command document.DeleteCommentCommand) error {
	err := repository.withCommentMutation(ctx, command.WorkspaceID, command.DocumentID, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		row, err := lockOwnedComment(tx, command.WorkspaceID, command.DocumentID, command.CommentID, command.ActorUserID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&Comment{}).Where("id = ? AND deleted_at IS NULL", row.ID).Updates(map[string]any{
			"body_json": deletedCommentBodyJSON, "deleted_by": command.ActorUserID, "deleted_at": now, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return document.ErrCommentNotFound
		}
		if err := tx.Where("workspace_id = ? AND comment_id = ?", command.WorkspaceID, row.ID).Delete(&CommentMention{}).Error; err != nil {
			return err
		}
		command.Effect.ResourceID = row.ID
		command.Effect.Payload["comment_id"] = row.ID
		command.Effect.Payload["thread_id"] = row.ThreadID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return err
}

func (repository *Repository) SetCommentThreadStatus(
	ctx context.Context,
	command document.SetCommentThreadStatusCommand,
) (document.CommentThread, error) {
	var updated document.CommentThread
	err := repository.withCommentMutation(ctx, command.WorkspaceID, command.DocumentID, func(tx *gorm.DB, recorder document.MutationRecorder) error {
		row, err := lockCommentThread(tx, command.WorkspaceID, command.DocumentID, command.ThreadID)
		if err != nil {
			return err
		}
		if command.To == document.CommentThreadResolved && row.CreatedBy != command.ActorUserID {
			return document.ErrForbidden
		}
		if row.Status != string(command.From) {
			return document.ErrCommentTransitionConflict
		}
		now := time.Now().UTC()
		updates := map[string]any{"status": string(command.To), "updated_at": now}
		if command.To == document.CommentThreadResolved {
			updates["resolved_by"], updates["resolved_at"] = command.ActorUserID, now
		} else {
			updates["resolved_by"], updates["resolved_at"] = nil, nil
		}
		result := tx.Model(&CommentThread{}).Where("id = ? AND status = ?", row.ID, string(command.From)).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return document.ErrCommentTransitionConflict
		}
		if err := tx.First(&row, row.ID).Error; err != nil {
			return err
		}
		updated, err = commentThreadFromRecord(row)
		if err != nil {
			return err
		}
		command.Effect.ResourceID = row.ID
		return recorder.RecordDocumentMutation(ctx, command.Effect)
	})
	return updated, err
}

func (repository *Repository) withCommentMutation(
	ctx context.Context,
	workspaceID, documentID uint,
	apply func(*gorm.DB, document.MutationRecorder) error,
) error {
	if repository == nil || repository.db == nil || repository.mutationRecorderFactory == nil ||
		workspaceID == 0 || documentID == 0 || apply == nil {
		return document.ErrCommentsUnavailable
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var body Document
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, documentID,
		).First(&body).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return document.ErrDocumentNotFound
			}
			return err
		}
		recorder, err := repository.mutationRecorderFactory(tx)
		if err != nil || recorder == nil {
			if err != nil {
				return err
			}
			return document.ErrCommentsUnavailable
		}
		return apply(tx, recorder)
	})
}

func lockCommentThread(tx *gorm.DB, workspaceID, documentID, threadID uint) (CommentThread, error) {
	var row CommentThread
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND document_id = ?", threadID, workspaceID, documentID,
	).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return CommentThread{}, document.ErrCommentThreadNotFound
	}
	return row, err
}

func lockOwnedComment(tx *gorm.DB, workspaceID, documentID, commentID, actorID uint) (Comment, error) {
	var row Comment
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ? AND document_id = ? AND author_user_id = ? AND deleted_at IS NULL",
		commentID, workspaceID, documentID, actorID,
	).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return Comment{}, document.ErrCommentNotFound
	}
	return row, err
}

func resolveReplyParent(tx *gorm.DB, command document.ReplyCommentCommand, threadID uint) (uint, error) {
	query := tx.Where(
		"workspace_id = ? AND document_id = ? AND thread_id = ?",
		command.WorkspaceID, command.DocumentID, threadID,
	)
	if command.ParentCommentID != nil {
		query = query.Where("id = ? AND deleted_at IS NULL", *command.ParentCommentID)
	} else {
		query = query.Where("parent_comment_id IS NULL").Order("id ASC")
	}
	var row Comment
	if err := query.First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, document.ErrCommentNotFound
		}
		return 0, err
	}
	return row.ID, nil
}

func createCommentMentions(tx *gorm.DB, workspaceID, commentID uint, userIDs []uint) error {
	for _, userID := range userIDs {
		if err := tx.Create(&CommentMention{WorkspaceID: workspaceID, CommentID: commentID, UserID: userID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func replaceCommentMentions(tx *gorm.DB, workspaceID, commentID uint, userIDs []uint) error {
	if err := tx.Where("workspace_id = ? AND comment_id = ?", workspaceID, commentID).Delete(&CommentMention{}).Error; err != nil {
		return err
	}
	return createCommentMentions(tx, workspaceID, commentID, userIDs)
}

var _ document.CommentStore = (*Repository)(nil)
