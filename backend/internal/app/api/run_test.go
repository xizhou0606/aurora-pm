package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	toolruntimepmtools "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/pmtools"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

type biToolTestAuthorizer struct{}

func (biToolTestAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{toolruntimepmtools.BIScreenGet}, []string{"workbench:read"})
}

type cascadeSafetyAuthorizer struct{ tool string }

func (authorizer cascadeSafetyAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{authorizer.tool}, []string{"project:write"})
}

func TestToolRuntimeCompositionFailsWithoutDatabase(t *testing.T) {
	if _, err := newPMToolRuntimeDependencies(nil, nil); err == nil {
		t.Fatal("expected PM Tool dependency bootstrap without database to fail")
	}
	if _, err := assembleToolRuntime(nil, ToolRuntimeDependencies{}); err == nil {
		t.Fatal("expected Tool Runtime route assembly without database to fail")
	}
}

func TestMCPAuthorizationURLUsesOnlyTheConfiguredPublicOrigin(t *testing.T) {
	if got, want := mcpAuthorizationURL("http://localhost:8081/ignored?secret=discarded#fragment"),
		"http://localhost:8081/workspace/mcp-cli/authorize?channel=mcp"; got != want {
		t.Fatalf("authorization URL=%q want=%q", got, want)
	}
	if got := mcpAuthorizationURL("not-an-origin"); got != "" {
		t.Fatalf("invalid public origin produced authorization URL %q", got)
	}
}

func TestNewHTTPServerBootstrapsGlobalAccessCatalog(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_rbac_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-rbac-test-secret-that-is-at-least-thirty-two-characters",
		RBACBootstrapUserID: 7,
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.User{ID: 7, ExternalSubject: "local:admin", Username: "admin", DisplayName: "Admin", Active: true}).Error; err != nil {
		t.Fatal(err)
	}
	engine, err := newHTTPServer(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(engine.Routes()) == 0 {
		t.Fatal("HTTP server has no routes")
	}
	var defaultHistoryGrants int64
	if err := db.Table("access_group_service_interfaces grants").
		Joins("JOIN access_service_interfaces i ON i.id = grants.service_interface_id").
		Where("i.action_code IN ?", []string{string(accesscatalog.ActionExternalToolingGrantsView), string(accesscatalog.ActionExternalToolingGrantsRevoke)}).
		Count(&defaultHistoryGrants).Error; err != nil {
		t.Fatal(err)
	}
	if defaultHistoryGrants != 0 {
		t.Fatalf("default group must not receive authorization history grants: %d", defaultHistoryGrants)
	}
	access := identitygorm.NewAccessRepository(db)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/platform/external-tooling/authorization-records"},
		{"POST", "/api/v1/platform/external-tooling/authorization-records/:id/revoke"},
	} {
		result, err := access.Authorize(context.Background(), 7, route.method, route.path)
		if err != nil || result != identity.AuthorizationAllowed {
			t.Fatalf("administrator history permission: %v %v", result, err)
		}
	}
	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, required := range []string{
		"GET /api/v1/platform/tool-runtime/catalog",
		"GET /api/v1/platform/external-tooling/configuration",
		"GET /api/v1/platform/external-tooling/effective-access",
		"PUT /api/v1/platform/external-tooling/configuration",
		"GET /api/v1/platform/external-tooling/grants",
		"POST /api/v1/platform/external-tooling/authorization-requests",
		"GET /api/v1/platform/external-tooling/authorization-requests/:id",
		"POST /api/v1/platform/external-tooling/authorization-requests/:id/decision",
		"POST /api/v1/platform/external-tooling/authorization-requests/:id/code",
		"GET /api/v1/platform/external-tooling/device-authorizations",
		"POST /api/v1/platform/external-tooling/device-authorizations",
		"POST /api/v1/tool-auth/device",
		"POST /api/v1/tool-auth/token",
		"POST /api/v1/tool-auth/revoke",
		"POST /api/v1/tool-auth/register",
		"GET /.well-known/oauth-protected-resource",
		"GET /.well-known/oauth-authorization-server",
		"GET /api/v1/tool-runtime/catalog",
		"GET /api/v1/tool-runtime/tools/:name",
		"POST /api/v1/tool-runtime/tools/:name/invoke",
		"GET /api/v1/tool-runtime/changesets/:id",
		"POST /api/v1/tool-runtime/changesets/:id/confirm",
		"GET /api/v1/mcp",
		"POST /api/v1/mcp",
	} {
		if _, ok := routes[required]; !ok {
			t.Fatalf("HTTP server is missing external tooling route %s", required)
		}
	}
	var role identitygorm.AccessRole
	if err := db.Where("code = ?", identity.AccessAdministratorRoleCode).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	var bindingCount, interfaceCount, interfaceGrantCount, menuGrantCount int64
	db.Model(&identitygorm.AccessUserRole{}).Where("user_id = ? AND role_id = ?", 7, role.ID).Count(&bindingCount)
	db.Model(&identitygorm.AccessServiceInterface{}).Count(&interfaceCount)
	db.Model(&identitygorm.AccessRoleServiceInterface{}).Where("role_id = ?", role.ID).Count(&interfaceGrantCount)
	db.Model(&identitygorm.AccessRoleMenu{}).Where("role_id = ?", role.ID).Count(&menuGrantCount)
	if bindingCount != 1 || interfaceCount != int64(len(accessAdministrationInterfaces())) || interfaceGrantCount != interfaceCount || menuGrantCount != 1 {
		t.Fatalf("bootstrap counts binding=%d interfaces=%d grants=%d menu_grants=%d", bindingCount, interfaceCount, interfaceGrantCount, menuGrantCount)
	}
	var menu workbenchgorm.AccessMenu
	if err := db.Where("code = ?", "access-administration").First(&menu).Error; err != nil || !menu.SystemManaged {
		t.Fatalf("system menu=%+v err=%v", menu, err)
	}
}

