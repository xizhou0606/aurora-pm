package documentgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
)

type commentRecorderStub struct {
	effects []document.MutationEffect
	err     error
}

func (stub *commentRecorderStub) RecordDocumentMutation(_ context.Context, effect document.MutationEffect) error {
	stub.effects = append(stub.effects, effect)
	return stub.err
}

func prepareCommentRepository(t *testing.T, recorder *commentRecorderStub) (*Repository, document.Document) {
	t.Helper()
	db := openDocumentDB(t)
	repository := NewRepository(db)
	repository.ConfigureCommentMutationRecorderFactory(func(*gorm.DB) (document.MutationRecorder, error) {
		return recorder, nil
	})
	space, err := repository.EnsureDefaultSpace(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	content, hash, plain, err := document.NormalizeContent(document.DefaultContent())
	if err != nil {
		t.Fatal(err)
	}
	_, body, err := repository.CreateNode(context.Background(), document.Node{
		WorkspaceID: 7, SpaceID: space.ID, Type: document.NodeDocument, Title: "Comments",
		SortOrder: 1, CreatedBy: 5, UpdatedBy: 5,
	}, &document.Document{
		WorkspaceID: 7, Content: content, SchemaVersion: 1, Revision: 1,
		ContentHash: hash, PlainText: plain, CreatedBy: 5, UpdatedBy: 5,
	})
	if err != nil || body == nil {
		t.Fatalf("body=%+v err=%v", body, err)
	}
	return repository, *body
}

func TestCommentRepositoryLifecycleAndWorkspaceIsolation(t *testing.T) {
	recorder := &commentRecorderStub{}
	repository, body := prepareCommentRepository(t, recorder)
	ctx := context.Background()
	thread, err := repository.CreateCommentThread(ctx, document.CreateCommentThreadCommand{
		Thread: document.CommentThread{
			WorkspaceID: 7, DocumentID: body.ID, Status: document.CommentThreadOpen,
			Anchor: document.CommentAnchor{Kind: document.CommentAnchorDocument}, CreatedBy: 5,
		},
		RootComment:      document.Comment{WorkspaceID: 7, DocumentID: body.ID, AuthorUserID: 5, Body: `{"type":"doc"}`},
		MentionedUserIDs: []uint{8}, Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Resource: "document_comment", Action: "comment_thread_created", Payload: map[string]any{}},
	})
	if err != nil || thread.ID == 0 || len(thread.Comments) != 1 || thread.Comments[0].MentionedUserIDs[0] != 8 {
		t.Fatalf("thread=%+v err=%v", thread, err)
	}
	reply, err := repository.ReplyToCommentThread(ctx, document.ReplyCommentCommand{
		WorkspaceID: 7, DocumentID: body.ID, ThreadID: thread.ID, ActorUserID: 6, Body: `{"type":"doc"}`,
		MentionedUserIDs: []uint{9}, Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 6, Resource: "document_comment", Action: "comment_replied", Payload: map[string]any{}},
	})
	if err != nil || reply.ParentCommentID == nil || *reply.ParentCommentID != thread.Comments[0].ID {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}
	if recorder.effects[len(recorder.effects)-1].CommentMention.CommentID != reply.ID {
		t.Fatal("missing reply notification metadata")
	}
	editCommand := document.EditCommentCommand{WorkspaceID: 7, DocumentID: body.ID, CommentID: reply.ID, ActorUserID: 6,
		Body: `{"type":"doc"}`, MentionedUserIDs: []uint{9, 10}, Effect: document.MutationEffect{Payload: map[string]any{}}}
	if _, err := repository.EditOwnComment(ctx, editCommand); err != nil {
		t.Fatal(err)
	}
	added := recorder.effects[len(recorder.effects)-1].CommentMention.RecipientUserIDs
	if len(added) != 1 || added[0] != 10 {
		t.Fatalf("added=%v", added)
	}
	if _, err := repository.EditOwnComment(ctx, editCommand); err != nil {
		t.Fatal(err)
	}
	if len(recorder.effects[len(recorder.effects)-1].CommentMention.RecipientUserIDs) != 0 {
		t.Fatal("unchanged mentions notified twice")
	}
	if _, err := repository.EditOwnComment(ctx, document.EditCommentCommand{
		WorkspaceID: 7, DocumentID: body.ID, CommentID: reply.ID, ActorUserID: 5, Body: `{"type":"doc"}`,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Resource: "document_comment", Action: "comment_edited", Payload: map[string]any{}},
	}); !errors.Is(err, document.ErrCommentNotFound) {
		t.Fatalf("foreign edit err=%v", err)
	}
	previousEffects := len(recorder.effects)
	if _, err := repository.SetCommentThreadStatus(ctx, document.SetCommentThreadStatusCommand{
		WorkspaceID: 7, DocumentID: body.ID, ThreadID: thread.ID, ActorUserID: 6,
		From: document.CommentThreadOpen, To: document.CommentThreadResolved,
	}); !errors.Is(err, document.ErrForbidden) {
		t.Fatalf("foreign resolve err=%v", err)
	}
	if len(recorder.effects) != previousEffects {
		t.Fatal("forbidden resolution emitted an effect")
	}
	if _, err := repository.SetCommentThreadStatus(ctx, document.SetCommentThreadStatusCommand{
		WorkspaceID: 7, DocumentID: body.ID, ThreadID: thread.ID, ActorUserID: 5,
		From: document.CommentThreadOpen, To: document.CommentThreadResolved,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Resource: "document_comment", Action: "comment_thread_resolved", Payload: map[string]any{}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplyToCommentThread(ctx, document.ReplyCommentCommand{
		WorkspaceID: 7, DocumentID: body.ID, ThreadID: thread.ID, ActorUserID: 6, Body: `{"type":"doc"}`,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 6, Resource: "document_comment", Action: "comment_replied", Payload: map[string]any{}},
	}); !errors.Is(err, document.ErrCommentThreadResolved) {
		t.Fatalf("resolved reply err=%v", err)
	}
	if err := repository.DeleteOwnComment(ctx, document.DeleteCommentCommand{
		WorkspaceID: 7, DocumentID: body.ID, CommentID: reply.ID, ActorUserID: 6,
		Effect: document.MutationEffect{WorkspaceID: 7, PrincipalID: 6, Resource: "document_comment", Action: "comment_deleted", Payload: map[string]any{}},
	}); err != nil {
		t.Fatal(err)
	}
	var deletedRow Comment
	if err := repository.db.First(&deletedRow, reply.ID).Error; err != nil {
		t.Fatal(err)
	}
	if deletedRow.DeletedAt == nil || !json.Valid([]byte(deletedRow.BodyJSON)) || deletedRow.BodyJSON != deletedCommentBodyJSON {
		t.Fatalf("deleted row=%+v", deletedRow)
	}
	var mentionCount int64
	if err := repository.db.Model(&CommentMention{}).Where("comment_id = ?", reply.ID).Count(&mentionCount).Error; err != nil || mentionCount != 0 {
		t.Fatalf("mention count=%d err=%v", mentionCount, err)
	}
	page, err := repository.ListCommentThreads(ctx, 7, body.ID, document.CommentThreadFilter{Status: document.CommentThreadAll, Limit: 20})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || len(page.Items[0].Comments) != 2 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].Comments[1].DeletedAt == nil || page.Items[0].Comments[1].Body != "" || len(page.Items[0].Comments[1].MentionedUserIDs) != 0 {
		t.Fatalf("deleted comment leaked data: %+v", page.Items[0].Comments[1])
	}
	foreign, err := repository.ListCommentThreads(ctx, 8, body.ID, document.CommentThreadFilter{Status: document.CommentThreadAll, Limit: 20})
	if err != nil || foreign.Total != 0 || len(foreign.Items) != 0 {
		t.Fatalf("foreign page=%+v err=%v", foreign, err)
	}
	if len(recorder.effects) != 6 {
		t.Fatalf("effects=%+v", recorder.effects)
	}
}

