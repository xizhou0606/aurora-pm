package database

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestMigrateAppliesCurrentVersionAndSkipsRecordedVersion(t *testing.T) {
	db := openDatabaseTestDB(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if !db.Migrator().HasTable(&identitygorm.User{}) {
		t.Fatal("first migration did not create users")
	}
	if !db.Migrator().HasTable("pm_runtime_settings") {
		t.Fatal("current migration did not create runtime settings")
	}
	if !db.Migrator().HasColumn(&identitygorm.User{}, "feishu_tenant_id") ||
		!db.Migrator().HasColumn(&identitygorm.User{}, "feishu_open_id") ||
		!db.Migrator().HasIndex(&identitygorm.User{}, "ux_users_feishu_tenant_display_name") ||
		!db.Migrator().HasTable(&identitygorm.LoginTransaction{}) ||
		!db.Migrator().HasColumn(&identitygorm.LoginTransaction{}, "nonce_hash") ||
		!db.Migrator().HasColumn(&identitygorm.LoginTransaction{}, "pkce_verifier") ||
		!db.Migrator().HasTable(&identitygorm.LoginCode{}) ||
		!db.Migrator().HasTable(&identitygorm.UserLoginIdentity{}) ||
		!db.Migrator().HasColumn(&identitygorm.UserLoginIdentity{}, "telegram_user_id") ||
		!db.Migrator().HasIndex(&identitygorm.UserLoginIdentity{}, "ux_user_login_provider") ||
		!db.Migrator().HasIndex(&identitygorm.UserLoginIdentity{}, "ux_login_identity_subject") ||
		!db.Migrator().HasIndex(&identitygorm.UserLoginIdentity{}, "ux_user_login_telegram_user_id") ||
		!db.Migrator().HasTable(&identitygorm.ExternalRegistrationTransaction{}) ||
		!db.Migrator().HasTable(&identitygorm.IdentityOperationTransaction{}) ||
		!db.Migrator().HasTable(&identitygorm.FeishuTenant{}) ||
		!db.Migrator().HasTable(&identitygorm.FeishuLoginApplication{}) {
		t.Fatal("current migration did not create PM login schema")
	}
	if !db.Migrator().HasTable(&identitygorm.AccessRole{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessUserRole{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessUserGroup{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessUserGroupBinding{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessGroupServiceInterface{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessGroupMenu{}) ||
		!db.Migrator().HasTable(&identitygorm.PersonnelAuditEvent{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessServiceInterface{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessRoleServiceInterface{}) ||
		!db.Migrator().HasTable(&identitygorm.AccessRoleMenu{}) ||
		!db.Migrator().HasTable(&workbenchgorm.AccessMenu{}) {
		t.Fatal("current migration did not create global RBAC schema")
	}
	if !db.Migrator().HasTable(&toolruntimegorm.ExternalAuthorizationGrant{}) ||
		!db.Migrator().HasColumn(&toolruntimegorm.ExternalAuthorizationGrant{}, "allowed_tools") {
		t.Fatal("current migration did not create external tool grant schema")
	}
	if !db.Migrator().HasTable(&toolruntimegorm.ExternalAuthorizationRequest{}) {
		t.Fatal("current migration did not create external tool authorization request schema")
	}
	for _, record := range toolruntimegorm.ExternalCredentialMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create external credential table for %T", record)
		}
	}
	for _, record := range toolruntimegorm.WorkspaceConfigurationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create workspace Tool configuration table for %T", record)
		}
	}
	for _, record := range toolruntimegorm.ChangeSetMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create ChangeSet table for %T", record)
		}
	}
	for _, record := range projectgorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create project Tool mutation table for %T", record)
		}
	}
	for _, record := range taskgorm.TestingMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create task testing Tool mutation table for %T", record)
		}
	}
	for _, record := range workflowgorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create workflow Tool mutation table for %T", record)
		}
	}
	for _, record := range taskgorm.CollaborationMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("current migration did not create task collaboration mutation table for %T", record)
		}
	}
	if !db.Migrator().HasColumn(&taskgorm.WorkTask{}, "parent_id") ||
		!db.Migrator().HasColumn(&taskgorm.WorkTask{}, "completed_at") {
		t.Fatal("current migration did not create task parent/completion columns")
	}
	if !db.Migrator().HasTable(&taskgorm.WorkTaskComment{}) ||
		!db.Migrator().HasColumn(&taskgorm.WorkTaskComment{}, "workspace_id") {
		t.Fatal("current migration did not create workspace-scoped task comments")
	}
	if !db.Migrator().HasTable(&taskgorm.WorkTaskAttachment{}) ||
		!db.Migrator().HasTable(&taskgorm.WorkTaskWatcher{}) ||
		!db.Migrator().HasColumn(&taskgorm.WorkTaskActivity{}, "field_key") {
		t.Fatal("current migration did not create task collaboration schema")
	}
	if !db.Migrator().HasColumn(&taskgorm.WorkTask{}, "reporter_user_id") ||
		!db.Migrator().HasColumn(&identitygorm.User{}, "avatar_url") {
		t.Fatal("current migration did not create exact task and people fields")
	}
	if !db.Migrator().HasColumn(&projectgorm.PlanUnit{}, "sort_order") {
		t.Fatal("current migration did not create plan unit sort order")
	}
	if !db.Migrator().HasColumn(&auditgorm.OutboxEvent{}, "claimed_at") ||
		!db.Migrator().HasColumn(&auditgorm.OutboxEvent{}, "claimed_by") ||
		!db.Migrator().HasColumn(&auditgorm.OutboxEvent{}, "next_attempt_at") {
		t.Fatal("current migration did not create outbox delivery claim columns")
	}
	if !db.Migrator().HasColumn(&workflowgorm.WorkflowToken{}, "assignee_user_id") {
		t.Fatal("current migration did not create workflow token assignee column")
	}
	if !db.Migrator().HasTable(&workflowgorm.WorkflowNodeOpinion{}) ||
		!db.Migrator().HasColumn(&workflowgorm.WorkflowNodeOpinion{}, "author_user_id") {
		t.Fatal("current migration did not create workflow node opinions")
	}
	if !db.Migrator().HasTable(&workbenchgorm.BITeamScoreProfile{}) ||
		!db.Migrator().HasTable(&workbenchgorm.BITeamScoreProfileItem{}) ||
		!db.Migrator().HasColumn(&workbenchgorm.BITeamScoreProfileItem{}, "metric_key") {
		t.Fatal("current migration did not create BI team score profile schema")
	}
	if !db.Migrator().HasTable(&workbenchgorm.BITeamMetricSettings{}) ||
		!db.Migrator().HasColumn(&workbenchgorm.BITeamMetricSettings{}, "approval_node_id") {
		t.Fatal("current migration did not create BI team metric settings schema")
	}
	if !db.Migrator().HasTable(&qualitygorm.QualitySceneOption{}) ||
		!db.Migrator().HasColumn(&qualitygorm.QualitySceneOption{}, "kind") ||
		!db.Migrator().HasColumn(&qualitygorm.QualitySceneOption{}, "normalized_name") {
		t.Fatal("current migration did not create quality scene option schema")
	}
	if !db.Migrator().HasTable(&qualitygorm.QualityBugComment{}) ||
		!db.Migrator().HasColumn(&qualitygorm.QualityBugComment{}, "workspace_id") ||
		!db.Migrator().HasColumn(&qualitygorm.QualityBugComment{}, "quality_bug_id") {
		t.Fatal("current migration did not create workspace-scoped quality comments")
	}
	requireCurrentSchemaVersion(t, db)

	if err := db.Migrator().DropTable(&identitygorm.User{}); err != nil {
		t.Fatalf("drop users: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migration with recorded version: %v", err)
	}
	if db.Migrator().HasTable(&identitygorm.User{}) {
		t.Fatal("recorded schema version should skip AutoMigrate")
	}
}

