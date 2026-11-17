package native

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"testing"
)

func TestRowMove(t *testing.T)         { r, s, b, _ := setup(t); testRowMove(t, r, s, b) }
func TestPostgresRowMove(t *testing.T) { r, s, b := postgresRepository(t); testRowMove(t, r, s, b) }
func testRowMove(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap) {
	ctx := context.Background()
	a := mustCreate(t, r, s, b, nil)
	middle := mustCreate(t, r, s, b, nil)
	z := mustCreate(t, r, s, b, nil)
	command := bitable.Command{Kind: "move", RecordID: z.ID, ExpectedRevision: rev(1), Position: &bitable.CreatePosition{RecordID: a.ID, Direction: "before", ExpectedRevision: rev(1)}}
	input := batch(b.SchemaRevision, command)
	out, e := r.Execute(ctx, s, input)
	if e != nil || len(out.Moves) != 1 || out.Moves[0].Before.RecordID != middle.ID {
		t.Fatal("move", out, e)
	}
	page, e := r.Query(ctx, s, query(b))
	if e != nil || page.Records[0].ID != z.ID {
		t.Fatal("order", page, e)
	}
	replay, e := r.Execute(ctx, s, input)
	if e != nil || replay.Records[0].Revision != 2 {
		t.Fatal("replay", e)
	}
	undo := bitable.Command{Kind: "move", RecordID: z.ID, ExpectedRevision: rev(2), Position: &out.Moves[0].Before}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, undo)); e != nil {
		t.Fatal("undo", e)
	}
	page, e = r.Query(ctx, s, query(b))
	if e != nil || page.Records[2].ID != z.ID {
		t.Fatal("undo order", e)
	}
	command.ExpectedRevision = rev(3)
	command.Position.ExpectedRevision = rev(999)
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, command)); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal("stale anchor", e)
	}
}
