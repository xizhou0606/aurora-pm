package documentgorm

import (
	"encoding/json"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func commentThreadRecord(value document.CommentThread) (CommentThread, error) {
	payload := "{}"
	documentRevision := uint(0)
	if value.Anchor.Selection != nil {
		encoded, err := json.Marshal(value.Anchor.Selection)
		if err != nil {
			return CommentThread{}, err
		}
		payload = string(encoded)
		documentRevision = value.Anchor.Selection.DocumentRevision
	}
	return CommentThread{
		ID: value.ID, WorkspaceID: value.WorkspaceID, DocumentID: value.DocumentID,
		AnchorKind: string(value.Anchor.Kind), AnchorPayloadJSON: payload,
		DocumentRevision: documentRevision, Status: string(value.Status), CreatedBy: value.CreatedBy,
		ResolvedBy: value.ResolvedBy, ResolvedAt: value.ResolvedAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func commentThreadFromRecord(row CommentThread) (document.CommentThread, error) {
	anchor := document.CommentAnchor{Kind: document.CommentAnchorKind(row.AnchorKind)}
	if anchor.Kind == document.CommentAnchorSelectionSnapshotV1 {
		var selection document.SelectionSnapshotV1
		if err := json.Unmarshal([]byte(row.AnchorPayloadJSON), &selection); err != nil {
			return document.CommentThread{}, err
		}
		if selection.DocumentRevision == 0 {
			selection.DocumentRevision = row.DocumentRevision
		}
		anchor.Selection = &selection
	}
	return document.CommentThread{
		ID: row.ID, WorkspaceID: row.WorkspaceID, DocumentID: row.DocumentID,
		Anchor: anchor, Status: document.CommentThreadStatus(row.Status), CreatedBy: row.CreatedBy,
		ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Comments: []document.Comment{},
	}, nil
}

func commentRecord(value document.Comment) Comment {
	return Comment{
		ID: value.ID, WorkspaceID: value.WorkspaceID, DocumentID: value.DocumentID,
		ThreadID: value.ThreadID, ParentCommentID: value.ParentCommentID,
		AuthorUserID: value.AuthorUserID, BodyJSON: value.Body,
		DeletedAt: value.DeletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func commentFromRecord(row Comment, mentions []uint) document.Comment {
	body := row.BodyJSON
	if row.DeletedAt != nil {
		body = ""
	}
	mentionedUserIDs := append([]uint{}, mentions...)
	return document.Comment{
		ID: row.ID, WorkspaceID: row.WorkspaceID, DocumentID: row.DocumentID,
		ThreadID: row.ThreadID, ParentCommentID: row.ParentCommentID,
		AuthorUserID: row.AuthorUserID, Body: body, MentionedUserIDs: mentionedUserIDs,
		DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
