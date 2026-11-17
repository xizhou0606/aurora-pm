package bitable

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type Comment struct {
	ID        string    `json:"id"`
	RecordID  string    `json:"recordId"`
	Body      string    `json:"body"`
	Author    UserRef   `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}
type CommentInput struct {
	Body        string `json:"body"`
	OperationID string `json:"operationId"`
}
type CommentPage struct {
	Items  []Comment `json:"items"`
	Total  int64     `json:"total"`
	Offset int       `json:"offset"`
}
type CommentStore interface {
	ListComments(context.Context, Scope, string, int, int) (CommentPage, error)
	CreateComment(context.Context, Scope, string, CommentInput) (Comment, error)
}

func ValidateComment(in CommentInput) error {
	if !ValidOperationID(in.OperationID) || !utf8.ValidString(in.Body) || strings.TrimSpace(in.Body) == "" || utf8.RuneCountInString(in.Body) > 4000 || strings.ContainsRune(in.Body, 0) {
		return ErrInvalidInput
	}
	return nil
}
func (s *Service) Comments(ctx context.Context, p Principal, ref, table, record string, offset, limit int) (CommentPage, error) {
	if !ValidOperationID(record) || offset < 0 || offset > MaxExportRows || limit < 1 || limit > MaxPageRows {
		return CommentPage{}, ErrInvalidInput
	}
	scope, binding, e := s.scope(ctx, p, ref, table, documentlibrary.OperationView)
	if e != nil {
		return CommentPage{}, e
	}
	store, ok := s.provider.(CommentStore)
	if !ok {
		return CommentPage{}, ErrUnavailable
	}
	result, e := store.ListComments(ctx, scope, record, offset, limit)
	if e != nil {
		return CommentPage{}, e
	}
	if e = s.resolveCommentAuthors(ctx, p, result.Items); e != nil {
		return CommentPage{}, e
	}
	if e = s.access.AuthorizeBitableResource(ctx, p, binding, documentlibrary.OperationView); e != nil {
		return CommentPage{}, e
	}
	return result, nil
}
func (s *Service) CreateComment(ctx context.Context, p Principal, ref, table, record string, in CommentInput) (Comment, error) {
	if !ValidOperationID(record) {
		return Comment{}, ErrInvalidInput
	}
	if e := ValidateComment(in); e != nil {
		return Comment{}, e
	}
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationComment)
	if e != nil {
		return Comment{}, e
	}
	store, ok := s.provider.(CommentStore)
	if !ok {
		return Comment{}, ErrUnavailable
	}
	result, e := store.CreateComment(ctx, scope, record, in)
	if e != nil {
		return Comment{}, e
	}
	items := []Comment{result}
	if e = s.resolveCommentAuthors(ctx, p, items); e != nil {
		return Comment{}, e
	}
	return items[0], nil
}
func (s *Service) resolveCommentAuthors(ctx context.Context, p Principal, items []Comment) error {
	if len(items) == 0 {
		return nil
	}
	if s.people == nil {
		return ErrUnavailable
	}
	cache := map[uint]UserRef{}
	for i, item := range items {
		author, ok := cache[item.Author.ID]
		if !ok {
			page, e := s.people.SearchWorkspaceUsers(ctx, p.WorkspaceID, identity.ToolUserFilter{ID: item.Author.ID, Limit: 1})
			if e != nil {
				return e
			}
			author = UserRef{ID: item.Author.ID, Name: strconv.FormatUint(uint64(item.Author.ID), 10)}
			if len(page.Items) == 1 && page.Items[0].ID == item.Author.ID {
				author.Name = strings.TrimSpace(page.Items[0].DisplayName)
				if author.Name == "" {
					author.Name = page.Items[0].Username
				}
			}
			cache[item.Author.ID] = author
		}
		items[i].Author = author
	}
	return nil
}
