package resourceaccessgorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ApplyToolMutation(ctx context.Context, p resourceaccess.Principal, id documentlibrary.ResourceID, key, digest string, apply func(resourceaccess.PolicyStore, resourceaccess.RequestStore, resourceaccess.LinkStore) (json.RawMessage, error)) (json.RawMessage, error) {
	// Actor identity is bound to the receipt without extending the existing schema.
	bound := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", p.UserID, digest)))
	digest = hex.EncodeToString(bound[:])
	var output json.RawMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt := MutationReceiptRecord{WorkspaceID: p.WorkspaceID, ResourceID: string(id), Operation: "tool.apply", IdempotencyKey: key, InputDigest: digest, CreatedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt).Error; err != nil {
			return err
		}
		var stored MutationReceiptRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND resource_id = ? AND operation = ? AND idempotency_key = ?", p.WorkspaceID, id, "tool.apply", key).First(&stored).Error; err != nil {
			return err
		}
		if stored.InputDigest != digest {
			return resourceaccess.ErrIdempotencyConflict
		}
		if stored.ResultJSON != "" {
			output = json.RawMessage(stored.ResultJSON)
			return nil
		}
		scoped := *r
		scoped.db = tx
		var err error
		output, err = apply(&scoped, &scoped, &scoped)
		if err != nil {
			return err
		}
		if !json.Valid(output) {
			return resourceaccess.ErrInvalidInput
		}
		return tx.Model(&stored).Update("result_json", string(output)).Error
	})
	return output, err
}
