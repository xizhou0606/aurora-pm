package documentgorm

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

type Repository struct {
	db                       *gorm.DB
	mutationRecorderFactory  MutationRecorderFactory
	resourceLifecycleFactory ResourceLifecycleFactory
	newCollaborationFactory  NewDocumentCollaborationFactory
}

type NewDocumentCollaborationFactory func(*gorm.DB) (document.NewDocumentCollaborationInitializer, error)

func (repository *Repository) ConfigureNewDocumentCollaboration(factory NewDocumentCollaborationFactory) {
	repository.newCollaborationFactory = factory
}

type ResourceLifecycleFactory func(*gorm.DB) (document.ResourceLifecycle, error)

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) ConfigureResourceLifecycleFactory(factory ResourceLifecycleFactory) {
	repository.resourceLifecycleFactory = factory
}

func (repository *Repository) EnsureDefaultSpace(ctx context.Context, workspaceID uint) (document.Space, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return document.Space{}, document.ErrInvalidInput
	}
	row := Space{WorkspaceID: workspaceID, Name: document.DefaultSpaceName, IsDefault: true}
	err := repository.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	if err != nil {
		return document.Space{}, err
	}
	if err := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&row).Error; err != nil {
		return document.Space{}, err
	}
	return spaceFromRecord(row), nil
}

func (repository *Repository) ListNodes(ctx context.Context, workspaceID, spaceID uint) ([]document.Node, error) {
	var rows []Node
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ? AND archived_at IS NULL", workspaceID, spaceID).
		Order("COALESCE(parent_id, 0) ASC, sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return repository.hydrateNodeRecords(ctx, workspaceID, rows)
}

func (repository *Repository) GetNode(ctx context.Context, workspaceID, id uint) (document.Node, error) {
	var row Node
	err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ? AND archived_at IS NULL", workspaceID, id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return document.Node{}, document.ErrNodeNotFound
	}
	if err != nil {
		return document.Node{}, err
	}
	values, err := repository.hydrateNodeRecords(ctx, workspaceID, []Node{row})
	if err != nil {
		return document.Node{}, err
	}
	if len(values) != 1 {
		return document.Node{}, document.ErrNodeNotFound
	}
	return values[0], nil
}

func (repository *Repository) NextSortOrder(
	ctx context.Context,
	workspaceID uint,
	spaceID uint,
	parentID *uint,
) (int64, error) {
	query := repository.db.WithContext(ctx).Model(&Node{}).
		Where("workspace_id = ? AND space_id = ? AND archived_at IS NULL", workspaceID, spaceID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	var maximum *int64
	if err := query.Select("MAX(sort_order)").Scan(&maximum).Error; err != nil {
		return 0, err
	}
	if maximum == nil {
		return 1024, nil
	}
	return *maximum + 1024, nil
}

func (repository *Repository) CreateNode(
	ctx context.Context,
	node document.Node,
	body *document.Document,
) (document.Node, *document.Document, error) {
	return repository.createNode(ctx, node, body, false)
}

func (repository *Repository) CreateCollaborativeNode(ctx context.Context, node document.Node, body *document.Document) (document.Node, *document.Document, error) {
	if repository.newCollaborationFactory == nil || body == nil || node.Type != document.NodeDocument {
		return document.Node{}, nil, document.ErrContextActionsUnavailable
	}
	return repository.createNode(ctx, node, body, true)
}

func (repository *Repository) createNode(ctx context.Context, node document.Node, body *document.Document, collaborative bool) (document.Node, *document.Document, error) {
	var createdNode document.Node
	var createdBody *document.Document
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nodeRow := nodeRecord(node)
		if err := tx.Create(&nodeRow).Error; err != nil {
			return err
		}
		createdNode = nodeFromRecord(nodeRow)
		if body == nil {
			return nil
		}
		if body.OwnerUserID == 0 {
			body.OwnerUserID = body.CreatedBy
		}
		body.NodeID = nodeRow.ID
		bodyRow := documentRecord(*body)
		if err := tx.Create(&bodyRow).Error; err != nil {
			return err
		}
		created := documentFromRecord(bodyRow)
		createdBody = &created
		createdNode.DocumentID = uintPointer(bodyRow.ID)
		if repository.resourceLifecycleFactory != nil {
			lifecycle, err := repository.resourceLifecycleFactory(tx)
			if err != nil || lifecycle == nil {
				if err != nil {
					return err
				}
				return document.ErrContextActionsUnavailable
			}
			resourceID, err := lifecycle.CreatePrivateDocumentResource(
				ctx,
				document.Principal{WorkspaceID: bodyRow.WorkspaceID, UserID: bodyRow.CreatedBy},
				createdNode,
				created,
			)
			if err != nil {
				return err
			}
			createdNode.ResourceID, createdBody.ResourceID = resourceID, resourceID
		}
		if collaborative {
			initializer, err := repository.newCollaborationFactory(tx)
			if err != nil {
				return err
			}
			if initializer == nil {
				return document.ErrContextActionsUnavailable
			}
			createdBody.CollaborationCutoverAt, err = initializer.InitializeNewDocument(ctx, *createdBody)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return createdNode, createdBody, err
}

func (repository *Repository) UpdateNode(ctx context.Context, node document.Node) (document.Node, error) {
	var updated document.Node
	var resourceID string
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ? AND archived_at IS NULL", node.WorkspaceID, node.ID,
		).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return document.ErrNodeNotFound
			}
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{
			"parent_id": node.ParentID, "title": node.Title, "sort_order": node.SortOrder,
			"updated_by": node.UpdatedBy, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&row, "workspace_id = ? AND id = ?", node.WorkspaceID, node.ID).Error; err != nil {
			return err
		}
		updated = nodeFromRecord(row)
		if row.Type != string(document.NodeDocument) || repository.resourceLifecycleFactory == nil {
			return nil
		}
		var bodyRow Document
		if err := tx.Where("workspace_id = ? AND node_id = ? AND archived_at IS NULL", node.WorkspaceID, node.ID).
			First(&bodyRow).Error; err != nil {
			return normalizeDocumentContextError(err)
		}
		body := documentFromRecord(bodyRow)
		updated.DocumentID = uintPointer(body.ID)
		lifecycle, err := repository.resourceLifecycleFactory(tx)
		if err != nil || lifecycle == nil {
			if err != nil {
				return err
			}
			return document.ErrContextActionsUnavailable
		}
		syncedResourceID, err := lifecycle.SynchronizeDocumentResource(
			ctx, document.Principal{WorkspaceID: node.WorkspaceID, UserID: node.UpdatedBy}, updated, body,
		)
		if err != nil {
			return err
		}
		updated.ResourceID = syncedResourceID
		resourceID = syncedResourceID
		return nil
	})
	if err != nil {
		return document.Node{}, err
	}
	updated, err = repository.GetNode(ctx, node.WorkspaceID, node.ID)
	if err == nil {
		updated.ResourceID = resourceID
	}
	return updated, err
}

