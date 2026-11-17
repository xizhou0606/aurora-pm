package collaboration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const (
	MaxDocumentUpdateBytes      = 256 * 1024
	MaxDocumentSnapshotBytes    = 4 * 1024 * 1024
	MaxDocumentStateVectorBytes = 256 * 1024
	MaxDocumentProjectionBytes  = 4 * 1024 * 1024
)

var documentHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type DocumentSeed struct {
	Room            Room
	SourceRevision  uint64
	SourceHash      string
	Snapshot        []byte
	SnapshotHash    string
	StateVector     []byte
	StateVectorHash string
	Projection      json.RawMessage
	ProjectionHash  string
	ActorUserID     uint
	PolicyRevision  uint64
}

type DocumentUpdate struct {
	Room                    Room
	OperationID             string
	ExpectedDurableRevision uint64
	BaseStateVectorHash     string
	Update                  []byte
	UpdateHash              string
	Snapshot                []byte
	SnapshotHash            string
	StateVector             []byte
	StateVectorHash         string
	Projection              json.RawMessage
	ProjectionHash          string
	ActorUserID             uint
	PolicyRevision          uint64
}

type DocumentState struct {
	Room            Room            `json:"room"`
	SourceRevision  uint64          `json:"source_revision"`
	SourceHash      string          `json:"source_hash"`
	DurableRevision uint64          `json:"durable_revision"`
	Snapshot        []byte          `json:"snapshot"`
	SnapshotHash    string          `json:"snapshot_hash"`
	StateVector     []byte          `json:"state_vector"`
	StateVectorHash string          `json:"state_vector_hash"`
	Projection      json.RawMessage `json:"projection"`
	ProjectionHash  string          `json:"projection_hash"`
	PolicyRevision  uint64          `json:"policy_revision"`
	SeededAt        time.Time       `json:"seeded_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type DocumentCommit struct {
	Room            Room      `json:"room"`
	OperationID     string    `json:"operation_id"`
	DurableRevision uint64    `json:"durable_revision"`
	SnapshotHash    string    `json:"snapshot_hash"`
	StateVectorHash string    `json:"state_vector_hash"`
	ProjectionHash  string    `json:"projection_hash"`
	PolicyRevision  uint64    `json:"policy_revision"`
	CommittedAt     time.Time `json:"committed_at"`
}

type DocumentCollaborationService struct {
	runner     DocumentTransactionRunner
	reader     DocumentStateReader
	access     DocumentCollaborationAccessEvaluator
	projection DocumentProjectionValidator
}

func NewDocumentCollaborationService(
	runner DocumentTransactionRunner,
	reader DocumentStateReader,
	access DocumentCollaborationAccessEvaluator,
	projection DocumentProjectionValidator,
) (*DocumentCollaborationService, error) {
	if runner == nil || reader == nil || access == nil || projection == nil {
		return nil, ErrInvalidInput
	}
	return &DocumentCollaborationService{runner: runner, reader: reader, access: access, projection: projection}, nil
}

func (service *DocumentCollaborationService) Seed(
	ctx context.Context,
	principal Principal,
	input DocumentSeed,
) (DocumentState, error) {
	if service == nil || principal.WorkspaceID != input.Room.WorkspaceID {
		return DocumentState{}, ErrInvalidInput
	}
	decision, err := service.authorize(ctx, principal, input.Room, true)
	if err != nil {
		return DocumentState{}, err
	}
	input.ActorUserID, input.PolicyRevision = principal.UserID, decision.PolicyRevision
	if err = ValidateDocumentSeed(input); err != nil {
		return DocumentState{}, err
	}
	if err = service.projection.ValidateDocumentProjection(ctx, principal, input.Room, input.Projection); err != nil {
		return DocumentState{}, err
	}
	var state DocumentState
	err = service.runner.WithinDocumentTransaction(ctx, func(store DocumentTransactionStore) error {
		var storeErr error
		state, storeErr = store.SeedDocument(ctx, input)
		return storeErr
	})
	return state, err
}

func (service *DocumentCollaborationService) Commit(
	ctx context.Context,
	principal Principal,
	input DocumentUpdate,
) (DocumentCommit, error) {
	if service == nil || principal.WorkspaceID != input.Room.WorkspaceID {
		return DocumentCommit{}, ErrInvalidInput
	}
	decision, err := service.authorize(ctx, principal, input.Room, true)
	if err != nil {
		return DocumentCommit{}, err
	}
	input.ActorUserID, input.PolicyRevision = principal.UserID, decision.PolicyRevision
	if err = ValidateDocumentUpdate(input); err != nil {
		return DocumentCommit{}, err
	}
	if err = service.projection.ValidateDocumentProjection(ctx, principal, input.Room, input.Projection); err != nil {
		return DocumentCommit{}, err
	}
	var commit DocumentCommit
	err = service.runner.WithinDocumentTransaction(ctx, func(store DocumentTransactionStore) error {
		var storeErr error
		commit, storeErr = store.CommitDocumentUpdate(ctx, principal, input)
		return storeErr
	})
	return commit, err
}

func (service *DocumentCollaborationService) Load(
	ctx context.Context,
	principal Principal,
	room Room,
) (DocumentState, error) {
	if service == nil || principal.WorkspaceID != room.WorkspaceID {
		return DocumentState{}, ErrInvalidInput
	}
	if _, err := service.authorize(ctx, principal, room, false); err != nil {
		return DocumentState{}, err
	}
	return service.reader.LoadDocumentState(ctx, room)
}

func (service *DocumentCollaborationService) authorize(
	ctx context.Context,
	principal Principal,
	room Room,
	write bool,
) (AccessDecision, error) {
	decision, err := service.access.EvaluateDocumentCollaborationAccess(ctx, principal, room)
	if err != nil {
		return AccessDecision{}, err
	}
	if !decision.CanView || write && !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
		return AccessDecision{}, ErrForbidden
	}
	return decision, nil
}

func SHA256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func CanonicalJSONHash(value []byte) (string, error) {
	var decoded any
	if len(value) == 0 || len(value) > MaxDocumentProjectionBytes || json.Unmarshal(value, &decoded) != nil {
		return "", ErrInvalidInput
	}
	if _, ok := decoded.(map[string]any); !ok {
		return "", ErrInvalidInput
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(decoded); err != nil {
		return "", ErrInvalidInput
	}
	return SHA256Hex(bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})), nil
}

func ValidateDocumentSeed(input DocumentSeed) error {
	projectionHash, err := CanonicalJSONHash(input.Projection)
	if !validDocumentRoom(input.Room) || input.SourceRevision == 0 || input.ActorUserID == 0 || input.PolicyRevision == 0 ||
		len(input.Snapshot) == 0 || len(input.Snapshot) > MaxDocumentSnapshotBytes ||
		len(input.StateVector) == 0 || len(input.StateVector) > MaxDocumentStateVectorBytes || err != nil ||
		!IsSHA256Hex(input.SourceHash) || input.SourceHash != projectionHash ||
		input.ProjectionHash != projectionHash || input.SnapshotHash != SHA256Hex(input.Snapshot) ||
		input.StateVectorHash != SHA256Hex(input.StateVector) {
		return ErrInvalidInput
	}
	return nil
}

func ValidateDocumentUpdate(input DocumentUpdate) error {
	projectionHash, err := CanonicalJSONHash(input.Projection)
	if !validDocumentRoom(input.Room) || !validDocumentOperationID(input.OperationID) ||
		input.ExpectedDurableRevision == 0 || input.ActorUserID == 0 || input.PolicyRevision == 0 ||
		len(input.Update) == 0 || len(input.Update) > MaxDocumentUpdateBytes ||
		len(input.Snapshot) == 0 || len(input.Snapshot) > MaxDocumentSnapshotBytes ||
		len(input.StateVector) == 0 || len(input.StateVector) > MaxDocumentStateVectorBytes || err != nil ||
		input.UpdateHash != SHA256Hex(input.Update) || input.SnapshotHash != SHA256Hex(input.Snapshot) ||
		input.StateVectorHash != SHA256Hex(input.StateVector) || input.ProjectionHash != projectionHash ||
		!IsSHA256Hex(input.BaseStateVectorHash) {
		return ErrInvalidInput
	}
	return nil
}

func IsSHA256Hex(value string) bool { return documentHashPattern.MatchString(value) }

func validDocumentRoom(room Room) bool {
	return room.WorkspaceID > 0 && len(room.ResourceID) > 0 && len(room.ResourceID) <= 64 &&
		strings.TrimSpace(room.ResourceID) == room.ResourceID
}

func validDocumentOperationID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}
