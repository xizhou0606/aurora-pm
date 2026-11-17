package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	bitablegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/bitable/gorm/native"
	codecontribgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/codecontrib/gorm"
	collaborationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/collaboration/gorm"
	designreviewgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview/gorm"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	documentlibrarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	notificationrecords "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm/records"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	resourceaccessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const (
	schemaVersionInitialPM             = "0001_initial_pm"
	schemaVersionNativeWorkbench       = "0002_native_workbench"
	schemaVersionQualitySources        = "0003_quality_source_fields"
	schemaVersionTaskHierarchy         = "0004_task_parent_completion"
	schemaVersionTaskComments          = "0005_task_comments_mentions"
	schemaVersionTaskCollab            = "0006_task_collaboration"
	schemaVersionExactTaskFields       = "0007_exact_task_fields"
	schemaVersionPlanUnitSort          = "0008_plan_unit_sort_order"
	schemaVersionOutboxDelivery        = "0009_outbox_delivery_claims"
	schemaVersionHostNavigation        = "0010_host_navigation_preferences"
	schemaVersionRequirementPool       = "0011_requirement_pool"
	schemaVersionWorkflow              = "0012_workflow_engine"
	schemaVersionAI                    = "0013_ai_runtime"
	schemaVersionWorkflowPlans         = "0014_workflow_node_plans"
	schemaVersionWorkflowArtifacts     = "0015_workflow_node_artifacts"
	schemaVersionWorkflowAssignees     = "0016_workflow_node_assignees"
	schemaVersionRequirementCovers     = "0017_requirement_covers"
	schemaVersionWorkflowOpinions      = "0018_workflow_node_opinions"
	schemaVersionTaskTesting           = "0019_task_testing"
	schemaVersionBITeamScore           = "0020_bi_team_score_profiles"
	schemaVersionBITeamMetrics         = "0021_bi_team_metric_settings"
	schemaVersionQualityStatusTime     = "0022_quality_status_changed_at"
	schemaVersionRequirementLaneOwners = "0023_requirement_lane_owners"
	schemaVersionQualitySceneOptions   = "0023_quality_scene_options"
	schemaVersionRequirementRoleOwners = "0024_requirement_role_owners"
	schemaVersionRoleOwnersAndScenes   = "0025_role_owners_and_quality_scenes"
	schemaVersionIdentityLogin         = "0026_identity_login"
	schemaVersionGlobalRBAC            = "0027_global_rbac"
	schemaVersionFeishuTenantLogin     = "0028_feishu_tenant_login"
	schemaVersionFeishuTenantUsers     = "0029_feishu_tenant_users"
	schemaVersionFeishuTenantNameKey   = "0030_feishu_tenant_name_key"
	schemaVersionQualityComments       = "0031_quality_bug_comments"
	schemaVersionFeishuLoginApp        = "0032_feishu_login_application"
	schemaVersionRuntimeSettings       = "0033_runtime_settings"
	schemaVersionFeishuIMOpenID        = "0033_feishu_im_open_id"
	schemaVersionFeishuIMAndRuntime    = "0034_feishu_im_and_runtime"
	schemaVersionRequirementMaterials  = "0035_requirement_materials_and_plan_unit_reports"
	schemaVersionGoogleLogin           = "0036_google_oidc_login"
	schemaVersionTelegramLogin         = "0037_telegram_oidc_login"
	schemaVersionIdentityAccount       = "0038_identity_account_management"
	schemaVersionPersonnelAccess       = "0039_pm_personnel_access"
	schemaVersionExternalTooling       = "0040_external_tooling"
	schemaVersionExternalCredentials   = "0041_external_tool_credentials"
	schemaVersionWorkspaceToolConfig   = "0042_workspace_tool_configuration"
	schemaVersionWorkspaceToolModes    = "0043_workspace_tool_configuration_modes"
	schemaVersionToolChangeSets        = "0044_tool_change_sets"
	schemaVersionProjectToolMutations  = "0045_project_tool_mutation_applies"
	schemaVersionQualityToolMutations  = "0046_quality_tool_mutation_applies"
	schemaVersionProjectDeletionSagas  = "0047_project_deletion_sagas"
	schemaVersionTaskTestingMutations  = "0048_task_testing_tool_mutation_applies"
	schemaVersionWorkflowToolMutations = "0049_workflow_tool_mutation_applies"
	schemaVersionTaskCollabMutations   = "0050_task_collaboration_mutations"
	schemaVersionProjectPlanMutations  = "0051_project_plan_unit_mutations"
	schemaVersionTaskCoreMutations     = "0052_task_core_mutations"
	schemaVersionTaskBatchMutations    = "0053_task_batch_mutations"
	schemaVersionRequirementMutations  = "0054_requirement_tool_mutations"
	schemaVersionExternalOAuth         = "0055_external_tool_oauth"
	// GORM cutover marker dual-written after golang-migrate succeeds so existing
	// validate-mode processes still accept 0055. Postgres schema after this
	// version lives in backend/migrations/. Do not advance this for new columns.
	currentSchemaVersion = schemaVersionExternalOAuth
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "sqlite":
		dsn := cfg.DatabaseDSN
		if !strings.Contains(dsn, "?") {
			dsn += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		}
		dialector = sqlite.Open(dsn)
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysqlDriver.Open(cfg.DatabaseDSN)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if cfg.DatabaseDriver == "sqlite" {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.SetMaxOpenConns(1)
		}
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := ensureFeishuIMOpenIDColumn(db); err != nil {
		return err
	}
	if isPostgres(db) {
		return migratePostgres(db)
	}
	return migrateLegacyGORM(db)
}

