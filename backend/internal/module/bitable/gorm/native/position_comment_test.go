package native

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func exercisePositions(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap) {
	t.Helper()
	ctx := context.Background()
	f := b.Fields[0].ID
	anchor := mustCreate(t, r, s, b, map[string]any{f: "anchor"})
	tail := mustCreate(t, r, s, b, map[string]any{f: "tail"})
	create := func(text, direction string) bitable.Command {
		return bitable.Command{Kind: "create", Values: map[string]any{f: text}, Position: &bitable.CreatePosition{RecordID: anchor.ID, Direction: direction, ExpectedRevision: rev(anchor.Revision)}}
	}
	in := batch(b.SchemaRevision, create("after1", "after"), create("after2", "after"), create("before1", "before"), create("before2", "before"))
	result, e := r.Execute(ctx, s, in)
	if e != nil {
		t.Fatal(e)
	}
	replay, e := r.Execute(ctx, s, in)
	if e != nil || !reflect.DeepEqual(replay, result) {
		t.Fatal("position receipt replay", e)
	}
	names := func() []any {
		page, e := r.Query(ctx, s, query(b))
		if e != nil {
			t.Fatal(e)
		}
		v := []any{}
		for _, row := range page.Records {
			v = append(v, row.Values[f])
		}
		return v
	}
	expected := []any{"before1", "before2", "anchor", "after1", "after2", "tail"}
	if got := names(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("order=%v", got)
	}
	broken := create("rollback", "before")
	stale := create("stale", "after")
	stale.Position.ExpectedRevision = rev(99)
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, broken, stale)); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	if got := names(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("shift rollback=%v", got)
	}
	removed, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: result.Records[0].ID, ExpectedRevision: rev(1)}))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "restore", RecordID: removed.Records[0].ID, ExpectedRevision: rev(removed.Records[0].Revision)})); e != nil {
		t.Fatal(e)
	}
	if got := names(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("restore order=%v", got)
	}
	foreign := create("foreign", "after")
	foreign.Position.RecordID = uuid.NewString()
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, foreign)); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: tail.ID, ExpectedRevision: rev(1)})); e != nil {
		t.Fatal(e)
	}
	foreign.Position.RecordID = tail.ID
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, foreign)); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
}
func TestAdjacentInsertOrderCASRollbackRestore(t *testing.T) {
	r, s, b, _ := setup(t)
	exercisePositions(t, r, s, b)
}
func TestPostgresAdjacentInsertOrderCASRollbackRestore(t *testing.T) {
	r, s, b := postgresRepository(t)
	exercisePositions(t, r, s, b)
}

func exerciseComments(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap) {
	t.Helper()
	ctx := context.Background()
	row := mustCreate(t, r, s, b, nil)
	input := bitable.CommentInput{OperationID: uuid.NewString(), Body: "first line\n第二行"}
	first, e := r.CreateComment(ctx, s, row.ID, input)
	if e != nil {
		t.Fatal(e)
	}
	again, e := r.CreateComment(ctx, s, row.ID, input)
	if e != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("comment replay", e)
	}
	if first.Author.ID != s.Principal.UserID || first.Body != input.Body {
		t.Fatal("comment projection")
	}
	changed := input
	changed.Body = "different"
	if _, e = r.CreateComment(ctx, s, row.ID, changed); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	other := s
	other.Principal.UserID++
	if _, e = r.CreateComment(ctx, other, row.ID, input); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	other = s
	other.Principal.WorkspaceID++
	if _, e = r.ListComments(ctx, other, row.ID, 0, 10); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
	page, e := r.ListComments(ctx, s, row.ID, 0, 1)
	if e != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatal(page, e)
	}
	page, e = r.ListComments(ctx, s, row.ID, 1, 1)
	if e != nil || page.Total != 1 || len(page.Items) != 0 || page.Offset != 1 {
		t.Fatal(page, e)
	}
	removed, e := r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(1)}))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.ListComments(ctx, s, row.ID, 0, 10); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e = r.CreateComment(ctx, s, row.ID, input); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "restore", RecordID: row.ID, ExpectedRevision: rev(removed.Records[0].Revision)})); e != nil {
		t.Fatal(e)
	}
	page, e = r.ListComments(ctx, s, row.ID, 0, 10)
	if e != nil || page.Total != 1 {
		t.Fatal(page, e)
	}
}
func TestRecordCommentsIsolationReceiptAndDeletedRows(t *testing.T) {
	r, s, b, _ := setup(t)
	exerciseComments(t, r, s, b)
}
func TestPostgresRecordCommentsIsolationReceiptAndDeletedRows(t *testing.T) {
	r, s, b := postgresRepository(t)
	exerciseComments(t, r, s, b)
}
func TestCommentAuditFailureRollsBack(t *testing.T) {
	r, s, b, audit := setup(t)
	row := mustCreate(t, r, s, b, nil)
	audit.fail = true
	in := bitable.CommentInput{OperationID: uuid.NewString(), Body: "rollback"}
	if _, e := r.CreateComment(context.Background(), s, row.ID, in); e == nil {
		t.Fatal("audit bypass")
	}
	var comments, receipts int64
	r.db.Model(&CommentRecord{}).Count(&comments)
	r.db.Model(&ReceiptRecord{}).Where("operation_id = ?", in.OperationID).Count(&receipts)
	if comments != 0 || receipts != 0 {
		t.Fatal("comment/receipt committed without audit")
	}
}
