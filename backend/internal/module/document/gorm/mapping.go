package documentgorm

import document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"

func spaceFromRecord(row Space) document.Space {
	return document.Space{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Name: row.Name, IsDefault: row.IsDefault,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func nodeFromRecord(row Node) document.Node {
	return document.Node{
		DirectoryOwnerUserID: row.DirectoryOwnerUserID,
		ID:                   row.ID, WorkspaceID: row.WorkspaceID, SpaceID: row.SpaceID, ParentID: row.ParentID,
		Type: document.NodeType(row.Type), Title: row.Title, SortOrder: row.SortOrder,
		ShortcutTargetNodeID: row.ShortcutTargetNodeID, SystemManaged: row.SystemManaged,
		SystemKind: document.NodeSystemKind(row.SystemKind), SystemOwnerUserID: row.SystemOwnerUserID,
		CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy,
		ArchivedAt: row.ArchivedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func nodeRecord(value document.Node) Node {
	return Node{
		DirectoryOwnerUserID: value.DirectoryOwnerUserID,
		ID:                   value.ID, WorkspaceID: value.WorkspaceID, SpaceID: value.SpaceID, ParentID: value.ParentID,
		Type: string(value.Type), Title: value.Title, SortOrder: value.SortOrder,
		ShortcutTargetNodeID: value.ShortcutTargetNodeID, SystemManaged: value.SystemManaged,
		SystemKind: string(value.SystemKind), SystemOwnerUserID: value.SystemOwnerUserID,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		ArchivedAt: value.ArchivedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func documentFromRecord(row Document) document.Document {
	return document.Document{
		ID: row.ID, WorkspaceID: row.WorkspaceID, NodeID: row.NodeID, Content: rawJSON(row.ContentJSON),
		SchemaVersion: row.SchemaVersion, Revision: row.Revision, ContentHash: row.ContentHash,
		PlainText: row.PlainText, OwnerUserID: row.OwnerUserID, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy,
		CollaborationCutoverAt: row.CollaborationCutoverAt,
		ArchivedAt:             row.ArchivedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AllowedActions: []string{},
	}
}

func documentRecord(value document.Document) Document {
	return Document{
		ID: value.ID, WorkspaceID: value.WorkspaceID, NodeID: value.NodeID, ContentJSON: string(value.Content),
		SchemaVersion: value.SchemaVersion, Revision: value.Revision, ContentHash: value.ContentHash,
		PlainText: value.PlainText, OwnerUserID: value.OwnerUserID, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		CollaborationCutoverAt: value.CollaborationCutoverAt,
		ArchivedAt:             value.ArchivedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func bindingFromRecord(row Binding) document.Binding {
	return document.Binding{
		ID: row.ID, WorkspaceID: row.WorkspaceID, DocumentID: row.DocumentID,
		RequirementID: row.RequirementID, Kind: document.BindingKind(row.Kind),
		CreatedBy: row.CreatedBy, ArchivedAt: row.ArchivedAt, CreatedAt: row.CreatedAt,
	}
}

func bindingRecord(value document.Binding) Binding {
	return Binding{
		ID: value.ID, WorkspaceID: value.WorkspaceID, DocumentID: value.DocumentID,
		RequirementID: value.RequirementID, Kind: string(value.Kind),
		CreatedBy: value.CreatedBy, ArchivedAt: value.ArchivedAt, CreatedAt: value.CreatedAt,
	}
}

func assetFromRecord(row Asset) document.Asset {
	return document.Asset{
		ID: row.ID, WorkspaceID: row.WorkspaceID, DocumentID: row.DocumentID,
		ObjectKey: row.ObjectKey, FileName: row.FileName, ContentType: row.ContentType,
		SizeBytes: row.SizeBytes, ContentHash: row.ContentHash, CreatedBy: row.CreatedBy,
		ArchivedAt: row.ArchivedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func assetRecord(value document.Asset) Asset {
	return Asset{
		ID: value.ID, WorkspaceID: value.WorkspaceID, DocumentID: value.DocumentID,
		ObjectKey: value.ObjectKey, FileName: value.FileName, ContentType: value.ContentType,
		SizeBytes: value.SizeBytes, ContentHash: value.ContentHash, CreatedBy: value.CreatedBy,
		ArchivedAt: value.ArchivedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
