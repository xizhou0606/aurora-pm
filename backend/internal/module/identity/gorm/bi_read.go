package identitygorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) GetBIWorkspace(ctx context.Context, workspaceID uint) (identity.BIWorkspace, error) {
	var row Workspace
	err := repository.db.WithContext(ctx).Where("id = ? AND enabled = ?", workspaceID, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return identity.BIWorkspace{}, identity.ErrBIReadInvalid
	}
	return identity.BIWorkspace{ID: row.ID, Name: row.Name}, err
}

func (repository *BIReadRepository) FindActiveBIUsersByFactIDs(ctx context.Context, ids []uint) (map[uint]identity.BIUser, error) {
	result := make(map[uint]identity.BIUser, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []User
	if err := repository.db.WithContext(ctx).Where("active = ?", true).
		Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		factID := row.ID
		if existing, exists := result[factID]; exists && existing.ID != row.ID {
			return nil, identity.ErrBIReadInvalid
		}
		result[factID] = identity.BIUser{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: row.AvatarURL}
	}
	return result, nil
}

var _ identity.BIReadStore = (*BIReadRepository)(nil)
