package native

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

// Field revisions live beside native values so rollout does not require a
// second persistence table or a second concurrency scheme. Field IDs are UUIDs,
// making this reserved non-UUID key unambiguous and safe to hide at projection.
const fieldRevisionsStorageKey = "__pm_field_revisions"

func decodeRowState(row RowRecord, fields []bitable.Field) (map[string]any, map[string]uint64, error) {
	stored := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(row.Values), &stored); err != nil {
		return nil, nil, err
	}
	revisions := map[string]uint64{}
	if raw, ok := stored[fieldRevisionsStorageKey]; ok {
		if err := json.Unmarshal(raw, &revisions); err != nil {
			return nil, nil, err
		}
		delete(stored, fieldRevisionsStorageKey)
	}
	values := map[string]any{}
	for key, raw := range stored {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, nil, err
		}
		values[key] = value
	}
	// Legacy rows are conservatively materialized from the one authoritative
	// row revision. Materializing every field together is what lets a later
	// update to another field keep the original base and merge safely.
	for _, field := range fields {
		if revisions[field.ID] == 0 {
			revisions[field.ID] = row.Revision
		}
	}
	return values, revisions, nil
}

func encodeRowState(values map[string]any, revisions map[string]uint64) (string, error) {
	stored := make(map[string]any, len(values)+1)
	for key, value := range values {
		stored[key] = value
	}
	stored[fieldRevisionsStorageKey] = revisions
	return encode(stored)
}

func contractsByField(contracts []bitable.CellContract) map[string]bitable.CellContract {
	result := make(map[string]bitable.CellContract, len(contracts))
	for _, contract := range contracts {
		result[contract.FieldID] = contract
	}
	return result
}

func (r *Repository) verifyCellContracts(
	ctx context.Context,
	tx *gorm.DB,
	s bitable.Scope,
	contracts []bitable.CellContract,
) error {
	if len(contracts) == 0 {
		return bitable.ErrInvalidInput
	}
	guard, err := r.cellGuard(tx)
	if err != nil {
		return err
	}
	ordered := append([]bitable.CellContract(nil), contracts...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].RecordID != ordered[j].RecordID {
			return ordered[i].RecordID < ordered[j].RecordID
		}
		return ordered[i].FieldID < ordered[j].FieldID
	})
	for _, contract := range ordered {
		if err = guard.VerifyCellMutation(ctx, s.Principal, mutationProof(s, bitable.CellLeaseProof{
			RecordID: contract.RecordID, FieldID: contract.FieldID, LeaseID: contract.LeaseID,
			FencingToken: contract.FencingToken, ConnectionID: contract.ConnectionID, EditorInstanceID: contract.EditorInstanceID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) verifyCellLeaseProofs(
	ctx context.Context,
	tx *gorm.DB,
	s bitable.Scope,
	proofs []bitable.CellLeaseProof,
) error {
	if len(proofs) == 0 {
		return bitable.ErrInvalidInput
	}
	guard, err := r.cellGuard(tx)
	if err != nil {
		return err
	}
	ordered := append([]bitable.CellLeaseProof(nil), proofs...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].RecordID != ordered[j].RecordID {
			return ordered[i].RecordID < ordered[j].RecordID
		}
		return ordered[i].FieldID < ordered[j].FieldID
	})
	for _, proof := range ordered {
		if err = guard.VerifyCellMutation(ctx, s.Principal, mutationProof(s, proof)); err != nil {
			return err
		}
	}
	return nil
}

func mutationProof(s bitable.Scope, proof bitable.CellLeaseProof) bitable.CellMutationProof {
	return bitable.CellMutationProof{
		WorkspaceID: s.Principal.WorkspaceID, ResourceID: s.DocumentResourceID, TableID: s.TableID,
		RecordID: proof.RecordID, FieldID: proof.FieldID, LeaseID: proof.LeaseID,
		FencingToken: proof.FencingToken, ConnectionID: proof.ConnectionID,
		EditorInstanceID: proof.EditorInstanceID,
	}
}

func (r *Repository) requireNoConflictingCellLeases(
	ctx context.Context,
	tx *gorm.DB,
	s bitable.Scope,
	scopes []bitable.CellLeaseConflictScope,
) error {
	if len(scopes) == 0 {
		return nil
	}
	guard, err := r.cellConflictGuard(tx)
	if err != nil {
		return err
	}
	for i := range scopes {
		scopes[i].WorkspaceID = s.Principal.WorkspaceID
		scopes[i].ResourceID = s.DocumentResourceID
		scopes[i].TableID = s.TableID
		sort.Strings(scopes[i].RecordIDs)
		sort.Strings(scopes[i].FieldIDs)
	}
	return guard.RequireNoConflictingCellLeases(ctx, s.Principal, scopes)
}
