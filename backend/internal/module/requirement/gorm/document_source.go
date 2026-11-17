package requirementgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

func (r *ResourceLinkRepository) ReadDocumentSource(ctx context.Context, workspaceID, id uint) (requirement.DocumentSource, error) {
	if workspaceID == 0 || id == 0 {
		return requirement.DocumentSource{}, requirement.ErrResourceLinkNotFound
	}
	var row Requirement
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error; err != nil {
		return requirement.DocumentSource{}, err
	}
	var ids []string
	if err := r.db.WithContext(ctx).Model(&ResourceLinkRecord{}).Where("workspace_id = ? AND requirement_id = ?", workspaceID, id).Order("resource_id").Pluck("resource_id", &ids).Error; err != nil {
		return requirement.DocumentSource{}, err
	}
	return requirement.DocumentSource{RequirementID: row.ID, Version: row.Version, ProjectID: row.ProjectSetID, ProductDocURL: row.ProductDocURL, ResourceIDs: ids}, nil
}
