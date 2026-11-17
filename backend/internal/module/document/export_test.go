package document

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestExportDocumentRequiresReadExportAndObjectScope(t *testing.T) {
	tests := []struct {
		name        string
		userID      uint
		workspaceID uint
		codes       []string
		want        error
	}{
		{"bound editor", 5, 7, []string{string(ActionContentView), string(ActionContentExport)}, nil},
		{"bound tester", 6, 7, []string{string(ActionContentView), string(ActionContentExport)}, nil},
		{"workspace scope", 9, 7, []string{string(ActionScopeAll), string(ActionContentView), string(ActionContentExport)}, nil},
		{"view is not export", 5, 7, []string{string(ActionContentView)}, ErrForbidden},
		{"scope is not export", 5, 7, []string{string(ActionScopeAll), string(ActionContentView)}, ErrForbidden},
		{"export is not view", 5, 7, []string{string(ActionScopeAll), string(ActionContentExport)}, ErrForbidden},
		{"actions are not object scope", 9, 7, []string{string(ActionContentView), string(ActionContentExport)}, ErrForbidden},
		{"foreign workspace", 5, 8, []string{string(ActionScopeAll), string(ActionContentView), string(ActionContentExport)}, ErrDocumentNotFound},
		{"missing identity", 0, 7, []string{string(ActionScopeAll), string(ActionContentView), string(ActionContentExport)}, ErrForbidden},
		{"missing workspace", 5, 0, []string{string(ActionScopeAll), string(ActionContentView), string(ActionContentExport)}, ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _ := newPolicyTestService(t, test.codes)
			store := service.store.(*stubStore)
			before := store.documents[11]
			body, err := service.ExportDocument(context.Background(), Principal{UserID: test.userID, WorkspaceID: test.workspaceID}, 11)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v want %v", err, test.want)
			}
			if test.want == nil {
				if !includesAction(body.AllowedActions, string(ActionContentExport)) {
					t.Fatalf("missing export: %v", body.AllowedActions)
				}
				body.AllowedActions = nil
				if !reflect.DeepEqual(body, before) {
					t.Fatalf("export differs from saved snapshot: %+v", body)
				}
			}
			if !reflect.DeepEqual(store.documents[11], before) {
				t.Fatal("export mutated document")
			}
		})
	}
}

func TestExportProjectionRequiresContentView(t *testing.T) {
	state := accessState{actions: map[string]struct{}{string(ActionContentExport): {}}}
	if includesAction(allowedActions(state, roleManager), string(ActionContentExport)) {
		t.Fatal("export projected without view")
	}
}

func TestExportUsesParticipantIdentityAndRevokedBinding(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionContentView), string(ActionContentExport)})
	principal := Principal{UserID: 99, ParticipantUserID: 5, WorkspaceID: 7}
	if _, err := service.ExportDocument(context.Background(), principal, 11); err != nil {
		t.Fatal(err)
	}
	service.store.(*stubStore).bindings = nil
	if _, err := service.ExportDocument(context.Background(), principal, 11); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked binding: %v", err)
	}
}
