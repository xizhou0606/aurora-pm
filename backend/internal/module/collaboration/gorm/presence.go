package collaborationgorm

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm/clause"
)

const presenceTTL = 30 * time.Second
const maxPresenceBytes = 8 * 1024

var reservedPresenceKeys = []string{"actor", "user", "userId", "workspaceId"}

func validatePresence(principal collaboration.Principal, room collaboration.Room, state []byte) error {
	if !validRoom(room) || principal.WorkspaceID != room.WorkspaceID || principal.UserID == 0 ||
		!identifierPattern.MatchString(principal.ConnectionID) || strings.TrimSpace(principal.DisplayName) == "" ||
		utf8.RuneCountInString(strings.TrimSpace(principal.DisplayName)) > 220 ||
		len(state) == 0 || len(state) > maxPresenceBytes || !json.Valid(state) {
		return collaboration.ErrInvalidInput
	}
	var object map[string]any
	if json.Unmarshal(state, &object) != nil || object == nil {
		return collaboration.ErrInvalidInput
	}
	for _, key := range reservedPresenceKeys {
		if _, exists := object[key]; exists {
			return collaboration.ErrInvalidInput
		}
	}
	return nil
}

func (repository *Repository) Heartbeat(
	ctx context.Context,
	principal collaboration.Principal,
	room collaboration.Room,
	state []byte,
) (collaboration.Presence, error) {
	if repository == nil || repository.db == nil || validatePresence(principal, room, state) != nil {
		return collaboration.Presence{}, collaboration.ErrInvalidInput
	}
	now, err := databaseTime(repository.db.WithContext(ctx))
	if err != nil {
		return collaboration.Presence{}, err
	}
	record := PresenceRecord{
		WorkspaceID: room.WorkspaceID, ResourceID: room.ResourceID, ConnectionID: principal.ConnectionID,
		ActorUserID: principal.UserID, ActorDisplayName: strings.TrimSpace(principal.DisplayName),
		State: append([]byte(nil), state...), ExpiresAt: now.Add(presenceTTL), UpdatedAt: now,
	}
	result := repository.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "resource_id"}, {Name: "connection_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"actor_user_id", "actor_display_name", "state", "expires_at", "updated_at"}),
		Where: clause.Where{Exprs: []clause.Expression{clause.Eq{
			Column: clause.Column{Table: "collaboration_presence", Name: "actor_user_id"}, Value: principal.UserID,
		}}},
	}).Create(&record)
	if err = result.Error; err != nil {
		return collaboration.Presence{}, err
	}
	if result.RowsAffected != 1 {
		return collaboration.Presence{}, collaboration.ErrForbidden
	}
	return toPresence(record), nil
}

func (repository *Repository) ListActive(ctx context.Context, room collaboration.Room) ([]collaboration.Presence, error) {
	if repository == nil || repository.db == nil || !validRoom(room) {
		return nil, collaboration.ErrInvalidInput
	}
	now, err := databaseTime(repository.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	var records []PresenceRecord
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND resource_id = ? AND expires_at > ?", room.WorkspaceID, room.ResourceID, now).
		Order("connection_id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]collaboration.Presence, 0, len(records))
	for _, record := range records {
		result = append(result, toPresence(record))
	}
	return result, nil
}

func (repository *Repository) Leave(ctx context.Context, principal collaboration.Principal, room collaboration.Room) error {
	if repository == nil || repository.db == nil || !validRoom(room) || principal.WorkspaceID != room.WorkspaceID ||
		principal.UserID == 0 || !identifierPattern.MatchString(principal.ConnectionID) {
		return collaboration.ErrInvalidInput
	}
	return repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND resource_id = ? AND connection_id = ? AND actor_user_id = ?",
		room.WorkspaceID, room.ResourceID, principal.ConnectionID, principal.UserID,
	).Delete(&PresenceRecord{}).Error
}

func (repository *Repository) DeleteExpired(ctx context.Context) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, collaboration.ErrInvalidInput
	}
	now, err := databaseTime(repository.db.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	result := repository.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&PresenceRecord{})
	return result.RowsAffected, result.Error
}

func toPresence(record PresenceRecord) collaboration.Presence {
	return collaboration.Presence{
		Room:         collaboration.Room{WorkspaceID: record.WorkspaceID, ResourceID: record.ResourceID},
		ConnectionID: record.ConnectionID, ActorUserID: record.ActorUserID, ActorName: record.ActorDisplayName,
		State: append([]byte(nil), record.State...), ExpiresAt: record.ExpiresAt, UpdatedAt: record.UpdatedAt,
	}
}

var _ collaboration.PresenceStore = (*Repository)(nil)
