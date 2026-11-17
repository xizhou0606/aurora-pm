package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type documentWriteHandler struct {
	documents *document.Service
	users     TaskObjectPrincipalResolver
	name      string
}

func DocumentWriteHandlerNames() []string {
	return []string{
		"document.content.replace", "document.node.create", "document.node.copy", "document.node.rename", "document.node.move", "document.node.archive", "document.node.shortcut.create", "document.node.preference.update", "document.owner.transfer", "document.binding.create", "document.binding.delete", "document.asset.delete", "document.comment_thread.create", "document.comment.reply", "document.comment.edit_own", "document.comment.delete_own", "document.comment_thread.resolve", "document.comment_thread.reopen",
	}
}
func NewDocumentWriteHandlers(documents *document.Service, users TaskObjectPrincipalResolver) (map[string]toolruntime.ToolHandler, error) {
	if documents == nil || users == nil {
		return nil, fmt.Errorf("document mutation dependencies are required")
	}
	handlers := map[string]toolruntime.ToolHandler{}
	for _, name := range DocumentWriteHandlerNames() {
		handlers[name] = documentWriteHandler{documents: documents, users: users, name: name}
	}
	return handlers, nil
}
func (handler documentWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}
func (handler documentWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	p, err := documentToolPrincipal(ctx, handler.users, call.Authorization)
	if err != nil {
		return nil, err
	}
	if call.ApplyKey == "" || call.ChangeSetID == "" {
		return nil, toolruntime.ErrChangeSetRequired
	}
	var input documentWriteInput
	if err := decodeInput(call.Input, &input); err != nil {
		return nil, toolruntime.ErrToolInputInvalid
	}
	digest := sha256.Sum256(append([]byte(handler.name+"\n"), call.Input...))
	input.OperationID = "tool-" + hex.EncodeToString(digest[:]) + "-" + call.ApplyKey
	operationHash := sha256.Sum256([]byte(input.OperationID))
	input.OperationID = "tool-" + hex.EncodeToString(operationHash[:])
	output, err := handler.documents.ApplyToolMutation(ctx, p, call.ApplyKey, hex.EncodeToString(digest[:]), func(service *document.Service) (json.RawMessage, error) {
		return applyDocumentWrite(ctx, service, p, handler.name, input)
	})
	if err != nil {
		return nil, documentToolError(err)
	}
	return output, nil
}

type documentWriteInput struct {
	ID                        uint                   `json:"id"`
	ParentID                  *uint                  `json:"parent_id"`
	Type                      document.NodeType      `json:"type"`
	Title                     string                 `json:"title"`
	Collaborative             bool                   `json:"collaborative"`
	BeforeNodeID              *uint                  `json:"before_node_id"`
	AfterNodeID               *uint                  `json:"after_node_id"`
	ExpectedDirectoryRevision *uint64                `json:"expected_directory_revision"`
	ConfirmNonEmpty           bool                   `json:"confirm_non_empty"`
	ExpectedDescendantCount   *int64                 `json:"expected_descendant_count"`
	ExpectedDocumentCount     *int64                 `json:"expected_document_count"`
	Pinned                    *bool                  `json:"pinned"`
	Favorited                 *bool                  `json:"favorited"`
	OwnerUserID               uint                   `json:"owner_user_id"`
	RequirementID             uint                   `json:"requirement_id"`
	Confirmed                 bool                   `json:"confirmed"`
	BindingID                 uint                   `json:"binding_id"`
	AssetID                   uint                   `json:"asset_id"`
	ThreadID                  uint                   `json:"thread_id"`
	CommentID                 uint                   `json:"comment_id"`
	ParentCommentID           *uint                  `json:"parent_comment_id"`
	Body                      string                 `json:"body"`
	Anchor                    document.CommentAnchor `json:"anchor"`
	ExpectedRevision          uint                   `json:"expected_revision"`
	SchemaVersion             uint                   `json:"schema_version"`
	Content                   json.RawMessage        `json:"content"`
	OperationID               string                 `json:"-"`
	IdempotencyKey            string                 `json:"idempotency_key"`
}

