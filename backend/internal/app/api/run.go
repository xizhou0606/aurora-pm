// Package api owns pm-api process assembly. It deliberately contains no HTTP
// endpoint or PM business behavior.
package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/app/documentread"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appworker "github.com/xizhou0606/aurora-pm/backend/internal/app/worker"
	accesscatalog "github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/airuntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishuim"
	figmaintegration "github.com/xizhou0606/aurora-pm/backend/internal/integration/figma"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/googlelogin"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/telegramlogin"
	ai "github.com/xizhou0606/aurora-pm/backend/internal/module/ai"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	designreview "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview"
	designreviewgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview/gorm"
	designreviewhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/designreview/http"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	identityhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/http"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	report "github.com/xizhou0606/aurora-pm/backend/internal/module/report"
	reporthttp "github.com/xizhou0606/aurora-pm/backend/internal/module/report/http"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workbenchhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/http"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"github.com/xizhou0606/aurora-pm/backend/internal/transport/http"
	designreviewworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/designreview"
	"github.com/xizhou0606/aurora-pm/backend/internal/worker/outbox"
	reportworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/report"
	workflowdelivery "github.com/xizhou0606/aurora-pm/backend/internal/worker/workflowdelivery"

	scmapp "github.com/xizhou0606/aurora-pm/backend/internal/app/scm"
	codecontribworker "github.com/xizhou0606/aurora-pm/backend/internal/worker/codecontrib"
)

// Run loads runtime dependencies and starts the PM HTTP API. It retains the
// existing embedded worker option for local development while keeping that
// process assembly out of cmd/pm-api.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}
	if err := database.PrepareSchema(db, cfg.SchemaMigrationMode); err != nil {
		return fmt.Errorf("schema preparation error: %w", err)
	}
	if err := resolveLocalJWTSecret(context.Background(), &cfg, db); err != nil {
		return fmt.Errorf("PM local session signing key error: %w", err)
	}
	if err := resolveFeishuLoginApplication(context.Background(), &cfg, db); err != nil {
		return fmt.Errorf("Feishu login configuration error: %w", err)
	}
	if err := cfg.ValidateAPIRuntime(); err != nil {
		return fmt.Errorf("API runtime configuration error: %w", err)
	}
	if cfg.Feishu.Enabled {
		log.Printf("Feishu document reader uses application %s", cfg.Feishu.AppID)
	}
	if err := database.Bootstrap(db, cfg.Bootstrap); err != nil {
		return fmt.Errorf("bootstrap error: %w", err)
	}
	engine, err := newHTTPServer(cfg, db)
	if err != nil {
		return err
	}
	if cfg.AI.Enabled && cfg.AI.EmbeddedWorker {
		var reader feishu.Reader
		if cfg.Feishu.Enabled {
			reader = feishu.New(cfg.Feishu)
		}
		go appworker.NewAIWorker(db, cfg, reader).Run(context.Background(), 0, cfg.AI.WorkerConcurrency)
		go designreviewworker.New(db, cfg).Run(context.Background(), 0)
		// The embedded local-development worker also carries the scheduled
		// reporting cadence so laptops exercise the same behavior as the
		// standalone pm-worker in production.
		reportScheduler, err := appworker.AssembleReportScheduler(db, cfg)
		if err != nil {
			return fmt.Errorf("report scheduler bootstrap error: %w", err)
		}
		go reportworker.New(reportScheduler).Run(context.Background())
		// Code contribution collection rides the embedded worker too; it is
		// a no-op until a workspace binds repositories. 触发判定在 sync
		// service 内：接入全量 + 每日 09:00 定时自然窗（same wiring as the
		// standalone pm-worker）.
		scmRuntime, err := scmapp.Assemble(db)
		if err != nil {
			return fmt.Errorf("codecontrib bootstrap error: %w", err)
		}
		go codecontribworker.New(scmRuntime.Sync).Run(context.Background())
		log.Printf("embedded AI worker enabled for local development")
	}
	if cfg.Notification.Enabled {
		sender, err := feishuim.New(cfg.FeishuLogin, cfg.Notification.RequestTimeout)
		if err != nil {
			return fmt.Errorf("notification publisher error: %w", err)
		}
		translator, err := feishuim.NewOpenIDTranslator(cfg.FeishuLogin, cfg.Notification.RequestTimeout)
		if err != nil {
			return fmt.Errorf("notification publisher error: %w", err)
		}
		var openIDTranslator outbox.OpenIDTranslator
		if translator != nil {
			openIDTranslator = translator
		}
		publisher, err := outbox.NewPublisher(
			cfg.Notification, db, identitygorm.NewLoginRepository(db), sender, openIDTranslator,
		)
		if err != nil {
			return fmt.Errorf("notification publisher error: %w", err)
		}
		go publisher.Run(context.Background())
		if translator != nil {
			log.Printf("notification open_id rewrite enabled from previous Feishu application")
		}
		log.Printf("notification outbox publisher enabled (batch=%d, poll=%s)",
			cfg.Notification.BatchSize, cfg.Notification.PollInterval)
	}
	// The delivery reconcile consumer is core workflow behavior (阶段 D)：
	// it re-evaluates requirement gates when evidence or scope changes.
	go func() {
		consumer, err := workflowdelivery.NewConsumer(db)
		if err != nil {
			log.Printf("delivery reconcile consumer bootstrap error: %v", err)
			return
		}
		consumer.Run(context.Background())
	}()
	log.Printf("delivery reconcile consumer enabled")
	log.Printf("pm-api listening on %s (auth=%s, database=%s, schema_migration=%s)",
		cfg.HTTPAddr, cfg.AuthMode, cfg.DatabaseDriver, cfg.SchemaMigrationMode)
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newHTTPServer(cfg config.Config, db *gorm.DB) (*gin.Engine, error) {
	toolDependencies, err := newPMToolRuntimeDependencies(db, objectstore.NewRemovalAdapter(cfg.ObjectStorage), cfg)
	if err != nil {
		return nil, err
	}
	return newHTTPServerWithToolRuntime(cfg, db, toolDependencies)
}

