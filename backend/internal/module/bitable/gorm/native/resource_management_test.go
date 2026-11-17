package native

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/bitable"
	"gorm.io/gorm"
	"testing"
)

func TestResourceManagement(t *testing.T) {
	r, s, b, audit := setup(t)
	testResourceManagement(t, r, s, b, audit)
}
func TestPostgresResourceManagement(t *testing.T) {
	r, s, b := postgresRepository(t)
	audit := &recorder{}
	r.ConfigureMutationRecorderFactory(func(*gorm.DB) (bitable.MutationRecorder, error) { return audit, nil })
	testResourceManagement(t, r, s, b, audit)
}
func testResourceManagement(t *testing.T, r *Repository, s bitable.Scope, b bitable.Bootstrap, audit *recorder) {
	ctx := context.Background()
	row := mustCreate(t, r, s, b, map[string]any{b.Fields[0].ID: "source"})
	in := bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 1, Name: "Renamed"}
	v, e := r.MutateResource(ctx, s.Principal, s.ResourceID, "name", in)
	if e != nil || v.Name != "Renamed" || v.Revision != 2 {
		t.Fatalf("rename: %+v %v", v, e)
	}
	replay, e := r.MutateResource(ctx, s.Principal, s.ResourceID, "name", in)
	if e != nil || replay.Revision != 2 {
		t.Fatal("replay", e)
	}
	in.OperationID = uuid.NewString()
	if _, e = r.MutateResource(ctx, s.Principal, s.ResourceID, "name", in); !errors.Is(e, bitable.ErrConflict) {
		t.Fatal("stale", e)
	}
	yes := true
	pref := bitable.ResourceMutation{OperationID: uuid.NewString(), Pinned: &yes}
	v, e = r.MutateResource(ctx, s.Principal, s.ResourceID, "pin", pref)
	if e != nil || !v.Pinned || v.PreferenceRevision != 1 {
		t.Fatal("pin", e)
	}
	other := s.Principal
	other.UserID++
	p, e := r.ResourcePreference(ctx, other, s.ResourceID)
	if e != nil || p.Pinned {
		t.Fatal("personal isolation", e)
	}
	copy, e := r.MutateResource(ctx, s.Principal, s.ResourceID, "duplicate", bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 2})
	if e != nil || copy.ID == s.ResourceID || len(copy.Tables) != 1 {
		t.Fatal("copy", e)
	}
	ns := s
	ns.ResourceID = copy.ID
	ns.TableID = copy.Tables[0].ID
	nb, e := r.Bootstrap(ctx, ns)
	if e != nil {
		t.Fatal(e)
	}
	page, e := r.Query(ctx, ns, query(nb))
	if e != nil || len(page.Records) != 1 || page.Records[0].ID == row.ID || page.Records[0].Values[nb.Fields[0].ID] != "source" {
		t.Fatal("copy rows", page, e)
	}
	link, e := r.MutateShortcut(ctx, s.Principal, s.ResourceID, "shortcut", bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 2})
	if e != nil || link.TargetResourceID != s.ResourceID {
		t.Fatal("link", e)
	}
	audit.fail = true
	if _, e = r.MutateResource(ctx, s.Principal, s.ResourceID, "delete", bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 2}); e == nil {
		t.Fatal("audit failure accepted")
	}
	audit.fail = false
	if _, e = r.Get(ctx, s.Principal.WorkspaceID, s.ResourceID); e != nil {
		t.Fatal("delete not rolled back", e)
	}
	del := bitable.ResourceMutation{OperationID: uuid.NewString(), ExpectedRevision: 2}
	if _, e = r.MutateResource(ctx, s.Principal, s.ResourceID, "delete", del); e != nil {
		t.Fatal(e)
	}
	if _, e = r.MutateResource(ctx, s.Principal, s.ResourceID, "delete", del); e != nil {
		t.Fatal("delete replay", e)
	}
	if _, e = r.Bootstrap(ctx, s); !errors.Is(e, bitable.ErrNotFound) {
		t.Fatal("deleted table readable", e)
	}
	links, e := r.Shortcuts(ctx, s.Principal)
	if e != nil || len(links) != 0 {
		t.Fatal("deleted target links", e)
	}
	if _, e = r.Get(ctx, s.Principal.WorkspaceID, copy.ID); e != nil {
		t.Fatal("copy lost", e)
	}
}
