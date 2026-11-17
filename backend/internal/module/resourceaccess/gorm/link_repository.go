package resourceaccessgorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) CreateShareLink(
	ctx context.Context, mutation resourceaccess.LinkMutation,
) (resourceaccess.ShareLink, error) {
	var result resourceaccess.ShareLink
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireLinkMode(tx, mutation.Principal.WorkspaceID, mutation.ResourceID); err != nil {
			return err
		}
		if err := rejectReplayedLinkCreation(tx, mutation); err != nil {
			return err
		}
		var existing []ShareLinkRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ? AND revoked_at IS NULL", mutation.Principal.WorkspaceID, mutation.ResourceID).Limit(2).Find(&existing).Error; err != nil {
			return err
		}
		// Historical duplicates must be explicitly revoked by the owner, never silently invalidated.
		if len(existing) > 1 {
			return resourceaccess.ErrConflict
		}
		row := ShareLinkRecord{
			ID: mutation.LinkID, WorkspaceID: mutation.Principal.WorkspaceID, ResourceID: string(mutation.ResourceID),
			SecretHash: mutation.SecretHash, Level: string(mutation.Level), Version: 1,
			ExpiresAt: mutation.ExpiresAt, CreatedBy: mutation.Principal.UserID, CreatedAt: mutation.Now, UpdatedAt: mutation.Now,
		}
		var before any
		action := "share_link.created"
		if len(existing) == 1 {
			row = existing[0]
			before = shareLinkFromRecord(row)
			action = "share_link.updated"
			if err := tx.Model(&row).Updates(map[string]any{"level": string(mutation.Level), "expires_at": mutation.ExpiresAt, "updated_at": mutation.Now}).Error; err != nil {
				return err
			}
			row.Level, row.ExpiresAt, row.UpdatedAt = string(mutation.Level), mutation.ExpiresAt, mutation.Now
		} else if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result = shareLinkFromRecord(row)
		if err := appendAuditAndOutbox(tx, mutation.Principal, mutation.ResourceID, action, before, result, mutation.Now); err != nil {
			return err
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Create(&MutationReceiptRecord{
			WorkspaceID: mutation.Principal.WorkspaceID, ResourceID: string(mutation.ResourceID),
			Operation: "share_link.create", IdempotencyKey: mutation.IdempotencyKey,
			InputDigest: mutation.InputDigest, ResultJSON: string(encoded), CreatedAt: mutation.Now,
		}).Error
	})
	return result, err
}

func rejectReplayedLinkCreation(tx *gorm.DB, mutation resourceaccess.LinkMutation) error {
	var receipt MutationReceiptRecord
	err := tx.Where(
		"workspace_id = ? AND resource_id = ? AND operation = ? AND idempotency_key = ?",
		mutation.Principal.WorkspaceID, mutation.ResourceID, "share_link.create", mutation.IdempotencyKey,
	).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// A link secret is intentionally never persisted. A replay therefore cannot
	// safely reproduce the original success body, but it must never mint another
	// link or a second secret for the same key.
	return resourceaccess.ErrIdempotencyConflict
}

func (repository *Repository) ListShareLinks(
	ctx context.Context, workspaceID uint, resourceID documentlibrary.ResourceID,
) ([]resourceaccess.ShareLink, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || resourceID == "" {
		return nil, resourceaccess.ErrInvalidInput
	}
	var rows []ShareLinkRecord
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND resource_id = ?", workspaceID, resourceID).
		Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]resourceaccess.ShareLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, shareLinkFromRecord(row))
	}
	return result, nil
}

