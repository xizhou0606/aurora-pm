package resourceaccessgorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"strings"
	"testing"
	"time"
)

func TestSingleShareLinkUpdatesRulesWithoutChangingSecret(t *testing.T) {
	db := testDB(t)
	now := time.Now().UTC()
	seedPolicy(t, db, "dr_single", resourceaccess.ModePrivate, nil, now)
	repo := NewRepository(db)
	ctx := context.Background()
	input := resourceaccess.LinkMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: "dr_single", LinkID: "lnk_first", SecretHash: hashString("first"), Level: resourceaccess.LevelViewer, ExpiresAt: timePointer(now.Add(time.Hour)), Now: now, IdempotencyKey: "first", InputDigest: strings.Repeat("a", 64)}
	first, err := repo.CreateShareLink(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.LinkID, input.SecretHash, input.IdempotencyKey = "lnk_unused", hashString("unused"), "update"
	input.Level, input.ExpiresAt = resourceaccess.LevelEditor, nil
	second, err := repo.CreateShareLink(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Version != first.Version || second.Level != resourceaccess.LevelEditor || second.ExpiresAt != nil {
		t.Fatalf("unexpected update: %+v", second)
	}
	_, resolved, err := repo.FindShareLinkBySecretHash(ctx, hashString("first"))
	if err != nil || resolved.Level != resourceaccess.LevelEditor {
		t.Fatal("original URL no longer resolves", err)
	}
	assertCount(t, db, &ShareLinkRecord{}, 1)
	if err := repo.RevokeShareLink(ctx, resourceaccess.LinkMutation{Principal: input.Principal, ResourceID: input.ResourceID, LinkID: first.ID, Now: now}); err != nil {
		t.Fatal(err)
	}
	input.IdempotencyKey = "after-revoke"
	next, err := repo.CreateShareLink(ctx, input)
	if err != nil || next.ID == first.ID {
		t.Fatal("explicit revoke must allow a new link", err)
	}
	assertCount(t, db, &ShareLinkRecord{}, 2)
}

func TestSingleShareLinkDoesNotSilentlyRevokeLegacyDuplicates(t *testing.T) {
	db := testDB(t)
	now := time.Now().UTC()
	seedPolicy(t, db, "dr_legacy", resourceaccess.ModePrivate, nil, now)
	for _, id := range []string{"lnk_a", "lnk_b"} {
		if err := db.Create(&ShareLinkRecord{ID: id, WorkspaceID: 1, ResourceID: "dr_legacy", SecretHash: hashString(id), Level: "viewer", Version: 1, CreatedBy: 7, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, err := NewRepository(db).CreateShareLink(context.Background(), resourceaccess.LinkMutation{Principal: resourceaccess.Principal{WorkspaceID: 1, UserID: 7}, ResourceID: "dr_legacy", IdempotencyKey: "legacy", Now: now})
	if !errors.Is(err, resourceaccess.ErrConflict) {
		t.Fatalf("want explicit cleanup conflict, got %v", err)
	}
	var count int64
	db.Model(&ShareLinkRecord{}).Where("revoked_at IS NULL").Count(&count)
	if count != 2 {
		t.Fatal("legacy links must remain valid until manually revoked")
	}
}
