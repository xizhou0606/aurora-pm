package native

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
)

type eventAppender struct {
	events *[]bitable.CommittedEvent
	err    error
}

func (a eventAppender) AppendBitableCommittedEvent(_ context.Context, event bitable.CommittedEvent) error {
	if a.err != nil {
		return a.err
	}
	*a.events = append(*a.events, event)
	return nil
}

func addTextField(t *testing.T, repository *Repository, scope bitable.Scope, schema uint64, name string) bitable.Field {
	t.Helper()
	field, err := repository.MutateField(context.Background(), scope, "", bitable.FieldMutation{
		Name: name, Type: bitable.FieldText, ExpectedSchemaRevision: schema, OperationID: uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return field
}

func TestFieldBasePreservesSameRowDifferentFieldsAndRejectsSameCellABA(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	second := addTextField(t, repository, scope, bootstrap.SchemaRevision, "Second")
	bootstrap, err := repository.Bootstrap(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	first := bootstrap.Fields[0]
	original := mustCreate(t, repository, scope, bootstrap, map[string]any{first.ID: "a", second.ID: "b"})

	firstWrite, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(original.ID, first.ID, original.FieldRevisions[first.ID], "a-1")))
	if err != nil {
		t.Fatal(err)
	}
	// This command was based on the original row, but on a different field. It
	// must merge instead of being rejected by the now-changed row revision.
	secondWrite, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(original.ID, second.ID, original.FieldRevisions[second.ID], "b-1")))
	if err != nil {
		t.Fatal("different-field write conflicted", err)
	}
	got := secondWrite.Records[0]
	if got.Values[first.ID] != "a-1" || got.Values[second.ID] != "b-1" ||
		got.FieldRevisions[first.ID] != firstWrite.Records[0].FieldRevisions[first.ID] ||
		got.FieldRevisions[second.ID] != original.FieldRevisions[second.ID]+1 {
		t.Fatalf("different-field merge lost data or revisions: %#v", got)
	}

	// Return the first cell to its original value. Its monotonically increasing
	// field revision still rejects an old base, closing the ABA hole.
	aba, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(original.ID, first.ID, got.FieldRevisions[first.ID], "a")))
	if err != nil || aba.Records[0].Values[first.ID] != "a" {
		t.Fatal("ABA setup failed", err)
	}
	if _, err = repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(original.ID, first.ID, original.FieldRevisions[first.ID], "late"))); !errors.Is(err, bitable.ErrConflict) {
		t.Fatalf("stale same-cell base survived ABA: %v", err)
	}
	if _, leaked := aba.Records[0].Values[fieldRevisionsStorageKey]; leaked {
		t.Fatal("private field revision metadata leaked as a cell value")
	}
}

func TestFieldBaseBatchConflictRollsBackAndDeleteKeepsRowCAS(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	first := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "first"})
	second := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "second"})

	advanced, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(second.ID, field.ID, second.FieldRevisions[field.ID], "second-new")))
	if err != nil {
		t.Fatal(err)
	}
	atomic := batch(bootstrap.SchemaRevision,
		update(first.ID, field.ID, first.FieldRevisions[field.ID], "must-rollback"),
		update(second.ID, field.ID, second.FieldRevisions[field.ID], "stale"),
	)
	if _, err = repository.Execute(context.Background(), scope, atomic); !errors.Is(err, bitable.ErrConflict) {
		t.Fatalf("stale batch accepted: %v", err)
	}
	page, err := repository.Query(context.Background(), scope, query(bootstrap))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]bitable.Record{}
	for _, record := range page.Records {
		byID[record.ID] = record
	}
	if byID[first.ID].Values[field.ID] != "first" || byID[second.ID].Values[field.ID] != "second-new" {
		t.Fatalf("atomic field batch partially committed: %#v", byID)
	}

	// Deletion deliberately remains on rowRevision. A revision captured before
	// a cell update cannot delete the newer row.
	if _, err = repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		bitable.Command{Kind: "delete", RecordID: second.ID, ExpectedRevision: rev(second.Revision)})); !errors.Is(err, bitable.ErrConflict) {
		t.Fatalf("stale row deletion accepted: %v", err)
	}
	if _, err = repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		bitable.Command{Kind: "delete", RecordID: second.ID, ExpectedRevision: rev(advanced.Records[0].Revision)})); err != nil {
		t.Fatal("fresh row deletion failed", err)
	}
}

