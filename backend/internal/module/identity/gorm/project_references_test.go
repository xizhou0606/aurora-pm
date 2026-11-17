package identitygorm

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

func TestProjectReferencesAreWorkspaceAndActiveUserScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Team{}, &User{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]Team{{ID: 11, WorkspaceID: 7, Name: "A"}, {ID: 12, WorkspaceID: 8, Name: "B"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]User{
		{ID: 21, ExternalSubject: "local:active", Username: "active", DisplayName: "Active", Active: true, Status: string(identity.AccountActive)},
		{ID: 22, ExternalSubject: "local:disabled", Username: "disabled", DisplayName: "Disabled", Active: false, Status: string(identity.AccountBanned)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewProjectReferenceRepository(db)
	if valid, err := repository.ProjectReferencesValid(context.Background(), 7, 11, []uint{21}); err != nil || !valid {
		t.Fatalf("valid=%t err=%v", valid, err)
	}
	for _, check := range []struct {
		team  uint
		users []uint
	}{{12, []uint{21}}, {11, []uint{22}}, {11, []uint{21, 999}}} {
		if valid, err := repository.ProjectReferencesValid(context.Background(), 7, check.team, check.users); err != nil || valid {
			t.Fatalf("check=%+v valid=%t err=%v", check, valid, err)
		}
	}
}
