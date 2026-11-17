package resourceaccessgorm

import (
	"context"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"testing"
	"time"
)

func TestLinksSurviveMemberPolicyAndRevocationKeepsPolicy(t *testing.T) {
	for _, mode := range []resourceaccess.Mode{resourceaccess.ModePrivate, resourceaccess.ModeRestricted, resourceaccess.ModeWorkspace} {
		t.Run(string(mode), func(t *testing.T) {
			db := testDB(t)
			now := time.Now().UTC()
			seedPolicy(t, db, "dr_independent", mode, nil, now)
			repo := NewRepository(db)
			principal := resourceaccess.Principal{WorkspaceID: 1, UserID: 7}
			link, err := repo.CreateShareLink(context.Background(), resourceaccess.LinkMutation{Principal: principal, ResourceID: "dr_independent", LinkID: "lnk_independent", SecretHash: hashString("secret"), Level: resourceaccess.LevelViewer, Now: now, IdempotencyKey: "key", InputDigest: hashString("input")})
			if err != nil {
				t.Fatal(err)
			}
			recipient := resourceaccess.Principal{WorkspaceID: 1, UserID: 9}
			if err = repo.CreateLinkAccessContext(context.Background(), recipient, link, hashString("secret"), hashString("context"), now, now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			levels, err := repo.ResolveLinkAccess(context.Background(), recipient, []documentlibrary.ResourceID{"dr_independent"}, hashString("context"), now)
			if err != nil || len(levels) != 1 {
				t.Fatalf("levels=%v err=%v", levels, err)
			}
			levels, err = repo.ResolveLinkAccess(context.Background(), recipient, []documentlibrary.ResourceID{"dr_independent"}, "", now.Add(time.Hour))
			if err != nil || len(levels) != 1 {
				t.Fatalf("account grant lost after browser context expiry: %v %v", levels, err)
			}
			foreign := resourceaccess.Principal{WorkspaceID: 1, UserID: 10}
			levels, err = repo.ResolveLinkAccess(context.Background(), foreign, []documentlibrary.ResourceID{"dr_independent"}, "", now)
			if err != nil || len(levels) != 0 {
				t.Fatalf("grant leaked to another user: %v %v", levels, err)
			}
			if err = repo.RevokeShareLink(context.Background(), resourceaccess.LinkMutation{Principal: principal, ResourceID: "dr_independent", LinkID: link.ID, Now: now}); err != nil {
				t.Fatal(err)
			}
			policy, err := repo.GetPolicy(context.Background(), 1, "dr_independent")
			if err != nil || policy.Mode != mode || policy.Revision != 1 {
				t.Fatalf("policy changed: %+v %v", policy, err)
			}
			levels, err = repo.ResolveLinkAccess(context.Background(), recipient, []documentlibrary.ResourceID{"dr_independent"}, "", now)
			if err != nil || len(levels) != 0 {
				t.Fatalf("revoked link remains valid: %v %v", levels, err)
			}
		})
	}
}
