package taskgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCreatorPolicyUsesFirstImmutableCreationActivityAndRequiresEveryTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTaskActivity{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	rows := []WorkTaskActivity{
		{WorkspaceID: 7, WorkTaskID: 11, UserID: 3, Action: "created", CreatedAt: now},
		{WorkspaceID: 7, WorkTaskID: 11, UserID: 4, Action: "updated", CreatedAt: now.Add(time.Second)},
		{WorkspaceID: 7, WorkTaskID: 12, UserID: 3, Action: "cloned", CreatedAt: now},
		{WorkspaceID: 8, WorkTaskID: 13, UserID: 3, Action: "created", CreatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewCreatorPolicyRepository(db)
	allowed, err := repository.AllCreatedBy(context.Background(), 7, 3, []uint{11, 12})
	if err != nil || !allowed {
		t.Fatalf("owned selection allowed=%t err=%v", allowed, err)
	}
	allowed, err = repository.AllCreatedBy(context.Background(), 7, 4, []uint{11})
	if err != nil || allowed {
		t.Fatalf("later actor allowed=%t err=%v", allowed, err)
	}
	allowed, err = repository.AllCreatedBy(context.Background(), 7, 3, []uint{11, 99})
	if err != nil || allowed {
		t.Fatalf("missing creator evidence allowed=%t err=%v", allowed, err)
	}
	allowed, err = repository.AllCreatedBy(context.Background(), 7, 3, []uint{13})
	if err != nil || allowed {
		t.Fatalf("cross-workspace creator allowed=%t err=%v", allowed, err)
	}
}
