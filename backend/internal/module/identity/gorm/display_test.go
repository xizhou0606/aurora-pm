package identitygorm

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestUserDisplayUsesInternalIDWithoutWorkspaceMembership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	users := []User{{ID: 109, ExternalSubject: "admin", Username: "admin", DisplayName: "Administrator"}, {ID: 1, ExternalSubject: "other", Username: "different", DisplayName: "Other user"}, {ID: 110, ExternalSubject: "sysadmin", Username: "sysadmin", DisplayName: "PM Administrator"}}
	if err = db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	// Legacy fixture only; the runtime User model intentionally has no external field.
	if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 1, 109).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 109, 1).Error; err != nil {
		t.Fatal(err)
	}
	got, err := NewUserDisplayRepository(db).ReadUserDisplays(context.Background(), []uint{109, 110})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[109].Name != "Administrator" || got[110].Name != "PM Administrator" || got[109].Username != "admin" || got[110].Username != "sysadmin" {
		t.Fatalf("profiles=%+v", got)
	}
}
