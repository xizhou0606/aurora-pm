package requirementcover

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
	StatusDisabled   = "disabled"
)

func SourceHash(rawURL string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(rawURL)))
	return hex.EncodeToString(digest[:])
}

func EligibleURL(rawURL string, cfg config.Config) (string, bool) {
	if !cfg.Feishu.Enabled || !cfg.ObjectStorage.Enabled {
		return "", false
	}
	normalized, err := feishu.NormalizeDocumentURL(rawURL, cfg.Feishu.DomainAllowlist)
	return normalized, err == nil
}

func generationEnabled(cfg config.Config) bool {
	return cfg.RequirementCoverEnabled && cfg.Feishu.Enabled && cfg.ObjectStorage.Enabled
}

// EnqueueTx records work inside the same transaction as the requirement write.
// Calling it for an unchanged source is a no-op.
func EnqueueTx(tx *gorm.DB, cfg config.Config, requirement requirementgorm.Requirement) error {
	// The API has no renderer credential. Keep existing covers intact when the
	// worker feature is off instead of creating work that cannot be completed.
	if !generationEnabled(cfg) {
		return nil
	}
	normalized, eligible := EligibleURL(requirement.ProductDocURL, cfg)
	var existing requirementgorm.RequirementCover
	err := tx.Where(
		"workspace_id = ? AND requirement_id = ?",
		requirement.WorkspaceID, requirement.ID,
	).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if !eligible {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return resetRow(tx, existing, "")
	}
	hash := SourceHash(normalized)
	if err == nil && existing.SourceHash == hash {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&requirementgorm.RequirementCover{
			WorkspaceID: requirement.WorkspaceID, RequirementID: requirement.ID,
			SourceHash: hash, Generation: 1, Status: StatusPending,
			NextAttemptAt: time.Now().UTC(),
		}).Error
	}
	return resetRow(tx, existing, hash)
}

func MarkDeletedTx(tx *gorm.DB, workspaceID uint, requirementID uint) error {
	return requirementgorm.MarkCoverDeletedTx(tx, workspaceID, requirementID)
}

func resetRow(tx *gorm.DB, existing requirementgorm.RequirementCover, sourceHash string) error {
	return tx.Model(&requirementgorm.RequirementCover{}).
		Where("id = ? AND generation = ?", existing.ID, existing.Generation).
		Updates(map[string]any{
			"source_hash": sourceHash, "generation": existing.Generation + 1,
			"status": StatusPending, "attempts": 0, "next_attempt_at": time.Now().UTC(),
			"claimed_at": nil, "claimed_by": "", "last_error": "", "generated_at": nil,
		}).Error
}