func TestNewHTTPServerMountsBothExternalAdaptersFromOneInjectedRuntime(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_external_tools_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-external-tool-test-secret-that-is-at-least-thirty-two-characters",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage))
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, definition := range dependencies.Catalog.Definitions() {
		if definition.Lifecycle == toolruntime.LifecycleActive {
			active++
		}
	}
	expectedActive := 1 + len(toolruntimepmtools.DocumentAccessWriteHandlerNames()) + len(toolruntimepmtools.DocumentAccessReadHandlerNames()) + len(toolruntimepmtools.DocumentReadHandlerNames()) + len(toolruntimepmtools.DocumentWriteHandlerNames()) + len(toolruntimepmtools.ReadHandlerNames()) +
		len(toolruntimepmtools.ProjectListHandlerNames()) +
		len(toolruntimepmtools.ProjectAccessHandlerNames()) +
		len(toolruntimepmtools.TaskReadHandlerNames()) +
		len(toolruntimepmtools.TaskClientOptionHandlerNames()) +
		len(toolruntimepmtools.TaskTestingReadHandlerNames()) +
		len(toolruntimepmtools.TaskTestingWriteHandlerNames()) +
		len(toolruntimepmtools.TaskCollaborationWriteHandlerNames()) +
		len(toolruntimepmtools.TaskCoreWriteHandlerNames()) +
		len(toolruntimepmtools.TaskBatchWriteHandlerNames()) +
		len(toolruntimepmtools.QualityReadHandlerNames()) +
		len(toolruntimepmtools.QualityWriteHandlerNames()) +
		len(toolruntimepmtools.ProjectReadHandlerNames()) +
		len(toolruntimepmtools.ProjectWriteHandlerNames()) +
		len(toolruntimepmtools.RequirementWriteHandlerNames()) +
		len(toolruntimepmtools.AccessReadHandlerNames()) +
		len(toolruntimepmtools.WorkbenchReadHandlerNames()) +
		len(toolruntimepmtools.BIScreenReadHandlerNames()) +
		len(toolruntimepmtools.WorkflowAuxReadHandlerNames()) +
		len(toolruntimepmtools.WorkflowWriteHandlerNames())
	if expectedActive != 132 {
		t.Fatalf("Tool activation inventory=%d, want 132", expectedActive)
	}
	if active != expectedActive || len(dependencies.Handlers) != active {
		t.Fatalf("active=%d handlers=%d expected=%d", active, len(dependencies.Handlers), expectedActive)
	}
	dependencies.ExternalTokenVerifier = toolruntime.ExternalTokenVerifierFunc(func(
		context.Context,
		string,
		toolruntime.DeliveryChannel,
		toolruntime.TokenAudience,
	) (toolruntime.ExecutionAuthorization, error) {
		return toolruntime.ExecutionAuthorization{}, toolruntime.ErrExternalTokenInvalid
	})
	engine, err := newHTTPServerWithToolRuntime(cfg, db, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, required := range []string{
		"GET /api/v1/tool-runtime/catalog",
		"GET /api/v1/tool-runtime/tools/:name",
		"POST /api/v1/tool-runtime/tools/:name/invoke",
		"GET /api/v1/mcp",
		"POST /api/v1/mcp",
	} {
		if _, ok := routes[required]; !ok {
			t.Fatalf("HTTP server is missing injected external Tool route %s", required)
		}
	}
}

func TestDisabledObjectStorageKeepsAttachmentMetadataAndReturnsRetryable(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_disabled_objectstore_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-object-store-test-secret-that-is-at-least-thirty-two-characters",
		ObjectStorage: config.ObjectStorage{Enabled: false},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	workTask := taskgorm.WorkTask{WorkspaceID: 7, TeamID: 8, ProjectSetID: 9, ProjectID: 10, RequirementID: 11, Kind: "task", TaskKey: "TASK-1", Title: "Task", Status: "todo", Priority: "normal"}
	if err := db.Create(&workTask).Error; err != nil {
		t.Fatal(err)
	}
	attachment := taskgorm.WorkTaskAttachment{
		WorkspaceID: 7, WorkTaskID: workTask.ID, UploaderUserID: 3, Filename: "report.txt", Size: 4,
		StorageBackend: "s3", ObjectKey: "workspace/7/tasks/1/report.txt", MutationState: "active",
	}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage))
	if err != nil {
		t.Fatalf("disabled object storage must not fail Tool runtime bootstrap: %v", err)
	}
	handler, ok := dependencies.Handlers[toolruntimepmtools.TaskAttachmentDelete].(toolruntime.IdempotentMutationHandler)
	if !ok {
		t.Fatal("attachment delete is not an idempotent mutation handler")
	}
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(fmt.Sprintf(`{"id":%d,"aid":%d}`, workTask.ID, attachment.ID)),
		ChangeSetID:   "changeset-disabled-storage", ApplyKey: "apply-disabled-storage",
	})
	if !errors.Is(err, toolruntime.ErrMutationRetryable) {
		t.Fatalf("attachment delete error=%v", err)
	}
	var stored taskgorm.WorkTaskAttachment
	if err := db.First(&stored, attachment.ID).Error; err != nil || stored.MutationState != "deleting" || stored.ObjectKey != attachment.ObjectKey {
		t.Fatalf("attachment metadata=%+v err=%v", stored, err)
	}
	var saga taskgorm.AttachmentDeletionSaga
	if err := db.Where("attachment_id = ?", attachment.ID).First(&saga).Error; err != nil || saga.Status != "pending" {
		t.Fatalf("attachment saga=%+v err=%v", saga, err)
	}
}

