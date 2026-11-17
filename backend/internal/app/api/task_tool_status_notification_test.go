package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"gorm.io/gorm"
	"testing"
)

type statusNotificationReadStore struct{ row task.Snapshot }

func (s statusNotificationReadStore) GetTask(_ context.Context, workspaceID uint, lookup task.Lookup) (task.Snapshot, error) {
	if workspaceID != s.row.WorkspaceID || lookup.ID != s.row.ID {
		return task.Snapshot{}, task.ErrTaskNotFound
	}
	return s.row, nil
}
func TestToolTaskStatusNotificationDisabledIsolationAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&auditgorm.OutboxEvent{}, &records.InboxReceipt{}, &identitygorm.User{}, &identitygorm.Workspace{}, &identitygorm.WorkspaceMember{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{&identitygorm.Workspace{ID: 7, Name: "local", Slug: "local", Enabled: true}, &identitygorm.User{ID: 1, Username: "actor", DisplayName: "操作者", Active: true, Status: "active"}, &identitygorm.WorkspaceMember{WorkspaceID: 7, UserID: 1, Enabled: true, Role: "member"}} {
		if err = db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner := uint(2)
	read, _ := task.NewReadService(statusNotificationReadStore{task.Snapshot{ID: 9, WorkspaceID: 7, TeamID: 8, PlanUnitID: 10, Title: "任务", Status: "completed", AssigneeID: &owner, ReporterUserID: &owner}})
	writer := taskToolAssignmentWriter{tx: db, tasks: read, config: config.Notification{Enabled: false}}
	ctx := context.Background()
	if err = writer.recordStatusChange(ctx, 8, 1, 9, "in_dev"); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("workspace isolation=%v", err)
	}
	if err = writer.recordStatusChange(ctx, 7, 1, 9, "completed"); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		w := writer
		w.tx = tx
		if err := w.recordStatusChange(ctx, 7, 1, 9, "in_dev"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var count int64
	db.Model(&auditgorm.OutboxEvent{}).Count(&count)
	if count != 0 {
		t.Fatalf("rolled-back or no-op emitted %d", count)
	}
	if err = writer.recordStatusChange(ctx, 7, 1, 9, "in_dev"); err != nil {
		t.Fatal(err)
	}
	var events []auditgorm.OutboxEvent
	db.Find(&events)
	if len(events) != 1 {
		t.Fatalf("recipient dedupe: %d", len(events))
	}
	var intent notification.Intent
	if err = json.Unmarshal([]byte(events[0].Payload), &intent); err != nil {
		t.Fatal(err)
	}
	if !intent.SkipExternal || intent.Vars["operator"] != "操作者" || intent.Scene != notification.SceneWorkTaskStatusChanged || intent.ResourceID != "9" {
		t.Fatalf("intent=%+v", intent)
	}
}