func newHTTPServerWithToolRuntime(
	cfg config.Config,
	db *gorm.DB,
	toolDependencies ToolRuntimeDependencies,
) (*gin.Engine, error) {
	if toolDependencies.Catalog == nil {
		return nil, fmt.Errorf("tool catalog bootstrap error: catalog is required")
	}
	login := identity.NewLoginService(identitygorm.NewLoginRepository(db), identity.LoginOptions{})
	if err := login.RegisterFeishuTenants(context.Background(), cfg.FeishuLogin.TenantKeys); err != nil {
		return nil, fmt.Errorf("Feishu tenant registration bootstrap error: %w", err)
	}

	accessRepository := identitygorm.NewAccessRepository(db)
	accessRepository.SetPasswordResetNotificationWriter(func(
		tx *gorm.DB,
		resetID uint,
		input identity.PasswordResetRequest,
		target identitygorm.User,
		actor identitygorm.User,
	) error {
		displayName := func(user identitygorm.User) string {
			if value := strings.TrimSpace(user.DisplayName); value != "" {
				return value
			}
			return strings.TrimSpace(user.Username)
		}
		vars := map[string]string{
			"user": displayName(target), "operator": displayName(actor), "url": input.Link,
			"expiresAt": input.ExpiresAt.UTC().Format(time.RFC3339),
		}
		return notification.Enqueue(tx, "identity_password_reset", resetID, notification.Intent{
			WorkspaceID: input.WorkspaceID, Scene: notification.SceneIdentityPasswordResetRequested,
			ActorUserID: input.ActorUserID, RecipientUserIDs: []uint{input.TargetUserID}, Vars: vars,
			FallbackPlainText: notification.PlainText(notification.SceneIdentityPasswordResetRequested, vars),
		})
	})
	access := identity.NewAccessService(accessRepository, nil)
	documentRoutes, documentService, err := assembleDocumentRoutes(db, access, cfg.ObjectStorage, cfg.Collaboration.Enabled, cfg.Notification)
	if err != nil {
		return nil, fmt.Errorf("document module bootstrap error: %w", err)
	}
	workspaceMemberships, err := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(db))
	if err != nil {
		return nil, fmt.Errorf("workspace membership bootstrap error: %w", err)
	}
	resourceAccess, err := assembleResourceAccess(db, access, workspaceMemberships, documentService, cfg.Notification)
	if err != nil {
		return nil, fmt.Errorf("document resource access bootstrap error: %w", err)
	}
	if err := documentService.InitializePersonalDirectories(context.Background()); err != nil {
		return nil, err
	}
	bitableRoutes, err := assembleBitableRoutes(db, access, resourceAccess, cfg.ObjectStorage, documentService)
	if err != nil {
		return nil, fmt.Errorf("bitable module bootstrap error: %w", err)
	}
	collaborationRoutes, err := assembleCollaborationRoutes(db, resourceAccess, documentService, cfg.Collaboration)
	if err != nil {
		return nil, fmt.Errorf("collaboration module bootstrap error: %w", err)
	}
	personnel := identity.NewPersonnelService(accessRepository)
	workspaceAdmin := identity.NewWorkspaceAdminService(accessRepository)
	menus := workbench.NewMenuService(workbenchgorm.NewAccessMenuRepository(db), access)
	access.SetMenuGrantValidator(menus)

	var feishuOrgSync *identity.FeishuOrgSyncService
	if cfg.Feishu.Enabled {
		if service, err := identity.NewFeishuOrgSyncService(
			feishuOrgSourceAdapter{reader: feishu.New(cfg.Feishu)},
			identitygorm.NewFeishuOrgStoreRepository(db),
			true,
		); err == nil {
			feishuOrgSync = service
		}
	}
	inventory := httpapi.NewRouteInventory()
	accessHandler := identityhttp.NewAccessHandler(access, inventory)
	accessHandler.SetPersonnelService(personnel)
	accessHandler.SetWorkspaceAdminService(workspaceAdmin)
	if feishuOrgSync != nil {
		accessHandler.SetFeishuOrg(feishuOrgSync)
	}
	menuHandler := workbenchhttp.NewMenuHandler(menus)
	loginRepository := identitygorm.NewLoginRepository(db)
	if feishuOrgSync != nil {
		// 新同事首次飞书登录建号后立刻触发一次组织同步，让他马上进入部门
		// 树；同步已在执行中时静默跳过（随后台/6h 刷新兜底）。
		loginRepository.SetNewUserHook(func() {
			go func() {
				hookCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if _, err := feishuOrgSync.SyncOnce(hookCtx); err != nil && !errors.Is(err, identity.ErrFeishuOrgSyncInProgress) {
					log.Printf("[feishu-org] post-registration sync failed: %v", err)
				}
			}()
		})
	}
	accountService := identity.NewAccountService(loginRepository, login, cfg.ExternalRegistrationEnabled)
	accountHandler := identityhttp.NewAccountHandler(accountService)
	passwordResetService := identity.NewPasswordResetService(accessRepository, identity.PasswordResetOptions{
		PublicBaseURL: cfg.PublicBaseURL,
	})
	accountHandler.SetPasswordResetService(passwordResetService)
	accessHandler.SetPasswordResetService(passwordResetService)
	baselineDirectoryService, err := identity.NewBaselineDirectoryService(
		identitygorm.NewBaselineDirectoryRepository(db),
	)
	if err != nil {
		return nil, fmt.Errorf("identity baseline directory bootstrap error: %w", err)
	}
	baselineDirectoryHandler := identityhttp.NewBaselineDirectoryHandler(baselineDirectoryService)
	var googleLoginHandler *identityhttp.GoogleLoginHandler
	var googleProvider identity.GoogleLoginProvider
	if cfg.GoogleLogin.Enabled {
		provider, err := googlelogin.New(context.Background(), cfg.GoogleLogin)
		if err != nil {
			return nil, fmt.Errorf("Google login client error: %w", err)
		}
		googleProvider = provider
		googleLoginHandler = identityhttp.NewGoogleLoginHandler(identity.NewGoogleLoginService(login, provider).WithAccountService(accountService)).WithAccountService(accountService)
	}
	var telegramLoginHandler *identityhttp.TelegramLoginHandler
	var telegramProvider identity.TelegramLoginProvider
	if cfg.TelegramLogin.Enabled {
		provider, err := telegramlogin.New(context.Background(), cfg.TelegramLogin)
		if err != nil {
			return nil, fmt.Errorf("Telegram login client error: %w", err)
		}
		telegramProvider = provider
		telegramLoginHandler = identityhttp.NewTelegramLoginHandler(identity.NewTelegramLoginService(login, provider).WithAccountService(accountService)).WithAccountService(accountService)
	}
	accountService.WithProviders(googleProvider, telegramProvider)
	projectAccess, err := newProjectAccessService(db)
	if err != nil {
		return nil, fmt.Errorf("project member access bootstrap error: %w", err)
	}
	designReviewStore, designReviewStoreErr := objectstore.New(cfg.ObjectStorage)
	designReviewService, err := designreview.NewService(
		designreviewgorm.NewRepository(db),
		projectAccess,
		access,
		designReviewStore,
		figmaintegration.New(cfg.DesignReview.FigmaBaseURL, cfg.DesignReview.FigmaToken, cfg.DesignReview.RequestTimeout),
		airuntime.NewDesignReviewClient(cfg.AI.RuntimeBaseURL, cfg.AI.InternalSecret, cfg.AI.RequestTimeout),
		designreview.Availability{
			ObjectStorage:   cfg.ObjectStorage.Enabled && designReviewStoreErr == nil,
			VisualModel:     cfg.AI.Enabled && strings.TrimSpace(cfg.AI.DesignReviewModel) != "",
			Figma:           strings.TrimSpace(cfg.DesignReview.FigmaToken) != "",
			Model:           cfg.AI.DesignReviewModel,
			ReasoningEffort: cfg.AI.DesignReviewReasoningEffort,
			ImageDetail:     cfg.AI.DesignReviewImageDetail,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("design review bootstrap error: %w", err)
	}
	designReviewHandler := designreviewhttp.NewHandler(designReviewService)
	toolRoutes, err := assembleToolRuntime(db, toolDependencies)
	requirementResourceRoutes, resourceLinkErr := assembleRequirementResourceLinks(db, access, workspaceMemberships, projectAccess, documentService)
	if resourceLinkErr != nil {
		return nil, resourceLinkErr
	}
	if err != nil {
		return nil, err
	}
	taskCreatorPolicy, err := task.NewCreatorPolicyService(taskgorm.NewCreatorPolicyRepository(db))
	if err != nil {
		return nil, fmt.Errorf("task creator policy bootstrap error: %w", err)
	}
	reportDirectory, err := identity.NewDirectoryService(identitygorm.NewReportingDirectoryRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report directory bootstrap error: %w", err)
	}
	var reportExporter report.DocumentExporter
	if cfg.Feishu.Enabled {
		feishuWriter := feishu.New(cfg.Feishu)
		reportExporter = report.DocumentExporterFunc(func(ctx context.Context, title, content string) (report.CreatedDocument, error) {
			document, err := feishuWriter.CreateTextDocument(ctx, title, content)
			if err != nil {
				return report.CreatedDocument{}, err
			}
			return report.CreatedDocument{DocumentID: document.DocumentID, URL: document.URL}, nil
		})
	}
	if feishuOrgSync != nil {
		// The Feishu contact org sync feeds the reporting directory's
		// department tree. It is best-effort display data: failures log and
		// the directory keeps its 用户组 grouping.
		reportDirectory = reportDirectory.WithFeishuOrg(feishuOrgSync)
		go func() {
			warmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := feishuOrgSync.SyncOnce(warmCtx); err != nil {
				log.Printf("[feishu-org] startup sync failed: %v", err)
			}
		}()
	}
	reportTasks, err := task.NewBIReadService(taskgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report task facts bootstrap error: %w", err)
	}
	reportBugs, err := quality.NewBIReadService(qualitygorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report bug facts bootstrap error: %w", err)
	}
	reportRequirements, err := requirement.NewBIReadService(requirementgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, fmt.Errorf("report requirement facts bootstrap error: %w", err)
	}
	reportRunStarts, err := ai.NewReportRunStartService(aigorm.NewReportRunRepository(db, cfg.AI.Enabled, cfg.AI.Model), cfg.AI.Enabled)
	if err != nil {
		return nil, fmt.Errorf("AI report run bootstrap error: %w", err)
	}
	reportRunReads, err := ai.NewReportRunReadService(aigorm.NewReportRunRepository(db, cfg.AI.Enabled, cfg.AI.Model))
	if err != nil {
		return nil, fmt.Errorf("AI report run read bootstrap error: %w", err)
	}
	scmRuntime, err := scmapp.Assemble(db)
	if err != nil {
		return nil, fmt.Errorf("codecontrib bootstrap error: %w", err)
	}

	reportService, err := report.NewService(report.Dependencies{
		Directory:     reportDirectory,
		Tasks:         reportTasks,
		Bugs:          reportBugs,
		Requirements:  reportRequirements,
		Runs:          reportRunStarts,
		RunResults:    reportRunReads,
		Exporter:      reportExporter,
		ExportRecords: aigorm.NewReportExportRepository(db),
		Contributions: scmRuntime.ReportReads,
	})
	if err != nil {
		return nil, fmt.Errorf("report service bootstrap error: %w", err)
	}
	reportHandler, err := reporthttp.NewHandler(reportService)
	if err != nil {
		return nil, fmt.Errorf("report handler bootstrap error: %w", err)
	}
	mountExternalLoginRoutes := func(group *gin.RouterGroup) {
		accountHandler.RegisterPublicRoutes(group)
		if googleLoginHandler != nil || telegramLoginHandler != nil {
			if googleLoginHandler != nil {
				googleLoginHandler.RegisterRoutes(group)
			}
			if telegramLoginHandler != nil {
				telegramLoginHandler.RegisterRoutes(group)
			}
		}
	}
	engine := httpapi.NewWithDependencies(cfg, db, httpapi.Dependencies{
		RequirementResourceLinks: func(tx *gorm.DB) *requirement.ResourceLinkService {
			return newRequirementResourceLinkService(tx, access, workspaceMemberships, projectAccess, documentService)
		},
		DocumentSources:      func(tx *gorm.DB) (*docsource.Service, error) { return documentread.NewSources(tx, nil) },
		Authorizer:           access,
		WorkspaceMemberships: workspaceMemberships,
		TaskCreatorPolicy:    taskCreatorPolicy,
		MountAccessRoutes: func(group *gin.RouterGroup) {
			accessHandler.RegisterRoutes(group)
		},
		MountMenuRoutes: func(group *gin.RouterGroup) {
			menuHandler.RegisterRoutes(group)
		},
		MountLoginRoutes:                 mountExternalLoginRoutes,
		MountAccountRoutes:               accountHandler.RegisterAuthenticatedRoutes,
		MountBaselineDirectoryRoutes:     baselineDirectoryHandler.RegisterAuthenticatedRoutes,
		MountWorkspaceDirectoryRoutes:    baselineDirectoryHandler.RegisterWorkspaceRoutes,
		MountProjectAccessGuideRoutes:    buildProjectAccessGuideRoutes(db, cfg.Notification),
		MountDocumentRoutes:              documentRoutes,
		MountBitableRoutes:               bitableRoutes,
		MountResourceAccessRoutes:        func(group *gin.RouterGroup) { resourceAccess.MountRoutes(group); requirementResourceRoutes(group) },
		MountCollaborationRoutes:         collaborationRoutes.MountRoutes,
		MountInternalCollaborationRoutes: collaborationRoutes.MountInternalRoutes,
		MountToolManagementRoutes:        toolRoutes.mountManagement,
		MountReportRoutes:                reportHandler.RegisterRoutes,
		MountCodeContribRoutes:           scmRuntime.Handler.RegisterRoutes,
		MountExternalToolRoutes:          toolRoutes.mountExternal,
		MountExternalToolDiscoveryRoutes: toolRoutes.mountDiscovery,
		MountDesignReviewRoutes:          designReviewHandler.RegisterRoutes,
		MountDesignReviewCollectorRoutes: designReviewHandler.RegisterCollectorRoutes,
		ProjectAccess:                    projectAccess,
		MemberFacts:                      newPlanUnitMemberRepository(db, cfg.Notification),
		ResponsibilityInspector:          projectgorm.NewRemovalImpactRepository(db),
		BIScreen:                         toolDependencies.BIScreen,
	})
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	systemMenus, err := menus.Bootstrap(context.Background(), accessAdministrationMenus())
	if err != nil {
		return nil, fmt.Errorf("RBAC menu bootstrap error: %w", err)
	}
	if len(systemMenus) != 1 {
		return nil, fmt.Errorf("RBAC menu bootstrap error: access-management catalog page is missing")
	}
	serviceInterfaces := accessAdministrationInterfaces()
	if err := access.Bootstrap(context.Background(), identity.BootstrapSpec{
		AdministratorUserID: cfg.RBACBootstrapUserID,
		ServiceInterfaces:   serviceInterfaces,
		DefaultGroupActionCodes: documentDefaultGroupActionCodes(
			serviceInterfaces,
		),
		DefaultGroupExcludedActionCodes: []string{
			string(accesscatalog.ActionExternalToolingGrantsView),
			string(accesscatalog.ActionExternalToolingGrantsRevoke),
			string(accesscatalog.ActionDocumentsScopeAll),
			string(accesscatalog.ActionDocumentsNodesCreate),
			string(accesscatalog.ActionDocumentsNodesCopy),
			string(accesscatalog.ActionDocumentsNodesUpdate),
			string(accesscatalog.ActionDocumentsNodesMove),
			string(accesscatalog.ActionDocumentsNodesArchive),
			string(accesscatalog.ActionDocumentsBindingsUpdate),
			string(accesscatalog.ActionDocumentsShortcutsCreate),
			string(accesscatalog.ActionDocumentsOwnershipTransfer),
		},
	}); err != nil {
		return nil, fmt.Errorf("RBAC bootstrap error: %w", err)
	}
	if err := access.EnsureAdministratorMenus(context.Background(), []uint{systemMenus[0].ID}); err != nil {
		return nil, fmt.Errorf("RBAC administrator menu grant error: %w", err)
	}
	return engine, nil
}

