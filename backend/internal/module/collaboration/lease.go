package collaboration

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DefaultCellLeaseTTL = 30 * time.Second

type CellKey struct {
	Room     Room   `json:"room"`
	TableID  string `json:"table_id"`
	RecordID string `json:"record_id"`
	FieldID  string `json:"field_id"`
}

type CellLease struct {
	Key              CellKey   `json:"key"`
	LeaseID          string    `json:"lease_id"`
	FencingToken     uint64    `json:"fencing_token"`
	ConnectionID     string    `json:"connection_id"`
	EditorInstanceID string    `json:"editor_instance_id"`
	ActorUserID      uint      `json:"actor_user_id"`
	ActorName        string    `json:"actor_display_name"`
	PolicyRevision   uint64    `json:"policy_revision"`
	ExpiresAt        time.Time `json:"expires_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CellLeaseProof struct {
	Key              CellKey
	LeaseID          string
	FencingToken     uint64
	ConnectionID     string
	EditorInstanceID string
}

type CellLeaseAcquire struct {
	Principal        Principal
	Key              CellKey
	LeaseID          string
	EditorInstanceID string
	PolicyRevision   uint64
	AccessExpiresAt  time.Time
	TTL              time.Duration
}

type CellLeaseRenewal struct {
	Principal       Principal
	Proof           CellLeaseProof
	PolicyRevision  uint64
	AccessExpiresAt time.Time
	TTL             time.Duration
}

type CellLeaseService struct {
	store  CellLeaseStore
	access CellLeaseAccessEvaluator
	ttl    time.Duration
}

func NewCellLeaseService(store CellLeaseStore, access CellLeaseAccessEvaluator, ttl time.Duration) (*CellLeaseService, error) {
	if store == nil || access == nil || ttl <= 0 || ttl > DefaultCellLeaseTTL {
		return nil, ErrInvalidInput
	}
	return &CellLeaseService{store: store, access: access, ttl: ttl}, nil
}

func (service *CellLeaseService) Acquire(
	ctx context.Context,
	principal Principal,
	key CellKey,
	editorInstanceID string,
) (CellLease, error) {
	if service == nil || principal.ConnectionID == "" || strings.TrimSpace(editorInstanceID) == "" {
		return CellLease{}, ErrInvalidInput
	}
	decision, err := service.authorize(ctx, principal, key)
	if err != nil {
		return CellLease{}, err
	}
	return service.store.AcquireCellLease(ctx, CellLeaseAcquire{
		Principal: principal, Key: key, LeaseID: uuid.NewString(), EditorInstanceID: editorInstanceID,
		PolicyRevision: decision.PolicyRevision, AccessExpiresAt: decision.ExpiresAt, TTL: service.ttl,
	})
}

func (service *CellLeaseService) Renew(
	ctx context.Context,
	principal Principal,
	proof CellLeaseProof,
) (CellLease, error) {
	if service == nil {
		return CellLease{}, ErrInvalidInput
	}
	decision, err := service.authorize(ctx, principal, proof.Key)
	if err != nil {
		return CellLease{}, err
	}
	return service.store.RenewCellLease(ctx, CellLeaseRenewal{
		Principal: principal, Proof: proof, PolicyRevision: decision.PolicyRevision,
		AccessExpiresAt: decision.ExpiresAt, TTL: service.ttl,
	})
}

func (service *CellLeaseService) Release(ctx context.Context, principal Principal, proof CellLeaseProof) error {
	if service == nil {
		return ErrInvalidInput
	}
	// An owner may release after access is revoked. Requiring edit permission here
	// would keep an obsolete lease alive until TTL.
	return service.store.ReleaseCellLease(ctx, principal, proof)
}

func (service *CellLeaseService) Disconnect(ctx context.Context, principal Principal, room Room) (int64, error) {
	if service == nil || principal.WorkspaceID == 0 || principal.UserID == 0 || principal.ConnectionID == "" ||
		room.WorkspaceID != principal.WorkspaceID {
		return 0, ErrInvalidInput
	}
	return service.store.ReleaseConnectionCellLeases(ctx, principal, room)
}

func (service *CellLeaseService) AcquireBatch(
	ctx context.Context,
	acquirer CellLeaseBatchAcquirer,
	principal Principal,
	editorInstanceID string,
	keys []CellKey,
) ([]CellLease, error) {
	if service == nil || acquirer == nil || principal.ConnectionID == "" || strings.TrimSpace(editorInstanceID) == "" {
		return nil, ErrInvalidInput
	}
	return acquirer.AcquireCellLeases(ctx, principal, editorInstanceID, keys, service.ttl)
}

func (service *CellLeaseService) authorize(
	ctx context.Context,
	principal Principal,
	key CellKey,
) (AccessDecision, error) {
	decision, err := service.access.EvaluateCellEditAccess(ctx, principal, key)
	if err != nil {
		return AccessDecision{}, err
	}
	if !decision.CanEdit || decision.PolicyRevision == 0 || !decision.ExpiresAt.After(time.Now()) {
		return AccessDecision{}, ErrForbidden
	}
	return decision, nil
}

func ProofFor(lease CellLease) CellLeaseProof {
	return CellLeaseProof{
		Key: lease.Key, LeaseID: lease.LeaseID, FencingToken: lease.FencingToken,
		ConnectionID: lease.ConnectionID, EditorInstanceID: lease.EditorInstanceID,
	}
}

// NormalizeCellKeys returns unique keys in the global lock order. The function
// intentionally does not validate storage-specific UUID formats.
func NormalizeCellKeys(keys []CellKey) ([]CellKey, error) {
	if len(keys) == 0 || len(keys) > 500 {
		return nil, ErrInvalidInput
	}
	result := append([]CellKey(nil), keys...)
	sort.Slice(result, func(left, right int) bool {
		a, b := result[left], result[right]
		if a.Room.WorkspaceID != b.Room.WorkspaceID {
			return a.Room.WorkspaceID < b.Room.WorkspaceID
		}
		for _, pair := range [][2]string{{a.Room.ResourceID, b.Room.ResourceID}, {a.TableID, b.TableID}, {a.RecordID, b.RecordID}, {a.FieldID, b.FieldID}} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return false
	})
	unique := result[:0]
	for _, key := range result {
		if len(unique) == 0 || unique[len(unique)-1] != key {
			unique = append(unique, key)
		}
	}
	return unique, nil
}