func (repository *Repository) RevokeShareLink(ctx context.Context, mutation resourceaccess.LinkMutation) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ShareLinkRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND workspace_id = ? AND resource_id = ?", mutation.LinkID, mutation.Principal.WorkspaceID, mutation.ResourceID,
		).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.RevokedAt != nil {
			return nil
		}
		before := shareLinkFromRecord(row)
		if err := tx.Model(&row).Updates(map[string]any{
			"revoked_at": mutation.Now, "updated_at": mutation.Now,
		}).Error; err != nil {
			return err
		}
		row.RevokedAt, row.UpdatedAt = &mutation.Now, mutation.Now
		return appendAuditAndOutbox(
			tx, mutation.Principal, mutation.ResourceID, "share_link.revoked", before, shareLinkFromRecord(row), mutation.Now,
		)
	})
}

func (repository *Repository) RotateShareLink(
	ctx context.Context, mutation resourceaccess.LinkMutation,
) (resourceaccess.ShareLink, error) {
	var result resourceaccess.ShareLink
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireLinkMode(tx, mutation.Principal.WorkspaceID, mutation.ResourceID); err != nil {
			return err
		}
		var row ShareLinkRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND workspace_id = ? AND resource_id = ?", mutation.LinkID, mutation.Principal.WorkspaceID, mutation.ResourceID,
		).First(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.RevokedAt != nil || (row.ExpiresAt != nil && !row.ExpiresAt.After(mutation.Now)) {
			return resourceaccess.ErrLinkInvalid
		}
		before := shareLinkFromRecord(row)
		nextVersion := row.Version + 1
		if err := tx.Model(&row).Updates(map[string]any{
			"secret_hash": mutation.SecretHash, "version": nextVersion, "updated_at": mutation.Now,
		}).Error; err != nil {
			return err
		}
		row.SecretHash, row.Version, row.UpdatedAt = mutation.SecretHash, nextVersion, mutation.Now
		result = shareLinkFromRecord(row)
		return appendAuditAndOutbox(tx, mutation.Principal, mutation.ResourceID, "share_link.rotated", before, result, mutation.Now)
	})
	return result, err
}

func (repository *Repository) FindShareLinkBySecretHash(
	ctx context.Context, secretHash string,
) (uint, resourceaccess.ShareLink, error) {
	if repository == nil || repository.db == nil || secretHash == "" {
		return 0, resourceaccess.ShareLink{}, resourceaccess.ErrLinkInvalid
	}
	var row ShareLinkRecord
	if err := repository.db.WithContext(ctx).Where("secret_hash = ?", secretHash).First(&row).Error; err != nil {
		return 0, resourceaccess.ShareLink{}, mapLinkNotFound(err)
	}
	return row.WorkspaceID, shareLinkFromRecord(row), nil
}

func (repository *Repository) CreateLinkAccessContext(
	ctx context.Context,
	principal resourceaccess.Principal,
	link resourceaccess.ShareLink,
	expectedLinkSecretHash string,
	tokenHash string,
	now, expiresAt time.Time,
) error {
	if repository == nil || repository.db == nil || expectedLinkSecretHash == "" || tokenHash == "" || !expiresAt.After(now) {
		return resourceaccess.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var live ShareLinkRecord
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where(
			"id = ? AND workspace_id = ? AND resource_id = ? AND version = ? AND secret_hash = ? AND revoked_at IS NULL",
			link.ID, principal.WorkspaceID, link.ResourceID, link.Version, expectedLinkSecretHash,
		).First(&live).Error; err != nil {
			return mapLinkNotFound(err)
		}
		if live.ExpiresAt != nil && !live.ExpiresAt.After(now) {
			return resourceaccess.ErrLinkInvalid
		}
		if err := requireLinkMode(tx, principal.WorkspaceID, link.ResourceID); err != nil {
			return resourceaccess.ErrLinkInvalid
		}
		return tx.Create(&LinkAccessContextRecord{
			WorkspaceID: principal.WorkspaceID, ResourceID: string(link.ResourceID), LinkID: link.ID,
			LinkVersion: link.Version, UserID: principal.UserID, TokenHash: tokenHash,
			ExpiresAt: expiresAt, CreatedAt: now,
		}).Error
	})
}