func migrateLegacyGORM(db *gorm.DB) error {
	applied, err := schemaVersionApplied(db, currentSchemaVersion)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := prepareUserAccountStatus(tx); err != nil {
			return err
		}
		if err := prepareLegacyFeishuTenantUsers(tx); err != nil {
			return err
		}
		if err := prepareQualityStatusChangedAt(tx); err != nil {
			return err
		}
		if err := validateFeishuTenantUserNames(tx); err != nil {
			return err
		}
		if err := tx.AutoMigrate(
			&schemaMigration{},
			&identitygorm.User{},
			&identitygorm.LoginTransaction{},
			&identitygorm.LoginCode{},
			&identitygorm.UserLoginIdentity{},
			&identitygorm.ExternalRegistrationTransaction{},
			&identitygorm.IdentityOperationTransaction{},
			&identitygorm.PasswordResetTransaction{},
			&toolruntimegorm.ExternalAuthorizationGrant{},
			&toolruntimegorm.ExternalAuthorizationRequest{},
			&identitygorm.FeishuTenant{},
			&identitygorm.FeishuLoginApplication{},
			&RuntimeSetting{},
			&identitygorm.AccessRole{},
			&identitygorm.AccessUserRole{},
			&identitygorm.AccessServiceInterface{},
			&identitygorm.AccessRoleServiceInterface{},
			&identitygorm.AccessGroupServiceInterface{},
			&identitygorm.AccessRoleMenu{},
			&identitygorm.AccessGroupMenu{},
			&identitygorm.AccessUserGroup{},
			&identitygorm.AccessUserGroupBinding{},
			&identitygorm.PersonnelAuditEvent{},
			&identitygorm.Workspace{},
			&identitygorm.WorkspaceMember{},
			&workbenchgorm.AccessMenu{},
			&workbenchgorm.HostNavigationPreference{},
			&workbenchgorm.BITeamScoreProfile{},
			&workbenchgorm.BITeamScoreProfileItem{},
			&workbenchgorm.BITeamMetricSettings{},
			&identitygorm.Team{},
			&identitygorm.TeamMember{},
			&projectgorm.ProjectSet{},
			&projectgorm.ProjectNews{},
			&projectgorm.PlanUnit{},
			&projectgorm.DeliveryNodeLabel{},
			&projectgorm.DeliveryNode{},
			&requirementgorm.Requirement{},
			&requirementgorm.ResourceLinkRecord{},
			&requirementgorm.RequirementCover{},
			&requirementgorm.RequirementMaterial{},
			&taskgorm.WorkTask{},
			&taskgorm.TaskClientOption{},
			&taskgorm.WorkTaskActivity{},
			&taskgorm.WorkTaskComment{},
			&taskgorm.WorkTaskAttachment{},
			&taskgorm.WorkTaskWatcher{},
			&taskgorm.TaskTestDefinition{},
			&taskgorm.TaskTestExecution{},
			&taskgorm.TaskTestExecutionRun{},
			&taskgorm.KeySequence{},
			&qualitygorm.QualityBug{},
			&qualitygorm.QualityBugActivity{},
			&qualitygorm.QualityBugComment{},
			&qualitygorm.QualitySceneOption{},
			&qualitygorm.PlanUnitTestReport{},
			&projectgorm.ProjectPermission{},
			&projectgorm.ProjectMemberRole{},
			&auditgorm.AuditLog{},
			&auditgorm.OutboxEvent{},
			&notificationrecords.InboxReceipt{},
			&workflowgorm.WorkflowTemplate{},
			&workflowgorm.WorkflowTemplateVersion{},
			&workflowgorm.WorkflowInstance{},
			&workflowgorm.WorkflowToken{},
			&workflowgorm.WorkflowNodeRun{},
			&workflowgorm.WorkflowApproval{},
			&workflowgorm.WorkflowNodeOpinion{},
			&workflowgorm.WorkflowJob{},
			&workflowgorm.WorkflowNodeArtifact{},
			&aigorm.AIRun{},
			&aigorm.AIRunSource{},
			&aigorm.AIArtifact{},
			&aigorm.AIArtifactApplication{},
			&aigorm.AIFeedback{},
			&aigorm.AIUsageDaily{},
		); err != nil {
			return err
		}
		if err := tx.AutoMigrate(toolruntimegorm.ExternalCredentialMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(toolruntimegorm.OAuthMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(toolruntimegorm.DeviceAuthorizationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(toolruntimegorm.WorkspaceConfigurationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(projectgorm.MemberMutationAuditMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(projectgorm.PlanUnitMemberMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(projectgorm.MutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(qualitygorm.MutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(projectgorm.DeletionMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(taskgorm.TestingMutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(workflowgorm.MutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(taskgorm.CollaborationMutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(projectgorm.PlanMutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(taskgorm.CoreMutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(taskgorm.BatchMutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(requirementgorm.MutationMigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(designreviewgorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(documentgorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(bitablegorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(documentlibrarygorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(resourceaccessgorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(collaborationgorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := tx.AutoMigrate(codecontribgorm.MigrationRecords()...); err != nil {
			return err
		}
		if err := backfillQualityStatusChangedAt(tx); err != nil {
			return err
		}
		if err := backfillRequirementRoleOwners(tx); err != nil {
			return err
		}
		if err := backfillUserAccountStatus(tx); err != nil {
			return err
		}
		if err := tx.Model(&identitygorm.AccessServiceInterface{}).
			Where("action_code = '' OR action_code IS NULL").UpdateColumn("action_code", gorm.Expr("code")).Error; err != nil {
			return err
		}
		if err := identitygorm.BackfillDefaultUserGroup(tx); err != nil {
			return err
		}
		if err := validateFeishuTenantUserNames(tx); err != nil {
			return err
		}
		return tx.FirstOrCreate(&schemaMigration{Version: currentSchemaVersion}).Error
	})
}

type legacyUserAccountStatus struct {
	Status string `gorm:"column:status"`
}

func (legacyUserAccountStatus) TableName() string { return "users" }

// prepareUserAccountStatus preserves the legacy active=false meaning before
// AutoMigrate adds the non-null status column with its active default.
func prepareUserAccountStatus(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&identitygorm.User{}) {
		return nil
	}
	if !tx.Migrator().HasColumn(&legacyUserAccountStatus{}, "status") {
		if err := tx.Migrator().AddColumn(&identitygorm.User{}, "Status"); err != nil {
			return err
		}
	}
	return tx.Table("users").
		Where("active = ? AND (status = '' OR status IS NULL OR status = ?)", false, "active").
		UpdateColumn("status", "banned").Error
}

func backfillUserAccountStatus(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&identitygorm.User{}) ||
		!tx.Migrator().HasColumn(&legacyUserAccountStatus{}, "status") {
		return nil
	}
	if err := tx.Model(&identitygorm.User{}).
		Where("active = ? AND (status = '' OR status IS NULL OR status = ?)", false, "active").
		Updates(map[string]any{"status": "banned", "status_reason": "migrated from legacy inactive account"}).Error; err != nil {
		return err
	}
	return tx.Model(&identitygorm.User{}).
		Where("active = ? AND (status = '' OR status IS NULL)", true).
		Update("status", "active").Error
}

// legacyFeishuUserColumns represents the application-scoped identity columns
// from schema 0028. The 0028 tenant_key column is dropped; feishu_open_id is
// later restored as the dedicated login application's delivery identity.
type legacyFeishuUserColumns struct {
	FeishuOpenID    *string `gorm:"column:feishu_open_id"`
	FeishuTenantKey *string `gorm:"column:feishu_tenant_key"`
}

func (legacyFeishuUserColumns) TableName() string { return "users" }

type legacyFeishuUserTenantID struct {
	FeishuTenantID *uint `gorm:"column:feishu_tenant_id"`
}

func (legacyFeishuUserTenantID) TableName() string { return "users" }

type legacyFeishuTenantKey struct {
	TenantKey string `gorm:"column:tenant_key"`
}

// prepareLegacyFeishuTenantUsers migrates the 0028 app-specific binding to
// the company record before removing the old columns. It only touches users
// that already have a stored legacy tenant key.
func prepareLegacyFeishuTenantUsers(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&identitygorm.User{}) {
		return nil
	}
	hasOpenID := tx.Migrator().HasColumn(&legacyFeishuUserColumns{}, "FeishuOpenID")
	hasTenantKey := tx.Migrator().HasColumn(&legacyFeishuUserColumns{}, "FeishuTenantKey")
	if !hasOpenID && !hasTenantKey {
		return nil
	}
	if err := tx.AutoMigrate(&identitygorm.FeishuTenant{}); err != nil {
		return err
	}
	if !tx.Migrator().HasColumn(&identitygorm.User{}, "feishu_tenant_id") {
		if err := tx.Migrator().AddColumn(&legacyFeishuUserTenantID{}, "FeishuTenantID"); err != nil {
			return err
		}
	}
	if hasTenantKey {
		if err := backfillLegacyFeishuTenantIDs(tx); err != nil {
			return err
		}
	}
	if err := validateFeishuTenantUserNames(tx); err != nil {
		return err
	}
	// Drop open_id only while converting 0028 (which also stored tenant_key).
	// After that cutover, feishu_open_id is PM's Feishu IM delivery identity.
	if hasOpenID && hasTenantKey {
		if err := tx.Migrator().DropColumn(&legacyFeishuUserColumns{}, "FeishuOpenID"); err != nil {
			return err
		}
	}
	if hasTenantKey {
		if err := tx.Migrator().DropColumn(&legacyFeishuUserColumns{}, "FeishuTenantKey"); err != nil {
			return err
		}
	}
	return nil
}

// ensureFeishuIMOpenIDColumn restores users.feishu_open_id even when the
// current schema version is already recorded. A previous migrator dropped that
// column after 0034, so later boots skipped AutoMigrate and never added it back.
func ensureFeishuIMOpenIDColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable(&identitygorm.User{}) {
		return nil
	}
	if db.Migrator().HasColumn(&identitygorm.User{}, "feishu_open_id") {
		return nil
	}
	return db.Migrator().AddColumn(&identitygorm.User{}, "FeishuOpenID")
}

func backfillLegacyFeishuTenantIDs(tx *gorm.DB) error {
	var keys []legacyFeishuTenantKey
	if err := tx.Table("users").
		Select("DISTINCT TRIM(feishu_tenant_key) AS tenant_key").
		Where("feishu_tenant_key IS NOT NULL AND TRIM(feishu_tenant_key) <> ''").
		Scan(&keys).Error; err != nil {
		return err
	}
	for _, key := range keys {
		var tenant identitygorm.FeishuTenant
		err := tx.Where("tenant_key = ?", key.TenantKey).First(&tenant).Error
		if err == gorm.ErrRecordNotFound {
			tenant = identitygorm.FeishuTenant{TenantKey: key.TenantKey, Enabled: true}
			if err := tx.Create(&tenant).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := tx.Table("users").
			Where("TRIM(feishu_tenant_key) = ? AND feishu_tenant_id IS NULL", key.TenantKey).
			Update("feishu_tenant_id", tenant.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

// validateFeishuTenantUserNames gives an actionable migration failure before
// the database creates the permanent company/name uniqueness index.
func validateFeishuTenantUserNames(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&identitygorm.User{}) ||
		!tx.Migrator().HasColumn(&identitygorm.User{}, "feishu_tenant_id") {
		return nil
	}
	var duplicates []struct {
		FeishuTenantID uint
		DisplayName    string
		Count          int64
	}
	if err := tx.Table("users").
		Select("feishu_tenant_id, display_name, COUNT(*) AS count").
		Where("feishu_tenant_id IS NOT NULL").
		Group("feishu_tenant_id, display_name").
		Having("COUNT(*) > 1").
		Limit(1).
		Scan(&duplicates).Error; err != nil {
		return err
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("users contain duplicate names for one Feishu tenant")
	}
	return nil
}

type nullableQualityStatusChangedAt struct {
	StatusChangedAt *time.Time `gorm:"column:status_changed_at"`
}

func (nullableQualityStatusChangedAt) TableName() string {
	return "quality_bugs"
}

func prepareQualityStatusChangedAt(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&qualitygorm.QualityBug{}) {
		return nil
	}
	if !tx.Migrator().HasColumn(&qualitygorm.QualityBug{}, "status_changed_at") {
		if err := tx.Migrator().AddColumn(
			&nullableQualityStatusChangedAt{},
			"StatusChangedAt",
		); err != nil {
			return err
		}
	}
	return backfillQualityStatusChangedAt(tx)
}

func backfillQualityStatusChangedAt(tx *gorm.DB) error {
	var bugs []qualitygorm.QualityBug
	if err := tx.Where("status_changed_at IS NULL OR status_changed_at = ?", time.Time{}).Find(&bugs).Error; err != nil {
		return err
	}
	for _, bug := range bugs {
		changedAt := bug.CreatedAt
		var activity qualitygorm.QualityBugActivity
		err := tx.Where(
			"workspace_id = ? AND quality_bug_id = ? AND field_key = ?",
			bug.WorkspaceID, bug.ID, "status",
		).Order("created_at DESC, id DESC").First(&activity).Error
		if err == nil {
			changedAt = activity.CreatedAt
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		if changedAt.IsZero() {
			changedAt = time.Now().UTC()
		}
		if err := tx.Model(&qualitygorm.QualityBug{}).Where("id = ?", bug.ID).
			Update("status_changed_at", changedAt.UTC()).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillRequirementRoleOwners(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&requirementgorm.Requirement{}) {
		return nil
	}
	var rows []requirementgorm.Requirement
	if err := tx.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		beforeProduct := append(requirementgorm.UintList(nil), row.ProductOwnerUserIDs...)
		beforeDesign := append(requirementgorm.UintList(nil), row.DesignOwnerUserIDs...)
		beforeFrontend := append(requirementgorm.UintList(nil), row.FrontendOwnerUserIDs...)
		beforeBackend := append(requirementgorm.UintList(nil), row.BackendOwnerUserIDs...)
		row.SyncRoleOwners()
		if sameUintList(beforeProduct, row.ProductOwnerUserIDs) &&
			sameUintList(beforeDesign, row.DesignOwnerUserIDs) &&
			sameUintList(beforeFrontend, row.FrontendOwnerUserIDs) &&
			sameUintList(beforeBackend, row.BackendOwnerUserIDs) {
			continue
		}
		if err := tx.Model(&requirementgorm.Requirement{}).Where("id = ?", row.ID).Updates(map[string]any{
			"product_owner_user_ids":  row.ProductOwnerUserIDs,
			"design_owner_user_ids":   row.DesignOwnerUserIDs,
			"frontend_owner_user_ids": row.FrontendOwnerUserIDs,
			"backend_owner_user_ids":  row.BackendOwnerUserIDs,
			"tester_user_ids":         row.TesterUserIDs,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func sameUintList(left, right requirementgorm.UintList) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// PrepareSchema controls whether this process owns schema changes. Shared
// development API processes should validate while one designated process
// applies migrations.
func PrepareSchema(db *gorm.DB, mode config.SchemaMigrationMode) error {
	switch mode {
	case "", config.SchemaMigrationApply:
		return Migrate(db)
	case config.SchemaMigrationValidate:
		if isPostgres(db) {
			if err := validatePostgresSQLMigrations(db); err != nil {
				return err
			}
		}
		applied, err := schemaVersionApplied(db, currentSchemaVersion)
		if err != nil {
			return err
		}
		if !applied {
			return fmt.Errorf(
				"database schema version %q is not applied; run the designated migrator with PM_SCHEMA_MIGRATION_MODE=apply",
				currentSchemaVersion,
			)
		}
		return nil
	case config.SchemaMigrationOff:
		return nil
	default:
		return fmt.Errorf("unsupported schema migration mode %q", mode)
	}
}

func schemaVersionApplied(db *gorm.DB, version string) (bool, error) {
	if !db.Migrator().HasTable(&schemaMigration{}) {
		return false, nil
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

type schemaMigration struct {
	Version   string    `gorm:"primaryKey;size:80"`
	AppliedAt time.Time `gorm:"autoCreateTime"`
}

func (schemaMigration) TableName() string { return "pm_schema_migrations" }
