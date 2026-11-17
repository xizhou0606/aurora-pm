package database

import (
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"testing"
)

func TestPersonalSpaceBootstrapIsScopedAndIdempotent(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	w := identitygorm.Workspace{Name: "space", Slug: "personal", Enabled: true}
	if err := db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"a", "b"} {
		u := identitygorm.User{Username: name, ExternalSubject: "personal:" + name, Active: true}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		m := identitygorm.WorkspaceMember{WorkspaceID: w.ID, UserID: u.ID, Enabled: true}
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if err := db.Model(&m).Update("enabled", false).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 2; i++ {
		if err := bootstrapPersonalDocumentSpaces(db); err != nil {
			t.Fatal(err)
		}
	}
	var spaces []documentgorm.PersonalSpace
	if err := db.Find(&spaces).Error; err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 || spaces[0].WorkspaceID != w.ID {
		t.Fatalf("spaces=%+v", spaces)
	}
}
