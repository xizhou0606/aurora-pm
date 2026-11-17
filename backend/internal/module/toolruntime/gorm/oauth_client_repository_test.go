package toolruntimegorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

func TestOAuthClientRepositoryPersistsOnlyPublicClientFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(OAuthMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	repository := NewOAuthClientRepository(db)
	client := toolruntime.OAuthClient{
		ID: "pm_mcp_public_client_123", Name: "Codex", Channel: toolruntime.DeliveryMCP,
		RedirectURIs: []string{"http://127.0.0.1/callback"}, CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := repository.Create(context.Background(), &client); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.FindByID(context.Background(), client.ID)
	if err != nil || loaded.ID != client.ID || loaded.RedirectURIs[0] != client.RedirectURIs[0] {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	columns, err := db.Migrator().ColumnTypes(&ExternalOAuthClient{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		switch column.Name() {
		case "client_secret", "access_token", "refresh_token", "authorization_code":
			t.Fatalf("OAuth client table persisted secret column %q", column.Name())
		}
	}
}