func (repository *Repository) ArchiveNodes(ctx context.Context, workspaceID uint, ids []uint, actorID uint) error {
	if len(ids) == 0 {
		return document.ErrInvalidInput
	}
	now := time.Now().UTC()
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Node{}).Where(
			"workspace_id = ? AND shortcut_target_node_id IN ? AND archived_at IS NULL", workspaceID, ids,
		).Updates(map[string]any{
			"archived_at": now, "archived_by": actorID, "updated_by": actorID, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		var documentIDs []uint
		if err := tx.Model(&Document{}).
			Where("workspace_id = ? AND node_id IN ? AND archived_at IS NULL", workspaceID, ids).
			Pluck("id", &documentIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&Node{}).Where("workspace_id = ? AND id IN ? AND archived_at IS NULL", workspaceID, ids).
			Updates(map[string]any{"archived_at": now, "archived_by": actorID, "updated_by": actorID, "updated_at": now}).Error; err != nil {
			return err
		}
		if len(documentIDs) == 0 {
			return nil
		}
		if repository.resourceLifecycleFactory != nil {
			var bodyRows []Document
			if err := tx.Where("workspace_id = ? AND id IN ? AND archived_at IS NULL", workspaceID, documentIDs).
				Find(&bodyRows).Error; err != nil {
				return err
			}
			bodies := make([]document.Document, 0, len(bodyRows))
			for _, bodyRow := range bodyRows {
				bodies = append(bodies, documentFromRecord(bodyRow))
			}
			lifecycle, err := repository.resourceLifecycleFactory(tx)
			if err != nil || lifecycle == nil {
				if err != nil {
					return err
				}
				return document.ErrContextActionsUnavailable
			}
			if err := lifecycle.ArchiveDocumentResources(
				ctx, document.Principal{WorkspaceID: workspaceID, UserID: actorID}, bodies,
			); err != nil {
				return err
			}
		}
		if err := tx.Model(&Document{}).Where("workspace_id = ? AND id IN ? AND archived_at IS NULL", workspaceID, documentIDs).
			Updates(map[string]any{"archived_at": now, "archived_by": actorID, "updated_by": actorID, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Binding{}).Where("workspace_id = ? AND document_id IN ? AND archived_at IS NULL", workspaceID, documentIDs).
			Updates(map[string]any{"archived_at": now, "archived_by": actorID}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND document_id IN ?", workspaceID, documentIDs).
			Delete(&AssetReference{}).Error; err != nil {
			return err
		}
		return tx.Model(&Asset{}).Where("workspace_id = ? AND document_id IN ? AND archived_at IS NULL", workspaceID, documentIDs).
			Updates(map[string]any{"archived_at": now, "archived_by": actorID, "updated_at": now}).Error
	})
}

func duplicateBindingError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique") || strings.Contains(message, "duplicate") {
		return document.ErrBindingConflict
	}
	return err
}

