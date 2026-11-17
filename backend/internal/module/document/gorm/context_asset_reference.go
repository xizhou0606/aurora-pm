package documentgorm

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

func copyAssetReferences(tx *gorm.DB, workspaceID, sourceDocumentID, targetDocumentID, actorID uint) error {
	var assets []Asset
	if err := tx.Where("workspace_id = ? AND document_id = ? AND archived_at IS NULL", workspaceID, sourceDocumentID).Find(&assets).Error; err != nil {
		return err
	}
	assetIDs := make([]uint, 0, len(assets))
	for _, asset := range assets {
		assetIDs = append(assetIDs, asset.ID)
	}
	var inherited []AssetReference
	if err := tx.Where("workspace_id = ? AND document_id = ?", workspaceID, sourceDocumentID).Find(&inherited).Error; err != nil {
		return err
	}
	for _, reference := range inherited {
		assetIDs = append(assetIDs, reference.AssetID)
	}
	sort.Slice(assetIDs, func(i, j int) bool { return assetIDs[i] < assetIDs[j] })
	now := time.Now().UTC()
	last := uint(0)
	for _, assetID := range assetIDs {
		if assetID == 0 || assetID == last {
			continue
		}
		last = assetID
		if err := tx.Create(&AssetReference{
			WorkspaceID: workspaceID, DocumentID: targetDocumentID,
			AssetID: assetID, CreatedBy: actorID, CreatedAt: now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