func TestCommentRepositoryRollsBackWhenOutboxEffectFails(t *testing.T) {
	recorder := &commentRecorderStub{err: errors.New("outbox unavailable")}
	repository, body := prepareCommentRepository(t, recorder)
	_, err := repository.CreateCommentThread(context.Background(), document.CreateCommentThreadCommand{
		Thread: document.CommentThread{
			WorkspaceID: 7, DocumentID: body.ID, Status: document.CommentThreadOpen,
			Anchor: document.CommentAnchor{Kind: document.CommentAnchorDocument}, CreatedBy: 5,
		},
		RootComment: document.Comment{WorkspaceID: 7, DocumentID: body.ID, AuthorUserID: 5, Body: `{"type":"doc"}`},
		Effect:      document.MutationEffect{WorkspaceID: 7, PrincipalID: 5, Resource: "document_comment", Action: "comment_thread_created", Payload: map[string]any{}},
	})
	if err == nil {
		t.Fatal("expected effect failure")
	}
	page, listErr := repository.ListCommentThreads(context.Background(), 7, body.ID, document.CommentThreadFilter{Status: document.CommentThreadAll, Limit: 20})
	if listErr != nil || page.Total != 0 || page.Items == nil {
		t.Fatalf("page=%+v err=%v", page, listErr)
	}
}

func TestCommentRepositoryKeepsEmptyMentionListsAsJSONArrays(t *testing.T) {
	repository, body := prepareCommentRepository(t, &commentRecorderStub{})
	thread, err := repository.CreateCommentThread(context.Background(), document.CreateCommentThreadCommand{
		Thread: document.CommentThread{
			WorkspaceID: 7, DocumentID: body.ID, Status: document.CommentThreadOpen,
			Anchor: document.CommentAnchor{Kind: document.CommentAnchorDocument}, CreatedBy: 5,
		},
		RootComment: document.Comment{
			WorkspaceID: 7, DocumentID: body.ID, AuthorUserID: 5, Body: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"请确认"}]}]}`,
		},
		MentionedUserIDs: nil,
		Effect: document.MutationEffect{
			WorkspaceID: 7, PrincipalID: 5, Resource: "document_comment", Action: "comment_thread_created", Payload: map[string]any{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if thread.Comments[0].MentionedUserIDs == nil {
		t.Fatalf("created comment returned nil mentions: %+v", thread.Comments[0])
	}

	page, err := repository.ListCommentThreads(context.Background(), 7, body.ID, document.CommentThreadFilter{
		Status: document.CommentThreadAll, Limit: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].Comments[0].MentionedUserIDs == nil {
		t.Fatalf("listed comment returned nil mentions: %+v", page.Items[0].Comments[0])
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"mentioned_user_ids":[]`) {
		t.Fatalf("comment page did not preserve the array contract: %s", encoded)
	}
}
