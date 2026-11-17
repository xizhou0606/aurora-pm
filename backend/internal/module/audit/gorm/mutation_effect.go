package auditgorm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"gorm.io/gorm"
)

type MutationEffectRepository struct{ db *gorm.DB }

func NewMutationEffectRepository(db *gorm.DB) *MutationEffectRepository {
	return &MutationEffectRepository{db: db}
}

func (repository *MutationEffectRepository) RecordMutationEffect(
	ctx context.Context,
	effect audit.MutationEffect,
) error {
	if repository == nil || repository.db == nil {
		return audit.ErrMutationEffectInvalid
	}
	payload, err := json.Marshal(effect.Payload)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	log := AuditLog{
		WorkspaceID: effect.WorkspaceID, UserID: effect.PrincipalID, StatusCode: 200,
		Resource: effect.Resource, ResourceID: effect.ResourceID, ResourceRef: effect.ResourceRef, ProjectID: effect.ProjectSetID,
		Action: effect.Action, Target: effect.Target, CreatedAt: now,
	}
	if err := repository.db.WithContext(ctx).Create(&log).Error; err != nil {
		return err
	}
	return repository.db.WithContext(ctx).Create(&OutboxEvent{
		WorkspaceID: effect.WorkspaceID, AggregateType: effect.Resource, AggregateID: effect.ResourceID, AggregateRef: effect.ResourceRef,
		EventType: effect.EventType, Payload: string(payload), OccurredAt: now,
	}).Error
}

var _ audit.MutationEffectStore = (*MutationEffectRepository)(nil)
