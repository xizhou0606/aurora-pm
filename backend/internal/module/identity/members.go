package identity

import (
	"context"
	"errors"
)

// MemberCandidate projects one active workspace member for cross-module binding.
// FactID is a compatibility field name for the canonical internal users.id.
type MemberCandidate struct {
	FactID   uint
	Username string
	Email    string
}

var ErrMemberCandidateInvalid = errors.New("identity member candidate query is invalid")

// MemberCandidateStore is the read-only member directory projection consumed
// by contribution binding flows. Implementations must scope strictly by
// workspace and only return active members of enabled memberships.
type MemberCandidateStore interface {
	FindActiveMemberCandidates(ctx context.Context, workspaceID uint) ([]MemberCandidate, error)
}

// MemberCandidateService guards the read port.
type MemberCandidateService struct{ store MemberCandidateStore }

func NewMemberCandidateService(store MemberCandidateStore) (*MemberCandidateService, error) {
	if store == nil {
		return nil, ErrMemberCandidateInvalid
	}
	return &MemberCandidateService{store: store}, nil
}

func (service *MemberCandidateService) FindActiveMemberCandidates(ctx context.Context, workspaceID uint) ([]MemberCandidate, error) {
	if workspaceID == 0 {
		return nil, ErrMemberCandidateInvalid
	}
	return service.store.FindActiveMemberCandidates(ctx, workspaceID)
}
