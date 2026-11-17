package pmtools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type documentReadHandler struct {
	documents *document.Service
	users     TaskObjectPrincipalResolver
	name      string
}

func DocumentReadHandlerNames() []string {
	return []string{"document.space.get", "document.node.tree", "document.get", "document.export", "document.binding.list", "document.node.deletion_impact", "document.binding.preview", "document.asset.list", "document.comment_thread.list", "document.access_scope.get"}
}
func NewDocumentReadHandlers(documents *document.Service, users TaskObjectPrincipalResolver) (map[string]toolruntime.ToolHandler, error) {
	if documents == nil || users == nil {
		return nil, fmt.Errorf("document tool dependencies are required")
	}
	result := map[string]toolruntime.ToolHandler{}
	for _, name := range DocumentReadHandlerNames() {
		result[name] = documentReadHandler{documents: documents, users: users, name: name}
	}
	return result, nil
}
func documentToolPrincipal(ctx context.Context, users TaskObjectPrincipalResolver, authorization toolruntime.ExecutionAuthorization) (document.Principal, error) {
	if authorization.WorkspaceID == 0 || authorization.PrincipalID == 0 {
		return document.Principal{}, toolruntime.ErrAuthorizationDenied
	}
	fact, err := users.FactUserID(ctx, authorization.PrincipalID)
	if err != nil {
		return document.Principal{}, err
	}
	if fact == 0 {
		return document.Principal{}, toolruntime.ErrAuthorizationDenied
	}
	return document.Principal{UserID: authorization.PrincipalID, ParticipantUserID: fact, WorkspaceID: authorization.WorkspaceID}, nil
}
func (handler documentReadHandler) Invoke(ctx context.Context, call toolruntime.ToolCall) (json.RawMessage, error) {
	p, err := documentToolPrincipal(ctx, handler.users, call.Authorization)
	if err != nil {
		return nil, err
	}
	var input struct {
		ID            uint                         `json:"id"`
		RequirementID uint                         `json:"requirement_id"`
		Status        document.CommentThreadStatus `json:"status"`
		Limit         int                          `json:"limit"`
		Offset        int                          `json:"offset"`
	}
	if err := decodeInput(call.Input, &input); err != nil {
		return nil, toolruntime.ErrToolInputInvalid
	}
	var value any
	switch handler.name {
	case "document.space.get":
		value, err = handler.documents.PersonalDefaultSpace(ctx, p)
	case "document.node.tree":
		value, err = handler.documents.PersonalTree(ctx, p)
	case "document.get":
		value, err = handler.documents.GetDocument(ctx, p, input.ID)
	case "document.export":
		value, err = handler.documents.ExportDocument(ctx, p, input.ID)
	case "document.binding.list":
		var items []document.Binding
		items, err = handler.documents.ListBindings(ctx, p, input.ID)
		if items == nil {
			items = []document.Binding{}
		}
		value = map[string]any{"items": items}
	case "document.node.deletion_impact":
		value, err = handler.documents.PersonalArchiveImpact(ctx, p, input.ID)
	case "document.binding.preview":
		value, err = handler.documents.PreviewBinding(ctx, p, input.ID, input.RequirementID)
	case "document.asset.list":
		var items []document.Asset
		items, err = handler.documents.ListAssets(ctx, p, input.ID)
		if items == nil {
			items = []document.Asset{}
		}
		value = map[string]any{"items": items}
	case "document.comment_thread.list":
		value, err = handler.documents.ListCommentThreads(ctx, p, input.ID, document.CommentThreadFilter{Status: input.Status, Limit: input.Limit, Offset: input.Offset})
	case "document.access_scope.get":
		err = handler.documents.RequireAllScope(ctx, p)
		value = map[string]any{"scope": "all"}
	default:
		return nil, toolruntime.ErrToolUnknown
	}
	if err != nil {
		return nil, documentToolError(err)
	}
	return json.Marshal(value)
}
