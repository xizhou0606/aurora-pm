package native

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
)

func TestAttachmentsArePrivateOwnedAtomicAndReplaySafe(t *testing.T) {
	r, s, _, audit := setup(t)
	ctx := context.Background()
	field, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Files", Type: bitable.FieldAttachment, ExpectedSchemaRevision: 1, OperationID: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	row := mustCreate(t, r, s, b, map[string]any{})
	in := bitable.AssetAppend{RecordID: row.ID, FieldID: field.ID, OperationID: uuid.NewString(), ExpectedSchemaRevision: b.SchemaRevision, CellContract: cell(row.ID, field.ID, row.FieldRevisions[field.ID]), Asset: bitable.Asset{AttachmentRef: bitable.AttachmentRef{ID: uuid.NewString(), Name: "sample.png", Size: 4, MimeType: "image/png"}, ObjectKey: "private/test", ContentHash: "content", RecordID: row.ID, FieldID: field.ID}}
	result, e := r.AppendAttachment(ctx, s, in)
	if e != nil || result.Records[0].Revision != 2 {
		t.Fatal(e)
	}
	asset, e := r.GetAttachment(ctx, s, row.ID, field.ID, in.Asset.ID)
	if e != nil || asset.ObjectKey != "private/test" || asset.PreviewPath == "" {
		t.Fatal(e)
	}
	oldID := in.Asset.ID
	in.Asset.ID = uuid.NewString()
	in.Asset.ObjectKey = "orphan-on-replay"
	replay, e := r.AppendAttachment(ctx, s, in)
	if e != nil || replay.Records[0].Revision != 2 {
		t.Fatal(e)
	}
	var count int64
	r.db.Model(&AssetRecord{}).Count(&count)
	if count != 1 {
		t.Fatal("replay duplicated asset")
	}
	in.Asset.ContentHash = "different"
	if _, e = r.AppendAttachment(ctx, s, in); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal(e)
	}
	if _, e = r.GetAttachment(ctx, s, row.ID, b.Fields[0].ID, oldID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("cross field attachment exposed", e)
	}
	other := s
	other.Principal.WorkspaceID++
	if _, e = r.GetAttachment(ctx, other, row.ID, field.ID, oldID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("cross workspace attachment exposed", e)
	}
	in.OperationID = uuid.NewString()
	in.CellContract = cell(row.ID, field.ID, result.Records[0].FieldRevisions[field.ID])
	in.Asset.ContentHash = "next"
	audit.fail = true
	if _, e = r.AppendAttachment(ctx, s, in); e == nil {
		t.Fatal("audit failure accepted")
	}
	r.db.Model(&AssetRecord{}).Count(&count)
	if count != 1 {
		t.Fatal("asset metadata not rolled back")
	}
	audit.fail = false
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(2)})); e != nil {
		t.Fatal(e)
	}
	if _, e = r.GetAttachment(ctx, s, row.ID, field.ID, oldID); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("deleted attachment readable", e)
	}
}

func TestAuditReadonlyAndPeopleNormalizerOnRestore(t *testing.T) {
	r, s, _, _ := setup(t)
	ctx := context.Background()
	auditField, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Created", Type: bitable.FieldCreatedBy, ExpectedSchemaRevision: 1, OperationID: uuid.NewString()})
	if e != nil || !auditField.ReadOnly {
		t.Fatal(e)
	}
	user, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "User", Type: bitable.FieldUser, ExpectedSchemaRevision: 2, OperationID: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "create", Values: map[string]any{auditField.ID: nil}})); !errors.Is(e, bitable.ErrForbidden) {
		t.Fatal(e)
	}
	values := map[string]any{user.ID: []bitable.UserRef{{ID: 10, Name: "forged"}}}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "create", Values: values})); !errors.Is(e, bitable.ErrUnavailable) {
		t.Fatal("missing user normalizer opened", e)
	}
	revoked := false
	r.ConfigureValueNormalizer(func(_ context.Context, _ bitable.Principal, _ []bitable.Field, v map[string]any) (map[string]any, error) {
		if revoked {
			return nil, bitable.ErrForbidden
		}
		v[user.ID] = []bitable.UserRef{{ID: 10, Name: "canonical"}}
		return v, nil
	})
	input := batch(b.SchemaRevision, bitable.Command{Kind: "create", Values: values})
	result, e := r.Execute(ctx, s, input)
	if e != nil {
		t.Fatal(e)
	}
	row := result.Records[0]
	input.Commands[0].Values[user.ID] = []bitable.UserRef{{ID: 10, Name: "renamed"}}
	replay, e := r.Execute(ctx, s, input)
	if e != nil || replay.Records[0].ID != row.ID {
		t.Fatal("canonical display name broke identity replay", e)
	}
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "delete", RecordID: row.ID, ExpectedRevision: rev(1)})); e != nil {
		t.Fatal(e)
	}
	revoked = true
	if _, e = r.Execute(ctx, s, batch(b.SchemaRevision, bitable.Command{Kind: "restore", RecordID: row.ID, ExpectedRevision: rev(2)})); !errors.Is(e, bitable.ErrForbidden) {
		t.Fatal("revoked user restored", e)
	}
}

func TestReceiptReplaySurvivesSchemaChange(t *testing.T) {
	r, s, b, _ := setup(t)
	ctx := context.Background()
	input := batch(1, bitable.Command{Kind: "create", Values: map[string]any{b.Fields[0].ID: "saved"}})
	saved, e := r.Execute(ctx, s, input)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Later", Type: bitable.FieldCheckbox, ExpectedSchemaRevision: 1, OperationID: uuid.NewString()}); e != nil {
		t.Fatal(e)
	}
	replay, e := r.Execute(ctx, s, input)
	if e != nil || replay.Records[0].ID != saved.Records[0].ID {
		t.Fatal("schema changed breaks replay", e)
	}
}
