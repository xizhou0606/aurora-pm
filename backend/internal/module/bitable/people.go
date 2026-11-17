package bitable

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type PeopleDirectory interface {
	SearchWorkspaceUsers(context.Context, uint, identity.ToolUserFilter) (identity.ToolUserPage, error)
}

func (s *Service) normalizePeople(ctx context.Context, p Principal, b CommandBatch, fields []Field) (CommandBatch, error) {
	users := map[string]bool{}
	for _, f := range fields {
		if f.Type == FieldUser {
			users[f.ID] = true
		}
	}
	result := b
	result.Commands = append([]Command(nil), b.Commands...)
	cache := map[uint]UserRef{}
	for i, c := range result.Commands {
		values := map[string]any{}
		for id, value := range c.Values {
			values[id] = value
			if !users[id] || value == nil {
				continue
			}
			if s.people == nil {
				return CommandBatch{}, ErrUnavailable
			}
			data, e := json.Marshal(value)
			if e != nil {
				return CommandBatch{}, ErrInvalidInput
			}
			var refs []UserRef
			if json.Unmarshal(data, &refs) != nil {
				return CommandBatch{}, ErrInvalidInput
			}
			for j, user := range refs {
				canonical, ok := cache[user.ID]
				if !ok {
					page, e := s.people.SearchWorkspaceUsers(ctx, p.WorkspaceID, identity.ToolUserFilter{ID: user.ID, Limit: 2})
					if e != nil || len(page.Items) != 1 || page.Items[0].ID != user.ID {
						return CommandBatch{}, ErrForbidden
					}
					u := page.Items[0]
					name := strings.TrimSpace(u.DisplayName)
					if name == "" {
						name = u.Username
					}
					canonical = UserRef{ID: u.ID, Name: name, Username: u.Username}
					cache[u.ID] = canonical
				}
				refs[j] = canonical
			}
			values[id] = refs
		}
		result.Commands[i].Values = values
	}
	return result, nil
}

type UserRef struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}
type PeoplePage struct {
	Items []UserRef `json:"items"`
	Total int64     `json:"total"`
}

func (s *Service) ConfigurePeopleDirectory(directory PeopleDirectory) { s.people = directory }

// NormalizeRecordUsers is the transaction adapter's callback for canonical
// people validation, including restore snapshots that never came from the client.
func (s *Service) NormalizeRecordUsers(ctx context.Context, p Principal, fields []Field, values map[string]any) (map[string]any, error) {
	b, e := s.normalizePeople(ctx, p, CommandBatch{Commands: []Command{{Values: values}}}, fields)
	if e != nil {
		return nil, e
	}
	return b.Commands[0].Values, nil
}
func (s *Service) People(ctx context.Context, p Principal, ref, table, query string, offset, limit int) (PeoplePage, error) {
	_, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return PeoplePage{}, e
	}
	if s.people == nil {
		return PeoplePage{}, ErrUnavailable
	}
	if offset < 0 || offset > MaxExportRows || limit < 1 || limit > MaxPageRows || len(query) > 800 {
		return PeoplePage{}, ErrInvalidInput
	}
	page, e := s.people.SearchWorkspaceUsers(ctx, p.WorkspaceID, identity.ToolUserFilter{Query: query, Offset: offset, Limit: limit})
	if e != nil {
		return PeoplePage{}, e
	}
	if e = s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationView); e != nil {
		return PeoplePage{}, e
	}
	result := PeoplePage{Items: []UserRef{}, Total: page.Total}
	for _, u := range page.Items {
		name := strings.TrimSpace(u.DisplayName)
		if name == "" {
			name = u.Username
		}
		result.Items = append(result.Items, UserRef{ID: u.ID, Name: name, Username: u.Username})
	}
	return result, nil
}
