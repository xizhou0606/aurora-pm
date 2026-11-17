package documentgorm

import (
	"context"
	"encoding/json"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type ToolMutation struct {
	ID          uint   `gorm:"primaryKey"`
	WorkspaceID uint   `gorm:"not null;uniqueIndex:ux_document_tool_mutation,priority:1"`
	ApplyKey    string `gorm:"size:128;not null;uniqueIndex:ux_document_tool_mutation,priority:2"`
	ActorUserID uint   `gorm:"not null"`
	InputDigest string `gorm:"size:64;not null"`
	ResultJSON  string `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

func (ToolMutation) TableName() string { return "pm_document_tool_mutations" }

func (repository *Repository) ApplyToolMutation(ctx context.Context, p document.Principal, key, digest string, apply func(document.Store, document.MutationRecorder) (json.RawMessage, error)) (json.RawMessage, error) {
	var output json.RawMessage
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		receipt := ToolMutation{WorkspaceID: p.WorkspaceID, ActorUserID: p.UserID, ApplyKey: key, InputDigest: digest, CreatedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt).Error; err != nil {
			return err
		}
		var stored ToolMutation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND apply_key = ?", p.WorkspaceID, key).First(&stored).Error; err != nil {
			return err
		}
		if stored.ActorUserID != p.UserID || stored.InputDigest != digest {
			return document.ErrInvalidInput
		}
		if stored.ResultJSON != "" {
			output = json.RawMessage(stored.ResultJSON)
			return nil
		}
		scoped := *repository
		scoped.db = tx
		var recorder document.MutationRecorder
		var err error
		if repository.mutationRecorderFactory != nil {
			recorder, err = repository.mutationRecorderFactory(tx)
			if err != nil {
				return err
			}
		}
		output, err = apply(&scoped, recorder)
		if err != nil {
			return err
		}
		if !json.Valid(output) {
			return document.ErrInvalidInput
		}
		return tx.Model(&stored).Update("result_json", string(output)).Error
	})
	return output, err
}
