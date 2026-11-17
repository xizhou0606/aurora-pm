package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"testing"
)

type documentToolActions struct{ codes []string }

func (s *documentToolActions) EffectiveDocumentActionCodes(context.Context, uint, uint) ([]string, error) {
	return s.codes, nil
}

type documentToolStore struct {
	document.Store
	row       document.Document
	workspace uint
}

func (s *documentToolStore) GetDocument(_ context.Context, w, id uint) (document.Document, error) {
	s.workspace = w
	if w != s.row.WorkspaceID || id != s.row.ID {
		return document.Document{}, document.ErrDocumentNotFound
	}
	return s.row, nil
}

type documentToolUsers struct{}

func (documentToolUsers) FactUserID(context.Context, uint) (uint, error) { return 21, nil }

type documentToolRequirements struct {
	document.RequirementParticipantResolver
}
type documentToolAccess struct {
	document.ResourceAccessResolver
	view      bool
	principal document.Principal
}

func (a *documentToolAccess) ResolveDocumentAccess(_ context.Context, p document.Principal, _ uint) (document.ResourceAccess, error) {
	a.principal = p
	return document.ResourceAccess{ResourceID: "resource-7", CanView: a.view}, nil
}
func TestDocumentReadToolsUseDomainAuthorization(t *testing.T) {
	actions := &documentToolActions{codes: []string{string(document.ActionContentView)}}
	store := &documentToolStore{row: document.Document{ID: 11, WorkspaceID: 7, Content: json.RawMessage(`{"type":"doc","content":[]}`)}}
	service, err := document.NewService(store, actions, documentToolRequirements{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	access := &documentToolAccess{view: true}
	service.ConfigureResourceAccess(access)
	handlers, err := NewDocumentReadHandlers(service, documentToolUsers{})
	if err != nil {
		t.Fatal(err)
	}
	call := toolruntime.ToolCall{Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7, PrincipalID: 9}, Input: json.RawMessage(`{"id":11}`)}
	if _, err = handlers["document.get"].Invoke(context.Background(), call); err != nil {
		t.Fatal(err)
	}
	if access.principal.UserID != 9 || access.principal.ParticipantUserID != 21 || store.workspace != 7 {
		t.Fatal("identity not server-owned")
	}
	assertCode := func(code string) {
		t.Helper()
		_, err = handlers["document.get"].Invoke(context.Background(), call)
		failure := toolruntime.PublicBusinessError(err)
		if failure == nil || failure.Code != code {
			t.Fatalf("want %s: %v", code, err)
		}
	}
	access.view = false
	assertCode("DOCUMENT_FORBIDDEN")
	access.view = true
	actions.codes = nil
	assertCode("DOCUMENT_FORBIDDEN")
	actions.codes = []string{string(document.ActionContentView)}
	call.Authorization.WorkspaceID = 8
	assertCode("DOCUMENT_NOT_FOUND")
	call.Authorization.WorkspaceID = 7
	call.Input = json.RawMessage(`{"id":11,"user_id":1}`)
	if _, err = handlers["document.get"].Invoke(context.Background(), call); !errors.Is(err, toolruntime.ErrToolInputInvalid) {
		t.Fatal("accepted forged actor")
	}
	writes, err := NewDocumentWriteHandlers(service, documentToolUsers{})
	if err != nil {
		t.Fatal(err)
	}
	for name, h := range writes {
		if _, err = h.Invoke(context.Background(), call); !errors.Is(err, toolruntime.ErrChangeSetRequired) {
			t.Fatalf("%s bypassed ChangeSet", name)
		}
	}
}
