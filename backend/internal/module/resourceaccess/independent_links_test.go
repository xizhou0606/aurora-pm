package resourceaccess

import (
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"testing"
)

func TestIndependentLinksUnionWithoutReplacingMembers(t *testing.T) {
	resource := documentlibrary.Resource{OwnerUserID: 7}
	subject := identity.SubjectRef{Kind: identity.SubjectUser, ID: "9"}
	matches := map[identity.SubjectRef]bool{subject: true}
	viewer := LevelViewer
	for _, mode := range []Mode{ModePrivate, ModeRestricted, ModeWorkspace, ModeLink} {
		policy := Policy{Mode: mode, WorkspaceLevel: &viewer, Rules: []Rule{{SubjectKind: subject.Kind, SubjectID: subject.ID, Level: LevelViewer}}}
		level, sources := effectiveObjectLevel(9, resource, policy, matches, LevelEditor)
		if level == nil || *level != LevelEditor || !containsSource(sources, "link") {
			t.Fatalf("%s: %v %v", mode, level, sources)
		}
		level, sources = effectiveObjectLevel(9, resource, policy, matches, "")
		member := mode == ModeRestricted || mode == ModeWorkspace
		if member && (level == nil || *level != LevelViewer) {
			t.Fatalf("%s: member permission lost", mode)
		}
		if !member && level != nil {
			t.Fatalf("%s: revoked link still grants access", mode)
		}
	}
	level, _ := effectiveObjectLevel(9, resource, Policy{Mode: ModeLegacy}, matches, LevelEditor)
	if level != nil {
		t.Fatal("legacy must not accept new link grants")
	}
}
