package teamscoperepair

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

var repairDBCounter atomic.Uint64

func TestDryRunAndApplyRepairAllTeamScopedRows(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)

	dryRunner, err := New(db, Options{WorkspaceID: 1, TargetTeamID: 10, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	dryReport, err := dryRunner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !dryReport.Ready || dryReport.Applied || len(dryReport.ProjectSets) != 1 {
		t.Fatalf("unexpected dry-run report: %+v", dryReport)
	}
	for _, table := range teamScopeTables {
		row := dryReport.Tables[table.Name]
		if row.Scoped != 1 || row.WouldUpdate != 1 || row.Updated != 0 {
			t.Fatalf("dry-run %s=%+v", table.Name, row)
		}
	}
	if dryReport.TeamMemberships.WouldInsert != 1 ||
		len(dryReport.TeamMemberships.MissingFactUserIDs) != 1 {
		t.Fatalf("dry-run team memberships=%+v", dryReport.TeamMemberships)
	}
	assertTeamID(t, db, "project_sets", 11, 0)
	assertTeamMember(t, db, 10, 100, false)

	applyRunner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := applyRunner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Ready || !applied.Applied || applied.RolledBack || totalIssues(applied.IssuesAfter) != 0 {
		t.Fatalf("unexpected apply report: %+v", applied)
	}
	for _, table := range teamScopeTables {
		row := applied.Tables[table.Name]
		if row.Updated != 1 {
			t.Fatalf("apply %s=%+v", table.Name, row)
		}
	}
	assertTeamID(t, db, "project_sets", 11, 10)
	assertTeamID(t, db, "plan_units", 12, 10)
	assertTeamID(t, db, "requirements", 13, 10)
	assertTeamID(t, db, "project_permissions", 14, 10)
	assertTeamID(t, db, "work_tasks", 15, 10)
	assertTeamID(t, db, "quality_bugs", 16, 10)
	assertTeamID(t, db, "project_sets", 21, 0)
	assertTeamMember(t, db, 10, 100, true)
	if applied.TeamMemberships.Inserted != 1 {
		t.Fatalf("applied team memberships=%+v", applied.TeamMemberships)
	}
}

func TestRepairBackfillsRosterForAlreadyScopedProject(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)
	first, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, row := range []any{
		&identitygorm.User{ID: 101, ExternalSubject: "shared:501", ExternalIssuer: "https://identity.example",
			Username: "later-member", DisplayName: "Later Member", Role: "user", Active: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: 101, Role: "member", Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).
		UpdateColumn("product_owner_user_id", 101).Error; err != nil {
		t.Fatal(err)
	}

	dryRunner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dryReport, err := dryRunner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !dryReport.Ready || len(dryReport.ProjectSets) != 1 ||
		dryReport.Tables["project_sets"].WouldUpdate != 0 ||
		dryReport.TeamMemberships.WouldInsert != 1 ||
		len(dryReport.TeamMemberships.MissingFactUserIDs) != 1 ||
		dryReport.TeamMemberships.MissingFactUserIDs[0] != 101 {
		t.Fatalf("unexpected rerun dry report: %+v", dryReport)
	}
	assertTeamMember(t, db, 10, 101, false)

	applyRunner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := applyRunner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.TeamMemberships.Inserted != 1 ||
		totalIssues(applied.IssuesAfter) != 0 {
		t.Fatalf("unexpected rerun apply report: %+v", applied)
	}
	assertTeamMember(t, db, 10, 101, true)
}

func TestRepairRefusesUnresolvedReferencedUserWithoutWriting(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)
	if err := db.Model(&projectgorm.ProjectPermission{}).Where("id = ?", 14).
		UpdateColumn("user_id", 999).Error; err != nil {
		t.Fatal(err)
	}
	runner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background())
	if err == nil || report.Ready || report.Applied || report.RolledBack {
		t.Fatalf("unexpected unresolved-user report=%+v err=%v", report, err)
	}
	if len(report.TeamMemberships.UnresolvedFactUserIDs) != 1 ||
		report.TeamMemberships.UnresolvedFactUserIDs[0] != 999 {
		t.Fatalf("unresolved users=%+v", report.TeamMemberships)
	}
	assertTeamID(t, db, "project_sets", 11, 0)
	assertTeamMember(t, db, 10, 100, false)
}

