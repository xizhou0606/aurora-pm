package documentlibrarygorm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReferenceCheckerFactory func(*gorm.DB) documentlibrary.ReferenceChecker
type Repository struct {
	db               *gorm.DB
	referenceChecker ReferenceCheckerFactory
}

func (r *Repository) ConfigureReferenceChecker(factory ReferenceCheckerFactory) {
	r.referenceChecker = factory
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return documentlibrary.ErrNotFound
	}
	return err
}

func toResource(record ResourceRecord) documentlibrary.Resource {
	return documentlibrary.Resource{
		ID: documentlibrary.ResourceID(record.ID), WorkspaceID: record.WorkspaceID,
		TypeKey: documentlibrary.TypeKey(record.TypeKey), Name: record.Name, ParentRef: record.ParentRef,
		OwnerUserID: record.OwnerUserID, CreatedBy: record.CreatedBy, Revision: record.Revision,
		Lifecycle: record.Lifecycle, ArchivedAt: record.ArchivedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

func toBinding(record ContentBindingRecord) documentlibrary.ContentBinding {
	return documentlibrary.ContentBinding{
		WorkspaceID: record.WorkspaceID, ResourceID: documentlibrary.ResourceID(record.ResourceID),
		TypeKey: documentlibrary.TypeKey(record.TypeKey), PayloadRef: record.PayloadRef,
	}
}

func (repository *Repository) Get(
	ctx context.Context,
	workspaceID uint,
	resourceID documentlibrary.ResourceID,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	var resource ResourceRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, string(resourceID)).
		First(&resource).Error; err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, mapError(err)
	}
	var binding ContentBindingRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND resource_id = ?", workspaceID, string(resourceID)).
		First(&binding).Error; err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, mapError(err)
	}
	return toResource(resource), toBinding(binding), nil
}

// LockActiveResource resolves one resource while holding its lifecycle row
// lock in the caller-owned transaction. It is intentionally a concrete GORM
// adapter seam: callers that authorize a later write can keep archive/delete
// transitions from racing the write without widening the domain Store API.
func (repository *Repository) LockActiveResource(
	ctx context.Context,
	workspaceID uint,
	resourceID documentlibrary.ResourceID,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || strings.TrimSpace(string(resourceID)) == "" {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, documentlibrary.ErrInvalidInput
	}
	var resource ResourceRecord
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND id = ? AND lifecycle = ? AND archived_at IS NULL",
		workspaceID, string(resourceID), "active",
	).First(&resource).Error; err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, mapError(err)
	}
	var binding ContentBindingRecord
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND resource_id = ?", workspaceID, string(resourceID),
	).First(&binding).Error; err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, mapError(err)
	}
	return toResource(resource), toBinding(binding), nil
}

func (repository *Repository) ResolveContent(
	ctx context.Context,
	workspaceID uint,
	typeKey documentlibrary.TypeKey,
	payloadRef string,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	var binding ContentBindingRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND type_key = ? AND payload_ref = ?", workspaceID, string(typeKey), payloadRef).
		First(&binding).Error; err != nil {
		return documentlibrary.Resource{}, documentlibrary.ContentBinding{}, mapError(err)
	}
	resource, _, err := repository.Get(ctx, workspaceID, documentlibrary.ResourceID(binding.ResourceID))
	return resource, toBinding(binding), err
}

func (repository *Repository) ResolveContents(
	ctx context.Context,
	workspaceID uint,
	typeKey documentlibrary.TypeKey,
	payloadRefs []string,
) (map[string]documentlibrary.Resource, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || typeKey == "" || len(payloadRefs) == 0 {
		return nil, documentlibrary.ErrInvalidInput
	}
	var bindings []ContentBindingRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND type_key = ? AND payload_ref IN ?", workspaceID, string(typeKey), payloadRefs).
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return map[string]documentlibrary.Resource{}, nil
	}
	resourceIDs := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		resourceIDs = append(resourceIDs, binding.ResourceID)
	}
	var records []ResourceRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND type_key = ? AND id IN ?", workspaceID, string(typeKey), resourceIDs).
		Find(&records).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]documentlibrary.Resource, len(records))
	for _, record := range records {
		byID[record.ID] = toResource(record)
	}
	result := make(map[string]documentlibrary.Resource, len(bindings))
	for _, binding := range bindings {
		resource, ok := byID[binding.ResourceID]
		if !ok {
			return nil, documentlibrary.ErrConflict
		}
		result[binding.PayloadRef] = resource
	}
	return result, nil
}

