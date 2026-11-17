package identitygorm

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestFeishuOpenIDsResolvesInternalIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	openID := "ou_fact_user"
	if err := db.Create(&User{
		ID: 74, Username: "qixing", DisplayName: "七星",
		Active: true, Role: "user", FeishuOpenID: &openID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	openIDs, err := NewLoginRepository(db).FeishuOpenIDs(context.Background(), []uint{104, 74, 1})
	if err != nil {
		t.Fatal(err)
	}
	if openIDs[104] != "" || openIDs[74] != openID || openIDs[1] != "" {
		t.Fatalf("lookup=%v", openIDs)
	}
}

func TestFeishuEmailsResolvesInternalIDsWithoutOpenID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&User{
		ID: 74, Username: "qixing", DisplayName: "七星",
		Email: "qixing@example.com", Active: true, Role: "user",
	}).Error; err != nil {
		t.Fatal(err)
	}

	emails, err := NewLoginRepository(db).FeishuEmails(context.Background(), []uint{104, 74, 1})
	if err != nil {
		t.Fatal(err)
	}
	if emails[104] != "" || emails[74] != "qixing@example.com" || emails[1] != "" {
		t.Fatalf("lookup=%v", emails)
	}
}

func TestSaveFeishuOpenIDUpdatesInternalUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	oldID := "ou_previous"
	if err := db.Create(&User{
		ID: 74, Username: "qixing", DisplayName: "七星",
		Active: true, Role: "user", FeishuOpenID: &oldID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := NewLoginRepository(db).SaveFeishuOpenID(context.Background(), 74, "ou_current"); err != nil {
		t.Fatal(err)
	}
	openIDs, err := NewLoginRepository(db).FeishuOpenIDs(context.Background(), []uint{74})
	if err != nil {
		t.Fatal(err)
	}
	if openIDs[74] != "ou_current" {
		t.Fatalf("lookup=%v", openIDs)
	}
}

func TestFeishuDeliveryNeverFollowsExternalIDCollision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	first, second := "ou_first", "ou_second"
	rows := []User{{ID: 74, Username: "first", ExternalSubject: "local:first", Active: true, FeishuOpenID: &first}, {ID: 104, Username: "second", ExternalSubject: "local:second", Active: true, FeishuOpenID: &second}}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	// Legacy fixture only; the runtime User model intentionally has no external field.
	if err := db.Exec("ALTER TABLE users ADD COLUMN external_user_id bigint").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 104, 74).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE users SET external_user_id = ? WHERE id = ?", 74, 104).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewLoginRepository(db)
	contacts, err := repo.FeishuOpenIDs(context.Background(), []uint{74, 104})
	if err != nil || contacts[74] != first || contacts[104] != second {
		t.Fatalf("crossed delivery identities: %v %v", contacts, err)
	}
	if err := repo.SaveFeishuOpenID(context.Background(), 104, "ou_updated"); err != nil {
		t.Fatal(err)
	}
	contacts, err = repo.FeishuOpenIDs(context.Background(), []uint{74, 104})
	if err != nil || contacts[74] != first || contacts[104] != "ou_updated" {
		t.Fatalf("updated wrong identity: %v %v", contacts, err)
	}
}
