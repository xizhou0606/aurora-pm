package toolruntime

import (
	"context"
	"errors"
	"time"
)

var ErrAuthorizationRecordNotFound = errors.New("authorization record not found")
var ErrAuthorizationHistoryForbidden = errors.New("authorization history access denied")
var ErrAuthorizationHistoryPage = errors.New("authorization history page is invalid")

type AuthorizationUser struct {
	Name     string
	Username string
}
type AuthorizationHistoryAccess interface {
	CanManage(context.Context, ManagementPrincipal) (bool, error)
	CanRevoke(context.Context, ManagementPrincipal) (bool, error)
	Names(context.Context, []uint) (map[uint]AuthorizationUser, error)
}
type AuthorizationHistoryScope struct {
	WorkspaceID, PrincipalID uint
	All                      bool
}
type AuthorizationHistoryRecord struct {
	ID                string     `json:"id"`
	UserCode          *string    `json:"user_code"`
	Channel           string     `json:"channel"`
	ClientName        string     `json:"client_name"`
	PrincipalID       *uint      `json:"principal_id"`
	PrincipalName     *string    `json:"principal_name"`
	PrincipalUsername *string    `json:"principal_username"`
	GrantTools        []string   `json:"grant_tools"`
	CreatedAt         time.Time  `json:"created_at"`
	DecidedAt         *time.Time `json:"decided_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	Status            string     `json:"status"`
	CodeValid         bool       `json:"code_valid"`
	CredentialValid   bool       `json:"credential_valid"`
	CanRevoke         bool       `json:"can_revoke"`
}
type AuthorizationHistoryPage struct {
	Items    []AuthorizationHistoryRecord `json:"items"`
	Total    int64                        `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}
type AuthorizationHistoryStore interface {
	ListHistory(context.Context, AuthorizationHistoryScope, int, int, time.Time) (AuthorizationHistoryPage, error)
	RevokeHistory(context.Context, AuthorizationHistoryScope, string, time.Time) error
}
type AuthorizationHistoryService struct {
	store  AuthorizationHistoryStore
	access AuthorizationHistoryAccess
	now    func() time.Time
}

func NewAuthorizationHistoryService(store AuthorizationHistoryStore, access AuthorizationHistoryAccess) *AuthorizationHistoryService {
	return &AuthorizationHistoryService{store: store, access: access, now: time.Now}
}
func (s *AuthorizationHistoryService) scope(ctx context.Context, p ManagementPrincipal) (AuthorizationHistoryScope, error) {
	if s == nil || s.store == nil || s.access == nil || validateManagementPrincipal(p) != nil {
		return AuthorizationHistoryScope{}, ErrAuthorizationHistoryForbidden
	}
	all, err := s.access.CanManage(ctx, p)
	return AuthorizationHistoryScope{WorkspaceID: p.WorkspaceID, PrincipalID: p.PrincipalID, All: all}, err
}
func (s *AuthorizationHistoryService) List(ctx context.Context, p ManagementPrincipal, page, size int) (AuthorizationHistoryPage, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return AuthorizationHistoryPage{}, ErrAuthorizationHistoryPage
	}
	scope, err := s.scope(ctx, p)
	if err != nil {
		return AuthorizationHistoryPage{}, err
	}
	result, err := s.store.ListHistory(ctx, scope, page, size, s.now().UTC())
	if err != nil {
		return result, err
	}
	revoke, err := s.access.CanRevoke(ctx, p)
	if err != nil {
		return AuthorizationHistoryPage{}, err
	}
	ids := []uint{}
	for _, r := range result.Items {
		if r.PrincipalID != nil {
			ids = append(ids, *r.PrincipalID)
		}
	}
	names, err := s.access.Names(ctx, ids)
	if err != nil {
		return AuthorizationHistoryPage{}, err
	}
	for i := range result.Items {
		r := &result.Items[i]
		r.CanRevoke = r.CanRevoke && revoke && (scope.All || (r.PrincipalID != nil && *r.PrincipalID == p.PrincipalID))
		if r.PrincipalID != nil {
			if u, ok := names[*r.PrincipalID]; ok {
				r.PrincipalName = &u.Name
				r.PrincipalUsername = &u.Username
			}
		}
	}
	return result, nil
}
func (s *AuthorizationHistoryService) Revoke(ctx context.Context, p ManagementPrincipal, id string) error {
	scope, err := s.scope(ctx, p)
	if err != nil {
		return err
	}
	allowed, err := s.access.CanRevoke(ctx, p)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAuthorizationHistoryForbidden
	}
	if len(id) < 3 || len(id) > 132 {
		return ErrAuthorizationRecordNotFound
	}
	return s.store.RevokeHistory(ctx, scope, id, s.now().UTC())
}
