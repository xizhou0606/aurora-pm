package document

import (
	"context"
)

func (service *Service) ListCommentThreads(
	ctx context.Context,
	principal Principal,
	documentID uint,
	filter CommentThreadFilter,
) (CommentThreadPage, error) {
	documentValue, state, role, err := service.commentAccess(ctx, principal, documentID, string(ActionCommentsView), roleViewer)
	if err != nil {
		return CommentThreadPage{}, err
	}
	filter, err = normalizeCommentThreadFilter(filter)
	if err != nil {
		return CommentThreadPage{}, err
	}
	page, err := service.comments.ListCommentThreads(ctx, principal.WorkspaceID, documentID, filter)
	if err != nil {
		return CommentThreadPage{}, err
	}
	for threadIndex := range page.Items {
		thread := &page.Items[threadIndex]
		thread.Anchor = resolveCommentAnchor(thread.Anchor, documentValue)
		thread.CanResolve = thread.CreatedBy == principal.UserID && thread.Status == CommentThreadOpen && role >= roleCommenter && state.has(string(ActionCommentsResolve))
		thread.CanReopen = thread.Status == CommentThreadResolved && role >= roleCommenter && state.has(string(ActionCommentsReopen))
		for commentIndex := range thread.Comments {
			comment := &thread.Comments[commentIndex]
			isAuthor := comment.DeletedAt == nil && comment.AuthorUserID == principal.UserID
			comment.CanEdit = isAuthor && role >= roleCommenter && state.has(string(ActionCommentsEditOwn))
			comment.CanDelete = isAuthor && role >= roleCommenter && state.has(string(ActionCommentsDeleteOwn))
		}
	}
	if page.Items == nil {
		page.Items = []CommentThread{}
	}
	return page, nil
}

func (service *Service) CreateCommentThread(
	ctx context.Context,
	principal Principal,
	documentID uint,
	input CreateCommentThreadInput,
) (CommentThread, error) {
	documentValue, state, _, err := service.commentAccess(ctx, principal, documentID, string(ActionCommentsCreate), roleCommenter)
	if err != nil {
		return CommentThread{}, err
	}
	anchor, err := normalizeCommentAnchor(input.Anchor, documentValue)
	if err != nil {
		return CommentThread{}, err
	}
	body, mentions, err := service.normalizeCommentBody(ctx, principal.WorkspaceID, input.Body)
	if err != nil {
		return CommentThread{}, err
	}
	thread, err := service.comments.CreateCommentThread(ctx, CreateCommentThreadCommand{
		Thread: CommentThread{
			WorkspaceID: principal.WorkspaceID, DocumentID: documentID,
			Anchor: anchor, Status: CommentThreadOpen, CreatedBy: principal.UserID,
		},
		RootComment: Comment{
			WorkspaceID: principal.WorkspaceID, DocumentID: documentID,
			AuthorUserID: principal.UserID, Body: body,
		},
		MentionedUserIDs: mentions,
		Effect: commentMutationEffect(principal, "document_comment_thread", "comment_thread_created", map[string]any{
			"document_id": documentID, "mention_user_ids": mentions,
		}),
	})
	if err != nil {
		return CommentThread{}, err
	}
	thread.Anchor = resolveCommentAnchor(thread.Anchor, documentValue)
	thread.CanResolve = state.has(string(ActionCommentsResolve))
	if len(thread.Comments) > 0 {
		thread.Comments[0].CanEdit = state.has(string(ActionCommentsEditOwn))
		thread.Comments[0].CanDelete = state.has(string(ActionCommentsDeleteOwn))
	}
	return thread, nil
}

func (service *Service) ReplyToCommentThread(
	ctx context.Context,
	principal Principal,
	documentID uint,
	threadID uint,
	input ReplyCommentInput,
) (Comment, error) {
	if threadID == 0 {
		return Comment{}, ErrInvalidInput
	}
	_, state, _, err := service.commentAccess(ctx, principal, documentID, string(ActionCommentsReply), roleCommenter)
	if err != nil {
		return Comment{}, err
	}
	body, mentions, err := service.normalizeCommentBody(ctx, principal.WorkspaceID, input.Body)
	if err != nil {
		return Comment{}, err
	}
	comment, err := service.comments.ReplyToCommentThread(ctx, ReplyCommentCommand{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, ThreadID: threadID,
		ParentCommentID: input.ParentCommentID, ActorUserID: principal.UserID,
		Body: body, MentionedUserIDs: mentions,
		Effect: commentMutationEffect(principal, "document_comment", "comment_replied", map[string]any{
			"document_id": documentID, "thread_id": threadID, "mention_user_ids": mentions,
		}),
	})
	if err == nil {
		comment.CanEdit = state.has(string(ActionCommentsEditOwn))
		comment.CanDelete = state.has(string(ActionCommentsDeleteOwn))
	}
	return comment, err
}

