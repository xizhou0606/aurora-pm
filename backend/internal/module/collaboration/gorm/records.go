package collaborationgorm

import "time"

type ResourceStreamRecord struct {
	WorkspaceID          uint      `gorm:"primaryKey;autoIncrement:false"`
	ResourceID           string    `gorm:"primaryKey;size:64"`
	NextSequence         uint64    `gorm:"not null;default:1"`
	RetainedFromSequence uint64    `gorm:"not null;default:1"`
	UpdatedAt            time.Time `gorm:"not null"`
}

func (ResourceStreamRecord) TableName() string { return "collaboration_resource_streams" }

type CommittedEventRecord struct {
	ID             uint64    `gorm:"primaryKey"`
	WorkspaceID    uint      `gorm:"not null;uniqueIndex:ux_collaboration_event_sequence,priority:1;uniqueIndex:ux_collaboration_event_operation,priority:1;index:idx_collaboration_events_replay,priority:1"`
	ResourceID     string    `gorm:"size:64;not null;uniqueIndex:ux_collaboration_event_sequence,priority:2;uniqueIndex:ux_collaboration_event_operation,priority:2;index:idx_collaboration_events_replay,priority:2"`
	Sequence       uint64    `gorm:"not null;uniqueIndex:ux_collaboration_event_sequence,priority:3;index:idx_collaboration_events_replay,priority:3"`
	OperationID    string    `gorm:"size:128;not null;uniqueIndex:ux_collaboration_event_operation,priority:3"`
	ActorUserID    uint      `gorm:"not null"`
	EventType      string    `gorm:"size:64;not null"`
	Payload        []byte    `gorm:"type:json;not null"`
	PolicyRevision uint64    `gorm:"not null"`
	CommittedAt    time.Time `gorm:"not null"`
}

func (CommittedEventRecord) TableName() string { return "collaboration_committed_events" }

type PresenceRecord struct {
	WorkspaceID      uint      `gorm:"primaryKey;autoIncrement:false"`
	ResourceID       string    `gorm:"primaryKey;size:64"`
	ConnectionID     string    `gorm:"primaryKey;size:128"`
	ActorUserID      uint      `gorm:"not null"`
	ActorDisplayName string    `gorm:"size:220;not null"`
	State            []byte    `gorm:"type:json;not null"`
	ExpiresAt        time.Time `gorm:"not null;index:idx_collaboration_presence_expiry"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (PresenceRecord) TableName() string { return "collaboration_presence" }

type CellLeaseRecord struct {
	WorkspaceID           uint    `gorm:"primaryKey;autoIncrement:false"`
	ResourceID            string  `gorm:"primaryKey;size:64"`
	TableID               string  `gorm:"primaryKey;size:64"`
	RecordID              string  `gorm:"primaryKey;size:64"`
	FieldID               string  `gorm:"primaryKey;size:64"`
	LeaseID               *string `gorm:"size:64"`
	FencingToken          uint64  `gorm:"not null;default:0"`
	OwnerConnectionID     *string `gorm:"size:128;index:idx_collaboration_cell_leases_connection,priority:2"`
	OwnerEditorInstanceID *string `gorm:"size:128"`
	ActorUserID           *uint
	ActorDisplayName      *string `gorm:"size:220"`
	PolicyRevision        *uint64
	ExpiresAt             *time.Time `gorm:"index:idx_collaboration_cell_leases_expiry"`
	UpdatedAt             time.Time  `gorm:"not null"`
}

func (CellLeaseRecord) TableName() string { return "collaboration_cell_leases" }

type DocumentStateRecord struct {
	WorkspaceID     uint      `gorm:"primaryKey;autoIncrement:false"`
	ResourceID      string    `gorm:"primaryKey;size:64"`
	SourceRevision  uint64    `gorm:"not null"`
	SourceHash      string    `gorm:"size:64;not null"`
	DurableRevision uint64    `gorm:"not null"`
	Snapshot        []byte    `gorm:"not null"`
	SnapshotHash    string    `gorm:"size:64;not null"`
	StateVector     []byte    `gorm:"not null"`
	StateVectorHash string    `gorm:"size:64;not null"`
	Projection      []byte    `gorm:"type:json;not null"`
	ProjectionHash  string    `gorm:"size:64;not null"`
	PolicyRevision  uint64    `gorm:"not null"`
	SeededAt        time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

func (DocumentStateRecord) TableName() string { return "collaboration_document_states" }

type DocumentUpdateRecord struct {
	ID                    uint64    `gorm:"primaryKey"`
	WorkspaceID           uint      `gorm:"not null;uniqueIndex:ux_collaboration_document_update_revision,priority:1;uniqueIndex:ux_collaboration_document_update_operation,priority:1"`
	ResourceID            string    `gorm:"size:64;not null;uniqueIndex:ux_collaboration_document_update_revision,priority:2;uniqueIndex:ux_collaboration_document_update_operation,priority:2"`
	DurableRevision       uint64    `gorm:"not null;uniqueIndex:ux_collaboration_document_update_revision,priority:3"`
	BaseDurableRevision   uint64    `gorm:"not null"`
	OperationID           string    `gorm:"size:128;not null;uniqueIndex:ux_collaboration_document_update_operation,priority:3"`
	ActorUserID           uint      `gorm:"not null"`
	Update                []byte    `gorm:"not null"`
	UpdateHash            string    `gorm:"size:64;not null"`
	BaseStateVectorHash   string    `gorm:"size:64;not null"`
	ResultStateVectorHash string    `gorm:"size:64;not null"`
	ResultSnapshotHash    string    `gorm:"size:64;not null"`
	ResultProjectionHash  string    `gorm:"size:64;not null"`
	PolicyRevision        uint64    `gorm:"not null"`
	CommittedAt           time.Time `gorm:"not null"`
}

func (DocumentUpdateRecord) TableName() string { return "collaboration_document_updates" }

func MigrationRecords() []any {
	return []any{
		&ResourceStreamRecord{}, &CommittedEventRecord{}, &PresenceRecord{}, &CellLeaseRecord{},
		&DocumentStateRecord{}, &DocumentUpdateRecord{},
	}
}
