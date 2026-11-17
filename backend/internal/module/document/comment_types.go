package document

import (
	"context"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

const (
	CommentBodyByteLimit        = 100_000
	CommentBodyVisibleRuneLimit = 5_000
	CommentMentionLimit         = 50
)

type CommentAnchorKind string

const (
	CommentAnchorDocument            CommentAnchorKind = "document"
	CommentAnchorSelectionSnapshotV1 CommentAnchorKind = "selection_snapshot_v1"
)

type CommentAnchorState string

const (
	CommentAnchorStateDocument CommentAnchorState = "document"
	CommentAnchorStateAnchored CommentAnchorState = "anchored"
	CommentAnchorStateOrphaned CommentAnchorState = "orphaned"
)

type CommentThreadStatus string

const (
	CommentThreadOpen     CommentThreadStatus = "open"
	CommentThreadResolved CommentThreadStatus = "resolved"
	CommentThreadAll      CommentThreadStatus = "all"
)

type SelectionSnapshotV1 struct {
	DocumentRevision uint   `json:"document_revision"`
	From             uint   `json:"from"`
	To               uint   `json:"to"`
	Quote            string `json:"quote"`
	Prefix           string `json:"prefix,omitempty"`
	Suffix           string `json:"suffix,omitempty"`
}

type CommentAnchor struct {
	Kind      CommentAnchorKind    `json:"kind"`
	State     CommentAnchorState   `json:"state"`
	Selection *SelectionSnapshotV1 `json:"selection,omitempty"`
}

type Comment struct {
	Author           *identity.UserDisplay `json:"author,omitempty"`
	ID               uint                  `json:"id"`
	WorkspaceID      uint                  `json:"workspace_id"`
	DocumentID       uint                  `json:"document_id"`
	ThreadID         uint                  `json:"thread_id"`
	ParentCommentID  *uint                 `json:"parent_comment_id,omitempty"`
	AuthorUserID     uint                  `json:"author_user_id"`
	Body             string                `json:"body"`
	MentionedUserIDs []uint                `json:"mentioned_user_ids"`
	CanEdit          bool                  `json:"can_edit"`
	CanDelete        bool                  `json:"can_delete"`
	DeletedAt        *time.Time            `json:"deleted_at,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type CommentThread struct {
	ID          uint                `json:"id"`
	WorkspaceID uint                `json:"workspace_id"`
	DocumentID  uint                `json:"document_id"`
	Anchor      CommentAnchor       `json:"anchor"`
	Status      CommentThreadStatus `json:"status"`
	CreatedBy   uint                `json:"created_by_user_id"`
	ResolvedBy  *uint               `json:"resolved_by_user_id,omitempty"`
	ResolvedAt  *time.Time          `json:"resolved_at,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Comments    []Comment           `json:"comments"`
	CanResolve  bool                `json:"can_resolve"`
	CanReopen   bool                `json:"can_reopen"`
}

type CommentThreadFilter struct {
	Status CommentThreadStatus
	Limit  int
	Offset int
}

type CommentThreadPage struct {
	Items []CommentThread `json:"items"`
	Total int64           `json:"total"`
}

type CreateCommentThreadInput struct {
	Anchor CommentAnchor `json:"anchor"`
	Body   string        `json:"body"`
}

type ReplyCommentInput struct {
	ParentCommentID *uint  `json:"parent_comment_id,omitempty"`
	Body            string `json:"body"`
}

type CreateCommentThreadCommand struct {
	Thread           CommentThread
	RootComment      Comment
	MentionedUserIDs []uint
	Effect           MutationEffect
}

type ReplyCommentCommand struct {
	WorkspaceID      uint
	DocumentID       uint
	ThreadID         uint
	ParentCommentID  *uint
	ActorUserID      uint
	Body             string
	MentionedUserIDs []uint
	Effect           MutationEffect
}

type EditCommentCommand struct {
	WorkspaceID      uint
	DocumentID       uint
	CommentID        uint
	ActorUserID      uint
	Body             string
	MentionedUserIDs []uint
	Effect           MutationEffect
}

type DeleteCommentCommand struct {
	WorkspaceID uint
	DocumentID  uint
	CommentID   uint
	ActorUserID uint
	Effect      MutationEffect
}

type SetCommentThreadStatusCommand struct {
	WorkspaceID uint
	DocumentID  uint
	ThreadID    uint
	ActorUserID uint
	From        CommentThreadStatus
	To          CommentThreadStatus
	Effect      MutationEffect
}

func (service *Service) commentAccess(
	ctx context.Context,
	principal Principal,
	documentID uint,
	action string,
	minimumRole objectRole,
) (Document, accessState, objectRole, error) {
	if service.comments == nil || service.mentions == nil {
		return Document{}, accessState{}, roleNone, ErrCommentsUnavailable
	}
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Document{}, accessState{}, roleNone, err
	}
	if documentID == 0 || !state.has(action) {
		return Document{}, accessState{}, roleNone, ErrForbidden
	}
	documentValue, err := service.store.GetDocument(ctx, principal.WorkspaceID, documentID)
	if err != nil {
		return Document{}, accessState{}, roleNone, err
	}
	role, err := service.documentRole(ctx, principal, state, documentID)
	if err != nil {
		return Document{}, accessState{}, roleNone, err
	}
	if role < minimumRole {
		return Document{}, accessState{}, roleNone, ErrForbidden
	}
	if documentValue.CollaborationCutoverAt != nil {
		if service.resourceAccess == nil {
			return Document{}, accessState{}, roleNone, ErrContextActionsUnavailable
		}
		access, accessErr := service.resourceAccess.ResolveDocumentAccess(ctx, principal, documentID)
		if accessErr != nil || !access.CanView || access.ResourceID == "" {
			return Document{}, accessState{}, roleNone, ErrForbidden
		}
		documentValue.ResourceID = access.ResourceID
		documentValue, err = service.applyCollaborationProjection(ctx, principal, documentValue)
		if err != nil {
			return Document{}, accessState{}, roleNone, err
		}
	}
	return documentValue, state, role, nil
}

func normalizeCommentThreadFilter(filter CommentThreadFilter) (CommentThreadFilter, error) {
	if filter.Status == "" {
		filter.Status = CommentThreadOpen
	}
	if filter.Status != CommentThreadOpen && filter.Status != CommentThreadResolved && filter.Status != CommentThreadAll {
		return CommentThreadFilter{}, ErrInvalidInput
	}
	if filter.Offset < 0 {
		return CommentThreadFilter{}, ErrInvalidInput
	}
	if filter.Limit < 1 {
		filter.Limit = 100
	} else if filter.Limit > 200 {
		filter.Limit = 200
	}
	return filter, nil
}