func resolveToolManagementPrincipal(c *gin.Context) (toolruntime.ManagementPrincipal, error) {
	principalValue, principalExists := c.Get(pmauth.PrincipalContextKey)
	principal, principalOK := principalValue.(pmauth.Principal)
	workspaceValue, workspaceExists := c.Get(pmauth.WorkspaceContextKey)
	workspace, workspaceOK := workspaceValue.(identitygorm.Workspace)
	if !principalExists || !principalOK || principal.UserID == 0 ||
		!workspaceExists || !workspaceOK || workspace.ID == 0 || !workspace.Enabled {
		return toolruntime.ManagementPrincipal{}, toolruntime.ErrAuthorizationRequestInvalid
	}
	// PM tool-runtime grants, memberships, and RBAC all key on the PM-internal
	// users.id (external_authorization_grants.PrincipalID,
	// workspace_members.user_id). PM project data keys on the fact ID instead;
	// the tool handlers that consume project data translate internally via
	// projectgorm fact-ID-aware queries. This resolver must keep returning the
	// internal ID so grant validation and membership checks keep matching.
	return toolruntime.ManagementPrincipal{
		PrincipalID:   principal.UserID,
		WorkspaceID:   workspace.ID,
		WorkspaceName: workspace.Name,
	}, nil
}

