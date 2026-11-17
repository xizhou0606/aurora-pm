package collaborationhttp

import (
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
)

func TestPublicPresenceUsesNonReplayableStableIdentifier(t *testing.T) {
	stored := collaboration.Presence{
		Room:         collaboration.Room{WorkspaceID: 7, ResourceID: "resource-1"},
		ConnectionID: "private-connection", ActorUserID: 9, ActorName: "Alice",
	}
	first, second := publicPresence(stored), publicPresence(stored)
	if first.ConnectionID != second.ConnectionID || first.ConnectionID == stored.ConnectionID ||
		!strings.HasPrefix(first.ConnectionID, "presence-") || strings.Contains(first.ConnectionID, stored.ConnectionID) {
		t.Fatalf("unsafe public presence id: stored=%q public=%q", stored.ConnectionID, first.ConnectionID)
	}
	other := stored
	other.ActorUserID++
	if publicPresence(other).ConnectionID == first.ConnectionID {
		t.Fatal("different actor reused public presence id")
	}
}
