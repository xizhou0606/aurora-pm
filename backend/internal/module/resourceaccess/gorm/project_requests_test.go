package resourceaccessgorm

import (
	"context"
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestMembershipRequestsIsolationIdempotencyAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&AccessRequestRecord{}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{"CREATE TABLE test_grants (workspace_id INTEGER, unit_id INTEGER, user_id INTEGER)", "CREATE TABLE test_notices (request_id INTEGER, decision TEXT)"} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	failNotify := false
	hooks := ProjectRequestHooks{
		Scope: func(tx *gorm.DB, a project.JoinActor, target project.JoinTarget, lock bool) (project.JoinScope, error) {
			scope := project.JoinScope{TargetKind: "plan_unit", AccessGuide: project.AccessGuide{WorkspaceID: 2, PlanUnitID: target.ID, OwnerUserID: 9}, ReviewerIDs: []uint{9}, WorkspaceMember: a.FactID != 7, CanReview: a.FactID == 9}
			if target.Kind == "workspace" {
				scope.TargetKind = "workspace"
				scope.WorkspaceID = target.ID
				scope.PlanUnitID = 0
			}
			return scope, nil
		},
		Grant: func(tx *gorm.DB, s project.JoinScope, user, actor uint, reason string) error {
			return tx.Exec("INSERT INTO test_grants VALUES(?,?,?)", s.WorkspaceID, s.PlanUnitID, user).Error
		},
		Notify: func(tx *gorm.DB, s project.JoinScope, m project.JoinMutation, r project.JoinRequest, recipient uint) error {
			if failNotify {
				return errors.New("notification unavailable")
			}
			return tx.Exec("INSERT INTO test_notices VALUES(?,?)", r.ID, m.Decision).Error
		},
		Names: func(tx *gorm.DB, ids []uint) (map[uint]string, error) {
			return map[uint]string{7: "outsider", 8: "applicant", 9: "reviewer"}, nil
		},
	}
	svc := project.JoinService{Store: NewProjectRequestRepository(db, hooks)}
	ctx := context.Background()
	applicant := project.JoinActor{UserID: 8, FactID: 8}
	outsider := project.JoinActor{UserID: 7, FactID: 7}
	reviewer := project.JoinActor{UserID: 9, FactID: 9}
	target := project.JoinTarget{Kind: "plan_unit", ID: 21}
	if _, err := svc.Create(ctx, outsider, target, "join"); !errors.Is(err, project.ErrJoinWorkspaceRequired) {
		t.Fatalf("workspace prerequisite: %v", err)
	}
	r, err := svc.Create(ctx, applicant, target, "join")
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := svc.Create(ctx, applicant, target, "retry")
	if err != nil || duplicate.ID != r.ID {
		t.Fatalf("duplicate: %+v %v", duplicate, err)
	}
	if _, err := svc.Review(ctx, applicant, target, r.ID, "approved"); !errors.Is(err, project.ErrJoinForbidden) {
		t.Fatalf("self approval: %v", err)
	}
	if _, err := svc.Review(ctx, reviewer, project.JoinTarget{Kind: "plan_unit", ID: 22}, r.ID, "approved"); !errors.Is(err, project.ErrGuideUnavailable) {
		t.Fatalf("wrong version: %v", err)
	}
	if _, err := svc.Review(ctx, reviewer, project.JoinTarget{Kind: "workspace", ID: 2}, r.ID, "approved"); !errors.Is(err, project.ErrGuideUnavailable) {
		t.Fatalf("wrong kind: %v", err)
	}
	failNotify = true
	if _, err := svc.Review(ctx, reviewer, target, r.ID, "approved"); err == nil {
		t.Fatal("expected rollback")
	}
	var count int64
	db.Table("test_grants").Count(&count)
	if count != 0 {
		t.Fatal("grant escaped rollback")
	}
	var stored AccessRequestRecord
	db.First(&stored, r.ID)
	if stored.Status != resourceaccess.RequestPending {
		t.Fatal("decision escaped rollback")
	}
	failNotify = false
	for i := 0; i < 2; i++ {
		if _, err := svc.Review(ctx, reviewer, target, r.ID, "approved"); err != nil {
			t.Fatal(err)
		}
	}
	db.Table("test_grants").Count(&count)
	if count != 1 {
		t.Fatalf("duplicate grant %d", count)
	}
	db.Table("test_notices").Count(&count)
	if count != 2 {
		t.Fatalf("duplicate notices %d", count)
	}
	if _, err := svc.Review(ctx, reviewer, target, r.ID, "rejected"); !errors.Is(err, project.ErrJoinConflict) {
		t.Fatalf("decision rewrite: %v", err)
	}
	workspace := project.JoinTarget{Kind: "workspace", ID: 2}
	wr, err := svc.Create(ctx, outsider, workspace, "join workspace")
	if err != nil {
		t.Fatal(err)
	}
	data, err := svc.Context(ctx, applicant, workspace)
	if err != nil || len(data.Items) != 0 {
		t.Fatalf("applicant isolation %+v %v", data, err)
	}
	data, err = svc.Context(ctx, reviewer, workspace)
	if err != nil || len(data.Items) != 1 || data.Items[0].ID != wr.ID {
		t.Fatalf("review scope %+v %v", data, err)
	}
	db.Model(&AccessRequestRecord{}).Where("id = ?", wr.ID).Update("created_at", time.Now().UTC().Add(-resourceaccess.RequestLifetime-time.Hour))
	if _, err := svc.Review(ctx, reviewer, workspace, wr.ID, "approved"); !errors.Is(err, project.ErrJoinConflict) {
		t.Fatalf("expired: %v", err)
	}
	next, err := svc.Create(ctx, outsider, workspace, "renew")
	if err != nil || next.ID == wr.ID {
		t.Fatalf("renew %+v %v", next, err)
	}
	var nulls int64
	db.Model(&AccessRequestRecord{}).Where("target_kind = ? AND plan_unit_id IS NULL AND resource_id IS NULL", "workspace").Count(&nulls)
	if nulls != 2 {
		t.Fatal("workspace target has fake project/document")
	}
}