func accessAdministrationInterfaces() []identity.ServiceInterfaceInput {
	records := accesscatalog.BootstrapActionRoutes()
	result := make([]identity.ServiceInterfaceInput, 0, len(records))
	for _, record := range records {
		result = append(result, identity.ServiceInterfaceInput{
			Code: record.InterfaceCode, ActionCode: string(record.ActionCode),
			HTTPMethod: record.HTTPMethod, RouteTemplate: record.RouteTemplate,
			Name: record.ActionName, Description: record.CopyKey, Enabled: true,
		})
	}
	return result
}

// documentDefaultGroupActionCodes is the binding participant action ceiling.
// It filters against the current canonical route inventory so later V1 asset
// routes become grants on the next idempotent bootstrap without inventing
// service-interface rows before those routes exist.
func documentDefaultGroupActionCodes(interfaces []identity.ServiceInterfaceInput) []string {
	available := make(map[string]struct{}, len(interfaces))
	for _, value := range interfaces {
		available[value.ActionCode] = struct{}{}
	}
	desired := []string{
		string(accesscatalog.ActionDocumentsNodesView),
		string(accesscatalog.ActionDocumentsContentView),
		string(accesscatalog.ActionDocumentsContentUpdate),
		string(accesscatalog.ActionDocumentsAssetsView),
		string(accesscatalog.ActionDocumentsAssetsUpload),
		string(accesscatalog.ActionDocumentsAssetsDownload),
		string(accesscatalog.ActionDocumentsAssetsDelete),
		string(accesscatalog.ActionDocumentsCommentsView),
		string(accesscatalog.ActionDocumentsCommentsCreate),
		string(accesscatalog.ActionDocumentsCommentsReply),
		string(accesscatalog.ActionDocumentsCommentsEditOwn),
		string(accesscatalog.ActionDocumentsCommentsDeleteOwn),
		string(accesscatalog.ActionDocumentsCommentsResolve),
		string(accesscatalog.ActionDocumentsCommentsReopen),
		string(accesscatalog.ActionDocumentsPreferencesUpdate),
	}
	result := make([]string, 0, len(desired))
	for _, code := range desired {
		if _, ok := available[code]; ok {
			result = append(result, code)
		}
	}
	return result
}