func TestMigrateAdvancesExternalCredentialsSchemaToCurrentToolRuntime(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionExternalCredentials}).Error; err != nil {
		t.Fatalf("record 0041 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0041 to 0045: %v", err)
	}
	for _, record := range toolruntimegorm.WorkspaceConfigurationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0043 migration did not create workspace Tool configuration table for %T", record)
		}
	}
	if !db.Migrator().HasColumn(&toolruntimegorm.ExternalToolingWorkspaceConfiguration{}, "mode") ||
		!db.Migrator().HasColumn(&toolruntimegorm.ExternalAuthorizationRequest{}, "configuration_version") {
		t.Fatal("0043 migration did not create configuration mode/version columns")
	}
	for _, record := range toolruntimegorm.ChangeSetMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0044 migration did not create ChangeSet table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesExistingWorkspaceToolConfigurationToModes(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &legacyWorkspaceToolConfiguration{}); err != nil {
		t.Fatalf("create 0042 schema: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionWorkspaceToolConfig}).Error; err != nil {
		t.Fatalf("record 0042 schema: %v", err)
	}
	if err := db.Create(&legacyWorkspaceToolConfiguration{
		WorkspaceID: 11, AllowedTools: `["project.list"]`, AllowedScopes: `["project:read"]`,
		EffectiveFrom: time.Date(2026, 8, 27, 3, 0, 0, 0, time.UTC),
		ExpiresAt:     time.Date(2026, 8, 28, 3, 0, 0, 0, time.UTC), UpdatedBy: 7, Version: 1,
		CreatedAt: time.Date(2026, 8, 27, 2, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 27, 2, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatalf("seed 0042 configuration: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0042 to 0043: %v", err)
	}
	var row toolruntimegorm.ExternalToolingWorkspaceConfiguration
	if err := db.First(&row, "workspace_id = ?", 11).Error; err != nil {
		t.Fatal(err)
	}
	if row.Mode != "window" || row.EffectiveFrom == nil || row.ExpiresAt == nil {
		t.Fatalf("migrated configuration=%+v", row)
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesWorkspaceToolModesToChangeSets(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionWorkspaceToolModes}).Error; err != nil {
		t.Fatalf("record 0043 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0043 to 0045: %v", err)
	}
	for _, record := range toolruntimegorm.ChangeSetMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0044 migration did not create ChangeSet table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesChangeSetsToProjectToolMutationLedger(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionToolChangeSets}).Error; err != nil {
		t.Fatalf("record 0044 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0044 to 0045: %v", err)
	}
	for _, record := range projectgorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0045 migration did not create project Tool mutation table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesProjectToolMutationsToQualityToolMutationLedger(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionProjectToolMutations}).Error; err != nil {
		t.Fatalf("record 0045 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0045 to 0046: %v", err)
	}
	for _, record := range qualitygorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0046 migration did not create quality Tool mutation table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesQualityToolMutationsToProjectDeletionSagas(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &projectgorm.ProjectSet{}); err != nil {
		t.Fatalf("create prior schema: %v", err)
	}
	if err := db.Migrator().DropColumn(&projectgorm.ProjectSet{}, "MutationState"); err != nil {
		t.Fatalf("remove future project mutation state: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionQualityToolMutations}).Error; err != nil {
		t.Fatalf("record 0046 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0046 to 0047: %v", err)
	}
	if !db.Migrator().HasTable(&projectgorm.ProjectDeletionSaga{}) {
		t.Fatal("0047 migration did not create project deletion saga table")
	}
	if !db.Migrator().HasColumn(&projectgorm.ProjectSet{}, "MutationState") {
		t.Fatal("0047 migration did not add project mutation state")
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsProjectPlanUnitMutationStateAt0051(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &projectgorm.PlanUnit{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&projectgorm.PlanUnit{}, "MutationState"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionWorkflowToolMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate to 0051: %v", err)
	}
	if !db.Migrator().HasColumn(&projectgorm.PlanUnit{}, "MutationState") {
		t.Fatal("0051 missing PlanUnit mutation_state")
	}
	if !db.Migrator().HasTable(&projectgorm.PlanDeletionSaga{}) || !db.Migrator().HasTable(&projectgorm.PlanCreationSaga{}) {
		t.Fatal("0051 missing plan-unit mutation saga tables")
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsTaskCoreMutationSagaAt0052(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &taskgorm.WorkTask{}); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&taskgorm.WorkTask{}, "MutationState") {
		if err := db.Migrator().DropColumn(&taskgorm.WorkTask{}, "MutationState"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionProjectPlanMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0051 through 0052: %v", err)
	}
	if !db.Migrator().HasColumn(&taskgorm.WorkTask{}, "MutationState") {
		t.Fatal("0052 migration did not add task mutation_state")
	}
	for _, record := range taskgorm.CoreMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0052 migration did not create task core record for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsTaskBatchDeletionFreezeAt0053(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionTaskCoreMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0052 through 0053: %v", err)
	}
	for _, record := range taskgorm.BatchMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0053 migration did not create task batch record for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsRequirementToolMutationsAt0054(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &requirementgorm.Requirement{}); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(&requirementgorm.Requirement{}, "MutationState") {
		if err := db.Migrator().DropColumn(&requirementgorm.Requirement{}, "MutationState"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionTaskBatchMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0053 through 0054: %v", err)
	}
	if !db.Migrator().HasColumn(&requirementgorm.Requirement{}, "MutationState") {
		t.Fatal("0054 migration did not add requirement mutation_state")
	}
	for _, record := range requirementgorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0054 migration did not create requirement mutation record for %T", record)
		}
	}
	if !db.Migrator().HasColumn(&requirementgorm.ToolMutationApply{}, "ChangeSetID") {
		t.Fatal("0054 mutation ledger does not bind ChangeSet ID")
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsExternalOAuthAt0055(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &toolruntimegorm.ExternalAuthorizationRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionRequirementMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0054 through 0055: %v", err)
	}
	if !db.Migrator().HasTable(&toolruntimegorm.ExternalOAuthClient{}) {
		t.Fatal("0055 migration did not create external OAuth clients")
	}
	for _, column := range []string{"OAuthState", "OAuthRedirectURI", "OAuthPKCEChallenge", "OAuthResource", "OAuthScope"} {
		if !db.Migrator().HasColumn(&toolruntimegorm.ExternalAuthorizationRequest{}, column) {
			t.Fatalf("0055 migration did not add authorization request column %s", column)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestFreshSQLiteMigrateAddsCLIDeviceAuthorizationTable(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&toolruntimegorm.ExternalDeviceAuthorization{}) {
		t.Fatal("fresh SQLite migration did not create CLI device authorization transactions")
	}
	for _, column := range []string{"DeviceCodeDigest", "UserCodeDigest", "WorkspaceID", "Status", "ExpiresAt", "PollIntervalSeconds"} {
		if !db.Migrator().HasColumn(&toolruntimegorm.ExternalDeviceAuthorization{}, column) {
			t.Fatalf("CLI device authorization table is missing column %s", column)
		}
	}
}

func TestMigrateAddsTaskCollaborationMutationSagaAt0050(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &taskgorm.WorkTaskAttachment{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&taskgorm.WorkTaskAttachment{}, "MutationState"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionWorkflowToolMutations}).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0049 through 0050: %v", err)
	}
	for _, record := range taskgorm.CollaborationMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0050 migration did not create task collaboration record for %T", record)
		}
	}
	if !db.Migrator().HasColumn(&taskgorm.WorkTaskAttachment{}, "MutationState") {
		t.Fatal("0050 migration did not add attachment mutation_state")
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesProjectDeletionSagasToTaskTestingMutationLedger(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create prior schema: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionProjectDeletionSagas}).Error; err != nil {
		t.Fatalf("record 0047 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0047 to 0048: %v", err)
	}
	for _, record := range taskgorm.TestingMutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0048 migration did not create task testing Tool mutation table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAdvancesTaskTestingMutationsToWorkflowMutationLedger(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create prior schema: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionTaskTestingMutations}).Error; err != nil {
		t.Fatalf("record 0048 schema: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate from 0048 to 0049: %v", err)
	}
	for _, record := range workflowgorm.MutationMigrationRecords() {
		if !db.Migrator().HasTable(record) {
			t.Fatalf("0049 migration did not create workflow Tool mutation table for %T", record)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

type legacyWorkspaceToolConfiguration struct {
	WorkspaceID   uint `gorm:"primaryKey"`
	AllowedTools  string
	AllowedScopes string
	EffectiveFrom time.Time
	ExpiresAt     time.Time
	UpdatedBy     uint
	Version       uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (legacyWorkspaceToolConfiguration) TableName() string {
	return "pm_external_tooling_workspace_configurations"
}

func TestLoadLocalJWTSecretReadsTheDatabaseSetting(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	const secret = "database-secret-that-is-at-least-thirty-two-characters"
	if err := db.Exec(
		"INSERT INTO pm_runtime_settings (setting_key, setting_value) VALUES (?, ?)",
		RuntimeSettingLocalJWTSecret, secret,
	).Error; err != nil {
		t.Fatalf("insert runtime setting: %v", err)
	}

	got, err := LoadLocalJWTSecret(db)
	if err != nil {
		t.Fatalf("load local JWT secret: %v", err)
	}
	if got != secret {
		t.Fatalf("local JWT secret=%q, want database value", got)
	}
}

func TestLoadLocalJWTSecretRejectsMissingOrShortSetting(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		if err := Migrate(db); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if _, err := LoadLocalJWTSecret(db); err == nil || !strings.Contains(err.Error(), "runtime setting") {
			t.Fatalf("missing setting error=%v", err)
		}
	})
	t.Run("short", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		if err := Migrate(db); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if err := db.Exec(
			"INSERT INTO pm_runtime_settings (setting_key, setting_value) VALUES (?, ?)",
			RuntimeSettingLocalJWTSecret, "too-short",
		).Error; err != nil {
			t.Fatalf("insert runtime setting: %v", err)
		}
		if _, err := LoadLocalJWTSecret(db); err == nil || !strings.Contains(err.Error(), "at least 32") {
			t.Fatalf("short setting error=%v", err)
		}
	})
}

func TestMigrateRetriesAfterFailedAutoMigrate(t *testing.T) {
	db := openDatabaseTestDB(t)

	if err := db.Exec("CREATE VIEW users AS SELECT 1 AS id").Error; err != nil {
		t.Fatalf("create conflicting users view: %v", err)
	}
	if err := Migrate(db); err == nil {
		t.Fatal("expected migration to fail with conflicting users view")
	}
	applied, err := schemaVersionApplied(db, currentSchemaVersion)
	if err != nil {
		t.Fatalf("check schema version after failure: %v", err)
	}
	if applied {
		t.Fatal("failed migration must not record the current schema version")
	}

	if err := db.Exec("DROP VIEW users").Error; err != nil {
		t.Fatalf("drop conflicting users view: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("retry migration: %v", err)
	}
	if !db.Migrator().HasTable(&identitygorm.User{}) {
		t.Fatal("retry migration did not create users")
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateConvertsLegacyFeishuIdentityToCompanyNameKey(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &legacyFeishuUser{}, &identitygorm.FeishuTenant{}); err != nil {
		t.Fatalf("create legacy login schema: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionFeishuTenantLogin}).Error; err != nil {
		t.Fatalf("record legacy schema version: %v", err)
	}
	tenantKey, firstOpenID, secondOpenID := "tenant_legacy", "ou_alice", "ou_bob"
	legacyUsers := []legacyFeishuUser{
		{ID: 7, ExternalSubject: "one2all|7", ExternalIssuer: "one2all", Username: "alice", DisplayName: "Alice", FeishuOpenID: &firstOpenID, FeishuTenantKey: &tenantKey, Role: "user", Active: true},
		{ID: 8, ExternalSubject: "one2all|8", ExternalIssuer: "one2all", Username: "bob", DisplayName: "Bob", FeishuOpenID: &secondOpenID, FeishuTenantKey: &tenantKey, Role: "user", Active: true},
	}
	if err := db.Create(&legacyUsers).Error; err != nil {
		t.Fatalf("insert legacy users: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate legacy Feishu identity: %v", err)
	}
	if db.Migrator().HasColumn(&legacyFeishuUserColumns{}, "FeishuTenantKey") ||
		!db.Migrator().HasColumn(&identitygorm.User{}, "feishu_open_id") ||
		!db.Migrator().HasColumn(&identitygorm.User{}, "feishu_tenant_id") ||
		!db.Migrator().HasIndex(&identitygorm.User{}, "ux_users_feishu_tenant_display_name") {
		t.Fatal("legacy app-scoped Feishu identity was not replaced by company/name identity")
	}
	var tenant identitygorm.FeishuTenant
	if err := db.Where("tenant_key = ?", tenantKey).First(&tenant).Error; err != nil {
		t.Fatalf("load migrated tenant: %v", err)
	}
	var users []identitygorm.User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		t.Fatalf("load migrated users: %v", err)
	}
	if len(users) != 2 || users[0].FeishuTenantID == nil || users[1].FeishuTenantID == nil ||
		*users[0].FeishuTenantID != tenant.ID || *users[1].FeishuTenantID != tenant.ID {
		t.Fatalf("migrated users=%+v tenant=%+v", users, tenant)
	}
	for _, user := range users {
		if user.FeishuOpenID != nil && strings.TrimSpace(*user.FeishuOpenID) != "" {
			t.Fatalf("legacy open_id must not be copied: %+v", user)
		}
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateKeepsFeishuIMOpenIDAfterV12Schema(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &identitygorm.User{}, &identitygorm.FeishuTenant{}); err != nil {
		t.Fatalf("create v1.2 login schema: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionFeishuIMOpenID}).Error; err != nil {
		t.Fatalf("record v1.2 schema version: %v", err)
	}
	tenant := identitygorm.FeishuTenant{TenantKey: "tenant_im", Enabled: true}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	openID := "ou_delivery"
	user := identitygorm.User{
		Username: "alice", DisplayName: "Alice", Role: "user", Active: true,
		FeishuOpenID: &openID, FeishuTenantID: &tenant.ID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("insert IM delivery user: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate v1.2 Feishu IM schema: %v", err)
	}
	if !db.Migrator().HasTable("pm_runtime_settings") {
		t.Fatal("v1.2 databases must still receive runtime settings")
	}
	var stored identitygorm.User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("load IM delivery user: %v", err)
	}
	if stored.FeishuOpenID == nil || *stored.FeishuOpenID != openID {
		t.Fatalf("feishu_open_id must stay the IM delivery identity: %+v", stored)
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsEmptyNodePlansToExistingWorkflowInstances(t *testing.T) {
	db := openDatabaseTestDB(t)

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionAI}).Error; err != nil {
		t.Fatalf("record previous schema version: %v", err)
	}
	if err := db.Exec(`CREATE TABLE workflow_instances (
		id integer PRIMARY KEY,
		workspace_id integer NOT NULL,
		template_id integer NOT NULL,
		template_version_id integer NOT NULL,
		subject_type text NOT NULL,
		subject_id integer NOT NULL,
		status text NOT NULL DEFAULT "running",
		current_node_ids text NOT NULL,
		started_by_user_id integer NOT NULL,
		started_at datetime NOT NULL,
		completed_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create previous workflow instance schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO workflow_instances (
		id, workspace_id, template_id, template_version_id, subject_type,
		subject_id, current_node_ids, started_by_user_id, started_at
	) VALUES (1, 2, 3, 4, 'requirement', 5, '[]', 6, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("insert existing workflow instance: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate existing workflow instance: %v", err)
	}
	if !db.Migrator().HasColumn(&workflowgorm.WorkflowInstance{}, "node_plans_json") {
		t.Fatal("migration did not add workflow node plans")
	}
	var nodePlans string
	if err := db.Raw("SELECT node_plans_json FROM workflow_instances WHERE id = 1").Scan(&nodePlans).Error; err != nil {
		t.Fatalf("read migrated workflow instance: %v", err)
	}
	if nodePlans != "{}" {
		t.Fatalf("migrated node plans=%q, want empty JSON object", nodePlans)
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateAddsNullableAssigneeToExistingWorkflowTokens(t *testing.T) {
	db := openDatabaseTestDB(t)

	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("create schema migration table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: schemaVersionWorkflowArtifacts}).Error; err != nil {
		t.Fatalf("record previous schema version: %v", err)
	}
	if err := db.Exec(`CREATE TABLE workflow_tokens (
		id integer PRIMARY KEY,
		workspace_id integer NOT NULL,
		instance_id integer NOT NULL,
		node_id text NOT NULL,
		status text NOT NULL DEFAULT "active",
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create previous workflow token schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO workflow_tokens (
		id, workspace_id, instance_id, node_id, status
	) VALUES (1, 2, 3, 'review', 'active')`).Error; err != nil {
		t.Fatalf("insert existing workflow token: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate existing workflow token: %v", err)
	}
	if !db.Migrator().HasColumn(&workflowgorm.WorkflowToken{}, "assignee_user_id") {
		t.Fatal("migration did not add workflow token assignee")
	}
	var token workflowgorm.WorkflowToken
	if err := db.Where("id = ?", 1).First(&token).Error; err != nil {
		t.Fatalf("read migrated workflow token: %v", err)
	}
	if token.AssigneeUserID != nil || token.NodeID != "review" || token.Status != "active" {
		t.Fatalf("existing workflow token was not preserved: %+v", token)
	}
	requireCurrentSchemaVersion(t, db)
}

func TestMigrateRestoresFeishuIMOpenIDColumnWhenVersionAlreadyCurrent(t *testing.T) {
	db := openDatabaseTestDB(t)
	if err := db.AutoMigrate(&schemaMigration{}, &identitygorm.User{}); err != nil {
		t.Fatalf("create current users table: %v", err)
	}
	if err := db.Create(&schemaMigration{Version: currentSchemaVersion}).Error; err != nil {
		t.Fatalf("record current schema version: %v", err)
	}
	if err := db.Migrator().DropColumn(&identitygorm.User{}, "FeishuOpenID"); err != nil {
		t.Fatalf("drop IM delivery column: %v", err)
	}
	if db.Migrator().HasColumn(&identitygorm.User{}, "feishu_open_id") {
		t.Fatal("test setup must remove feishu_open_id")
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate after current version with missing IM column: %v", err)
	}
	if !db.Migrator().HasColumn(&identitygorm.User{}, "feishu_open_id") {
		t.Fatal("feishu_open_id must be restored even when schema version is already current")
	}
}

func TestPrepareSchemaModes(t *testing.T) {
	t.Run("validate rejects an unmigrated database without changing it", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		err := PrepareSchema(db, config.SchemaMigrationValidate)
		if err == nil || !strings.Contains(err.Error(), "PM_SCHEMA_MIGRATION_MODE=apply") {
			t.Fatalf("validate error=%v", err)
		}
		if db.Migrator().HasTable(&schemaMigration{}) {
			t.Fatal("validate mode must not create the schema migration table")
		}
	})

	t.Run("validate accepts the current version", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		if err := PrepareSchema(db, config.SchemaMigrationApply); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
		if err := PrepareSchema(db, config.SchemaMigrationValidate); err != nil {
			t.Fatalf("validate current schema: %v", err)
		}
	})

	t.Run("off neither validates nor migrates", func(t *testing.T) {
		db := openDatabaseTestDB(t)
		if err := PrepareSchema(db, config.SchemaMigrationOff); err != nil {
			t.Fatalf("disable schema preparation: %v", err)
		}
		if db.Migrator().HasTable(&schemaMigration{}) {
			t.Fatal("off mode must not create the schema migration table")
		}
	})
}

func openDatabaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := Open(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseDSN:    fmt.Sprintf("file:%s?mode=memory&cache=shared", name),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	return db
}

func requireCurrentSchemaVersion(t *testing.T, db *gorm.DB) {
	t.Helper()
	applied, err := schemaVersionApplied(db, currentSchemaVersion)
	if err != nil {
		t.Fatalf("check current schema version: %v", err)
	}
	if !applied {
		t.Fatalf("schema version %q was not recorded", currentSchemaVersion)
	}
}

type legacyFeishuUser struct {
	ID              uint    `gorm:"primaryKey"`
	ExternalSubject string  `gorm:"size:191;uniqueIndex"`
	ExternalIssuer  string  `gorm:"size:191;index"`
	ExternalUserID  uint    `gorm:"index"`
	Username        string  `gorm:"size:128;uniqueIndex;not null"`
	DisplayName     string  `gorm:"size:128"`
	FeishuOpenID    *string `gorm:"size:191;uniqueIndex"`
	FeishuTenantKey *string `gorm:"size:191;index"`
	Role            string  `gorm:"size:40;not null;default:user"`
	Active          bool    `gorm:"not null;default:true"`
}

func (legacyFeishuUser) TableName() string { return "users" }
