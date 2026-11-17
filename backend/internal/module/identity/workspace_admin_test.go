package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type workspaceAdminStoreStub struct {
	created identity.WorkspaceInput
	member  identity.WorkspaceMemberInput
	filter  identity.WorkspaceMemberFilter
}

func (stub *workspaceAdminStoreStub) ListManagedWorkspaces(context.Context, uint, bool) ([]identity.ManagedWorkspace, error) {
	return []identity.ManagedWorkspace{}, nil
}
func (stub *workspaceAdminStoreStub) FindManagedWorkspace(context.Context, uint) (identity.ManagedWorkspace, error) {
	return identity.ManagedWorkspace{ID: 1}, nil
}
func (stub *workspaceAdminStoreStub) FindVisibleManagedWorkspace(context.Context, uint, uint) (identity.ManagedWorkspace, error) {
	return identity.ManagedWorkspace{ID: 1}, nil
}
func (stub *workspaceAdminStoreStub) CreateManagedWorkspace(_ context.Context, input identity.WorkspaceInput) (identity.ManagedWorkspace, error) {
	stub.created = input
	return identity.ManagedWorkspace{ID: 1, Name: input.Name, Slug: input.Slug}, nil
}
func (stub *workspaceAdminStoreStub) UpdateManagedWorkspace(context.Context, uint, identity.WorkspaceUpdate) (identity.ManagedWorkspace, error) {
	return identity.ManagedWorkspace{ID: 1}, nil
}
func (stub *workspaceAdminStoreStub) DeleteManagedWorkspace(context.Context, uint) error { return nil }
func (stub *workspaceAdminStoreStub) ListManagedWorkspaceMembers(_ context.Context, _ uint, filter identity.WorkspaceMemberFilter) (identity.ManagedWorkspaceMemberPage, error) {
	stub.filter = filter
	return identity.ManagedWorkspaceMemberPage{}, nil
}
func (stub *workspaceAdminStoreStub) CreateManagedWorkspaceMember(_ context.Context, _ uint, input identity.WorkspaceMemberInput) (identity.ManagedWorkspaceMember, error) {
	stub.member = input
	return identity.ManagedWorkspaceMember{UserID: input.UserID, Role: input.Role}, nil
}
func (stub *workspaceAdminStoreStub) UpdateManagedWorkspaceMember(context.Context, uint, uint, identity.WorkspaceMemberUpdate) (identity.ManagedWorkspaceMember, error) {
	return identity.ManagedWorkspaceMember{UserID: 1}, nil
}
func (stub *workspaceAdminStoreStub) DeleteManagedWorkspaceMember(context.Context, uint, uint) error {
	return nil
}

func TestWorkspaceAdminServiceNormalizesWorkspaceAndMember(t *testing.T) {
	store := &workspaceAdminStoreStub{}
	service := identity.NewWorkspaceAdminService(store)
	workspace, err := service.Create(context.Background(), identity.WorkspaceInput{
		Name: " 研发工作区 ", Slug: " DEV-TEAM ", Description: " 研发 ", Enabled: true,
	})
	if err != nil || workspace.Slug != "dev-team" || store.created.Description != "研发" {
		t.Fatalf("workspace=%+v input=%+v err=%v", workspace, store.created, err)
	}
	member, err := service.AddMember(context.Background(), 99, workspace.ID, identity.WorkspaceMemberInput{
		UserID: 7, Role: " OWNER ", Enabled: true,
	})
	if err != nil || member.Role != "owner" || store.member.Role != "owner" {
		t.Fatalf("member=%+v input=%+v err=%v", member, store.member, err)
	}
	if _, err := service.ListMembers(context.Background(), 99, workspace.ID, identity.WorkspaceMemberFilter{
		Query: "  member  ", PageSize: 200,
	}); err != nil || store.filter.Query != "member" || store.filter.Page != 1 || store.filter.PageSize != 100 {
		t.Fatalf("member filter=%+v err=%v", store.filter, err)
	}
}

func TestWorkspaceAdminServiceRejectsInvalidInput(t *testing.T) {
	service := identity.NewWorkspaceAdminService(&workspaceAdminStoreStub{})
	if _, err := service.List(context.Background(), 0, true); !errors.Is(err, identity.ErrAccessUserNotFound) {
		t.Fatalf("invalid principal err=%v", err)
	}
	if _, err := service.Create(context.Background(), identity.WorkspaceInput{Name: "", Slug: "dev"}); !errors.Is(err, identity.ErrWorkspaceNameInvalid) {
		t.Fatalf("invalid name err=%v", err)
	}
	if _, err := service.AddMember(context.Background(), 99, 1, identity.WorkspaceMemberInput{UserID: 7, Role: "admin"}); !errors.Is(err, identity.ErrWorkspaceMemberRoleInvalid) {
		t.Fatalf("invalid role err=%v", err)
	}
	if _, err := service.ListMembers(context.Background(), 99, 1, identity.WorkspaceMemberFilter{Query: strings.Repeat("长", 201)}); !errors.Is(err, identity.ErrWorkspaceMemberQueryInvalid) {
		t.Fatalf("invalid member query err=%v", err)
	}
}