func accessAdministrationMenus() []workbench.MenuInput {
	for _, page := range accesscatalog.Tree() {
		for _, module := range page.Modules {
			for _, action := range module.Actions {
				if action.Code == accesscatalog.ActionAccessManagementPersonnelView && len(page.FrontendRoutes) > 0 {
					return []workbench.MenuInput{{
						Code: "access-administration", Name: page.Name, MenuType: workbench.MenuTypePage,
						Path: page.FrontendRoutes[0], Icon: "shield-check", HostKey: "pm", SortOrder: 0, Enabled: true,
					}}
				}
			}
		}
	}
	return nil
}

// feishuOrgSourceAdapter adapts the Feishu document reader's contact methods
// to the identity-owned org sync port.
type feishuOrgSourceAdapter struct{ reader *feishu.SDKReader }

func (adapter feishuOrgSourceAdapter) ListFeishuDepartments(
	ctx context.Context,
) ([]identity.FeishuDepartment, error) {
	departments, err := adapter.reader.ListFeishuDepartments(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]identity.FeishuDepartment, 0, len(departments))
	for _, department := range departments {
		result = append(result, identity.FeishuDepartment{
			OpenID: department.OpenID, ParentID: department.ParentID,
			Name: department.Name, Order: department.Order, MemberCount: department.MemberCount,
		})
	}
	return result, nil
}

func (adapter feishuOrgSourceAdapter) ListFeishuDepartmentUsers(
	ctx context.Context,
	departmentID string,
) ([]identity.FeishuDepartmentUser, error) {
	users, err := adapter.reader.ListFeishuDepartmentUsers(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	result := make([]identity.FeishuDepartmentUser, 0, len(users))
	for _, user := range users {
		result = append(result, identity.FeishuDepartmentUser{
			OpenID: user.OpenID, Name: user.Name, DepartmentID: user.DepartmentID,
			JobTitle: user.JobTitle,
		})
	}
	return result, nil
}
