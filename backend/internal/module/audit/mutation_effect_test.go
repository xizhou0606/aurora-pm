package audit

import (
	"context"
	"strings"
	"testing"
)

type effectStore struct{ calls int }

func (s *effectStore) RecordMutationEffect(context.Context, MutationEffect) error {
	s.calls++
	return nil
}
func TestNumericAndOpaqueResourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		id    uint
		ref   string
		valid bool
	}{{"numeric", 1, "", true}, {"uuid", 0, "550e8400-e29b-41d4-a716-446655440000", true}, {"neither", 0, "", false}, {"control", 0, "bad\nref", false}, {"url", 0, "https://invalid", false}, {"too-long", 0, strings.Repeat("a", 129), false}, {"invalid-with-id", 1, "bad ref", false}} {
		t.Run(tc.name, func(t *testing.T) {
			store := &effectStore{}
			service, _ := NewMutationEffectService(store)
			e := service.RecordMutationEffect(context.Background(), MutationEffect{WorkspaceID: 1, PrincipalID: 2, Resource: "bitable", ResourceID: tc.id, ResourceRef: tc.ref, Action: "created", EventType: "bitable.created", Payload: map[string]any{}})
			if (e == nil) != tc.valid || store.calls != boolCount(tc.valid) {
				t.Fatalf("valid=%v err=%v calls=%d", tc.valid, e, store.calls)
			}
		})
	}
}
func boolCount(v bool) int {
	if v {
		return 1
	}
	return 0
}
