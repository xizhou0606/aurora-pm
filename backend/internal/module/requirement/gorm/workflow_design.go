package requirementgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowDesignRepository struct{ db *gorm.DB }

func NewWorkflowDesignRepository(db *gorm.DB) *WorkflowDesignRepository {
	return &WorkflowDesignRepository{db: db}
}

func (repository *WorkflowDesignRepository) AttachWorkflowDesign(
	ctx context.Context,
	workspaceID, requirementID uint,
	rawURL string,
) error {
	if repository == nil || repository.db == nil {
		return requirement.ErrWorkflowDesignConflict
	}
	var row Requirement
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"id = ? AND workspace_id = ?", requirementID, workspaceID,
	).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return requirement.ErrWorkflowDesignConflict
		}
		return err
	}
	if row.DesignURL == rawURL {
		return nil
	}
	updated := repository.db.WithContext(ctx).Model(&Requirement{}).Where(
		"id = ? AND workspace_id = ? AND version = ?", row.ID, row.WorkspaceID, row.Version,
	).Updates(map[string]any{"design_url": rawURL, "version": gorm.Expr("version + 1")})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return requirement.ErrWorkflowDesignConflict
	}
	return nil
}

var _ requirement.WorkflowDesignStore = (*WorkflowDesignRepository)(nil)
