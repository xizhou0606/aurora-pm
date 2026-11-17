package auditgorm

import (
	"context"
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"gorm.io/gorm"
	"testing"
)

func TestOpaqueResourceReferenceRecordedAtomically(t *testing.T) {
	db, e := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&AuditLog{}, &OutboxEvent{}); e != nil {
		t.Fatal(e)
	}
	ref := "550e8400-e29b-41d4-a716-446655440000"
	effect := audit.MutationEffect{WorkspaceID: 1, PrincipalID: 2, Resource: "bitable", ResourceRef: ref, Action: "created", EventType: "bitable.created", Payload: map[string]any{"resourceId": ref}}
	if e = db.Transaction(func(tx *gorm.DB) error {
		s, _ := audit.NewMutationEffectService(NewMutationEffectRepository(tx))
		return s.RecordMutationEffect(context.Background(), effect)
	}); e != nil {
		t.Fatal(e)
	}
	var log AuditLog
	var event OutboxEvent
	db.First(&log)
	db.First(&event)
	if log.ResourceRef != ref || event.AggregateRef != ref || log.ResourceID != 0 || event.AggregateID != 0 {
		t.Fatal("UUID not preserved")
	}
	sentinel := errors.New("rollback")
	_ = db.Transaction(func(tx *gorm.DB) error {
		s, _ := audit.NewMutationEffectService(NewMutationEffectRepository(tx))
		if e := s.RecordMutationEffect(context.Background(), effect); e != nil {
			return e
		}
		return sentinel
	})
	var count int64
	db.Model(&AuditLog{}).Count(&count)
	if count != 1 {
		t.Fatal("audit escaped caller transaction")
	}
	db.Model(&OutboxEvent{}).Count(&count)
	if count != 1 {
		t.Fatal("outbox escaped caller transaction")
	}
}
