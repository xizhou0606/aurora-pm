package collaborationgorm

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxPayloadBytes = 64 * 1024

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	resourcePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	eventTypePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
)

type Appender struct{ tx *gorm.DB }
type Repository struct {
	db                         *gorm.DB
	documentCommitGuardFactory DocumentCommitGuardFactory
}

type DocumentCommitGuardFactory func(*gorm.DB) (collaboration.DocumentCommitGuard, error)

func NewAppender(tx *gorm.DB) (*Appender, error) {
	if tx == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &Appender{tx: tx}, nil
}

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, collaboration.ErrInvalidInput
	}
	return &Repository{db: db}, nil
}

func (repository *Repository) ConfigureDocumentCommitGuardFactory(factory DocumentCommitGuardFactory) {
	repository.documentCommitGuardFactory = factory
}

func validRoom(room collaboration.Room) bool {
	return room.WorkspaceID > 0 && resourcePattern.MatchString(room.ResourceID)
}

func validateAppend(input collaboration.AppendInput) error {
	if !validRoom(input.Room) || !identifierPattern.MatchString(input.OperationID) ||
		!eventTypePattern.MatchString(input.Type) || input.ActorUserID == 0 || input.PolicyRevision == 0 ||
		len(input.Payload) == 0 || len(input.Payload) > maxPayloadBytes || !json.Valid(input.Payload) {
		return collaboration.ErrInvalidInput
	}
	var value any
	if json.Unmarshal(input.Payload, &value) != nil {
		return collaboration.ErrInvalidInput
	}
	if _, ok := value.(map[string]any); !ok {
		return collaboration.ErrInvalidInput
	}
	return nil
}

func (appender *Appender) Append(ctx context.Context, input collaboration.AppendInput) (collaboration.Event, error) {
	if appender == nil || appender.tx == nil || validateAppend(input) != nil {
		return collaboration.Event{}, collaboration.ErrInvalidInput
	}
	tx := appender.tx.WithContext(ctx)
	createdAt, err := databaseTime(tx)
	if err != nil {
		return collaboration.Event{}, err
	}
	stream := ResourceStreamRecord{
		WorkspaceID: input.Room.WorkspaceID, ResourceID: input.Room.ResourceID,
		NextSequence: 1, RetainedFromSequence: 1, UpdatedAt: createdAt,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&stream).Error; err != nil {
		return collaboration.Event{}, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND resource_id = ?", input.Room.WorkspaceID, input.Room.ResourceID).
		First(&stream).Error; err != nil {
		return collaboration.Event{}, mapError(err)
	}
	if existing, found, err := findOperation(tx, input); err != nil {
		return collaboration.Event{}, err
	} else if found {
		if !sameOperation(existing, input) {
			return collaboration.Event{}, collaboration.ErrConflict
		}
		return toEvent(existing), nil
	}
	// Read the database clock only after acquiring the resource lock. This makes
	// committed_at non-decreasing in the same order as the allocated sequence.
	now, err := databaseTime(tx)
	if err != nil {
		return collaboration.Event{}, err
	}
	record := CommittedEventRecord{
		WorkspaceID: input.Room.WorkspaceID, ResourceID: input.Room.ResourceID,
		Sequence: stream.NextSequence, OperationID: input.OperationID, ActorUserID: input.ActorUserID,
		EventType: input.Type, Payload: append([]byte(nil), input.Payload...),
		PolicyRevision: input.PolicyRevision, CommittedAt: now,
	}
	if err := tx.Create(&record).Error; err != nil {
		return collaboration.Event{}, err
	}
	result := tx.Model(&ResourceStreamRecord{}).
		Where("workspace_id = ? AND resource_id = ? AND next_sequence = ?", input.Room.WorkspaceID, input.Room.ResourceID, stream.NextSequence).
		Updates(map[string]any{"next_sequence": stream.NextSequence + 1, "updated_at": now})
	if result.Error != nil {
		return collaboration.Event{}, result.Error
	}
	if result.RowsAffected != 1 {
		return collaboration.Event{}, collaboration.ErrConflict
	}
	return toEvent(record), nil
}

func findOperation(tx *gorm.DB, input collaboration.AppendInput) (CommittedEventRecord, bool, error) {
	var record CommittedEventRecord
	err := tx.Where("workspace_id = ? AND resource_id = ? AND operation_id = ?",
		input.Room.WorkspaceID, input.Room.ResourceID, input.OperationID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, false, nil
	}
	return record, err == nil, err
}

func sameOperation(record CommittedEventRecord, input collaboration.AppendInput) bool {
	return record.ActorUserID == input.ActorUserID && record.EventType == input.Type &&
		record.PolicyRevision == input.PolicyRevision && jsonEqual(record.Payload, input.Payload)
}

func jsonEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}

func databaseTime(db *gorm.DB) (time.Time, error) {
	if db.Dialector.Name() == "sqlite" {
		var seconds int64
		if err := db.Raw("SELECT unixepoch()").Scan(&seconds).Error; err != nil {
			return time.Time{}, err
		}
		return time.Unix(seconds, 0).UTC(), nil
	}
	var now time.Time
	expression := "CURRENT_TIMESTAMP"
	if db.Dialector.Name() == "postgres" {
		expression = "clock_timestamp()"
	}
	if err := db.Raw("SELECT " + expression).Scan(&now).Error; err != nil {
		return time.Time{}, err
	}
	return now.UTC(), nil
}

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return collaboration.ErrNotFound
	}
	return err
}

func toEvent(record CommittedEventRecord) collaboration.Event {
	return collaboration.Event{
		Room:     collaboration.Room{WorkspaceID: record.WorkspaceID, ResourceID: record.ResourceID},
		Sequence: record.Sequence, OperationID: record.OperationID, ActorUserID: record.ActorUserID,
		Type: record.EventType, Payload: append([]byte(nil), record.Payload...),
		PolicyRevision: record.PolicyRevision, CommittedAt: record.CommittedAt,
	}
}

var _ collaboration.EventAppender = (*Appender)(nil)
var _ collaboration.EventReader = (*Repository)(nil)
