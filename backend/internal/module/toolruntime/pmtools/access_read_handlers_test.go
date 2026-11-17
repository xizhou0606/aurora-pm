package pmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type accessReadFake struct{ workspaces []uint }

func (fake *accessReadFake) CurrentWorkspaceUser(_ context.Context, workspaceID, userID uint) (identity.ToolUser, error) {
	fake.workspaces = append(fake.workspaces, workspaceID)
	return identity.ToolUser{ID: userID, Username: "owner", DisplayName: "Owner"}, nil
}

func (fake *accessReadFake) ListWorkspaceMembers(_ context.Context, workspaceID uint) (identity.ToolUserPage, error) {
	fake.workspaces = append(fake.workspaces, workspaceID)
	return identity.ToolUserPage{Items: []identity.ToolUser{{ID: 9, Username: "owner", DisplayName: "Owner"}}, Total: 1}, nil
}

func (fake *accessReadFake) SearchWorkspaceUsers(_ context.Context, workspaceID uint, _ identity.ToolUserFilter) (identity.ToolUserPage, error) {
	fake.workspaces = append(fake.workspaces, workspaceID)
	return identity.ToolUserPage{Items: []identity.ToolUser{{ID: 9, Username: "owner", DisplayName: "Owner"}}, Total: 1}, nil
}

func (fake *accessReadFake) ListTeamMembers(_ context.Context, workspaceID, _ uint) (identity.ToolTeamMemberPage, error) {
	fake.workspaces = append(fake.workspaces, workspaceID)
	return identity.ToolTeamMemberPage{Items: []identity.ToolTeamMember{{ToolUser: identity.ToolUser{ID: 9, Username: "owner", DisplayName: "Owner"}, Role: "admin"}}, Total: 1}, nil
}

type accessReadAuthorizer struct{}

func (accessReadAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(AccessReadHandlerNames(), []string{"identity:read", "workspace:read"})
}

func TestAccessReadHandlersUseServerWorkspaceThroughApplication(t *testing.T) {
	fake := &accessReadFake{}
	handlers, err := NewAccessReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: AccessReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, accessReadAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 9, WorkspaceID: 7, GrantID: 8, ClientID: "access-read-test",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	for name, input := range map[string]json.RawMessage{
		UserMe:              json.RawMessage(`{}`),
		UserSearch:          json.RawMessage(`{"q":"owner","limit":10,"offset":0}`),
		TeamMemberList:      json.RawMessage(`{"id":11}`),
		WorkspaceMemberList: json.RawMessage(`{"id":7}`),
	} {
		result, invokeErr := application.Invoke(context.Background(), toolruntime.Invocation{Name: name, Input: input, Authorization: authorization})
		if invokeErr != nil {
			t.Fatalf("invoke %s: %v", name, invokeErr)
		}
		if result.Name != name || len(result.Output) == 0 {
			t.Fatalf("result=%+v", result)
		}
	}
	for _, workspaceID := range fake.workspaces {
		if workspaceID != 7 {
			t.Fatalf("workspace=%d", workspaceID)
		}
	}
}

func TestWorkspaceMemberListRejectsCrossWorkspaceInputBeforeQuery(t *testing.T) {
	fake := &accessReadFake{}
	handlers, err := NewAccessReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{WorkspaceMemberList}})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, accessReadAuthorizer{}, map[string]toolruntime.ToolHandler{
		WorkspaceMemberList: handlers[WorkspaceMemberList],
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.Invoke(context.Background(), toolruntime.Invocation{
		Name: WorkspaceMemberList, Input: json.RawMessage(`{"id":8}`),
		Authorization: toolruntime.ExecutionAuthorization{
			PrincipalID: 9, WorkspaceID: 7, GrantID: 8, ClientID: "access-read-test",
			Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
		},
	})
	if err == nil {
		t.Fatal("cross-workspace input was accepted")
	}
	if len(fake.workspaces) != 0 {
		t.Fatalf("cross-workspace input reached query: %v", fake.workspaces)
	}
}
