package toolruntimegorm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGrantRepositoryPersistsCanonicalDelegationFacts(t *testing.T) {
	db := grantTestDB(t)
	repository := NewGrantRepository(db)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	grant, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: 42, WorkspaceID: 8, ClientID: "  pm-cli  ", Channel: toolruntime.DeliveryCLI,
		AllowedTools:  []string{"task.list", "project.list", "task.list"},
		AllowedScopes: []string{"task:read", "project:read", "task:read"},
		EffectiveFrom: now.Add(time.Hour), ExpiresAt: now.Add(25 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), &grant); err != nil {
		t.Fatal(err)
	}
	if grant.ID == 0 {
		t.Fatal("repository did not assign grant ID")
	}

	loaded, err := repository.FindByID(context.Background(), grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ClientID != "pm-cli" || loaded.PrincipalID != 42 || loaded.WorkspaceID != 8 || loaded.Channel != toolruntime.DeliveryCLI {
		t.Fatalf("loaded identity facts drifted: %+v", loaded)
	}
	if strings.Join(loaded.AllowedScopes, ",") != "project:read,task:read" {
		t.Fatalf("loaded scopes=%v", loaded.AllowedScopes)
	}
	if strings.Join(loaded.AllowedTools, ",") != "project.list,task.list" {
		t.Fatalf("loaded tools=%v", loaded.AllowedTools)
	}
	if loaded.Status(now) != toolruntime.GrantScheduled || loaded.Status(now.Add(time.Hour)) != toolruntime.GrantActive {
		t.Fatalf("loaded grant lost UAT window: %+v", loaded)
	}
	if err := loaded.Authorize(toolruntime.AccessRequest{
		GrantID: loaded.ID, PrincipalID: 42, WorkspaceID: 8, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
		Audience: toolruntime.AudienceCLI, Tool: "task.list", Scope: "task:read",
	}, now.Add(time.Hour)); err != nil {
		t.Fatalf("persisted grant denied valid access: %v", err)
	}
}

func TestGrantRepositoryRevocationIsImmediateAndIdempotent(t *testing.T) {
	db := grantTestDB(t)
	repository := NewGrantRepository(db)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	grant := newRepositoryGrant(t, now)
	if err := repository.Create(context.Background(), &grant); err != nil {
		t.Fatal(err)
	}

	revoked, err := repository.Revoke(context.Background(), grant.ID, now.Add(time.Minute))
	if err != nil || revoked.Status(now) != toolruntime.GrantRevoked {
		t.Fatalf("revoked=%+v err=%v", revoked, err)
	}
	again, err := repository.Revoke(context.Background(), grant.ID, now.Add(2*time.Minute))
	if err != nil || again.RevokedAt == nil || !again.RevokedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("idempotent revoke changed first fact: %+v err=%v", again, err)
	}
	request := toolruntime.AccessRequest{
		GrantID: grant.ID, PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID, ClientID: grant.ClientID,
		Channel: grant.Channel, Audience: toolruntime.AudienceCLI, Tool: "project.list", Scope: "project:read",
	}
	if err := revoked.Authorize(request, now.Add(time.Minute)); !errors.Is(err, toolruntime.ErrGrantNotActive) {
		t.Fatalf("revoked authorization err=%v", err)
	}
}

func TestGrantRepositoryFailsClosedForMissingOrCorruptRows(t *testing.T) {
	db := grantTestDB(t)
	repository := NewGrantRepository(db)
	if _, err := repository.FindByID(context.Background(), 999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing err=%v", err)
	}
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	row := ExternalAuthorizationGrant{
		PrincipalID: 1, WorkspaceID: 2, ClientID: "client", Channel: string(toolruntime.DeliveryCLI),
		AllowedTools: `["project.list"]`, AllowedScopes: `{"not":"an array"}`,
		EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindByID(context.Background(), row.ID); !errors.Is(err, toolruntime.ErrGrantInvalid) {
		t.Fatalf("corrupt row err=%v", err)
	}
}

func TestGrantRecordContainsNoRawTokenColumns(t *testing.T) {
	db := grantTestDB(t)
	columns, err := db.Migrator().ColumnTypes(&ExternalAuthorizationGrant{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		name := strings.ToLower(column.Name())
		if name == "token" || name == "access_token" || name == "refresh_token" || name == "raw_token" {
			t.Fatalf("raw token column must not exist: %s", name)
		}
	}
}

func grantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ExternalAuthorizationGrant{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newRepositoryGrant(t *testing.T, now time.Time) toolruntime.Grant {
	t.Helper()
	grant, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: 3, WorkspaceID: 5, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
		AllowedTools:  []string{"project.list"},
		AllowedScopes: []string{"project:read"}, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
	}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return grant
}