func TestCellGuardFailureAndCommittedEventAreInOwningTransaction(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	scope.DocumentResourceID = "document-resource-1"
	scope.PolicyRevision = 9
	field := bootstrap.Fields[0]
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "before"})

	guard := &mutationGuard{err: bitable.ErrConflict}
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return guard, nil })
	if _, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(row.ID, field.ID, row.FieldRevisions[field.ID], "blocked"))); !errors.Is(err, bitable.ErrConflict) {
		t.Fatalf("guard failure not propagated: %v", err)
	}
	page, _ := repository.Query(context.Background(), scope, query(bootstrap))
	if page.Records[0].Values[field.ID] != "before" {
		t.Fatal("guard failure committed the row")
	}

	guard.err = nil
	events := []bitable.CommittedEvent{}
	repository.ConfigureCommittedEventAppenderFactory(func(*gorm.DB) (bitable.CommittedEventAppender, error) {
		return eventAppender{events: &events, err: errors.New("event unavailable")}, nil
	})
	input := batch(bootstrap.SchemaRevision, update(row.ID, field.ID, row.FieldRevisions[field.ID], "after"))
	if _, err := repository.Execute(context.Background(), scope, input); err == nil {
		t.Fatal("event failure did not roll back owning mutation")
	}
	page, _ = repository.Query(context.Background(), scope, query(bootstrap))
	if page.Records[0].Values[field.ID] != "before" {
		t.Fatal("event failure committed the row")
	}

	repository.ConfigureCommittedEventAppenderFactory(func(*gorm.DB) (bitable.CommittedEventAppender, error) {
		return eventAppender{events: &events}, nil
	})
	result, err := repository.Execute(context.Background(), scope, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].WorkspaceID != scope.Principal.WorkspaceID ||
		events[0].ResourceID != scope.DocumentResourceID || events[0].OperationID != result.ID ||
		events[0].ActorUserID != scope.Principal.UserID || events[0].PolicyRevision != scope.PolicyRevision ||
		events[0].Type != "bitable.records.committed" {
		t.Fatalf("wrong committed event: %#v", events)
	}
	if _, err = repository.Execute(context.Background(), scope, input); err != nil || len(events) != 1 {
		t.Fatalf("receipt replay duplicated event: events=%d err=%v", len(events), err)
	}
}

func TestRealtimeAppenderFailsClosedWithoutUnifiedPolicyRevision(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "before"})
	scope.PolicyRevision = 0
	events := []bitable.CommittedEvent{}
	repository.ConfigureCommittedEventAppenderFactory(func(*gorm.DB) (bitable.CommittedEventAppender, error) {
		return eventAppender{events: &events}, nil
	})
	if _, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(row.ID, field.ID, row.FieldRevisions[field.ID], "after"))); !errors.Is(err, bitable.ErrUnavailable) {
		t.Fatalf("missing unified policy revision did not fail closed: %v", err)
	}
}

func TestMutationFailsClosedWithoutGuardOrEventFactory(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "before"})
	input := batch(bootstrap.SchemaRevision, update(row.ID, field.ID, row.FieldRevisions[field.ID], "after"))
	repository.ConfigureCellMutationGuardFactory(nil)
	if _, err := repository.Execute(context.Background(), scope, input); !errors.Is(err, bitable.ErrUnavailable) {
		t.Fatalf("missing cell guard did not fail closed: %v", err)
	}
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return &mutationGuard{}, nil })
	repository.ConfigureCommittedEventAppenderFactory(nil)
	if _, err := repository.Execute(context.Background(), scope, input); !errors.Is(err, bitable.ErrUnavailable) {
		t.Fatalf("missing event appender did not fail closed: %v", err)
	}
}

func TestFieldTypeChangeRemainsUnsupportedAndOldCellClientIsRejected(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "before"})
	old := batch(bootstrap.SchemaRevision, bitable.Command{
		Kind: "update", RecordID: row.ID, ExpectedRevision: rev(row.Revision), Values: map[string]any{field.ID: "old-client"},
	})
	if _, err := repository.Execute(context.Background(), scope, old); !errors.Is(err, bitable.ErrInvalidInput) {
		t.Fatalf("old row-CAS client was not explicitly rejected: %v", err)
	}
	if _, err := repository.MutateField(context.Background(), scope, field.ID, bitable.FieldMutation{
		Name: field.Name, Type: bitable.FieldNumber, ExpectedSchemaRevision: bootstrap.SchemaRevision, OperationID: uuid.NewString(),
	}); !errors.Is(err, bitable.ErrInvalidInput) {
		t.Fatalf("unsupported field type change accepted: %v", err)
	}
}

