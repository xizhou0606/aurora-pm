package documentgorm

import (
	"context"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func (repository *Repository) ListCommentThreads(
	ctx context.Context,
	workspaceID, documentID uint,
	filter document.CommentThreadFilter,
) (document.CommentThreadPage, error) {
	query := repository.db.WithContext(ctx).Model(&CommentThread{}).
		Where("workspace_id = ? AND document_id = ?", workspaceID, documentID)
	if filter.Status != document.CommentThreadAll {
		query = query.Where("status = ?", string(filter.Status))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return document.CommentThreadPage{}, err
	}
	var rows []CommentThread
	if err := query.Order("updated_at DESC, id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return document.CommentThreadPage{}, err
	}
	items := make([]document.CommentThread, 0, len(rows))
	threadIndexes := make(map[uint]int, len(rows))
	threadIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		thread, err := commentThreadFromRecord(row)
		if err != nil {
			return document.CommentThreadPage{}, err
		}
		threadIndexes[thread.ID] = len(items)
		threadIDs = append(threadIDs, thread.ID)
		items = append(items, thread)
	}
	if len(threadIDs) == 0 {
		return document.CommentThreadPage{Items: items, Total: total}, nil
	}
	var commentRows []Comment
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND document_id = ? AND thread_id IN ?", workspaceID, documentID, threadIDs).
		Order("id ASC").Find(&commentRows).Error; err != nil {
		return document.CommentThreadPage{}, err
	}
	mentionsByComment, err := repository.commentMentions(ctx, workspaceID, commentRows)
	if err != nil {
		return document.CommentThreadPage{}, err
	}
	for _, row := range commentRows {
		index, exists := threadIndexes[row.ThreadID]
		if !exists {
			continue
		}
		items[index].Comments = append(items[index].Comments, commentFromRecord(row, mentionsByComment[row.ID]))
	}
	return document.CommentThreadPage{Items: items, Total: total}, nil
}

func (repository *Repository) commentMentions(ctx context.Context, workspaceID uint, comments []Comment) (map[uint][]uint, error) {
	ids := make([]uint, 0, len(comments))
	for _, row := range comments {
		if row.DeletedAt == nil {
			ids = append(ids, row.ID)
		}
	}
	result := make(map[uint][]uint, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []CommentMention
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND comment_id IN ?", workspaceID, ids).
		Order("comment_id ASC, user_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.CommentID] = append(result[row.CommentID], row.UserID)
	}
	return result, nil
}