func TestRepairRefusesStructuralCorruptionWithoutWriting(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)
	if err := db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).
		UpdateColumn("version_task_new_id", 999).Error; err != nil {
		t.Fatal(err)
	}
	runner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("structurally corrupt hierarchy was repaired")
	}
	if report.Ready || report.Applied || report.RolledBack {
		t.Fatalf("unexpected refusal report: %+v", report)
	}
	assertTeamID(t, db, "project_sets", 11, 0)
	assertTeamID(t, db, "requirements", 13, 0)
}

func TestRepairRefusesValidTeamReassignmentAndForeignTarget(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)
	if err := db.Create(&identitygorm.Team{ID: 11, WorkspaceID: 1, Name: "Other"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&projectgorm.ProjectSet{}).Where("id = ?", 11).
		UpdateColumn("team_id", 11).Error; err != nil {
		t.Fatal(err)
	}
	runner, err := New(db, Options{
		WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("valid project team reassignment was accepted")
	}
	foreignRunner, err := New(db, Options{WorkspaceID: 1, TargetTeamID: 20, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreignRunner.Run(context.Background()); err == nil {
		t.Fatal("foreign target team was accepted")
	}
	assertTeamID(t, db, "project_sets", 11, 11)
}

func TestApplyRequiresExplicitProjectSetIDs(t *testing.T) {
	db := repairTestDB(t)
	if _, err := New(db, Options{WorkspaceID: 1, TargetTeamID: 10, DryRun: false}); err == nil {
		t.Fatal("unscoped apply was accepted")
	}
}

func repairTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Open(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:team_scope_repair_%d?mode=memory&cache=shared",
			repairDBCounter.Add(1)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedRepairHierarchy(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []any{
		&identitygorm.User{ID: 100, ExternalSubject: "local:100", Username: "project-member", DisplayName: "Project Member", Role: "user", Active: true},
		&identitygorm.Workspace{ID: 1, Name: "A", Slug: "a", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "B", Slug: "b", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: 100, Role: "member", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Team B"},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 0, Name: "Legacy A", Code: "A"},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, Title: "V1"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, PlanUnitID: 12, Title: "Req"},
		&projectgorm.ProjectPermission{ID: 14, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, UserID: 100, AccessLevel: "member"},
		&taskgorm.WorkTask{ID: 15, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-1", Title: "Task"},
		&qualitygorm.QualityBug{ID: 16, WorkspaceID: 1, TeamID: 0, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, BugKey: "BUG-16", Title: "Bug", Status: "triage", Priority: "medium"},
		&projectgorm.ProjectSet{ID: 21, WorkspaceID: 2, TeamID: 0, Name: "Legacy B", Code: "B"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func assertTeamID(t *testing.T, db *gorm.DB, table string, id, want uint) {
	t.Helper()
	var row struct{ TeamID uint }
	if err := db.Table(table).Select("team_id").Where("id = ?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.TeamID != want {
		t.Fatalf("%s id=%d team_id=%d want=%d", table, id, row.TeamID, want)
	}
}

func assertTeamMember(t *testing.T, db *gorm.DB, teamID, userID uint, want bool) {
	t.Helper()
	var count int64
	if err := db.Model(&identitygorm.TeamMember{}).
		Where("workspace_id = ? AND team_id = ? AND user_id = ?", 1, teamID, userID).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if (count == 1) != want {
		t.Fatalf("team member team=%d user=%d count=%d want=%v", teamID, userID, count, want)
	}
}

func TestRepairDoesNotAliasExternalUserIDs(t *testing.T) {
	db := repairTestDB(t)
	seedRepairHierarchy(t, db)
	for _, row := range []any{
		&identitygorm.User{ID: 101, ExternalSubject: "shared:501", Username: "imported", Role: "user", Active: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: 101, Role: "member", Enabled: true},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&requirementgorm.Requirement{}).Where("id = ?", 13).UpdateColumn("product_owner_user_id", 501).Error; err != nil {
		t.Fatal(err)
	}
	runner, err := New(db, Options{WorkspaceID: 1, TargetTeamID: 10, ProjectSetIDs: []uint{11}, DryRun: false})
	if err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background())
	if err == nil || report.Ready || report.Applied {
		t.Fatalf("external reference accepted: %+v %v", report, err)
	}
	if len(report.TeamMemberships.UnresolvedFactUserIDs) != 1 || report.TeamMemberships.UnresolvedFactUserIDs[0] != 501 {
		t.Fatalf("unresolved=%+v", report.TeamMemberships)
	}
	assertTeamID(t, db, "project_sets", 11, 0)
	assertTeamMember(t, db, 10, 101, false)
}
