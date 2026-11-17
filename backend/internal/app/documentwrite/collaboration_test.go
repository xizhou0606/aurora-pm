package documentwrite

import (
	"context"
	collaborationclient "github.com/xizhou0606/aurora-pm/backend/internal/integration/collaboration"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	collaborationhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"strings"
	"testing"
)

type replayOperation struct{}

func (replayOperation) LookupDocumentCommit(context.Context, collaboration.Principal, collaboration.Room, string) (collaboration.DocumentCommit, bool, error) {
	return collaboration.DocumentCommit{DurableRevision: 9, ProjectionHash: strings.Repeat("a", 64)}, true, nil
}

type unreachableTickets struct{}

func (unreachableTickets) IssueDocumentConnectionTicket(context.Context, collaboration.Principal, collaboration.Room) (collaborationhttp.ConnectionTicket, error) {
	panic("replay must not request a new ticket")
}
func TestCommittedReplacementRecoversWithoutContactingNode(t *testing.T) {
	r := CollaborationReplacer{Operations: replayOperation{}, Tickets: unreachableTickets{}, Client: collaborationclient.ReplacementClient{URL: "http://127.0.0.1:1"}}
	result, err := r.Replace(context.Background(), document.Principal{WorkspaceID: 7, UserID: 5}, document.Document{ID: 11, ResourceID: "doc_11"}, document.ContentReplacement{OperationID: "tool-replay", ExpectedRevision: 1})
	if err != nil || result.ID != 11 || result.Revision != 9 {
		t.Fatalf("%+v %v", result, err)
	}
}