func TestCellAffectingSchemaChangeRequiresEveryLeaseAndInvalidatesOldSchema(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	options := bitable.FieldOptions{Choices: []bitable.Choice{{ID: "one", Name: "One"}}}
	field, err := repository.MutateField(context.Background(), scope, "", bitable.FieldMutation{
		Name: "Choice", Type: bitable.FieldSingleSelect, Options: options,
		ExpectedSchemaRevision: bootstrap.SchemaRevision, OperationID: uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, _ = repository.Bootstrap(context.Background(), scope)
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "one"})
	changed := bitable.FieldOptions{Choices: []bitable.Choice{{ID: "one", Name: "Renamed"}}}
	mutation := bitable.FieldMutation{
		Name: field.Name, Type: field.Type, Options: changed, ExpectedSchemaRevision: bootstrap.SchemaRevision,
		OperationID: uuid.NewString(), CellContracts: []bitable.CellContract{cell(row.ID, field.ID, row.FieldRevisions[field.ID])},
	}
	guard := &mutationGuard{err: bitable.ErrCellLeaseLost}
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return guard, nil })
	if _, err = repository.MutateField(context.Background(), scope, field.ID, mutation); !errors.Is(err, bitable.ErrCellLeaseLost) {
		t.Fatalf("cell-affecting schema change bypassed lease guard: %v", err)
	}
	guard.err = nil
	if _, err = repository.MutateField(context.Background(), scope, field.ID, mutation); err != nil {
		t.Fatal("guarded schema change failed", err)
	}
	if _, err = repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		update(row.ID, field.ID, row.FieldRevisions[field.ID], "one"))); !errors.Is(err, bitable.ErrConflict) {
		t.Fatalf("old schema revision survived field change: %v", err)
	}
}

func TestCellGuardReceivesTrustedScopeAndStableBatchOrder(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	first := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "first"})
	second := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "second"})
	guard := &mutationGuard{}
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return guard, nil })
	commands := []bitable.Command{
		update(second.ID, field.ID, second.FieldRevisions[field.ID], "second-new"),
		update(first.ID, field.ID, first.FieldRevisions[field.ID], "first-new"),
	}
	if _, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision, commands...)); err != nil {
		t.Fatal(err)
	}
	if len(guard.calls) != 2 || guard.calls[0].RecordID > guard.calls[1].RecordID {
		t.Fatalf("guard lock order is not stable: %#v", guard.calls)
	}
	for _, proof := range guard.calls {
		if proof.WorkspaceID != scope.Principal.WorkspaceID || proof.ResourceID != scope.DocumentResourceID ||
			proof.TableID != scope.TableID || proof.FieldID != field.ID || proof.FencingToken != 1 {
			t.Fatalf("guard proof did not use trusted scope: %#v", proof)
		}
	}
}

func TestRowMutationConflictGuardIsTransactionBoundAndFailClosed(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	first := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "first"})
	second := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "second"})
	conflicts := &leaseConflictGuard{err: bitable.ErrCellLeaseConflict}
	repository.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) { return conflicts, nil })
	input := batch(bootstrap.SchemaRevision,
		update(first.ID, field.ID, first.FieldRevisions[field.ID], "must-rollback"),
		bitable.Command{Kind: "delete", RecordID: second.ID, ExpectedRevision: rev(second.Revision)},
	)
	if _, err := repository.Execute(context.Background(), scope, input); !errors.Is(err, bitable.ErrCellLeaseConflict) {
		t.Fatalf("active row lease did not block mutation: %v", err)
	}
	page, err := repository.Query(context.Background(), scope, query(bootstrap))
	if err != nil || page.Total != 2 || page.Records[0].Values[field.ID] != "first" {
		t.Fatalf("conflict guard did not preserve atomic rollback: %#v %v", page, err)
	}
	if len(conflicts.scopes) != 1 || conflicts.scopes[0].WorkspaceID != scope.Principal.WorkspaceID ||
		conflicts.scopes[0].ResourceID != scope.DocumentResourceID || conflicts.scopes[0].TableID != scope.TableID ||
		len(conflicts.scopes[0].RecordIDs) != 1 || conflicts.scopes[0].RecordIDs[0] != second.ID ||
		len(conflicts.scopes[0].FieldIDs) != 1 || conflicts.scopes[0].FieldIDs[0] != field.ID {
		t.Fatalf("conflict guard received untrusted/wrong scope: %#v", conflicts.scopes)
	}

	repository.ConfigureCellLeaseConflictGuardFactory(nil)
	if _, err = repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		bitable.Command{Kind: "move", RecordID: first.ID, ExpectedRevision: rev(first.Revision), Position: &bitable.CreatePosition{RecordID: second.ID, Direction: "before", ExpectedRevision: rev(second.Revision)}})); !errors.Is(err, bitable.ErrUnavailable) {
		t.Fatalf("missing row conflict guard did not fail closed: %v", err)
	}
}

