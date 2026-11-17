package document

import "time"

const FavoritesFolderTitle = "收藏"

type CopyNodeInput struct {
	ParentID *uint
}

type CreateShortcutInput struct {
	directoryOwner *uint
	ParentID       *uint
}

type UpdateNodePreferenceInput struct {
	Pinned    *bool
	Favorited *bool
}

type NodePreference struct {
	NodeID    uint       `json:"node_id"`
	PinnedAt  *time.Time `json:"pinned_at,omitempty"`
	Favorited bool       `json:"favorited"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CopyDocumentNodeCommand struct {
	WorkspaceID uint
	ActorUserID uint
	SourceNode  Node
	Source      Document
	ParentID    *uint
	Effect      MutationEffect
}

type CreateShortcutCommand struct {
	DirectoryOwnerUserID *uint
	WorkspaceID          uint
	ActorUserID          uint
	TargetNode           Node
	ParentID             *uint
	Effect               MutationEffect
}

type UpdateNodePreferenceCommand struct {
	WorkspaceID uint
	ActorUserID uint
	TargetNode  Node
	Pinned      *bool
	Favorited   *bool
	Effect      MutationEffect
}

type TransferDocumentOwnerCommand struct {
	WorkspaceID uint
	ActorUserID uint
	DocumentID  uint
	OwnerUserID uint
	Effect      MutationEffect
}
