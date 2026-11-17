package native

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recorder struct {
	fail  bool
	calls int
}

type mutationGuard struct {
	err   error
	calls []bitable.CellMutationProof
}

type leaseConflictGuard struct {
	err    error
	scopes []bitable.CellLeaseConflictScope
}

func (g *leaseConflictGuard) RequireNoConflictingCellLeases(_ context.Context, _ bitable.Principal, scopes []bitable.CellLeaseConflictScope) error {
	g.scopes = append(g.scopes, scopes...)
	return g.err
}

type discardEventAppender struct{}

func (discardEventAppender) AppendBitableCommittedEvent(context.Context, bitable.CommittedEvent) error {
	return nil
}

func (g *mutationGuard) VerifyCellMutation(_ context.Context, _ bitable.Principal, proof bitable.CellMutationProof) error {
	g.calls = append(g.calls, proof)
	return g.err
}

func (r *recorder) RecordBitableMutation(context.Context, bitable.MutationEffect) error {
	r.calls++
	if r.fail {
		return errors.New("audit unavailable")
	}
	return nil
}
func setup(t *testing.T) (*Repository, bitable.Scope, bitable.Bootstrap, *recorder) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sql, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sql.Close() })
	if e = db.AutoMigrate(MigrationRecords()...); e != nil {
		t.Fatal(e)
	}
	repo := NewRepository(db)
	audit := &recorder{}
	guard := &mutationGuard{}
	repo.ConfigureMutationRecorderFactory(func(*gorm.DB) (bitable.MutationRecorder, error) { return audit, nil })
	repo.ConfigureCellMutationGuardFactory(func(*gorm.DB) (bitable.CellMutationGuard, error) { return guard, nil })
	repo.ConfigureCellLeaseConflictGuardFactory(func(*gorm.DB) (bitable.CellLeaseConflictGuard, error) { return &leaseConflictGuard{}, nil })
	repo.ConfigureCommittedEventAppenderFactory(func(*gorm.DB) (bitable.CommittedEventAppender, error) { return discardEventAppender{}, nil })
	p := bitable.Principal{UserID: 10, WorkspaceID: 20}
	resource, e := repo.Create(context.Background(), p, "Synthetic", uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	scope := bitable.Scope{Principal: p, ResourceID: resource.ID, DocumentResourceID: "document-" + resource.ID, PolicyRevision: 1, TableID: resource.Tables[0].ID}
	boot, e := repo.Bootstrap(context.Background(), scope)
	if e != nil {
		t.Fatal(e)
	}
	return repo, scope, boot, audit
}
func batch(schema uint64, commands ...bitable.Command) bitable.CommandBatch {
	return bitable.CommandBatch{OperationID: uuid.NewString(), ExpectedSchemaRevision: schema, Commands: commands, Atomic: true}
}
func rev(value uint64) *uint64 { return &value }
func cell(recordID, fieldID string, base uint64) bitable.CellContract {
	return bitable.CellContract{
		RecordID: recordID, FieldID: fieldID, BaseRevision: base, LeaseID: uuid.NewString(), FencingToken: 1,
		ConnectionID: "connection-1", EditorInstanceID: "editor-1",
	}
}
func leaseProof(recordID, fieldID string) bitable.CellLeaseProof {
	return bitable.CellLeaseProof{
		RecordID: recordID, FieldID: fieldID, LeaseID: uuid.NewString(), FencingToken: 1,
		ConnectionID: "connection-1", EditorInstanceID: "editor-1",
	}
}
func update(recordID, fieldID string, base uint64, value any) bitable.Command {
	return bitable.Command{Kind: "update", RecordID: recordID, Values: map[string]any{fieldID: value}, CellContracts: []bitable.CellContract{cell(recordID, fieldID, base)}}
}
func mustCreate(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap, values map[string]any) bitable.Record {
	t.Helper()
	result, e := r.Execute(context.Background(), s, batch(b.SchemaRevision, bitable.Command{Kind: "create", Values: values}))
	if e != nil {
		t.Fatal(e)
	}
	return result.Records[0]
}
func query(b bitable.Bootstrap) bitable.Query { return bitable.Query{ViewID: b.Views[0].ID, Limit: 10} }

