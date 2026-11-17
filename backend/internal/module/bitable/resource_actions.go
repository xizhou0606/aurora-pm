package bitable

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"strings"
	"unicode/utf8"
)

var resourceOperations = map[string]documentlibrary.Operation{
	"name": documentlibrary.OperationStructure, "location": documentlibrary.OperationView,
	"delete": documentlibrary.OperationDelete, "duplicate": documentlibrary.OperationCopy,
	"pin": documentlibrary.OperationView, "favorite": documentlibrary.OperationView,
	"owner": documentlibrary.OperationManageAccess, "shortcut": documentlibrary.OperationCopy,
}

func (s *Service) prepareResourceMutation(ctx context.Context, p Principal, id, kind string, in *ResourceMutation) (ResourceManager, ResourceBinding, error) {
	operation, ok := resourceOperations[kind]
	if !ok {
		return nil, ResourceBinding{}, ErrInvalidInput
	}
	binding, e := s.requireResource(ctx, p, id, operation)
	if e != nil {
		return nil, ResourceBinding{}, e
	}
	if !ValidOperationID(in.OperationID) || in.ExpectedRevision > MaxSafeRevision {
		return nil, ResourceBinding{}, ErrInvalidInput
	}
	if kind != "pin" && kind != "favorite" && in.ExpectedRevision == 0 {
		return nil, ResourceBinding{}, ErrInvalidInput
	}
	if in.Pinned != nil && kind != "pin" || in.Favorited != nil && kind != "favorite" || in.OwnerUserID != 0 && kind != "owner" || in.Name != "" && kind != "name" && kind != "duplicate" || in.ParentFolderID != nil && kind != "location" && kind != "shortcut" && kind != "duplicate" {
		return nil, ResourceBinding{}, ErrInvalidInput
	}
	if kind == "name" || kind == "duplicate" {
		in.Name = strings.TrimSpace(in.Name)
		if !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) > 200 || kind == "name" && in.Name == "" {
			return nil, ResourceBinding{}, ErrInvalidInput
		}
	}
	if kind == "pin" && in.Pinned == nil || kind == "favorite" && in.Favorited == nil {
		return nil, ResourceBinding{}, ErrInvalidInput
	}
	if kind == "duplicate" {
		if s.creation == nil {
			return nil, ResourceBinding{}, ErrUnavailable
		}
		if e := s.access.RequireBitableCreate(ctx, p); e != nil {
			return nil, ResourceBinding{}, e
		}
	}
	if in.ParentFolderID != nil {
		if s.folders == nil {
			return nil, ResourceBinding{}, ErrUnavailable
		}
		if e := s.folders.ValidateBitableFolder(ctx, p, *in.ParentFolderID); e != nil {
			return nil, ResourceBinding{}, e
		}
	}
	if kind == "owner" {
		if in.OwnerUserID == 0 || s.people == nil {
			return nil, ResourceBinding{}, ErrInvalidInput
		}
		page, e := s.people.SearchWorkspaceUsers(ctx, p.WorkspaceID, identity.ToolUserFilter{ID: in.OwnerUserID, Limit: 2})
		if e != nil {
			return nil, ResourceBinding{}, e
		}
		if len(page.Items) != 1 || page.Items[0].ID != in.OwnerUserID {
			return nil, ResourceBinding{}, ErrForbidden
		}
	}
	if kind == "location" {
		return nil, binding, nil
	}
	if kind != "pin" && kind != "favorite" && kind != "shortcut" {
		if s.creation == nil {
			return nil, ResourceBinding{}, ErrUnavailable
		}
		return nil, binding, nil
	}
	m, ok := s.store.(ResourceManager)
	if !ok {
		return nil, ResourceBinding{}, ErrUnavailable
	}
	return m, binding, nil
}
func (s *Service) MutateResource(ctx context.Context, p Principal, id, kind string, in ResourceMutation) (Resource, error) {
	m, binding, e := s.prepareResourceMutation(ctx, p, id, kind, &in)
	if e != nil {
		return Resource{}, e
	}
	if kind == "location" {
		return s.moveDirectoryResource(ctx, p, binding, in)
	}
	if kind == "shortcut" {
		return Resource{}, ErrInvalidInput
	}
	if kind == "duplicate" {
		v, created, err := s.creation.DuplicatePrivateBitable(ctx, p, binding, in)
		if err != nil {
			return Resource{}, err
		}
		if err := validateResourceBinding(created, v.ID); err != nil {
			return Resource{}, err
		}
		v = projectResource(v, created)
		v.AllowedActions = s.allowedActions(ctx, p, created)
		return v, nil
	}
	if kind != "pin" && kind != "favorite" {
		v, updated, err := s.creation.MutateBitableResource(ctx, p, binding, kind, in)
		if err != nil {
			return Resource{}, err
		}
		if err := validateResourceBinding(updated, v.ID); err != nil {
			return Resource{}, err
		}
		v = projectResource(v, updated)
		v.AllowedActions = s.allowedActions(ctx, p, updated)
		return v, nil
	}
	v, e := m.MutateResource(ctx, p, binding.NativeResourceID, kind, in)
	if e != nil {
		return v, e
	}
	v = projectResource(v, binding)
	v.AllowedActions = s.allowedActions(ctx, p, binding)
	return v, nil
}
func (s *Service) Shortcuts(ctx context.Context, p Principal) ([]Shortcut, error) {
	if s.access == nil {
		return nil, ErrUnavailable
	}
	m, ok := s.store.(ResourceManager)
	if !ok {
		return nil, ErrUnavailable
	}
	items, e := m.Shortcuts(ctx, p)
	if e != nil {
		return nil, e
	}
	if len(items) == 0 {
		return []Shortcut{}, nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TargetResourceID)
	}
	bindings, e := s.access.FilterBitableResources(ctx, p, ids, documentlibrary.OperationView)
	if e != nil {
		return nil, e
	}
	result := make([]Shortcut, 0, len(items))
	for _, item := range items {
		if binding, ok := bindings[item.TargetResourceID]; ok {
			item.TargetResourceID = string(binding.ResourceID)
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *Service) MutateShortcut(ctx context.Context, p Principal, id, kind string, in ResourceMutation) (Shortcut, error) {
	if kind != "shortcut" && kind != "location" && kind != "delete" {
		return Shortcut{}, ErrInvalidInput
	}
	if kind == "shortcut" {
		m, binding, e := s.prepareResourceMutation(ctx, p, id, kind, &in)
		if e != nil {
			return Shortcut{}, e
		}
		result, e := m.MutateShortcut(ctx, p, binding.NativeResourceID, kind, in)
		result.TargetResourceID = string(binding.ResourceID)
		return result, e
	}
	if !ValidOperationID(id) || !ValidOperationID(in.OperationID) {
		return Shortcut{}, ErrInvalidInput
	}
	m, ok := s.store.(ResourceManager)
	if !ok || s.access == nil {
		return Shortcut{}, ErrUnavailable
	}
	if kind == "location" && in.ParentFolderID != nil {
		if s.folders == nil {
			return Shortcut{}, ErrUnavailable
		}
		if err := s.folders.ValidateBitableFolder(ctx, p, *in.ParentFolderID); err != nil {
			return Shortcut{}, err
		}
	}
	current, e := m.GetShortcut(ctx, p, id)
	if e != nil {
		return Shortcut{}, e
	}
	bindings, e := s.access.FilterBitableResources(ctx, p, []string{current.TargetResourceID}, documentlibrary.OperationView)
	if e != nil {
		return Shortcut{}, e
	}
	binding, allowed := bindings[current.TargetResourceID]
	if !allowed {
		return Shortcut{}, ErrNotFound
	}
	result, e := m.MutateShortcut(ctx, p, id, kind, in)
	result.TargetResourceID = string(binding.ResourceID)
	return result, e
}
