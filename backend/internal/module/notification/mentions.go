package notification

import (
	"context"
	"time"
)

// Mention is the recipient-facing projection, not an outbox delivery record.
type Mention struct {
	RecipientRole  string     `json:"recipient_role,omitempty"`
	ProjectID      uint       `json:"project_id,omitempty"`
	NodeID         string     `json:"node_id,omitempty"`
	NodeName       string     `json:"node_name,omitempty"`
	PreviousStatus string     `json:"previous_status,omitempty"`
	Status         string     `json:"status,omitempty"`
	MemberRole     string     `json:"member_role,omitempty"`
	ActorName      string     `json:"actor_name,omitempty"`
	ResourceTitle  string     `json:"resource_title,omitempty"`
	RequestStatus  string     `json:"request_status,omitempty"`
	GrantLevel     string     `json:"grant_level,omitempty"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CommentID      uint       `json:"comment_id,omitempty"`
	ResourceType   string     `json:"resource_type,omitempty"`
	ResourceID     string     `json:"resource_id,omitempty"`
	Scene          string     `json:"scene"`
	RequestID      uint       `json:"request_id,omitempty"`
	ID             uint       `json:"id"`
	DocumentID     uint       `json:"document_id"`
	ThreadID       uint       `json:"thread_id"`
	ActorUserID    uint       `json:"actor_user_id"`
	OccurredAt     time.Time  `json:"occurred_at"`
}

type MentionPage struct {
	Items      []Mention `json:"items"`
	NextCursor uint      `json:"next_cursor,omitempty"`
}

type MentionReader interface {
	ListMentions(context.Context, uint, uint, uint) (MentionPage, error)
}

type MembershipVerifier interface {
	VerifyWorkspaceMembership(context.Context, uint, uint) error
}
