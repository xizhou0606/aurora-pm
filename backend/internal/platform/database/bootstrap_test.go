package database

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestBootstrapExistingUserSkipsPasswordHash(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	existing := identitygorm.User{
		ExternalSubject: "local:existing",
		Username:        "existing",
		DisplayName:     "Existing User",
		PasswordHash:    "existing-hash",
		Role:            "super_admin",
		Active:          true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	hashCalls := 0
	err := bootstrap(db, bootstrapTestSpec(), func([]byte, int) ([]byte, error) {
		hashCalls++
		return nil, errors.New("password hashing should not run")
	})
	if err != nil {
		t.Fatalf("bootstrap existing user: %v", err)
	}
	if hashCalls != 0 {
		t.Fatalf("password hash calls = %d, want 0", hashCalls)
	}
	var workspaceMembers, teamMembers int64
	db.Model(&identitygorm.WorkspaceMember{}).Where("user_id = ?", existing.ID).Count(&workspaceMembers)
	db.Model(&identitygorm.TeamMember{}).Where("user_id = ?", existing.ID).Count(&teamMembers)
	if workspaceMembers != 0 || teamMembers != 0 {
		t.Fatalf("bootstrap unexpectedly created authorization membership rows: workspace=%d team=%d", workspaceMembers, teamMembers)
	}
}

func TestBootstrapRejectsExistingSharedIdentityUsername(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := db.Create(&identitygorm.User{
		ExternalSubject: "one2all|9", ExternalIssuer: "one2all", Username: "existing", DisplayName: "Shared User", Role: "user", Active: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, bootstrapTestSpec(), func([]byte, int) ([]byte, error) {
		return []byte("must-not-run"), nil
	}); err == nil {
		t.Fatal("shared identity username collision was accepted")
	}
}

func TestBootstrapNewUserHashesPasswordOnce(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	hashCalls := 0
	err := bootstrap(db, bootstrapTestSpec(), func(password []byte, cost int) ([]byte, error) {
		hashCalls++
		if string(password) != "bootstrap-password" {
			t.Fatalf("password = %q, want bootstrap-password", password)
		}
		if cost != bcrypt.DefaultCost {
			t.Fatalf("bcrypt cost = %d, want %d", cost, bcrypt.DefaultCost)
		}
		return []byte("generated-hash"), nil
	})
	if err != nil {
		t.Fatalf("bootstrap new user: %v", err)
	}
	if hashCalls != 1 {
		t.Fatalf("password hash calls = %d, want 1", hashCalls)
	}

	var user identitygorm.User
	if err := db.Where("username = ?", "existing").First(&user).Error; err != nil {
		t.Fatalf("load bootstrapped user: %v", err)
	}
	if user.PasswordHash != "generated-hash" {
		t.Fatalf("password hash = %q, want generated-hash", user.PasswordHash)
	}
}

func TestBootstrapReusesExplicitExistingWorkspaceWithoutCreatingPreviewData(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	workspace := identitygorm.Workspace{ID: 2, Name: "Production Copy", Slug: "production-copy", Enabled: true, IsDefault: true}
	team := identitygorm.Team{ID: 7, WorkspaceID: workspace.ID, Name: "Imported Team"}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatal(err)
	}
	spec := bootstrapTestSpec()
	spec.WorkspaceName = "Must Not Replace Production Copy"
	spec.WorkspaceSlug = workspace.Slug
	spec.TeamName = "Must Not Create Preview Team"
	if err := bootstrap(db, spec, func([]byte, int) ([]byte, error) {
		return []byte("generated-hash"), nil
	}); err != nil {
		t.Fatalf("bootstrap existing workspace: %v", err)
	}
	var workspaceCount, teamCount, memberCount, teamMemberCount int64
	db.Model(&identitygorm.Workspace{}).Count(&workspaceCount)
	db.Model(&identitygorm.Team{}).Where("workspace_id = ?", workspace.ID).Count(&teamCount)
	db.Model(&identitygorm.WorkspaceMember{}).Where("workspace_id = ?", workspace.ID).Count(&memberCount)
	db.Model(&identitygorm.TeamMember{}).Where("workspace_id = ? AND team_id = ?", workspace.ID, team.ID).
		Count(&teamMemberCount)
	if workspaceCount != 1 || teamCount != 1 || memberCount != 0 || teamMemberCount != 0 {
		t.Fatalf(
			"bootstrap created unexpected membership rows: workspaces=%d teams=%d members=%d team_members=%d",
			workspaceCount, teamCount, memberCount, teamMemberCount,
		)
	}
	var stored identitygorm.Workspace
	if err := db.First(&stored, workspace.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != workspace.Name {
		t.Fatalf("workspace name=%q want=%q", stored.Name, workspace.Name)
	}
}

func TestBootstrapDoesNotCreateASecondDefaultWorkspace(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	existingDefault := identitygorm.Workspace{
		Name: "JiGuang", Slug: "jiguang", Enabled: true, IsDefault: true,
	}
	if err := db.Create(&existingDefault).Error; err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(db, bootstrapTestSpec(), func([]byte, int) ([]byte, error) {
		return []byte("generated-hash"), nil
	}); err != nil {
		t.Fatalf("bootstrap with existing default: %v", err)
	}
	var defaults []identitygorm.Workspace
	if err := db.Where("is_default = ?", true).Find(&defaults).Error; err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 1 || defaults[0].ID != existingDefault.ID {
		t.Fatalf("default workspaces=%+v", defaults)
	}
	var bootstrapWorkspace identitygorm.Workspace
	if err := db.Where("slug = ?", bootstrapTestSpec().WorkspaceSlug).First(&bootstrapWorkspace).Error; err != nil {
		t.Fatal(err)
	}
	if bootstrapWorkspace.IsDefault {
		t.Fatalf("bootstrap workspace unexpectedly became default: %+v", bootstrapWorkspace)
	}
}

func bootstrapTestSpec() config.Bootstrap {
	return config.Bootstrap{
		Username:      "existing",
		Password:      "bootstrap-password",
		DisplayName:   "Bootstrap Administrator",
		WorkspaceName: "Bootstrap Workspace",
		WorkspaceSlug: "bootstrap",
		TeamName:      "Bootstrap Team",
	}
}