func TestRequirementDeleteToolApplicationFailsTerminallyWithoutMutatingAttachedTask(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_requirement_cascade_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-requirement-cascade-secret-at-least-thirty-two-characters",
		ObjectStorage: config.ObjectStorage{Enabled: false},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	projectSet := projectgorm.ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Project", Code: "P", Status: "active"}
	plan := projectgorm.PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: projectSet.ID, Title: "Plan", Type: "version", Status: "planning"}
	requirement := requirementgorm.Requirement{ID: 13, WorkspaceID: 7, TeamID: 8, ProjectSetID: projectSet.ID, PlanUnitID: plan.ID, Version: 1, Title: "Requirement", Priority: "normal", Status: "grooming"}
	task := taskgorm.WorkTask{ID: 14, WorkspaceID: 7, TeamID: 8, ProjectSetID: projectSet.ID, ProjectID: plan.ID, RequirementID: requirement.ID,
		Kind: "task", TaskKey: "P-1", Title: "Task", Status: "todo", Priority: "normal", MutationState: "active"}
	attachment := taskgorm.WorkTaskAttachment{ID: 15, WorkspaceID: 7, WorkTaskID: task.ID, UploaderUserID: 3,
		Filename: "report.txt", StorageBackend: "s3", ObjectKey: "workspace/7/report.txt", MutationState: "active"}
	for _, row := range []any{&projectSet, &plan, &requirement, &task, &attachment} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage), cfg)
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(dependencies.Catalog,
		cascadeSafetyAuthorizer{tool: toolruntimepmtools.ProjectRequirementDelete}, dependencies.Handlers,
		toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5,
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: toolruntimepmtools.ProjectRequirementDelete, Input: json.RawMessage(`{"id":13}`),
		IdempotencyKey: "requirement-delete-attached-0001", Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	failed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || failed.Status != toolruntime.ChangeSetFailed || failed.ErrorCode != "TOOL_EXECUTION_FAILED" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	for name, target := range map[string]any{
		"requirement": &requirementgorm.Requirement{}, "task": &taskgorm.WorkTask{}, "attachment": &taskgorm.WorkTaskAttachment{},
	} {
		var count int64
		if err := db.Model(target).Where("workspace_id = ?", 7).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s mutated after terminal conflict: count=%d err=%v", name, count, err)
		}
	}
	var applies int64
	if err := db.Model(&requirementgorm.ToolMutationApply{}).Where("workspace_id = ?", 7).Count(&applies).Error; err != nil || applies != 0 {
		t.Fatalf("requirement mutation ledger leaked: count=%d err=%v", applies, err)
	}
}

