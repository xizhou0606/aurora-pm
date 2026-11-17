package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

func TestMemberTaskHandoverAtomic(t *testing.T) {
	for _, mode := range []string{"clear", "assign", "stale", "new-task", "duplicate", "foreign", "audit-failure", "removed-target", "inactive-target"} {
		t.Run(mode, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err = database.Migrate(db); err != nil {
				t.Fatal(err)
			}
			for _, id := range []uint{45, 41, 42} {
				if err = db.Create(&identitygorm.User{ID: id, Username: fmt.Sprintf("member-%d", id), ExternalSubject: fmt.Sprint(id), Active: true, Status: "active"}).Error; err != nil {
					t.Fatal(err)
				}
				if err = db.Create(&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: id, Enabled: true}).Error; err != nil {
					t.Fatal(err)
				}
			}
			owner, removed, target := uint(45), uint(41), uint(42)
			unit := projectgorm.PlanUnit{ID: 59, WorkspaceID: 2, TeamID: 3, ProjectSetID: 18, Title: "Handover", OwnerUserID: &owner}
			for _, row := range []any{&projectgorm.ProjectSet{ID: 18, WorkspaceID: 2, TeamID: 3, Name: "Set", CreatedByUserID: &owner}, &unit,
				&projectgorm.PlanUnitParticipant{WorkspaceID: 2, PlanUnitID: 59, UserID: 41}, &projectgorm.PlanUnitParticipant{WorkspaceID: 2, PlanUnitID: 59, UserID: 42}} {
				if err = db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i := 1; i <= 45; i++ {
				if err = db.Create(&taskgorm.WorkTask{ID: uint(i), WorkspaceID: 2, TeamID: 3, ProjectSetID: 18, ProjectID: 59, RequirementID: 1, Kind: "task", TaskKey: fmt.Sprintf("PM-%d", i), Title: "Synthetic handover task", Status: "in_dev", AssigneeID: &removed}).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range []taskgorm.WorkTask{
				{ID: 46, WorkspaceID: 2, ProjectID: 59, Status: "completed", TaskKey: "PM-46", AssigneeID: &removed},
				{ID: 47, WorkspaceID: 3, ProjectID: 59, Status: "in_dev", TaskKey: "PM-47", AssigneeID: &removed},
				{ID: 48, WorkspaceID: 2, ProjectID: 60, Status: "in_dev", TaskKey: "PM-48", AssigneeID: &removed},
			} {
				if err = db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := newPlanUnitMemberRepository(db, config.Notification{})
			ctx := context.Background()
			tasks, err := repo.ListPlanUnitMemberTasks(ctx, 2, 59, 41)
			if err != nil || len(tasks) != 45 {
				t.Fatalf("preview count=%d err=%v", len(tasks), err)
			}
			resolution := &task.MemberTaskResolution{Tasks: tasks}
			switch mode {
			case "assign":
				resolution.AssigneeID = &target
			case "stale":
				db.Model(&taskgorm.WorkTask{}).Where("id = ?", 1).Update("updated_at", time.Now().Add(time.Hour))
			case "new-task":
				db.Create(&taskgorm.WorkTask{ID: 49, WorkspaceID: 2, ProjectID: 59, Status: "in_dev", TaskKey: "PM-49", AssigneeID: &removed})
			case "duplicate":
				resolution.Tasks[0] = resolution.Tasks[1]
			case "foreign":
				resolution.Tasks[0].ID = 47
			case "removed-target":
				resolution.AssigneeID = &removed
			case "inactive-target":
				resolution.AssigneeID = &target
				db.Model(&identitygorm.User{}).Where("id = ?", target).Update("active", false)
			case "audit-failure":
				db.Callback().Create().Before("gorm:create").Register("reject-member-audit", func(tx *gorm.DB) {
					if tx.Statement.Table == "plan_unit_member_mutation_audits" {
						tx.AddError(errors.New("audit failure"))
					}
				})
			}
			_, err = repo.SavePlanUnitMemberState(ctx, 2, 59, project.MemberState{OwnerUserID: &owner, ParticipantUserIDs: []uint{42}}, project.MemberMutationMeta{ActorUserID: 45, ExpectedAt: &unit.UpdatedAt, TaskResolution: resolution})
			success := mode == "clear" || mode == "assign"
			if success && err != nil || !success && err == nil {
				t.Fatalf("mode %s err=%v", mode, err)
			}
			var changed int64
			db.Model(&taskgorm.WorkTask{}).Where("id <= 45 AND (assignee_id IS NULL OR assignee_id <> 41)").Count(&changed)
			if success && changed != 45 || !success && changed != 0 {
				t.Fatalf("changed=%d err=%v", changed, err)
			}
			var preserved int64
			db.Model(&taskgorm.WorkTask{}).Where("id IN ? AND assignee_id = 41", []uint{46, 47, 48}).Count(&preserved)
			if preserved != 3 {
				t.Fatalf("outside scope tasks changed: %d", preserved)
			}
			var memberCount, activities int64
			db.Model(&projectgorm.PlanUnitParticipant{}).Where("plan_unit_id=59 AND user_id=41").Count(&memberCount)
			db.Model(&taskgorm.WorkTaskActivity{}).Where("field_key='assignee_id'").Count(&activities)
			if success && (memberCount != 0 || activities != 45) || !success && (memberCount != 1 || activities != 0) {
				t.Fatalf("member=%d activities=%d", memberCount, activities)
			}
		})
	}
}
