package document

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type stubCommentStore struct {
	page          CommentThreadPage
	createCommand CreateCommentThreadCommand
	replyCommand  ReplyCommentCommand
	editCommand   EditCommentCommand
	deleteCommand DeleteCommentCommand
	statusCommand SetCommentThreadStatusCommand
}

func TestCutoverCommentAnchorsUseDurableCollaborationProjection(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionCommentsCreate)})
	comments := &stubCommentStore{}
	service.ConfigureComments(comments, &stubCommentMentions{})
	cutoverAt := time.Now().UTC()
	service.store.(*stubStore).documents[11] = Document{
		ID: 11, WorkspaceID: 7, Revision: 1, CollaborationCutoverAt: &cutoverAt,
		Content: []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"legacy"}]}]}`),
	}
	service.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{
		11: {ResourceID: "resource-11", CanView: true, CanComment: true},
	}})
	service.ConfigureCollaborationProjection(stubCollaborationProjection{value: CollaborationProjection{
		Content:       []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"durable text"}]}]}`),
		ContentHash:   "durable-hash",
		PlainText:     "durable text",
		SchemaVersion: SchemaVersionV3,
		Revision:      8,
	}})
	_, err := service.CreateCommentThread(context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, CreateCommentThreadInput{
		Anchor: CommentAnchor{Kind: CommentAnchorSelectionSnapshotV1, Selection: &SelectionSnapshotV1{
			DocumentRevision: 8, From: 1, To: 8, Quote: "durable",
		}},
		Body: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"review"}]}]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if comments.createCommand.Thread.Anchor.Selection == nil || comments.createCommand.Thread.Anchor.Selection.DocumentRevision != 8 {
		t.Fatalf("anchor=%+v", comments.createCommand.Thread.Anchor)
	}
}

func (stub *stubCommentStore) ListCommentThreads(context.Context, uint, uint, CommentThreadFilter) (CommentThreadPage, error) {
	return stub.page, nil
}

func (stub *stubCommentStore) CreateCommentThread(_ context.Context, command CreateCommentThreadCommand) (CommentThread, error) {
	stub.createCommand = command
	command.Thread.ID = 31
	command.RootComment.ID = 41
	command.RootComment.ThreadID = 31
	command.Thread.Comments = []Comment{command.RootComment}
	return command.Thread, nil
}

func (stub *stubCommentStore) ReplyToCommentThread(_ context.Context, command ReplyCommentCommand) (Comment, error) {
	stub.replyCommand = command
	return Comment{ID: 42, WorkspaceID: command.WorkspaceID, DocumentID: command.DocumentID, ThreadID: command.ThreadID, AuthorUserID: command.ActorUserID, Body: command.Body}, nil
}

func (stub *stubCommentStore) EditOwnComment(_ context.Context, command EditCommentCommand) (Comment, error) {
	stub.editCommand = command
	return Comment{ID: command.CommentID, WorkspaceID: command.WorkspaceID, DocumentID: command.DocumentID, AuthorUserID: command.ActorUserID, Body: command.Body}, nil
}

func (stub *stubCommentStore) DeleteOwnComment(_ context.Context, command DeleteCommentCommand) error {
	stub.deleteCommand = command
	return nil
}

func (stub *stubCommentStore) SetCommentThreadStatus(_ context.Context, command SetCommentThreadStatusCommand) (CommentThread, error) {
	stub.statusCommand = command
	return CommentThread{ID: command.ThreadID, WorkspaceID: command.WorkspaceID, DocumentID: command.DocumentID, Status: command.To}, nil
}

type stubCommentMentions struct {
	invalid bool
	ids     []uint
}

func (stub *stubCommentMentions) ValidateCommentMentionUserIDs(_ context.Context, _ uint, ids []uint) error {
	stub.ids = append([]uint(nil), ids...)
	if stub.invalid {
		return errors.New("inactive user")
	}
	return nil
}