func TestProjectDeleteToolApplicationFailsTerminallyBeforeStartingSagaForAttachedTask(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_project_cascade_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-project-cascade-secret-at-least-thirty-two-characters",
		ObjectStorage: config.ObjectStorage{Enabled: false},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	projectSet := projectgorm.ProjectSet{ID: 21, WorkspaceID: 7, TeamID: 8, Name: "Project", Code: "PX", Status: "active"}
	task := taskgorm.WorkTask{ID: 22, WorkspaceID: 7, TeamID: 8, ProjectSetID: projectSet.ID,
		Kind: "task", TaskKey: "PX-1", Title: "Task", Status: "todo", Priority: "normal", MutationState: "active"}
	attachment := taskgorm.WorkTaskAttachment{ID: 23, WorkspaceID: 7, WorkTaskID: task.ID, UploaderUserID: 3,
		Filename: "report.txt", StorageBackend: "s3", ObjectKey: "workspace/7/project-report.txt", MutationState: "active"}
	for _, row := range []any{&projectSet, &task, &attachment} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage), cfg)
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(dependencies.Catalog,
		cascadeSafetyAuthorizer{tool: toolruntimepmtools.ProjectSetDelete}, dependencies.Handlers,
		toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5,
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: toolruntimepmtools.ProjectSetDelete, Input: json.RawMessage(`{"id":21}`),
		IdempotencyKey: "project-delete-attached-0001", Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	failed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || failed.Status != toolruntime.ChangeSetFailed || failed.ErrorCode != "TOOL_EXECUTION_FAILED" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	for name, target := range map[string]any{
		"project": &projectgorm.ProjectSet{}, "task": &taskgorm.WorkTask{}, "attachment": &taskgorm.WorkTaskAttachment{},
	} {
		var count int64
		if err := db.Model(target).Where("workspace_id = ?", 7).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s mutated after terminal conflict: count=%d err=%v", name, count, err)
		}
	}
	var sagas int64
	if err := db.Model(&projectgorm.ProjectDeletionSaga{}).Where("workspace_id = ?", 7).Count(&sagas).Error; err != nil || sagas != 0 {
		t.Fatalf("project deletion saga started despite preflight conflict: count=%d err=%v", sagas, err)
	}
}

