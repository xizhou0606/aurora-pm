package requirementgorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ResourceLinkRecord struct {
	WorkspaceID   uint      `gorm:"primaryKey"`
	RequirementID uint      `gorm:"primaryKey"`
	ResourceID    string    `gorm:"primaryKey;size:64"`
	CreatedBy     uint      `gorm:"not null"`
	CreatedAt     time.Time `gorm:"not null"`
}

func (ResourceLinkRecord) TableName() string { return "requirement_resource_links" }

type ResourceLinkAccessFactory func(*gorm.DB) (requirement.ResourceLinkAccess, error)
type ResourceLinkRepository struct {
	db     *gorm.DB
	access ResourceLinkAccessFactory
}

func NewResourceLinkRepository(db *gorm.DB, f ResourceLinkAccessFactory) *ResourceLinkRepository {
	return &ResourceLinkRepository{db: db, access: f}
}

func (r *ResourceLinkRepository) HasResourceReferences(ctx context.Context, w uint, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ResourceLinkRecord{}).Where("workspace_id = ? AND resource_id = ?", w, id).Count(&count).Error
	return count > 0, err
}

func (r *ResourceLinkRepository) ReadResourceLinks(ctx context.Context, w, u, id uint, candidates bool) (requirement.ResourceLinkPage, error) {
	a, err := r.access(r.db.WithContext(ctx))
	if err != nil {
		return requirement.ResourceLinkPage{}, err
	}
	var row Requirement
	if err = r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", w, id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = requirement.ErrResourceLinkNotFound
		}
		return requirement.ResourceLinkPage{}, err
	}
	if err = a.RequireRequirement(ctx, w, u, row.ProjectSetID, candidates); err != nil {
		return requirement.ResourceLinkPage{}, err
	}
	page := requirement.ResourceLinkPage{Items: []requirement.ResourceLink{}, RequirementVersion: row.Version, ProjectSetID: row.ProjectSetID, ProductDocURL: row.ProductDocURL}
	if candidates {
		page.Items, err = a.Candidates(ctx, w, u)
		return page, err
	}
	var links []ResourceLinkRecord
	if err = r.db.WithContext(ctx).Where("workspace_id = ? AND requirement_id = ?", w, id).Order("created_at, resource_id").Find(&links).Error; err != nil {
		return page, err
	}
	for _, link := range links {
		value, resolveErr := a.Resource(ctx, w, u, link.ResourceID, false)
		if resolveErr == nil {
			page.Items = append(page.Items, value)
		} else if !errors.Is(resolveErr, requirement.ErrResourceLinkNotFound) {
			return page, resolveErr
		}
	}
	return page, nil
}

func (r *ResourceLinkRepository) ApplyResourceLink(ctx context.Context, c requirement.ResourceLinkCommand) (requirement.ResourceLinkPage, error) {
	raw, _ := json.Marshal(c)
	digest := sha256.Sum256(raw)
	inputDigest := hex.EncodeToString(digest[:])
	keyDigest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", c.WorkspaceID, c.UserID, c.OperationID)))
	key := "resource-link:" + hex.EncodeToString(keyDigest[:])
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		a, err := r.access(tx)
		if err != nil {
			return err
		}
		var row Requirement
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", c.WorkspaceID, c.RequirementID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = requirement.ErrResourceLinkNotFound
			}
			return err
		}
		if err = a.RequireRequirement(ctx, c.WorkspaceID, c.UserID, row.ProjectSetID, true); err != nil {
			return err
		}
		var receipt ToolMutationApply
		found := tx.Where("workspace_id = ? AND apply_key = ?", c.WorkspaceID, key).Limit(1).Find(&receipt)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected > 0 {
			if receipt.InputDigest != inputDigest {
				return requirement.ErrResourceLinkConflict
			}
			return nil
		}
		if row.Version != c.ExpectedVersion {
			return requirement.ErrResourceLinkConflict
		}
		var linkCount int64
		if err = tx.Model(&ResourceLinkRecord{}).Where("workspace_id = ? AND requirement_id = ?", c.WorkspaceID, c.RequirementID).Count(&linkCount).Error; err != nil {
			return err
		}
		// One requirement document source. Never silently discard legacy links.
		if (!c.Remove && linkCount != 0) || (c.ProductDocURL != nil && linkCount != 1) {
			return requirement.ErrResourceLinkConflict
		}
		if err = a.LockResource(ctx, c.WorkspaceID, c.ResourceID); err != nil {
			return err
		}
		if _, err = a.Resource(ctx, c.WorkspaceID, c.UserID, c.ResourceID, true); err != nil {
			return err
		}
		if c.Remove {
			result := tx.Where("workspace_id = ? AND requirement_id = ? AND resource_id = ?", c.WorkspaceID, c.RequirementID, c.ResourceID).Delete(&ResourceLinkRecord{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return requirement.ErrResourceLinkConflict
			}
		} else {
			record := ResourceLinkRecord{WorkspaceID: c.WorkspaceID, RequirementID: c.RequirementID, ResourceID: c.ResourceID, CreatedBy: c.UserID, CreatedAt: time.Now().UTC()}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return requirement.ErrResourceLinkConflict
			}
		}
		// The policy revision/event and link fact commit or roll back together.
		if err = a.ApplyResourcePolicy(ctx, c); err != nil {
			return err
		}
		updates := map[string]any{"version": row.Version + 1, "updated_at": time.Now().UTC()}
		if !c.Remove {
			updates["product_doc_url"] = ""
		} else if c.ProductDocURL != nil {
			updates["product_doc_url"] = *c.ProductDocURL
		}
		if err = tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		// Retain the scoped association command as the immutable mutation receipt,
		// including detach targets that no longer exist in the live relation table.
		return tx.Create(&ToolMutationApply{WorkspaceID: c.WorkspaceID, ApplyKey: key, InputDigest: inputDigest, Operation: "requirement.resource_link", ResultJSON: string(raw)}).Error
	})
	if err != nil {
		return requirement.ResourceLinkPage{}, err
	}
	return r.ReadResourceLinks(ctx, c.WorkspaceID, c.UserID, c.RequirementID, false)
}

// Resolve the CURRENT requirement project, never a copied project/member list.
func (r *ResourceLinkRepository) ResourceLinkProjectSets(ctx context.Context, w uint, ids []string) (map[string][]uint, error) {
	result := map[string][]uint{}
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		ResourceID   string
		ProjectSetID uint
	}
	err := r.db.WithContext(ctx).Table("requirement_resource_links AS l").Select("l.resource_id, r.project_new_id AS project_set_id").Joins("JOIN requirements r ON r.id = l.requirement_id AND r.workspace_id = l.workspace_id").Where("l.workspace_id = ? AND l.resource_id IN ?", w, ids).Distinct().Scan(&rows).Error
	for _, row := range rows {
		result[row.ResourceID] = append(result[row.ResourceID], row.ProjectSetID)
	}
	return result, err
}