func TestTesterCreatesSelectionCommentWithCanonicalPMMentions(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionCommentsCreate)})
	comments := &stubCommentStore{}
	mentions := &stubCommentMentions{}
	service.ConfigureComments(comments, mentions)
	service.store.(*stubStore).documents[11] = Document{
		ID: 11, WorkspaceID: 7, Revision: 1,
		Content: []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"这个方案可行"}]}]}`),
	}
	body := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"8","label":"Other","username":"other","open_id":"forbidden"}},{"type":"text","text":" 请复核"}]}]}`
	thread, err := service.CreateCommentThread(context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, CreateCommentThreadInput{
		Anchor: CommentAnchor{Kind: CommentAnchorSelectionSnapshotV1, Selection: &SelectionSnapshotV1{
			DocumentRevision: 1, From: 3, To: 5, Quote: "方案", Prefix: "这个", Suffix: "可行",
		}},
		Body: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if thread.Anchor.State != CommentAnchorStateAnchored || thread.CanResolve || len(thread.Comments) != 1 ||
		thread.Comments[0].CanEdit || thread.Comments[0].CanDelete {
		t.Fatalf("thread=%+v", thread)
	}
	if len(mentions.ids) != 1 || mentions.ids[0] != 8 {
		t.Fatalf("mention ids=%v", mentions.ids)
	}
	if strings.Contains(comments.createCommand.RootComment.Body, "open_id") ||
		!strings.Contains(comments.createCommand.RootComment.Body, `"attrs":{"id":"8","label":"Other","username":"other"}`) {
		t.Fatalf("canonical body=%s", comments.createCommand.RootComment.Body)
	}
	if _, exists := comments.createCommand.Effect.Payload["body"]; exists {
		t.Fatalf("comment body leaked into outbox effect: %+v", comments.createCommand.Effect.Payload)
	}
}

func TestCommentListMarksSnapshotOrphanAfterDocumentRevisionChanges(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionCommentsView), string(ActionCommentsEditOwn), string(ActionCommentsDeleteOwn),
		string(ActionCommentsResolve), string(ActionCommentsReopen),
	})
	service.store.(*stubStore).documents[11] = Document{ID: 11, WorkspaceID: 7, Revision: 2}
	comments := &stubCommentStore{page: CommentThreadPage{Total: 1, Items: []CommentThread{{
		ID: 31, WorkspaceID: 7, DocumentID: 11, Status: CommentThreadOpen, CreatedBy: 5,
		Anchor:   CommentAnchor{Kind: CommentAnchorSelectionSnapshotV1, Selection: &SelectionSnapshotV1{DocumentRevision: 1, From: 1, To: 2, Quote: "x"}},
		Comments: []Comment{{ID: 41, AuthorUserID: 5}, {ID: 42, AuthorUserID: 9}},
	}}}}
	service.ConfigureComments(comments, &stubCommentMentions{})
	page, err := service.ListCommentThreads(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, CommentThreadFilter{Status: CommentThreadAll})
	if err != nil {
		t.Fatal(err)
	}
	comments.page.Items[0].CreatedBy = 9
	otherPage, err := service.ListCommentThreads(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, CommentThreadFilter{Status: CommentThreadAll})
	if err != nil || otherPage.Items[0].CanResolve {
		t.Fatalf("another author's thread must not be resolvable: %v", err)
	}
	comments.page.Items[0].CreatedBy = 5
	page, err = service.ListCommentThreads(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, CommentThreadFilter{Status: CommentThreadAll})
	if err != nil {
		t.Fatal(err)
	}
	thread := page.Items[0]
	if thread.Anchor.State != CommentAnchorStateOrphaned || !thread.CanResolve || thread.CanReopen {
		t.Fatalf("thread=%+v", thread)
	}
	if !thread.Comments[0].CanEdit || !thread.Comments[0].CanDelete || thread.Comments[1].CanEdit || thread.Comments[1].CanDelete {
		t.Fatalf("comments=%+v", thread.Comments)
	}
}

func TestCommentAnchorRelocatesAcrossOrdinaryDocumentEdits(t *testing.T) {
	stored := CommentAnchor{
		Kind: CommentAnchorSelectionSnapshotV1,
		Selection: &SelectionSnapshotV1{
			DocumentRevision: 1,
			From:             7,
			To:               11,
			Quote:            "beta",
			Prefix:           "Alpha ",
			Suffix:           " gamma",
		},
	}
	tests := []struct {
		name    string
		content string
		from    uint
		to      uint
	}{
		{
			name:    "unchanged content with a newer revision",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha beta gamma"}]}]}`,
			from:    7,
			to:      11,
		},
		{
			name:    "empty block inserted before the selection",
			content: `{"type":"doc","content":[{"type":"paragraph"},{"type":"paragraph","content":[{"type":"text","text":"Alpha beta gamma"}]}]}`,
			from:    9,
			to:      13,
		},
		{
			name:    "nearby prefix shortened while the quote stays unique",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"A beta gamma"}]}]}`,
			from:    3,
			to:      7,
		},
		{
			name:    "selected text replaced while surrounding context remains",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha changed gamma"}]}]}`,
			from:    7,
			to:      14,
		},
		{
			name:    "selected text grows at its trailing edge",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha beta extended gamma"}]}]}`,
			from:    7,
			to:      20,
		},
		{
			name:    "selected text grows at its leading edge",
			content: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha extended beta gamma"}]}]}`,
			from:    7,
			to:      20,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := resolveCommentAnchor(stored, Document{Revision: 2, Content: []byte(test.content)})
			if resolved.State != CommentAnchorStateAnchored || resolved.Selection == nil ||
				resolved.Selection.DocumentRevision != 2 || resolved.Selection.From != test.from || resolved.Selection.To != test.to {
				t.Fatalf("resolved=%+v", resolved)
			}
		})
	}
}