func TestProductionBIScreenToolInvokesSharedDomainService(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: "file:api_bi_tool?mode=memory&cache=shared",
		AuthMode:    "local", LocalJWTSecret: "api-bi-tool-test-secret-that-is-at-least-thirty-two-characters",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&identitygorm.Workspace{ID: 7, Name: "研发", Slug: "bi-tool", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage))
	if err != nil {
		t.Fatal(err)
	}
	if dependencies.BIScreen == nil {
		t.Fatal("production BI screen service is not shared with HTTP")
	}
	application, err := toolruntime.NewApplication(dependencies.Catalog, biToolTestAuthorizer{}, dependencies.Handlers)
	if err != nil {
		t.Fatal(err)
	}
	result, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: toolruntimepmtools.BIScreenGet, Input: json.RawMessage(`{"period":"week"}`),
		Authorization: toolruntime.ExecutionAuthorization{
			PrincipalID: 9, WorkspaceID: 7, GrantID: 8, ClientID: "bi-production-test",
			Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != toolruntimepmtools.BIScreenGet || len(result.Output) == 0 ||
		!json.Valid(result.Output) {
		t.Fatalf("result=%+v", result)
	}
	var output struct {
		WorkspaceID   uint   `json:"workspace_id"`
		WorkspaceName string `json:"workspace_name"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil || output.WorkspaceID != 7 || output.WorkspaceName != "研发" {
		t.Fatalf("output=%+v err=%v", output, err)
	}
}

func TestNewHTTPServerFailsVisibleWhenActiveToolHandlerIsMissing(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_tool_handler_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-tool-handler-test-secret-that-is-at-least-thirty-two-characters",
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	dependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage))
	if err != nil {
		t.Fatal(err)
	}
	delete(dependencies.Handlers, toolruntimepmtools.ReadHandlerNames()[0])
	if _, err := newHTTPServerWithToolRuntime(cfg, db, dependencies); err == nil {
		t.Fatal("expected active Tool without handler to fail startup")
	}
}

func TestNewHTTPServerRegistersConfiguredFeishuTenants(t *testing.T) {
	cfg := config.Config{
		Environment: "test", DatabaseDriver: "sqlite",
		DatabaseDSN: fmt.Sprintf("file:api_feishu_tenant_%s?mode=memory&cache=shared", t.Name()),
		AuthMode:    "local", LocalJWTSecret: "api-feishu-tenant-test-secret-that-is-at-least-thirty-two-characters",
		FeishuLogin: config.FeishuLogin{
			AppID: "app", AppSecret: "secret", BaseURL: "https://feishu.test",
			RedirectURL: "https://pm.test/api/v1/core/auth/feishu/callback", TenantKeys: []string{"tenant_pm"},
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := newHTTPServer(cfg, db); err != nil {
		t.Fatal(err)
	}
	var tenant identitygorm.FeishuTenant
	if err := db.Where("tenant_key = ?", "tenant_pm").First(&tenant).Error; err != nil || !tenant.Enabled {
		t.Fatalf("registered tenant=%+v err=%v", tenant, err)
	}
}