func (repository *Repository) List(ctx context.Context, workspaceID uint) ([]documentlibrary.Resource, error) {
	var records []ResourceRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND lifecycle = ? AND archived_at IS NULL", workspaceID, "active").
		Order("updated_at DESC, id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	resources := make([]documentlibrary.Resource, 0, len(records))
	for _, record := range records {
		resources = append(resources, toResource(record))
	}
	return resources, nil
}

func (repository *Repository) Create(
	ctx context.Context,
	principal documentlibrary.Principal,
	typeKey documentlibrary.TypeKey,
	payloadRef, name string,
	parentRef *string,
) (documentlibrary.Resource, documentlibrary.ContentBinding, error) {
	var resource documentlibrary.Resource
	var binding documentlibrary.ContentBinding
	if principal.UserID == 0 || principal.WorkspaceID == 0 || strings.TrimSpace(payloadRef) == "" || strings.TrimSpace(name) == "" {
		return resource, binding, documentlibrary.ErrInvalidInput
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		resourceRecord := ResourceRecord{
			ID: uuid.NewString(), WorkspaceID: principal.WorkspaceID, TypeKey: string(typeKey),
			Name: strings.TrimSpace(name), ParentRef: parentRef, OwnerUserID: principal.UserID,
			CreatedBy: principal.UserID, Revision: 1, Lifecycle: "active", CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&resourceRecord).Error; err != nil {
			return err
		}
		bindingRecord := ContentBindingRecord{
			WorkspaceID: principal.WorkspaceID, ResourceID: resourceRecord.ID,
			TypeKey: string(typeKey), PayloadRef: payloadRef, CreatedAt: now,
		}
		if err := tx.Create(&bindingRecord).Error; err != nil {
			return err
		}
		resource, binding = toResource(resourceRecord), toBinding(bindingRecord)
		return nil
	})
	return resource, binding, err
}

func (repository *Repository) UpdateMetadata(
	ctx context.Context,
	principal documentlibrary.Principal,
	resourceID documentlibrary.ResourceID,
	expectedRevision uint64,
	name string,
	parentRef *string,
	ownerUserID *uint,
) (documentlibrary.Resource, error) {
	var result documentlibrary.Resource
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record ResourceRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND id = ? AND lifecycle = ?", principal.WorkspaceID, string(resourceID), "active").
			First(&record).Error; err != nil {
			return mapError(err)
		}
		if expectedRevision == 0 || record.Revision != expectedRevision {
			return documentlibrary.ErrConflict
		}
		updates := map[string]any{"revision": record.Revision + 1, "updated_at": time.Now().UTC()}
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			updates["name"] = trimmed
		}
		if parentRef != nil {
			updates["parent_ref"] = parentRef
		}
		if ownerUserID != nil {
			if *ownerUserID == 0 {
				return documentlibrary.ErrInvalidInput
			}
			updates["owner_user_id"] = *ownerUserID
		}
		if err := tx.Model(&record).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&record, "workspace_id = ? AND id = ?", principal.WorkspaceID, string(resourceID)).Error; err != nil {
			return mapError(err)
		}
		result = toResource(record)
		return nil
	})
	return result, err
}

func (repository *Repository) ReplaceMetadata(
	ctx context.Context,
	principal documentlibrary.Principal,
	resourceID documentlibrary.ResourceID,
	expectedRevision uint64,
	name string,
	parentRef *string,
	ownerUserID uint,
) (documentlibrary.Resource, error) {
	var result documentlibrary.Resource
	if repository == nil || repository.db == nil || principal.UserID == 0 || principal.WorkspaceID == 0 ||
		expectedRevision == 0 || strings.TrimSpace(name) == "" || ownerUserID == 0 {
		return result, documentlibrary.ErrInvalidInput
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record ResourceRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ? AND lifecycle = ? AND archived_at IS NULL",
			principal.WorkspaceID, string(resourceID), "active",
		).First(&record).Error; err != nil {
			return mapError(err)
		}
		if record.Revision != expectedRevision {
			return documentlibrary.ErrConflict
		}
		if err := tx.Model(&record).Updates(map[string]any{
			"name": strings.TrimSpace(name), "parent_ref": parentRef, "owner_user_id": ownerUserID,
			"revision": record.Revision + 1, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.First(&record, "workspace_id = ? AND id = ?", principal.WorkspaceID, string(resourceID)).Error; err != nil {
			return mapError(err)
		}
		result = toResource(record)
		return nil
	})
	return result, err
}

func (repository *Repository) Archive(
	ctx context.Context,
	principal documentlibrary.Principal,
	resourceID documentlibrary.ResourceID,
	expectedRevision uint64,
) error {
	if repository == nil || repository.db == nil || principal.UserID == 0 || principal.WorkspaceID == 0 || expectedRevision == 0 {
		return documentlibrary.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record ResourceRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"workspace_id = ? AND id = ? AND lifecycle = ? AND archived_at IS NULL",
			principal.WorkspaceID, string(resourceID), "active",
		).First(&record).Error; err != nil {
			return mapError(err)
		}
		if record.Revision != expectedRevision {
			return documentlibrary.ErrConflict
		}
		if repository.referenceChecker != nil {
			referenced, err := repository.referenceChecker(tx).HasResourceReferences(ctx, principal.WorkspaceID, string(resourceID))
			if err != nil {
				return err
			}
			if referenced {
				return documentlibrary.ErrConflict
			}
		}
		now := time.Now().UTC()
		result := tx.Model(&record).Updates(map[string]any{
			"lifecycle": "archived", "archived_at": now, "revision": record.Revision + 1, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return documentlibrary.ErrConflict
		}
		return nil
	})
}
