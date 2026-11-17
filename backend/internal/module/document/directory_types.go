package document

import (
	"context"
	"errors"
)

var ErrDirectoryConflict = errors.New("personal directory revision conflict")

type DirectoryEntry struct {
	NodeID    uint
	ParentID  *uint
	SortOrder int64
	Title     string
	Removed   bool
}
type DirectorySnapshot struct {
	SpaceID  uint
	Revision uint64
	Entries  []DirectoryEntry
}
type DirectoryCommand struct {
	NodeID           uint
	ExpectedRevision *uint64
	ParentID         *uint
	BeforeNodeID     *uint
	AfterNodeID      *uint
	Title            *string
	Remove           bool
}

// Node IDs are compatibility locators only; placement belongs to this actor's space.
type DirectoryStore interface {
	DirectoryPrincipals(context.Context) ([]Principal, error)
	ReadDirectory(context.Context, Principal, []Node) (DirectorySnapshot, error)
	ChangeDirectory(context.Context, Principal, DirectoryCommand) error
	InDirectoryTransaction(context.Context, func(Store, MutationRecorder) error) error
}

// Resource placements let other content modules reuse this space without becoming document DTOs.
type ResourceDirectoryLocation struct {
	ParentID *uint
	Revision uint64
}
type ResourceDirectoryStore interface {
	ResourceDirectoryLocation(context.Context, Principal, string, *uint) (ResourceDirectoryLocation, error)
	MoveResourceDirectory(context.Context, Principal, string, *uint, uint64, string) (ResourceDirectoryLocation, error)
}
