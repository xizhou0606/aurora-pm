package collaborationgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxReplayLimit = 500

func (repository *Repository) Replay(
	ctx context.Context,
	room collaboration.Room,
	after uint64,
	limit int,
) (collaboration.ReplayPage, error) {
	if repository == nil || repository.db == nil || !validRoom(room) || limit <= 0 {
		return collaboration.ReplayPage{}, collaboration.ErrInvalidInput
	}
	if limit > maxReplayLimit {
		limit = maxReplayLimit
	}
	var stream ResourceStreamRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND resource_id = ?", room.WorkspaceID, room.ResourceID).
		First(&stream).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		page := collaboration.ReplayPage{Events: []collaboration.Event{}, RetainedFromSequence: 1}
		if after > 0 {
			page.ResetRequired, page.ResetReason = true, "stream_not_initialized"
		}
		return page, nil
	} else if err != nil {
		return collaboration.ReplayPage{}, mapError(err)
	}
	page := collaboration.ReplayPage{
		HeadSequence:         stream.NextSequence - 1,
		RetainedFromSequence: stream.RetainedFromSequence,
		Events:               []collaboration.Event{},
	}
	if after > page.HeadSequence {
		page.ResetRequired, page.ResetReason = true, "cursor_ahead_of_head"
		return page, nil
	}
	if after+1 < stream.RetainedFromSequence {
		page.ResetRequired, page.ResetReason = true, "cursor_outside_retention"
		return page, nil
	}
	var records []CommittedEventRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND resource_id = ? AND sequence > ?", room.WorkspaceID, room.ResourceID, after).
		Order("sequence ASC").Limit(limit).Find(&records).Error; err != nil {
		return collaboration.ReplayPage{}, err
	}
	expected := after + 1
	if expected < stream.RetainedFromSequence {
		expected = stream.RetainedFromSequence
	}
	for _, record := range records {
		if record.Sequence != expected {
			page.ResetRequired, page.ResetReason = true, "event_gap"
			page.Events = nil
			return page, nil
		}
		page.Events = append(page.Events, toEvent(record))
		expected++
	}
	if len(records) == 0 && after < page.HeadSequence {
		page.ResetRequired, page.ResetReason = true, "event_gap"
	}
	return page, nil
}

// PruneBefore advances the retention floor and removes older events atomically.
// It is intended for a controlled compaction job after a full resource snapshot exists.
func (repository *Repository) PruneBefore(ctx context.Context, room collaboration.Room, retainFrom uint64) error {
	if repository == nil || repository.db == nil || !validRoom(room) || retainFrom == 0 {
		return collaboration.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stream ResourceStreamRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND resource_id = ?", room.WorkspaceID, room.ResourceID).
			First(&stream).Error; err != nil {
			return mapError(err)
		}
		if retainFrom < stream.RetainedFromSequence || retainFrom > stream.NextSequence {
			return collaboration.ErrInvalidInput
		}
		if err := tx.Where("workspace_id = ? AND resource_id = ? AND sequence < ?",
			room.WorkspaceID, room.ResourceID, retainFrom).Delete(&CommittedEventRecord{}).Error; err != nil {
			return err
		}
		return tx.Model(&ResourceStreamRecord{}).
			Where("workspace_id = ? AND resource_id = ?", room.WorkspaceID, room.ResourceID).
			Update("retained_from_sequence", retainFrom).Error
	})
}
