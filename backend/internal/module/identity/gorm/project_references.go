package identitygorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type ProjectReferenceRepository struct{ db *gorm.DB }

func NewProjectReferenceRepository(db *gorm.DB) *ProjectReferenceRepository {
	return &ProjectReferenceRepository{db: db}
}

func (repository *ProjectReferenceRepository) ProjectReferencesValid(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
	userIDs []uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return false, identity.ErrProjectReferencesInvalid
	}
	db := repository.db.WithContext(ctx)
	if teamID > 0 {
		var teams int64
		if err := db.Model(&Team{}).Where("id = ? AND workspace_id = ?", teamID, workspaceID).Count(&teams).Error; err != nil {
			return false, err
		}
		if teams != 1 {
			return false, nil
		}
	}
	if len(userIDs) == 0 {
		return true, nil
	}
	var users int64
	if err := db.Model(&User{}).Where("id IN ? AND active = ? AND status = ?", userIDs, true, string(identity.AccountActive)).Count(&users).Error; err != nil {
		return false, err
	}
	return users == int64(len(userIDs)), nil
}

var _ identity.ProjectReferenceStore = (*ProjectReferenceRepository)(nil)
