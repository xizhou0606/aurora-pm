package document

import (
	"context"
	"io"
	"time"
)

type Store interface {
	EnsureDefaultSpace(context.Context, uint) (Space, error)
	ListNodes(context.Context, uint, uint) ([]Node, error)
	GetNode(context.Context, uint, uint) (Node, error)
	NextSortOrder(context.Context, uint, uint, *uint) (int64, error)
	CreateNode(context.Context, Node, *Document) (Node, *Document, error)
	UpdateNode(context.Context, Node) (Node, error)
	ArchiveNodes(context.Context, uint, []uint, uint) error
	GetDocument(context.Context, uint, uint) (Document, error)
	UpdateDocument(context.Context, Document, uint, uint) (Document, error)
	ListBindings(context.Context, uint, uint) ([]Binding, error)
	ListBindingsByDocumentIDs(context.Context, uint, []uint) ([]Binding, error)
	CreateBinding(context.Context, Binding) (Binding, error)
	DeleteBinding(context.Context, uint, uint, uint) error
	ListAssets(context.Context, uint, uint) ([]Asset, error)
	GetAsset(context.Context, uint, uint, uint) (Asset, error)
	CreateAsset(context.Context, Asset) (Asset, error)
	ArchiveAsset(context.Context, uint, uint, uint, uint) error
	CopyDocumentNode(context.Context, CopyDocumentNodeCommand) (CreateNodeResult, error)
	CreateShortcut(context.Context, CreateShortcutCommand) (Node, error)
	ListNodePreferences(context.Context, uint, uint, []uint) ([]NodePreference, error)
	UpdateNodePreference(context.Context, UpdateNodePreferenceCommand) (NodePreference, error)
	TransferDocumentOwner(context.Context, TransferDocumentOwnerCommand) (Document, error)
}

type AssetStorage interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
	GetObject(context.Context, string) (io.ReadCloser, error)
	RemoveObject(context.Context, string) error
}

type ActionResolver interface {
	EffectiveDocumentActionCodes(context.Context, uint, uint) ([]string, error)
}

// ResourceAccessResolver is the document module's narrow seam to the unified
// document-resource policy. It accepts legacy document ids only as content
// references and returns the canonical resource id plus effective capabilities.
// Implementations must resolve the workspace-scoped binding server-side.
type ResourceAccessResolver interface {
	ResolveDocumentAccess(context.Context, Principal, uint) (ResourceAccess, error)
	ResolveDocumentsAccess(context.Context, Principal, []uint) (map[uint]ResourceAccess, error)
}

type ResourceAccess struct {
	ResourceID string
	CanView    bool
	CanComment bool
	CanEdit    bool
	CanManage  bool
}

type CollaborationProjection struct {
	Content       []byte
	ContentHash   string
	PlainText     string
	SchemaVersion uint
	Revision      uint64
}

// CollaborationProjectionReader is configured only by the application root.
// Once CollaborationCutoverAt is set, all legacy JSON/read/export projections
// must come from this one durable collaboration fact.
type CollaborationProjectionReader interface {
	LoadDocumentProjection(context.Context, Principal, string) (CollaborationProjection, error)
}

// ResourceLifecycle is created from the same transaction as the legacy
// document rows. It keeps unified resource metadata and policy creation from
// becoming an eventually-consistent afterthought.
type ResourceLifecycle interface {
	CreatePrivateDocumentResource(context.Context, Principal, Node, Document) (string, error)
	SynchronizeDocumentResource(context.Context, Principal, Node, Document) (string, error)
	ArchiveDocumentResources(context.Context, Principal, []Document) error
}

// NewDocumentCollaborationInitializer is created from the insertion transaction.
// It may initialize only newly inserted blank documents, never existing content.
type NewDocumentCollaborationInitializer interface {
	InitializeNewDocument(context.Context, Document) (*time.Time, error)
}

type CollaborativeNodeCreator interface {
	CreateCollaborativeNode(context.Context, Node, *Document) (Node, *Document, error)
}

type RequirementParticipants struct {
	RequirementID        uint
	WorkspaceID          uint
	Version              uint
	Active               bool
	ProductOwnerUserIDs  []uint
	DesignOwnerUserIDs   []uint
	FrontendOwnerUserIDs []uint
	BackendOwnerUserIDs  []uint
	TesterUserIDs        []uint
	DeveloperUserIDs     []uint
}

// RequirementParticipantResolver is deliberately owned by the document
// consumer. Its implementation may use requirement's public read service but
// must never expose or query requirement GORM records here.
type RequirementParticipantResolver interface {
	ResolveRequirementParticipants(context.Context, uint, uint) (RequirementParticipants, error)
}

type MutationEffect struct {
	CommentMention *CommentMentionNotification
	WorkspaceID    uint
	PrincipalID    uint
	Resource       string
	ResourceID     uint
	Action         string
	Payload        map[string]any
}

// CommentMentionNotification carries only routing metadata, never private content.
type CommentMentionNotification struct {
	DocumentID       uint
	ThreadID         uint
	CommentID        uint
	RecipientUserIDs []uint
}

type MutationRecorder interface {
	RecordDocumentMutation(context.Context, MutationEffect) error
}

type CommentStore interface {
	ListCommentThreads(context.Context, uint, uint, CommentThreadFilter) (CommentThreadPage, error)
	CreateCommentThread(context.Context, CreateCommentThreadCommand) (CommentThread, error)
	ReplyToCommentThread(context.Context, ReplyCommentCommand) (Comment, error)
	EditOwnComment(context.Context, EditCommentCommand) (Comment, error)
	DeleteOwnComment(context.Context, DeleteCommentCommand) error
	SetCommentThreadStatus(context.Context, SetCommentThreadStatusCommand) (CommentThread, error)
}

// CommentMentionResolver validates canonical PM user ids against the current
// workspace. Display labels and external identities never enter comment facts.
type CommentMentionResolver interface {
	ValidateCommentMentionUserIDs(context.Context, uint, []uint) error
}

// WorkspaceUserResolver validates canonical PM users through identity's active
// workspace membership service. Owner ids remain display facts, never grants.
type WorkspaceUserResolver interface {
	ValidateActiveWorkspaceUser(context.Context, uint, uint) error
}
