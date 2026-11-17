package identity

import (
	"context"
	"errors"
	"testing"
)

type toolDirectoryStoreStub struct {
	page ToolUserPage
	user ToolUser
	err  error
}

func (store *toolDirectoryStoreStub) FindActiveUser(context.Context, uint) (ToolUser, error) {
	return store.user, store.err
}

func (*toolDirectoryStoreStub) ListWorkspaceMembers(context.Context, uint) (ToolUserPage, error) {
	return ToolUserPage{}, nil
}

func (store *toolDirectoryStoreStub) SearchWorkspaceUsers(_ context.Context, _ uint, filter ToolUserFilter) (ToolUserPage, error) {
	if filter.ID == 0 || filter.Limit != 2 {
		return ToolUserPage{}, ErrToolDirectoryQueryInvalid
	}
	return store.page, store.err
}

func (*toolDirectoryStoreStub) ListTeamMembers(context.Context, uint, uint) (ToolTeamMemberPage, error) {
	return ToolTeamMemberPage{}, nil
}

func TestCurrentWorkspaceUserReturnsAuthoritativeActivePrincipal(t *testing.T) {
	store := &toolDirectoryStoreStub{user: ToolUser{ID: 47, Username: "tieao", DisplayName: "铁奥"}}
	service, err := NewToolDirectoryService(store)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.CurrentWorkspaceUser(context.Background(), 2, 47)
	if err != nil || user.ID != 47 || user.Username != "tieao" || user.DisplayName != "铁奥" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
}

func TestCurrentWorkspaceUserFailsClosedWhenPrincipalIsNotActive(t *testing.T) {
	service, err := NewToolDirectoryService(&toolDirectoryStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CurrentWorkspaceUser(context.Background(), 2, 47); !errors.Is(err, ErrToolCurrentUserUnavailable) {
		t.Fatalf("err=%v", err)
	}
}