func TestCommentAnchorBecomesOrphanOnlyAfterSelectedRangeIsFullyDeleted(t *testing.T) {
	resolved := resolveCommentAnchor(CommentAnchor{
		Kind: CommentAnchorSelectionSnapshotV1,
		Selection: &SelectionSnapshotV1{
			DocumentRevision: 1,
			From:             7,
			To:               11,
			Quote:            "beta",
			Prefix:           "Alpha ",
			Suffix:           " gamma",
		},
	}, Document{
		Revision: 2,
		Content:  []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha  gamma"}]}]}`),
	})
	if resolved.State != CommentAnchorStateOrphaned {
		t.Fatalf("resolved=%+v", resolved)
	}
}

func TestCommentAnchorRelocationUsesOriginalPositionWhenOnlyOneContextSideExists(t *testing.T) {
	resolved := resolveCommentAnchor(CommentAnchor{
		Kind: CommentAnchorSelectionSnapshotV1,
		Selection: &SelectionSnapshotV1{
			DocumentRevision: 1,
			From:             11,
			To:               15,
			Quote:            "beta",
			Suffix:           " gamma",
		},
	}, Document{
		Revision: 2,
		Content:  []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"changed beta gamma"}]},{"type":"paragraph","content":[{"type":"text","text":"beta changed gamma"}]}]}`),
	})
	if resolved.State != CommentAnchorStateAnchored || resolved.Selection == nil ||
		resolved.Selection.From != 1 || resolved.Selection.To != 13 {
		t.Fatalf("resolved=%+v", resolved)
	}
}

func TestCommentAnchorRelocationFailsClosedForAmbiguousQuote(t *testing.T) {
	resolved := resolveCommentAnchor(CommentAnchor{
		Kind: CommentAnchorSelectionSnapshotV1,
		Selection: &SelectionSnapshotV1{
			DocumentRevision: 1,
			From:             7,
			To:               11,
			Quote:            "beta",
		},
	}, Document{
		Revision: 2,
		Content:  []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"beta and beta"}]}]}`),
	})
	if resolved.State != CommentAnchorStateOrphaned {
		t.Fatalf("resolved=%+v", resolved)
	}
}

func TestViewerCannotCreateCommentAndInvalidMentionFailsClosed(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionCommentsCreate)})
	comments := &stubCommentStore{}
	mentions := &stubCommentMentions{}
	service.ConfigureComments(comments, mentions)
	body := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`
	if _, err := service.CreateCommentThread(context.Background(), Principal{UserID: 9, WorkspaceID: 7}, 11, CreateCommentThreadInput{
		Anchor: CommentAnchor{Kind: CommentAnchorDocument}, Body: body,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer create err=%v", err)
	}
	mentions.invalid = true
	body = `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"88"}}]}]}`
	if _, err := service.CreateCommentThread(context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, CreateCommentThreadInput{
		Anchor: CommentAnchor{Kind: CommentAnchorDocument}, Body: body,
	}); !errors.Is(err, ErrCommentMentionInvalid) {
		t.Fatalf("invalid mention err=%v", err)
	}
}

func TestCommentActionsAppearOnlyWithinObjectRoleCeiling(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionContentView), string(ActionCommentsView), string(ActionCommentsCreate), string(ActionCommentsReply),
	})
	service.ConfigureComments(&stubCommentStore{}, &stubCommentMentions{})
	tester, err := service.GetDocument(context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11)
	if err != nil {
		t.Fatal(err)
	}
	if !includesAction(tester.AllowedActions, string(ActionCommentsView)) || !includesAction(tester.AllowedActions, string(ActionCommentsCreate)) ||
		!includesAction(tester.AllowedActions, string(ActionCommentsReply)) {
		t.Fatalf("tester actions=%v", tester.AllowedActions)
	}
	viewer := allowedActions(accessState{actions: map[string]struct{}{string(ActionCommentsCreate): {}}}, roleViewer)
	if includesAction(viewer, string(ActionCommentsCreate)) {
		t.Fatalf("viewer actions=%v", viewer)
	}
}
