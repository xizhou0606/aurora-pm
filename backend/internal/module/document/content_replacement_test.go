package document

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testContentReplacer struct {
	called bool
	err    error
}

func (r *testContentReplacer) Replace(_ context.Context, _ Principal, d Document, _ ContentReplacement) (ContentReplacementResult, error) {
	r.called = true
	return ContentReplacementResult{ID: d.ID, Revision: 2}, r.err
}
func TestToolContentReplacementPermissionsRevisionAndCollaboration(t *testing.T) {
	p := Principal{WorkspaceID: 7, UserID: 5}
	input := ContentReplacement{ExpectedRevision: 1, SchemaVersion: 1, Content: DefaultContent(), OperationID: "tool-test"}
	for _, tc := range []struct {
		name    string
		actions []string
		canEdit bool
		collab  bool
		failure error
	}{
		{"legacy", []string{string(ActionContentView), string(ActionContentUpdate)}, true, false, nil},
		{"no action", []string{string(ActionContentView)}, true, false, ErrForbidden},
		{"object viewer", []string{string(ActionContentView), string(ActionContentUpdate)}, false, false, ErrForbidden},
		{"collaborator active", []string{string(ActionContentView), string(ActionContentUpdate)}, true, true, ErrEditingInProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newPolicyTestService(t, tc.actions)
			s.ConfigureResourceAccess(stubResourceAccess{values: map[uint]ResourceAccess{11: {ResourceID: "doc_11", CanView: true, CanEdit: tc.canEdit}}})
			replacer := &testContentReplacer{err: ErrEditingInProgress}
			s.ConfigureContentReplacer(replacer)
			if tc.collab {
				store := s.store.(*stubStore)
				d := store.documents[11]
				now := time.Now()
				d.CollaborationCutoverAt = &now
				store.documents[11] = d
				s.ConfigureCollaborationProjection(stubCollaborationProjection{value: CollaborationProjection{Content: DefaultContent(), SchemaVersion: 1, Revision: 1}})
			}
			result, err := s.ReplaceToolContent(context.Background(), p, 11, input)
			if !errors.Is(err, tc.failure) {
				t.Fatalf("%v", err)
			}
			if tc.failure == nil {
				if result.Revision != 2 {
					t.Fatal(result)
				}
				if _, err = s.ReplaceToolContent(context.Background(), p, 11, input); !errors.Is(err, ErrRevisionConflict) {
					t.Fatal(err)
				}
			}
			if tc.collab != replacer.called {
				t.Fatalf("collaboration called=%v", replacer.called)
			}
		})
	}
}
