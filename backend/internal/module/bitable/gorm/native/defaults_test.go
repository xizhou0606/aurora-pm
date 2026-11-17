package native

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func exerciseDefaults(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap) {
	t.Helper()
	ctx := context.Background()
	old := mustCreate(t, r, s, b, map[string]any{b.Fields[0].ID: "old"})
	options := bitable.FieldOptions{Choices: []bitable.Choice{{ID: "a", Name: "A", Color: "blue"}, {ID: "b", Name: "B", Color: "green"}}, DefaultValue: "a"}
	field, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Choice", Type: bitable.FieldSingleSelect, Options: options, ExpectedSchemaRevision: b.SchemaRevision, OperationID: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	b, e = r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	page, e := r.Query(ctx, s, query(b))
	if e != nil || page.Records[0].Values[field.ID] != nil {
		t.Fatal("new schema backfilled old row", e)
	}
	blank := batch(b.SchemaRevision, bitable.Command{Kind: "create"}, bitable.Command{Kind: "create", Values: map[string]any{field.ID: nil}}, bitable.Command{Kind: "create", Values: map[string]any{field.ID: "b"}})
	created, e := r.Execute(ctx, s, blank)
	if e != nil {
		t.Fatal(e)
	}
	if created.Records[0].Values[field.ID] != "a" || created.Records[1].Values[field.ID] != nil || created.Records[2].Values[field.ID] != "b" {
		t.Fatal("defaults overwrote explicit values")
	}
	imported := batch(b.SchemaRevision, bitable.Command{Kind: "create"}, bitable.Command{Kind: "create", Values: map[string]any{field.ID: "b"}})
	imported.Importing = true
	im, e := r.Execute(ctx, s, imported)
	if e != nil || im.Records[0].Values[field.ID] != nil || im.Records[1].Values[field.ID] != "b" {
		t.Fatal("import preview changed", e)
	}
	options.DefaultValue = "b"
	options.Choices[0].Color = "purple"
	page, e = r.Query(ctx, s, query(b))
	if e != nil {
		t.Fatal(e)
	}
	contracts := make([]bitable.CellContract, 0, len(page.Records))
	for _, record := range page.Records {
		contracts = append(contracts, cell(record.ID, field.ID, record.FieldRevisions[field.ID]))
	}
	field, e = r.MutateField(ctx, s, field.ID, bitable.FieldMutation{Name: field.Name, Type: field.Type, Options: options, ExpectedSchemaRevision: b.SchemaRevision, OperationID: uuid.NewString(), CellContracts: contracts})
	if e != nil {
		t.Fatal(e)
	}
	b, e = r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	if b.Fields[1].Options.Choices[0].Color != "purple" || b.Fields[1].Options.DefaultValue != "b" {
		t.Fatal("options not persisted")
	}
	replay, e := r.Execute(ctx, s, blank)
	if e != nil || !reflect.DeepEqual(replay, created) {
		t.Fatal("changed default corrupted old receipt", e)
	}
	newer := mustCreate(t, r, s, b, nil)
	if newer.Values[field.ID] != "b" {
		t.Fatal("new default ignored")
	}
	updated, e := r.Execute(ctx, s, batch(b.SchemaRevision, update(old.ID, b.Fields[0].ID, old.FieldRevisions[b.Fields[0].ID], "updated")))
	if e != nil || updated.Records[0].Values[field.ID] != nil {
		t.Fatal("update backfilled default", e)
	}
	removed, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: created.Records[1].ID, ExpectedRevision: rev(1)}))
	if e != nil {
		t.Fatal(e)
	}
	restored, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "restore", RecordID: created.Records[1].ID, ExpectedRevision: rev(removed.Records[0].Revision)}))
	if e != nil || restored.Records[0].Values[field.ID] != nil {
		t.Fatal("restore backfilled default", e)
	}
	multi, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Many", Type: bitable.FieldMultipleSelect, Options: bitable.FieldOptions{Choices: options.Choices, DefaultValue: []string{"a", "b"}}, ExpectedSchemaRevision: b.SchemaRevision, OperationID: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	b, e = r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	newer = mustCreate(t, r, s, b, nil)
	if !reflect.DeepEqual(newer.Values[multi.ID], []any{"a", "b"}) {
		t.Fatal("multiple default not applied", newer.Values)
	}
}
func TestChoiceDefaultsCreateOnlyAndReceipts(t *testing.T) {
	r, s, b, _ := setup(t)
	exerciseDefaults(t, r, s, b)
}
func TestPostgresChoiceDefaultsCreateOnlyAndReceipts(t *testing.T) {
	r, s, b := postgresRepository(t)
	exerciseDefaults(t, r, s, b)
}
func TestAppliedDefaultsRespectAtomicCellBudget(t *testing.T) {
	r, s, b, _ := setup(t)
	ctx := context.Background()
	for i := 0; i < 21; i++ {
		_, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: fmt.Sprintf("Choice %d", i), Type: bitable.FieldSingleSelect, Options: bitable.FieldOptions{Choices: []bitable.Choice{{ID: "a", Name: "A"}}, DefaultValue: "a"}, ExpectedSchemaRevision: b.SchemaRevision, OperationID: uuid.NewString()})
		if e != nil {
			t.Fatal(e)
		}
		b, e = r.Bootstrap(ctx, s)
		if e != nil {
			t.Fatal(e)
		}
	}
	commands := make([]bitable.Command, 100)
	for i := range commands {
		commands[i] = bitable.Command{Kind: "create"}
	}
	_, e := r.Execute(ctx, s, batch(b.SchemaRevision, commands...))
	if !errors.Is(e, bitable.ErrBudgetExceeded) {
		t.Fatal(e)
	}
	page, e := r.Query(ctx, s, query(b))
	if e != nil || page.Total != 0 {
		t.Fatal("over-budget defaults partially committed", e)
	}
}
