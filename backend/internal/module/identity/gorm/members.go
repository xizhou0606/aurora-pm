package identitygorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

// MemberCandidateRepository projects active workspace members as identity
// matching candidates for contribution binding flows.
type MemberCandidateRepository struct{ db *gorm.DB }

func NewMemberCandidateRepository(db *gorm.DB) *MemberCandidateRepository {
	return &MemberCandidateRepository{db: db}
}

func (repository *MemberCandidateRepository) FindActiveMemberCandidates(
	ctx context.Context, workspaceID uint,
) ([]identity.MemberCandidate, error) {
	var rows []User
	err := repository.db.WithContext(ctx).
		Joins("JOIN workspace_members ON workspace_members.user_id = users.id"+
			" AND workspace_members.workspace_id = ? AND workspace_members.enabled = ?",
			workspaceID, true).
		Where("users.active = ?", true).
		Order("users.id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	candidates := make([]identity.MemberCandidate, 0, len(rows))
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		factID := row.ID
		if _, duplicate := seen[factID]; duplicate {
			continue
		}
		seen[factID] = struct{}{}
		candidates = append(candidates, identity.MemberCandidate{
			FactID: factID, Username: row.Username, Email: row.Email,
		})
	}
	return candidates, nil
}

var _ identity.MemberCandidateStore = (*MemberCandidateRepository)(nil)