func TestAtomicCreateAndWorkspaceIsolation(t *testing.T) {
	r, s, _, audit := setup(t)
	ctx := context.Background()
	key := uuid.NewString()
	first, e := r.Create(ctx, s.Principal, "New", key)
	if e != nil {
		t.Fatal(e)
	}
	second, e := r.Create(ctx, s.Principal, "New", key)
	if e != nil || first.ID != second.ID {
		t.Fatalf("replay %v", e)
	}
	if _, e = r.Create(ctx, s.Principal, "Different", key); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	if _, e = r.Get(ctx, 999, first.ID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
	audit.fail = true
	if _, e = r.Create(ctx, s.Principal, "Rollback", uuid.NewString()); e == nil {
		t.Fatal("audit failure committed")
	}
	var count int64
	r.db.Model(&ResourceRecord{}).Where("name = ?", "Rollback").Count(&count)
	if count != 0 {
		t.Fatal("resource not rolled back")
	}
}
func TestBatchCASReceiptAndRollback(t *testing.T) {
	r, s, b, _ := setup(t)
	ctx := context.Background()
	f := b.Fields[0].ID
	row := mustCreate(t, r, s, b, map[string]any{f: "before"})
	input := batch(b.SchemaRevision, update(row.ID, f, row.FieldRevisions[f], "after"), bitable.Command{Kind: "create", Values: map[string]any{f: "second"}})
	result, e := r.Execute(ctx, s, input)
	if e != nil || len(result.Records) != 2 {
		t.Fatalf("batch %v", e)
	}
	replay, e := r.Execute(ctx, s, input)
	if e != nil || replay.Records[1].ID != result.Records[1].ID {
		t.Fatalf("replay %v", e)
	}
	input.Commands[0].Values[f] = "changed"
	if _, e = r.Execute(ctx, s, input); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	stale := batch(b.SchemaRevision, bitable.Command{Kind: "create", Values: map[string]any{f: "never"}}, update(row.ID, f, row.FieldRevisions[f], "bad"))
	if _, e = r.Execute(ctx, s, stale); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	page, e := r.Query(ctx, s, query(b))
	if e != nil || page.Total != 2 {
		t.Fatalf("rollback total %d %v", page.Total, e)
	}
	if _, e = r.Operation(ctx, s, stale.OperationID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("failed batch receipt exists")
	}
	s.Principal.UserID++
	if _, e = r.Operation(ctx, s, result.ID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("other actor receipt leaked")
	}
}
func TestStableDeleteRestoreAndAuditRollback(t *testing.T) {
	r, s, b, audit := setup(t)
	ctx := context.Background()
	row := mustCreate(t, r, s, b, map[string]any{b.Fields[0].ID: "saved"})
	deleted, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(1)}))
	if e != nil || deleted.Records[0].Revision != 2 {
		t.Fatal(e)
	}
	page, _ := r.Query(ctx, s, query(b))
	if page.Total != 0 {
		t.Fatal("deleted row visible")
	}
	restored, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "restore", RecordID: row.ID, ExpectedRevision: rev(2)}))
	if e != nil || restored.Records[0].ID != row.ID || restored.Records[0].Revision != 3 {
		t.Fatal(e)
	}
	audit.fail = true
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(3)})); e == nil {
		t.Fatal("missing audit")
	}
	page, _ = r.Query(ctx, s, query(b))
	if page.Total != 1 || page.Records[0].Revision != 3 {
		t.Fatal("audit rollback failed")
	}
}
func TestSchemaAndViewsPersistAndValidateIDs(t *testing.T) {
	r, s, b, _ := setup(t)
	ctx := context.Background()
	input := bitable.FieldMutation{Name: "Count", Type: bitable.FieldNumber, ExpectedSchemaRevision: 1, OperationID: uuid.NewString()}
	field, e := r.MutateField(ctx, s, "", input)
	if e != nil {
		t.Fatal(e)
	}
	same, e := r.MutateField(ctx, s, "", input)
	if e != nil || same.ID != field.ID {
		t.Fatal(e)
	}
	input.OperationID = uuid.NewString()
	if _, e = r.MutateField(ctx, s, "", input); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	boot, e := r.Bootstrap(ctx, s)
	if e != nil || boot.SchemaRevision != 2 || len(boot.Fields) != 2 {
		t.Fatal(e)
	}
	viewInput := bitable.ViewMutation{Name: "Sorted", Kind: "grid", OperationID: uuid.NewString(), Config: b.Views[0].Config}
	viewInput.Config.Sorts = []bitable.Sort{{FieldID: field.ID, Direction: "desc"}}
	viewInput.Config.ColumnOrder = []string{field.ID, b.Fields[0].ID}
	view, e := r.MutateView(ctx, s, "", viewInput)
	if e != nil {
		t.Fatal(e)
	}
	exportQuery := query(boot)
	exportQuery.ViewID = view.ID
	snapshot, e := r.Export(ctx, s, bitable.ExportQuery{Query: exportQuery})
	if e != nil || len(snapshot.Fields) != 2 || snapshot.Fields[0].ID != field.ID {
		t.Fatalf("export must preserve view column order: %+v %v", snapshot.Fields, e)
	}
	viewInput.OperationID = uuid.NewString()
	viewInput.ExpectedRevision = rev(0)
	if _, e = r.MutateView(ctx, s, view.ID, viewInput); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	viewInput.OperationID = uuid.NewString()
	viewInput.ExpectedRevision = nil
	viewInput.Config.HiddenFieldIDs = []string{"missing"}
	if _, e = r.MutateView(ctx, s, "", viewInput); !errors.Is(e, bitable.ErrInvalidInput) {
		t.Fatal(e)
	}
}
func TestSearchAndExportAreNotPageLimitedAndLocateHonorsFilter(t *testing.T) {
	r, s, b, _ := setup(t)
	ctx := context.Background()
	field := b.Fields[0].ID
	commands := []bitable.Command{}
	for i := 0; i < 25; i++ {
		commands = append(commands, bitable.Command{Kind: "create", Values: map[string]any{field: fmt.Sprintf("row-%02d", i)}})
	}
	result, e := r.Execute(ctx, s, batch(1, commands...))
	if e != nil {
		t.Fatal(e)
	}
	q := query(b)
	q.Limit = 5
	searched, e := r.Search(ctx, s, bitable.Search{Query: q, Text: "row-24"})
	if e != nil || searched.Total != 1 || searched.Matches[0].Ordinal != 24 {
		t.Fatalf("search %+v %v", searched, e)
	}
	q.LocateRecordID = result.Records[24].ID
	page, e := r.Query(ctx, s, q)
	if e != nil || page.Offset != 20 || len(page.Records) != 5 {
		t.Fatalf("locate %+v %v", page, e)
	}
	q.Filters = []bitable.Filter{{FieldID: field, Operator: "eq", Value: "row-00"}}
	if _, e = r.Query(ctx, s, q); !errors.Is(e, bitable.ErrRecordFilteredOut) {
		t.Fatal(e)
	}
	q.LocateRecordID = ""
	snapshot, e := r.Export(ctx, s, bitable.ExportQuery{Query: q, All: true})
	if e != nil || len(snapshot.Records) != 25 {
		t.Fatal(e)
	}
	filtered, e := r.Export(ctx, s, bitable.ExportQuery{Query: q})
	if e != nil || len(filtered.Records) != 1 {
		t.Fatal(e)
	}
	s.Principal.WorkspaceID++
	if _, e = r.Query(ctx, s, q); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("cross workspace read", e)
	}
}
