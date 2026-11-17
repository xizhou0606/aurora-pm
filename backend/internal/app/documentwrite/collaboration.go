package documentwrite

import (
	"context"
	"github.com/google/uuid"
	"strconv"

	collaborationclient "github.com/xizhou0606/aurora-pm/backend/internal/integration/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

type CommittedOperationReader interface {
	LookupDocumentCommit(context.Context, collaboration.Principal, collaboration.Room, string) (collaboration.DocumentCommit, bool, error)
}
type CollaborationReplacer struct {
	Tickets    collaborationhttp.ConnectionTicketIssuer
	Operations CommittedOperationReader
	Client     collaborationclient.ReplacementClient
}

func (adapter CollaborationReplacer) Replace(ctx context.Context, p document.Principal, current document.Document, input document.ContentReplacement) (document.ContentReplacementResult, error) {
	input.ConnectionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(input.OperationID)).String()
	principal := collaboration.Principal{WorkspaceID: p.WorkspaceID, UserID: p.UserID, DisplayName: strconv.FormatUint(uint64(p.UserID), 10), ConnectionID: input.ConnectionID, LinkAccessSecret: p.LinkAccessSecret}
	room := collaboration.Room{WorkspaceID: p.WorkspaceID, ResourceID: current.ResourceID}
	if adapter.Tickets == nil || adapter.Operations == nil || adapter.Client.URL == "" {
		return document.ContentReplacementResult{}, document.ErrEditingStateUnavailable
	}
	// A lost Node response must not reapply the edit. The operation ID includes
	// the tool input digest and ApplyKey; Go's durable receipt survives restarts.
	prior, found, err := adapter.Operations.LookupDocumentCommit(ctx, principal, room, input.OperationID)
	if err != nil {
		return document.ContentReplacementResult{}, document.ErrEditingStateUnavailable
	}
	if found {
		return document.ContentReplacementResult{ID: current.ID, Revision: prior.DurableRevision, ContentHash: prior.ProjectionHash}, nil
	}
	ticket, err := adapter.Tickets.IssueDocumentConnectionTicket(ctx, principal, room)
	if err != nil {
		return document.ContentReplacementResult{}, document.ErrForbidden
	}
	result, err := adapter.Client.Replace(ctx, strconv.FormatUint(uint64(p.WorkspaceID), 10), current.ResourceID, ticket.Ticket, input)
	result.ID = current.ID
	return result, err
}