func (service *Service) EditOwnComment(
	ctx context.Context,
	principal Principal,
	documentID uint,
	commentID uint,
	bodyInput string,
) (Comment, error) {
	if commentID == 0 {
		return Comment{}, ErrInvalidInput
	}
	_, state, _, err := service.commentAccess(ctx, principal, documentID, string(ActionCommentsEditOwn), roleCommenter)
	if err != nil {
		return Comment{}, err
	}
	body, mentions, err := service.normalizeCommentBody(ctx, principal.WorkspaceID, bodyInput)
	if err != nil {
		return Comment{}, err
	}
	comment, err := service.comments.EditOwnComment(ctx, EditCommentCommand{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, CommentID: commentID,
		ActorUserID: principal.UserID, Body: body, MentionedUserIDs: mentions,
		Effect: commentMutationEffect(principal, "document_comment", "comment_edited", map[string]any{
			"document_id": documentID, "mention_user_ids": mentions,
		}),
	})
	if err == nil {
		comment.CanEdit = true
		comment.CanDelete = state.has(string(ActionCommentsDeleteOwn))
	}
	return comment, err
}

func (service *Service) DeleteOwnComment(
	ctx context.Context,
	principal Principal,
	documentID uint,
	commentID uint,
) error {
	if commentID == 0 {
		return ErrInvalidInput
	}
	if _, _, _, err := service.commentAccess(ctx, principal, documentID, string(ActionCommentsDeleteOwn), roleCommenter); err != nil {
		return err
	}
	return service.comments.DeleteOwnComment(ctx, DeleteCommentCommand{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, CommentID: commentID,
		ActorUserID: principal.UserID,
		Effect: commentMutationEffect(
			principal, "document_comment", "comment_deleted", map[string]any{"document_id": documentID},
		),
	})
}

func (service *Service) ResolveCommentThread(ctx context.Context, principal Principal, documentID, threadID uint) (CommentThread, error) {
	return service.setCommentThreadStatus(ctx, principal, documentID, threadID, string(ActionCommentsResolve), CommentThreadOpen, CommentThreadResolved)
}

func (service *Service) ReopenCommentThread(ctx context.Context, principal Principal, documentID, threadID uint) (CommentThread, error) {
	return service.setCommentThreadStatus(ctx, principal, documentID, threadID, string(ActionCommentsReopen), CommentThreadResolved, CommentThreadOpen)
}

func (service *Service) setCommentThreadStatus(
	ctx context.Context,
	principal Principal,
	documentID, threadID uint,
	action string,
	from, to CommentThreadStatus,
) (CommentThread, error) {
	if threadID == 0 {
		return CommentThread{}, ErrInvalidInput
	}
	documentValue, state, role, err := service.commentAccess(ctx, principal, documentID, action, roleCommenter)
	if err != nil {
		return CommentThread{}, err
	}
	actionName := "comment_thread_resolved"
	if to == CommentThreadOpen {
		actionName = "comment_thread_reopened"
	}
	thread, err := service.comments.SetCommentThreadStatus(ctx, SetCommentThreadStatusCommand{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, ThreadID: threadID,
		ActorUserID: principal.UserID, From: from, To: to,
		Effect: commentMutationEffect(principal, "document_comment_thread", actionName, map[string]any{
			"document_id": documentID, "thread_id": threadID,
		}),
	})
	if err != nil {
		return CommentThread{}, err
	}
	thread.Anchor = resolveCommentAnchor(thread.Anchor, documentValue)
	thread.CanResolve = thread.CreatedBy == principal.UserID && thread.Status == CommentThreadOpen && role >= roleCommenter && state.has(string(ActionCommentsResolve))
	thread.CanReopen = thread.Status == CommentThreadResolved && role >= roleCommenter && state.has(string(ActionCommentsReopen))
	return thread, nil
}

func commentMutationEffect(principal Principal, resource, action string, payload map[string]any) MutationEffect {
	return MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID,
		Resource: resource, Action: action, Payload: payload,
	}
}