func TestCompleteRowLeaseProofCoverageReusesCellGuard(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	secondField := addTextField(t, repository, scope, bootstrap.SchemaRevision, "Second")
	bootstrap, _ = repository.Bootstrap(context.Background(), scope)
	row := mustCreate(t, repository, scope, bootstrap, map[string]any{bootstrap.Fields[0].ID: "first", secondField.ID: "second"})
	cellGuard := &mutationGuard{}
	conflicts := &leaseConflictGuard{err: bitable.ErrCellLeaseConflict}
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return cellGuard, nil })
	repository.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) { return conflicts, nil })
	proofs := []bitable.CellLeaseProof{leaseProof(row.ID, secondField.ID), leaseProof(row.ID, bootstrap.Fields[0].ID)}
	if _, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision,
		bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(row.Revision), CellLeaseProofs: proofs})); err != nil {
		t.Fatal("complete owned proof coverage did not bypass conflict scan", err)
	}
	if len(cellGuard.calls) != len(bootstrap.Fields) || len(conflicts.scopes) != 0 {
		t.Fatalf("wrong complete-proof path: cell=%d conflict=%d", len(cellGuard.calls), len(conflicts.scopes))
	}

	// Partial coverage is ambiguous and must never fall through to a permissive
	// range scan or silently ignore the supplied proof.
	row = mustCreate(t, repository, scope, bootstrap, nil)
	partial := bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(row.Revision), CellLeaseProofs: proofs[:1]}
	partial.CellLeaseProofs[0].RecordID = row.ID
	if _, err := repository.Execute(context.Background(), scope, batch(bootstrap.SchemaRevision, partial)); !errors.Is(err, bitable.ErrInvalidInput) {
		t.Fatalf("partial row proof coverage accepted: %v", err)
	}
}

func TestRowMutationBatchRejectsMixedProofAndConflictScanLockStrategies(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	field := bootstrap.Fields[0]
	first := mustCreate(t, repository, scope, bootstrap, nil)
	second := mustCreate(t, repository, scope, bootstrap, nil)
	repository.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) {
		return &mutationGuard{}, nil
	})
	repository.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) {
		return &leaseConflictGuard{}, nil
	})
	input := batch(bootstrap.SchemaRevision,
		bitable.Command{Kind: "delete", RecordID: first.ID, ExpectedRevision: rev(first.Revision),
			CellLeaseProofs: []bitable.CellLeaseProof{leaseProof(first.ID, field.ID)}},
		bitable.Command{Kind: "delete", RecordID: second.ID, ExpectedRevision: rev(second.Revision)},
	)
	if _, err := repository.Execute(context.Background(), scope, input); !errors.Is(err, bitable.ErrInvalidInput) {
		t.Fatalf("mixed row lease strategies accepted: %v", err)
	}
}

func TestSchemaConflictGuardCoversEveryLiveCellWithoutProofs(t *testing.T) {
	repository, scope, bootstrap, _ := setup(t)
	options := bitable.FieldOptions{Choices: []bitable.Choice{{ID: "one", Name: "One"}}}
	field, err := repository.MutateField(context.Background(), scope, "", bitable.FieldMutation{
		Name: "Choice", Type: bitable.FieldSingleSelect, Options: options,
		ExpectedSchemaRevision: bootstrap.SchemaRevision, OperationID: uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, _ = repository.Bootstrap(context.Background(), scope)
	first := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "one"})
	second := mustCreate(t, repository, scope, bootstrap, map[string]any{field.ID: "one"})
	conflicts := &leaseConflictGuard{err: bitable.ErrCellLeaseConflict}
	repository.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) { return conflicts, nil })
	changed := bitable.FieldMutation{
		Name: field.Name, Type: field.Type,
		Options:                bitable.FieldOptions{Choices: []bitable.Choice{{ID: "one", Name: "Renamed"}}},
		ExpectedSchemaRevision: bootstrap.SchemaRevision, OperationID: uuid.NewString(),
	}
	if _, err = repository.MutateField(context.Background(), scope, field.ID, changed); !errors.Is(err, bitable.ErrCellLeaseConflict) {
		t.Fatalf("schema mutation ignored active lease range: %v", err)
	}
	if len(conflicts.scopes) != 1 || len(conflicts.scopes[0].RecordIDs) != 2 ||
		conflicts.scopes[0].RecordIDs[0] != first.ID && conflicts.scopes[0].RecordIDs[1] != first.ID ||
		conflicts.scopes[0].RecordIDs[0] != second.ID && conflicts.scopes[0].RecordIDs[1] != second.ID ||
		len(conflicts.scopes[0].FieldIDs) != 1 || conflicts.scopes[0].FieldIDs[0] != field.ID {
		t.Fatalf("schema guard range incomplete: %#v", conflicts.scopes)
	}
	current, _ := repository.Bootstrap(context.Background(), scope)
	if current.SchemaRevision != bootstrap.SchemaRevision {
		t.Fatal("blocked schema mutation committed")
	}
}
