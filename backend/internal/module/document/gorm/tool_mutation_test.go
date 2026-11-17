package documentgorm

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"strings"
	"testing"
)

func TestDocumentToolReceiptAtomicityAndIdentity(t *testing.T) {
	db := openDocumentDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	p := document.Principal{WorkspaceID: 7, UserID: 9}
	calls := 0
	apply := func(store document.Store, _ document.MutationRecorder) (json.RawMessage, error) {
		calls++
		space, err := store.EnsureDefaultSpace(ctx, 7)
		if err != nil {
			return nil, err
		}
		return json.Marshal(space)
	}
	first, err := repo.ApplyToolMutation(ctx, p, "changeset-001", strings.Repeat("a", 64), apply)
	if err != nil {
		t.Fatal(err)
	}
	again, err := repo.ApplyToolMutation(ctx, p, "changeset-001", strings.Repeat("a", 64), apply)
	if err != nil || calls != 1 || string(first) != string(again) {
		t.Fatalf("replay calls=%d err=%v", calls, err)
	}
	for _, other := range []struct {
		p      document.Principal
		digest string
	}{{document.Principal{WorkspaceID: 7, UserID: 10}, strings.Repeat("a", 64)}, {p, strings.Repeat("b", 64)}} {
		if _, err = repo.ApplyToolMutation(ctx, other.p, "changeset-001", other.digest, apply); !errors.Is(err, document.ErrInvalidInput) {
			t.Fatal(err)
		}
	}
	_, err = repo.ApplyToolMutation(ctx, p, "rollback-001", strings.Repeat("a", 64), func(store document.Store, _ document.MutationRecorder) (json.RawMessage, error) {
		_, err := store.EnsureDefaultSpace(ctx, 88)
		if err != nil {
			return nil, err
		}
		return nil, document.ErrForbidden
	})
	if !errors.Is(err, document.ErrForbidden) {
		t.Fatal(err)
	}
	var count int64
	db.Model(&Space{}).Where("workspace_id = ?", 88).Count(&count)
	if count != 0 {
		t.Fatal("domain write escaped rollback")
	}
	db.Model(&ToolMutation{}).Where("apply_key = ?", "rollback-001").Count(&count)
	if count != 0 {
		t.Fatal("failed receipt persisted")
	}
}