func applyDocumentWrite(ctx context.Context, s *document.Service, p document.Principal, name string, i documentWriteInput) (json.RawMessage, error) {
	var value any
	var err error
	switch name {
	case "document.content.replace":
		value, err = s.ReplaceToolContent(ctx, p, i.ID, document.ContentReplacement{ExpectedRevision: i.ExpectedRevision, SchemaVersion: i.SchemaVersion, Content: i.Content, OperationID: i.OperationID})
	case "document.node.create":
		value, err = s.PersonalCreateNode(ctx, p, document.CreateNodeInput{Type: i.Type, Title: i.Title, ParentID: i.ParentID, Collaborative: i.Collaborative})
	case "document.node.copy":
		value, err = s.PersonalCopyNode(ctx, p, i.ID, document.CopyNodeInput{ParentID: i.ParentID})
	case "document.node.rename":
		value, err = s.PersonalRenameNode(ctx, p, i.ID, i.Title)
	case "document.node.move":
		value, err = s.PersonalMoveNode(ctx, p, i.ID, document.MoveNodeInput{ParentID: i.ParentID, BeforeNodeID: i.BeforeNodeID, AfterNodeID: i.AfterNodeID, ExpectedDirectoryRevision: i.ExpectedDirectoryRevision})
	case "document.node.archive":
		err = s.PersonalArchiveNode(ctx, p, i.ID, document.ArchiveConfirmation{ConfirmNonEmpty: i.ConfirmNonEmpty, ExpectedDescendantCount: i.ExpectedDescendantCount, ExpectedDocumentCount: i.ExpectedDocumentCount})
		value = map[string]any{"id": i.ID, "archived": true}
	case "document.node.shortcut.create":
		value, err = s.PersonalCreateShortcut(ctx, p, i.ID, document.CreateShortcutInput{ParentID: i.ParentID})
	case "document.node.preference.update":
		value, err = s.UpdateNodePreference(ctx, p, i.ID, document.UpdateNodePreferenceInput{Pinned: i.Pinned, Favorited: i.Favorited})
	case "document.owner.transfer":
		value, err = s.TransferDocumentOwner(ctx, p, i.ID, i.OwnerUserID)
	case "document.binding.create":
		value, err = s.BindRequirement(ctx, p, i.ID, i.RequirementID, i.Confirmed)
	case "document.binding.delete":
		err = s.UnbindRequirement(ctx, p, i.ID, i.BindingID)
		value = map[string]any{"id": i.BindingID, "deleted": true}
	case "document.asset.delete":
		err = s.DeleteAsset(ctx, p, i.ID, i.AssetID)
		value = map[string]any{"id": i.AssetID, "deleted": true}
	case "document.comment_thread.create":
		value, err = s.CreateCommentThread(ctx, p, i.ID, document.CreateCommentThreadInput{Anchor: i.Anchor, Body: i.Body})
	case "document.comment.reply":
		value, err = s.ReplyToCommentThread(ctx, p, i.ID, i.ThreadID, document.ReplyCommentInput{ParentCommentID: i.ParentCommentID, Body: i.Body})
	case "document.comment.edit_own":
		value, err = s.EditOwnComment(ctx, p, i.ID, i.CommentID, i.Body)
	case "document.comment.delete_own":
		err = s.DeleteOwnComment(ctx, p, i.ID, i.CommentID)
		value = map[string]any{"id": i.CommentID, "deleted": true}
	case "document.comment_thread.resolve":
		value, err = s.ResolveCommentThread(ctx, p, i.ID, i.ThreadID)
	case "document.comment_thread.reopen":
		value, err = s.ReopenCommentThread(ctx, p, i.ID, i.ThreadID)
	default:
		return nil, toolruntime.ErrToolUnknown
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