func (repository *Repository) ResolveLinkAccess(
	ctx context.Context,
	principal resourceaccess.Principal,
	resourceIDs []documentlibrary.ResourceID,
	tokenHash string,
	now time.Time,
) (map[documentlibrary.ResourceID]resourceaccess.LinkAccessDecision, error) {
	result := make(map[documentlibrary.ResourceID]resourceaccess.LinkAccessDecision)
	if repository == nil || repository.db == nil || principal.UserID == 0 || principal.WorkspaceID == 0 || len(resourceIDs) == 0 {
		return result, nil
	}
	type linkLevelRow struct {
		ResourceID      string
		Level           string
		AccessExpiresAt time.Time  `gorm:"column:access_expires_at"`
		LinkExpiresAt   *time.Time `gorm:"column:link_expires_at"`
	}
	var rows []linkLevelRow
	query := repository.db.WithContext(ctx).Table("document_resource_link_access_contexts lac").
		Select("lac.resource_id, l.level, lac.expires_at AS access_expires_at, l.expires_at AS link_expires_at").
		Joins("JOIN document_resource_share_links l ON l.id = lac.link_id AND l.workspace_id = lac.workspace_id AND l.resource_id = lac.resource_id AND l.version = lac.link_version").
		Joins("JOIN document_resource_access_policies p ON p.workspace_id = lac.workspace_id AND p.resource_id = lac.resource_id AND p.mode IN ?", []resourceaccess.Mode{resourceaccess.ModePrivate, resourceaccess.ModeRestricted, resourceaccess.ModeWorkspace, resourceaccess.ModeLink}).
		Where("lac.workspace_id = ? AND lac.user_id = ? AND lac.resource_id IN ?", principal.WorkspaceID, principal.UserID, stringResourceIDs(resourceIDs)).
		Where("l.revoked_at IS NULL AND (l.expires_at IS NULL OR l.expires_at > ?)", now)
	// A successful exchange is an account-bound grant for the live link's
	// lifetime. Refresh/navigation no longer depend on a browser memory token.
	if tokenHash != "" {
		query = query.Where("lac.token_hash = ? AND lac.expires_at > ?", tokenHash, now)
	}
	err := query.Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		expiresAt := row.AccessExpiresAt
		if tokenHash == "" {
			expiresAt = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
		}
		if row.LinkExpiresAt != nil && row.LinkExpiresAt.Before(expiresAt) {
			expiresAt = *row.LinkExpiresAt
		}
		id := documentlibrary.ResourceID(row.ResourceID)
		current := result[id]
		if (current.Level == resourceaccess.LevelEditor && resourceaccess.Level(row.Level) == resourceaccess.LevelViewer) || (current.Level == resourceaccess.Level(row.Level) && current.ExpiresAt.After(expiresAt)) {
			continue
		}
		result[id] = resourceaccess.LinkAccessDecision{
			Level: resourceaccess.Level(row.Level), ExpiresAt: expiresAt,
		}
	}
	return result, nil
}

func requireLinkMode(tx *gorm.DB, workspaceID uint, resourceID documentlibrary.ResourceID) error {
	var row PolicyRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"workspace_id = ? AND resource_id = ? AND mode IN ?", workspaceID, resourceID, []resourceaccess.Mode{resourceaccess.ModePrivate, resourceaccess.ModeRestricted, resourceaccess.ModeWorkspace, resourceaccess.ModeLink},
	).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return resourceaccess.ErrConflict
		}
		return err
	}
	return nil
}

func shareLinkFromRecord(row ShareLinkRecord) resourceaccess.ShareLink {
	return resourceaccess.ShareLink{
		ID: row.ID, ResourceID: documentlibrary.ResourceID(row.ResourceID), Level: resourceaccess.Level(row.Level),
		Version: row.Version, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapLinkNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return resourceaccess.ErrLinkInvalid
	}
	return err
}

var _ resourceaccess.LinkStore = (*Repository)(nil)
