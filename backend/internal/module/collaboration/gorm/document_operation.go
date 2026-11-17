package collaborationgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

// LookupDocumentCommit is scoped to both room and actor. Authorization is
// performed by the document use case before this recovery-only repository call.
func (repository *Repository) LookupDocumentCommit(ctx context.Context, p collaboration.Principal, room collaboration.Room, operation string) (collaboration.DocumentCommit, bool, error) {
	if p.WorkspaceID != room.WorkspaceID || p.UserID == 0 {
		return collaboration.DocumentCommit{}, false, collaboration.ErrForbidden
	}
	row, found, err := findDocumentOperation(repository.db.WithContext(ctx), room, operation)
	if err != nil || !found {
		return collaboration.DocumentCommit{}, false, err
	}
	if row.ActorUserID != p.UserID {
		return collaboration.DocumentCommit{}, false, collaboration.ErrForbidden
	}
	return documentCommit(row), true, nil
}
