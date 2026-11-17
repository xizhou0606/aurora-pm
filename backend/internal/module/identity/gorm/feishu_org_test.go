package identitygorm

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

func newFeishuOrgTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	// Mirror the 20260908200000_add_users_feishu_job_title migration: the
	// repository writes feishu_job_title with raw SQL, so the column must
	// exist even though the deprecated GORM record does not carry it yet.
	if err := db.Exec("ALTER TABLE users ADD COLUMN feishu_job_title TEXT NOT NULL DEFAULT ''").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

type feishuUserRow struct {
	ID                 uint
	FeishuOpenID       *string
	FeishuDepartmentID string
	FeishuJobTitle     string
}

func feishuUserRowByID(t *testing.T, db *gorm.DB, id uint) feishuUserRow {
	t.Helper()
	var row feishuUserRow
	err := db.Raw(
		"SELECT id, feishu_open_id, feishu_department_id, feishu_job_title FROM users WHERE id = ?", id,
	).Scan(&row).Error
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSetUserDepartmentsWritesDepartmentAndJobTitle(t *testing.T) {
	db := newFeishuOrgTestDB(t)
	for _, user := range []User{
		{ID: 11, ExternalSubject: "sub-11", Username: "zhangsan", DisplayName: "张三",
			Active: true, Role: "user", Status: string(identity.AccountActive)},
		{ID: 12, ExternalSubject: "sub-12", Username: "lisi", DisplayName: "李四",
			Active: true, Role: "user", Status: string(identity.AccountActive)},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}

	repository := NewFeishuOrgStoreRepository(db)
	err := repository.SetUserDepartments(context.Background(), []identity.FeishuUserDepartmentAssignment{
		{UserID: 11, OpenID: "ou-1", DepartmentID: "od-a", JobTitle: "后端工程师"},
		{UserID: 12, OpenID: "ou-2", DepartmentID: "od-b", JobTitle: ""},
	})
	if err != nil {
		t.Fatal(err)
	}

	row := feishuUserRowByID(t, db, 11)
	if row.FeishuOpenID == nil || *row.FeishuOpenID != "ou-1" {
		t.Fatalf("open id must be adopted: %+v", row)
	}
	if row.FeishuDepartmentID != "od-a" || row.FeishuJobTitle != "后端工程师" {
		t.Fatalf("department/job title wrong: %+v", row)
	}
	row = feishuUserRowByID(t, db, 12)
	if row.FeishuDepartmentID != "od-b" || row.FeishuJobTitle != "" {
		t.Fatalf("missing job title must stay empty: %+v", row)
	}
}

func TestSetUserDepartmentsOverwritesJobTitleAndKeepsOpenID(t *testing.T) {
	db := newFeishuOrgTestDB(t)
	existing := "ou_previous"
	if err := db.Create(&User{
		ID: 11, Username: "zhangsan", DisplayName: "张三",
		Active: true, Role: "user", Status: string(identity.AccountActive),
		FeishuOpenID: &existing, FeishuDepartmentID: "od-old",
	}).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewFeishuOrgStoreRepository(db)
	if err := repository.SetUserDepartments(context.Background(), []identity.FeishuUserDepartmentAssignment{
		{UserID: 11, OpenID: "ou_current", DepartmentID: "od-new", JobTitle: "高级工程师"},
	}); err != nil {
		t.Fatal(err)
	}

	row := feishuUserRowByID(t, db, 11)
	// open_id is a fact and is never overwritten; department and job title
	// follow the latest contact payload on every sync.
	if row.FeishuOpenID == nil || *row.FeishuOpenID != "ou_previous" {
		t.Fatalf("existing open id must be preserved: %+v", row)
	}
	if row.FeishuDepartmentID != "od-new" || row.FeishuJobTitle != "高级工程师" {
		t.Fatalf("department/job title must be overwritten: %+v", row)
	}
}

func TestClearUserDepartmentsClearsDepartmentAndJobTitle(t *testing.T) {
	db := newFeishuOrgTestDB(t)
	for _, user := range []User{
		{ID: 11, ExternalSubject: "sub-11", Username: "zhangsan", DisplayName: "张三", Active: true, Role: "user",
			Status: string(identity.AccountActive), FeishuDepartmentID: "od-a"},
		{ID: 12, ExternalSubject: "sub-12", Username: "lisi", DisplayName: "李四", Active: true, Role: "user",
			Status: string(identity.AccountActive), FeishuDepartmentID: "od-b"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("UPDATE users SET feishu_job_title = ? WHERE id = ?", "测试工程师", 12).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewFeishuOrgStoreRepository(db)
	if err := repository.ClearUserDepartments(context.Background(), []uint{11}); err != nil {
		t.Fatal(err)
	}

	row := feishuUserRowByID(t, db, 12)
	if row.FeishuDepartmentID != "" || row.FeishuJobTitle != "" {
		t.Fatalf("departed user must lose department and job title: %+v", row)
	}
	if kept := feishuUserRowByID(t, db, 11); kept.FeishuDepartmentID != "od-a" {
		t.Fatalf("kept user must be untouched: %+v", kept)
	}

	// An empty keep-set clears everyone still mapped.
	if err := repository.ClearUserDepartments(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if row := feishuUserRowByID(t, db, 11); row.FeishuDepartmentID != "" || row.FeishuJobTitle != "" {
		t.Fatalf("empty keep-set must clear everyone: %+v", row)
	}
}
