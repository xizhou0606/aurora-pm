package designreviewgorm

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
)

func (repository *Repository) SaveAsset(ctx context.Context, input designreview.AssetRecord) (*string, error) {
	oldKeys, err := repository.SaveAssets(ctx, []designreview.AssetRecord{input})
	if err != nil || len(oldKeys) == 0 {
		return nil, err
	}
	return &oldKeys[0], nil
}

func (repository *Repository) SaveAssets(ctx context.Context, inputs []designreview.AssetRecord) ([]string, error) {
	if len(inputs) == 0 {
		return nil, designreview.ErrInvalid
	}
	workspaceID, reviewID := inputs[0].WorkspaceID, inputs[0].ReviewID
	for _, input := range inputs {
		if input.WorkspaceID != workspaceID || input.ReviewID != reviewID {
			return nil, designreview.ErrInvalid
		}
	}

	oldKeys := make([]string, 0, len(inputs))
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, input := range inputs {
			oldKey, err := repository.replaceAsset(tx, input)
			if err != nil {
				return err
			}
			if oldKey != "" {
				oldKeys = append(oldKeys, oldKey)
				if err := repository.enqueueObjectCleanup(tx, input.WorkspaceID, oldKey); err != nil {
					return err
				}
			}
		}
		return repository.refreshAssetDimensions(tx, workspaceID, reviewID)
	})
	if err != nil {
		return nil, err
	}
	return oldKeys, nil
}

func (repository *Repository) enqueueObjectCleanup(tx *gorm.DB, workspaceID uint, objectKey string) error {
	now := repository.now()
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "object_key"}}, DoNothing: true}).Create(&ObjectCleanupJob{
		ID: newID(), WorkspaceID: workspaceID, ObjectKey: objectKey,
		AvailableAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (repository *Repository) replaceAsset(tx *gorm.DB, input designreview.AssetRecord) (string, error) {
	var old Asset
	found := tx.Where("workspace_id = ? AND review_id = ? AND kind = ?", input.WorkspaceID, input.ReviewID, input.Kind).First(&old)
	if found.Error != nil && !errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return "", found.Error
	}
	if found.Error == nil {
		if err := tx.Delete(&old).Error; err != nil {
			return "", err
		}
	}
	row := Asset{ID: input.ID, WorkspaceID: input.WorkspaceID, ReviewID: input.ReviewID, Kind: input.Kind, ObjectKey: input.ObjectKey, ContentType: input.ContentType, ByteSize: input.ByteSize, SHA256: input.SHA256, Width: input.Width, Height: input.Height, CreatedAt: input.CreatedAt}
	if err := tx.Create(&row).Error; err != nil {
		return "", err
	}
	if err := repository.applyAssetMetadata(tx, input); err != nil {
		return "", err
	}
	if err := appendAudit(tx, input.WorkspaceID, input.ActorUserID, "design_review_asset", input.ID, "uploaded", 0); err != nil {
		return "", err
	}
	return old.ObjectKey, nil
}

func (repository *Repository) applyAssetMetadata(tx *gorm.DB, input designreview.AssetRecord) error {
	updates := map[string]any{}
	if input.Width != nil && input.Height != nil && (input.Kind == "design_normalized" || input.Kind == "design_original") {
		updates["design_width"], updates["design_height"] = *input.Width, *input.Height
	}
	if input.Kind == "design_structure" {
		updates["design_structure_available"] = true
	}
	if input.Kind == "implementation_structure" && input.StructureComplete != nil {
		updates["implementation_structure_complete"] = *input.StructureComplete
		if len(input.CaptureWarnings) > 0 {
			updates["capture_warnings"] = input.CaptureWarnings
		}
	}
	if input.Kind == "dom_normalization_manifest" && len(input.DOMNormalization) > 0 {
		updates["dom_normalization"] = input.DOMNormalization
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = repository.now()
	return tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", input.WorkspaceID, input.ReviewID).Updates(updates).Error
}

func (repository *Repository) refreshAssetDimensions(tx *gorm.DB, workspaceID uint, reviewID string) error {
	var design, implementation Asset
	designResult := tx.Where("workspace_id = ? AND review_id = ? AND kind IN ?", workspaceID, reviewID, []string{"design_normalized", "design_original"}).Order("CASE WHEN kind = 'design_normalized' THEN 0 ELSE 1 END").First(&design)
	implementationResult := tx.Where("workspace_id = ? AND review_id = ? AND kind IN ?", workspaceID, reviewID, []string{"implementation_normalized", "implementation_original"}).Order("CASE WHEN kind = 'implementation_normalized' THEN 0 ELSE 1 END").First(&implementation)
	if designResult.Error != nil || implementationResult.Error != nil || design.Width == nil || design.Height == nil || implementation.Width == nil || implementation.Height == nil {
		return nil
	}
	matches := *design.Width == *implementation.Width && *design.Height == *implementation.Height
	return tx.Model(&Review{}).Where("workspace_id = ? AND id = ?", workspaceID, reviewID).Updates(map[string]any{
		"design_width": *design.Width, "design_height": *design.Height,
		"dimensions_match": matches, "updated_at": repository.now(),
	}).Error
}