func (repository *Repository) ListAssets(ctx context.Context, workspaceID, documentID uint) ([]document.Asset, error) {
	var rows []Asset
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND document_id = ? AND archived_at IS NULL", workspaceID, documentID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]document.Asset, 0, len(rows))
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		result = append(result, assetFromRecord(row))
		seen[row.ID] = struct{}{}
	}
	var referencedRows []Asset
	if err := repository.db.WithContext(ctx).Table("document_assets AS assets").
		Select("assets.*").Joins(
		"JOIN document_asset_references AS refs ON refs.asset_id = assets.id AND refs.workspace_id = assets.workspace_id",
	).Where("refs.workspace_id = ? AND refs.document_id = ?", workspaceID, documentID).
		Order("assets.id ASC").Scan(&referencedRows).Error; err != nil {
		return nil, err
	}
	for _, row := range referencedRows {
		if _, exists := seen[row.ID]; exists {
			continue
		}
		asset := assetFromRecord(row)
		asset.IsReference = true
		result = append(result, asset)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

func (repository *Repository) GetAsset(ctx context.Context, workspaceID, documentID, assetID uint) (document.Asset, error) {
	var row Asset
	result := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND document_id = ? AND id = ? AND archived_at IS NULL", workspaceID, documentID, assetID).
		First(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		var referenced Asset
		referenceResult := repository.db.WithContext(ctx).Table("document_assets AS assets").
			Select("assets.*").Joins(
			"JOIN document_asset_references AS refs ON refs.asset_id = assets.id AND refs.workspace_id = assets.workspace_id",
		).Where(
			"refs.workspace_id = ? AND refs.document_id = ? AND refs.asset_id = ?",
			workspaceID, documentID, assetID,
		).First(&referenced)
		if errors.Is(referenceResult.Error, gorm.ErrRecordNotFound) {
			return document.Asset{}, document.ErrAssetNotFound
		}
		asset := assetFromRecord(referenced)
		asset.IsReference = true
		return asset, referenceResult.Error
	}
	asset := assetFromRecord(row)
	if result.Error == nil {
		var references int64
		if err := repository.db.WithContext(ctx).Model(&AssetReference{}).Where(
			"workspace_id = ? AND asset_id = ?", workspaceID, assetID,
		).Count(&references).Error; err != nil {
			return document.Asset{}, err
		}
		asset.RetainObject = references > 0
	}
	return asset, result.Error
}

// LockCollaborationAssetReferences locks every active asset fact used by a
// candidate collaborative projection. Reference rows are locked first so
// removing a cross-document reference cannot race the durable commit; asset
// rows are then locked in ID order so direct-asset archive cannot race it.
func (repository *Repository) LockCollaborationAssetReferences(
	ctx context.Context,
	workspaceID, documentID uint,
	assetIDs []uint,
) error {
	if repository == nil || repository.db == nil || workspaceID == 0 || documentID == 0 {
		return document.ErrInvalidInput
	}
	if len(assetIDs) == 0 {
		return nil
	}
	ids := append([]uint(nil), assetIDs...)
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	unique := ids[:0]
	for _, assetID := range ids {
		if assetID == 0 {
			return document.ErrInvalidInput
		}
		if len(unique) == 0 || unique[len(unique)-1] != assetID {
			unique = append(unique, assetID)
		}
	}

	var references []AssetReference
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND document_id = ? AND asset_id IN ?", workspaceID, documentID, unique,
	).Order("asset_id ASC").Find(&references).Error; err != nil {
		return err
	}
	referenced := make(map[uint]struct{}, len(references))
	for _, reference := range references {
		referenced[reference.AssetID] = struct{}{}
	}

	var assets []Asset
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id IN ? AND archived_at IS NULL", workspaceID, unique,
	).Order("id ASC").Find(&assets).Error; err != nil {
		return err
	}
	owned := make(map[uint]struct{}, len(assets))
	for _, asset := range assets {
		if asset.DocumentID == documentID {
			owned[asset.ID] = struct{}{}
			continue
		}
		if _, ok := referenced[asset.ID]; ok {
			owned[asset.ID] = struct{}{}
		}
	}
	if len(owned) != len(unique) {
		return document.ErrAssetNotFound
	}
	return nil
}

func (repository *Repository) CreateAsset(ctx context.Context, value document.Asset) (document.Asset, error) {
	row := assetRecord(value)
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return document.Asset{}, err
	}
	return assetFromRecord(row), nil
}

func (repository *Repository) ArchiveAsset(ctx context.Context, workspaceID, documentID, assetID, actorID uint) error {
	now := time.Now().UTC()
	referenceResult := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND document_id = ? AND asset_id = ?", workspaceID, documentID, assetID,
	).Delete(&AssetReference{})
	if referenceResult.Error != nil {
		return referenceResult.Error
	}
	if referenceResult.RowsAffected == 1 {
		return nil
	}
	result := repository.db.WithContext(ctx).Model(&Asset{}).
		Where("workspace_id = ? AND document_id = ? AND id = ? AND archived_at IS NULL", workspaceID, documentID, assetID).
		Updates(map[string]any{"archived_at": now, "archived_by": actorID, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return document.ErrAssetNotFound
	}
	return nil
}

var _ document.Store = (*Repository)(nil)

func uintPointer(value uint) *uint { return &value }

func rawJSON(value string) json.RawMessage { return json.RawMessage(value) }
