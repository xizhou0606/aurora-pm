package native

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"testing"
)

func TestPostgresResourceCopyAttachmentForeignKey(t *testing.T) {
	r, s, _ := postgresRepository(t)
	ctx := context.Background()
	// AutoMigrate models intentionally do not own relationship traversal. Reuse
	// the migration's actual FK in this isolated random test schema.
	if e := r.db.Exec("ALTER TABLE pm_bitable_assets ADD CONSTRAINT test_asset_record_fk FOREIGN KEY (record_id) REFERENCES pm_bitable_records(id)").Error; e != nil {
		t.Fatal(e)
	}
	f, e := r.MutateField(ctx, s, "", bitable.FieldMutation{Name: "Files", Type: bitable.FieldAttachment, ExpectedSchemaRevision: 1, OperationID: uuid.NewString()})
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.Bootstrap(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	row := mustCreate(t, r, s, b, nil)
	in := bitable.AssetAppend{RecordID: row.ID, FieldID: f.ID, OperationID: uuid.NewString(), ExpectedSchemaRevision: b.SchemaRevision, CellContract: cell(row.ID, f.ID, row.FieldRevisions[f.ID]), Asset: bitable.Asset{AttachmentRef: bitable.AttachmentRef{ID: uuid.NewString(), Name: "file.png", Size: 4, MimeType: "image/png"}, ObjectKey: "immutable/object", ContentHash: "hash", RecordID: row.ID, FieldID: f.ID}}
	if _, e = r.AppendAttachment(ctx, s, in); e != nil {
		t.Fatal(e)
	}
	copied, e := r.MutateResource(ctx, s.Principal, s.ResourceID, "duplicate", bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 1})
	if e != nil {
		t.Fatal(e)
	}
	ns := s
	ns.ResourceID = copied.ID
	ns.TableID = copied.Tables[0].ID
	nb, e := r.Bootstrap(ctx, ns)
	if e != nil {
		t.Fatal(e)
	}
	q, e := r.Query(ctx, ns, query(nb))
	if e != nil {
		t.Fatal(e)
	}
	var refs []bitable.AttachmentRef
	data, _ := json.Marshal(q.Records[0].Values[nb.Fields[1].ID])
	if e = json.Unmarshal(data, &refs); e != nil || len(refs) != 1 {
		t.Fatal(e)
	}
	asset, e := r.GetAttachment(ctx, ns, q.Records[0].ID, nb.Fields[1].ID, refs[0].ID)
	if e != nil || asset.ID == in.Asset.ID || asset.ObjectKey != "immutable/object" {
		t.Fatal("copy attachment", asset, e)
	}
}
