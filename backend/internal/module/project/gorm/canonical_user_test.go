package projectgorm

import (
	"context"
	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"gorm.io/gorm"
	"testing"
)

func TestProjectUserResolverUsesInternalIDDespiteExternalCollision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&identitygorm.User{}); err != nil {
		t.Fatal(err)
	}
	rows := []identitygorm.User{{ID: 109, Username: "imported", ExternalSubject: "local:imported", Active: true, Status: "active"}, {ID: 1, Username: "local", ExternalSubject: "local:local", Active: true, Status: "active"}, {ID: 110, Username: "inactive", ExternalSubject: "local:inactive", Active: false, Status: "banned"}}
	if err := db.Create(&rows).Error; err != nil {
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
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 300, 110).Error; err != nil {
		t.Fatal(err)
	}
	r := NewFactUserResolver(db)
	for _, test := range []struct{ input, want uint }{{109, 109}, {1, 1}, {110, 0}, {300, 0}} {
		got, err := r.FactUserID(context.Background(), test.input)
		if err != nil || got != test.want {
			t.Fatalf("input %d got %d want %d err %v", test.input, got, test.want, err)
		}
	}
}
