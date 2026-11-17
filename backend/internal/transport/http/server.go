package httpapi

import (
	"context"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishulogin"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/objectstore"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/saassso"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	quality "github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	qualityhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/http"
	releasenotes "github.com/xizhou0606/aurora-pm/backend/internal/module/releasenotes"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	requirementhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/http"
	task "github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	taskhttp "github.com/xizhou0606/aurora-pm/backend/internal/module/task/http"
	workbench "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench"
	biscreen "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/biscreen"
	workbenchgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workbench/gorm"
	workflow "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	qatestcase "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/testcase"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type Server struct {
	requirementResourceLinks         func(*gorm.DB) *requirement.ResourceLinkService
	documentSources                  func(*gorm.DB) (*docsource.Service, error)
	cfg                              config.Config
	db                               *gorm.DB
	tokens                           *pmauth.Tokens
	authCacheMu                      sync.Mutex
	authCache                        map[string]cachedIdentity
	materialStore                    materialObjectStore
	materialStoreErr                 error
	storageReadyMu                   sync.Mutex
	storageReadyAt                   time.Time
	storageReadyErr                  error
	testCaseDelivery                 *qatestcase.Materializer
	qualitySceneOptions              *qualityhttp.SceneOptionHandler
	taskClientOptions                *taskhttp.ClientOptionHandler
	materials                        *requirement.MaterialService
	reports                          *quality.TestReportService
	releaseNotes                     *releasenotes.Service
	requirementMaterials             *requirementhttp.MaterialHandler
	loginService                     *identity.LoginService
	feishuLogin                      *feishulogin.Client
	ssoLogin                         *saassso.Client
	loginInitErr                     error
	authorizer                       identity.Authorizer
	workspaceMemberships             *identity.WorkspaceMembershipService
	mountAccessRoutes                func(*gin.RouterGroup)
	mountMenuRoutes                  func(*gin.RouterGroup)
	mountLoginRoutes                 func(*gin.RouterGroup)
	mountAccountRoutes               func(*gin.RouterGroup)
	mountBaselineDirectoryRoutes     func(*gin.RouterGroup)
	mountWorkspaceDirectoryRoutes    func(*gin.RouterGroup)
	mountProjectAccessGuideRoutes    func(*gin.RouterGroup)
	mountDocumentRoutes              func(*gin.RouterGroup)
	mountBitableRoutes               func(*gin.RouterGroup)
	mountResourceAccessRoutes        func(*gin.RouterGroup)
	mountCollaborationRoutes         func(*gin.RouterGroup)
	mountInternalCollaborationRoutes func(*gin.RouterGroup)
	mountToolManagementRoutes        func(*gin.RouterGroup)
	mountReportRoutes                func(*gin.RouterGroup)
	mountCodeContribRoutes           func(*gin.RouterGroup)
	mountExternalToolRoutes          func(*gin.RouterGroup)
	mountExternalToolDiscoveryRoutes func(gin.IRoutes)
	mountDesignReviewRoutes          func(*gin.RouterGroup)
	mountDesignReviewCollectorRoutes func(*gin.RouterGroup)
	biScreen                         *workbench.BIScreenService
	taskCreatorPolicy                *task.CreatorPolicyService
	// projectAccess is the gate for project-level data access. nil disables
	// enforcement (prepare mode).
	projectAccess project.AccessChecker
	// memberFacts reads and writes plan unit owner + participant state.
	memberFacts project.PlanUnitMemberFactsRepository
	// responsibilityInspector checks outstanding responsibilities before
	// plan unit participant removal.
	responsibilityInspector project.PlanUnitResponsibilityInspector
}

// Dependencies are assembled by app/api. HTTP transport receives only the
// domain-facing contracts and route registration callbacks; it never creates
// an RBAC repository or reads RBAC GORM records itself.
type Dependencies struct {
	RequirementResourceLinks         func(*gorm.DB) *requirement.ResourceLinkService
	DocumentSources                  func(*gorm.DB) (*docsource.Service, error)
	Authorizer                       identity.Authorizer
	WorkspaceMemberships             *identity.WorkspaceMembershipService
	MountAccessRoutes                func(*gin.RouterGroup)
	MountMenuRoutes                  func(*gin.RouterGroup)
	MountLoginRoutes                 func(*gin.RouterGroup)
	MountAccountRoutes               func(*gin.RouterGroup)
	MountBaselineDirectoryRoutes     func(*gin.RouterGroup)
	MountWorkspaceDirectoryRoutes    func(*gin.RouterGroup)
	MountProjectAccessGuideRoutes    func(*gin.RouterGroup)
	MountDocumentRoutes              func(*gin.RouterGroup)
	MountBitableRoutes               func(*gin.RouterGroup)
	MountResourceAccessRoutes        func(*gin.RouterGroup)
	MountCollaborationRoutes         func(*gin.RouterGroup)
	MountInternalCollaborationRoutes func(*gin.RouterGroup)
	MountToolManagementRoutes        func(*gin.RouterGroup)
	MountReportRoutes                func(*gin.RouterGroup)
	MountCodeContribRoutes           func(*gin.RouterGroup)
	MountExternalToolRoutes          func(*gin.RouterGroup)
	MountExternalToolDiscoveryRoutes func(gin.IRoutes)
	MountDesignReviewRoutes          func(*gin.RouterGroup)
	MountDesignReviewCollectorRoutes func(*gin.RouterGroup)
	BIScreen                         *workbench.BIScreenService
	TaskCreatorPolicy                *task.CreatorPolicyService
	// ProjectAccess is the gate for project-level data access. When nil the
	// data-access gate is disabled (prepare mode or legacy).
	ProjectAccess project.AccessChecker
	// MemberFacts is the plan-unit owner + participant read/write store.
	MemberFacts project.PlanUnitMemberFactsRepository
	// ResponsibilityInspector checks outstanding responsibilities before
	// plan unit participant removal.
	ResponsibilityInspector project.PlanUnitResponsibilityInspector
}

func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	store, storeErr := objectstore.New(cfg.ObjectStorage)
	return newEngineWithDependencies(cfg, db, store, storeErr, Dependencies{})
}

// NewWithDependencies builds the HTTP transport with services constructed by
// the application composition root.
func NewWithDependencies(cfg config.Config, db *gorm.DB, dependencies Dependencies) *gin.Engine {
	store, storeErr := objectstore.New(cfg.ObjectStorage)
	return newEngineWithDependencies(cfg, db, store, storeErr, dependencies)
}

func newEngine(
	cfg config.Config,
	db *gorm.DB,
	store materialObjectStore,
	storeErr error,
) *gin.Engine {
	return newEngineWithDependencies(cfg, db, store, storeErr, Dependencies{})
}

func newEngineWithDependencies(
	cfg config.Config,
	db *gorm.DB,
	store materialObjectStore,
	storeErr error,
	dependencies Dependencies,
) *gin.Engine {
	workspaceMemberships := dependencies.WorkspaceMemberships
	if workspaceMemberships == nil {
		workspaceMemberships, _ = identity.NewWorkspaceMembershipService(
			identitygorm.NewWorkspaceMembershipRepository(db),
		)
	}
	s := &Server{
		documentSources:          dependencies.DocumentSources,
		requirementResourceLinks: dependencies.RequirementResourceLinks,
		cfg:                      cfg, db: db, tokens: pmauth.NewTokens(cfg), authCache: map[string]cachedIdentity{},
		materialStore: store, materialStoreErr: storeErr,
		testCaseDelivery: qatestcase.NewMaterializer(db, cfg.ObjectStorage, store, storeErr),
		loginService:     identity.NewLoginService(identitygorm.NewLoginRepository(db), identity.LoginOptions{}),
		qualitySceneOptions: qualityhttp.NewSceneOptionHandler(
			quality.NewSceneOptionService(qualitygorm.NewSceneOptionRepository(db)),
		),
		taskClientOptions: func() *taskhttp.ClientOptionHandler {
			service, _ := task.NewClientOptionService(taskgorm.NewClientOptionRepository(db))
			return taskhttp.NewClientOptionHandler(service)
		}(),
		materials:                        requirement.NewMaterialService(requirementgorm.NewMaterialRepository(db)),
		reports:                          quality.NewTestReportService(qualitygorm.NewTestReportRepository(db)),
		releaseNotes:                     newReleaseNotesService(db),
		authorizer:                       dependencies.Authorizer,
		workspaceMemberships:             workspaceMemberships,
		mountAccessRoutes:                dependencies.MountAccessRoutes,
		mountMenuRoutes:                  dependencies.MountMenuRoutes,
		mountLoginRoutes:                 dependencies.MountLoginRoutes,
		mountAccountRoutes:               dependencies.MountAccountRoutes,
		mountBaselineDirectoryRoutes:     dependencies.MountBaselineDirectoryRoutes,
		mountWorkspaceDirectoryRoutes:    dependencies.MountWorkspaceDirectoryRoutes,
		mountProjectAccessGuideRoutes:    dependencies.MountProjectAccessGuideRoutes,
		mountDocumentRoutes:              dependencies.MountDocumentRoutes,
		mountBitableRoutes:               dependencies.MountBitableRoutes,
		mountResourceAccessRoutes:        dependencies.MountResourceAccessRoutes,
		mountCollaborationRoutes:         dependencies.MountCollaborationRoutes,
		mountInternalCollaborationRoutes: dependencies.MountInternalCollaborationRoutes,
		mountToolManagementRoutes:        dependencies.MountToolManagementRoutes,
		mountReportRoutes:                dependencies.MountReportRoutes,
		mountCodeContribRoutes:           dependencies.MountCodeContribRoutes,
		mountExternalToolRoutes:          dependencies.MountExternalToolRoutes,
		mountExternalToolDiscoveryRoutes: dependencies.MountExternalToolDiscoveryRoutes,
		mountDesignReviewRoutes:          dependencies.MountDesignReviewRoutes,
		mountDesignReviewCollectorRoutes: dependencies.MountDesignReviewCollectorRoutes,
		biScreen:                         dependencies.BIScreen,
		taskCreatorPolicy:                dependencies.TaskCreatorPolicy,
		projectAccess:                    dependencies.ProjectAccess,
		memberFacts:                      dependencies.MemberFacts,
		responsibilityInspector:          dependencies.ResponsibilityInspector,
	}
	if s.biScreen == nil {
		s.biScreen, _ = newBIScreenService(db)
	}
	if !cfg.LocalPreview() {
		s.feishuLogin, s.loginInitErr = feishulogin.New(cfg.FeishuLogin)
	}
	if s.loginInitErr == nil && cfg.SSO.Enabled {
		s.ssoLogin, s.loginInitErr = saassso.New(cfg.SSO)
	}
	s.requirementMaterials = requirementhttp.NewMaterialHandler(s.materials)
	engine := gin.New()
	engine.Use(gin.Recovery(), s.cors)
	engine.GET("/health", s.health)
	engine.GET("/health/live", s.live)
	engine.GET("/health/ready", s.ready)
	engine.GET("/internal/health", s.health)
	if s.mountInternalCollaborationRoutes != nil {
		s.mountInternalCollaborationRoutes(engine.Group(""))
	}
	engine.POST("/internal/release-notes", s.createReleaseNote)
	engine.GET("/internal/release-notes/latest", s.latestReleaseNote)
	engine.GET("/release-notes", s.listReleaseNotes)
	engine.GET("/api/v1/public/runtime", s.runtime)
	if s.mountExternalToolDiscoveryRoutes != nil {
		s.mountExternalToolDiscoveryRoutes(engine)
	}
	if s.mountExternalToolRoutes != nil {
		s.mountExternalToolRoutes(engine.Group("/api/v1"))
	}
	if s.mountDesignReviewCollectorRoutes != nil {
		s.mountDesignReviewCollectorRoutes(engine.Group("/api/v1/platform/design-review-mobile-captures"))
	}
	s.mountCore(engine.Group("/api/v1/core"))
	s.mountCore(engine.Group("/core"))
	s.mountPlatform(engine.Group("/api/v1/platform"))
	s.mountPlatform(engine.Group("/platform"))
	return engine
}

func newBIScreenService(db *gorm.DB) (*workbench.BIScreenService, error) {
	tasks, err := task.NewBIReadService(taskgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	bugs, err := quality.NewBIReadService(qualitygorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	projects, err := project.NewBIReadService(projectgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	requirements, err := requirement.NewBIReadService(requirementgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	identities, err := identity.NewBIReadService(identitygorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	configuration, err := workbench.NewBIConfigurationReadService(workbenchgorm.NewBIConfigurationReadRepository(db))
	if err != nil {
		return nil, err
	}
	workflows, err := workflow.NewBIReadService(workflowgorm.NewBIReadRepository(db))
	if err != nil {
		return nil, err
	}
	backend, err := biscreen.NewBIScreenBackend(workbench.BIScreenReadDependencies{
		Tasks: tasks, Quality: bugs, Projects: projects, Requirements: requirements,
		Identities: identities, Configuration: configuration, Workflows: workflows,
	})
	if err != nil {
		return nil, err
	}
	return workbench.NewBIScreenService(backend)
}

func (s *Server) health(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		fail(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pm-api", "time": time.Now().UTC()})
}

type objectStorageReadiness interface {
	CheckReady(context.Context) error
}

func (s *Server) objectStorageReady(ctx context.Context) error {
	if !s.cfg.ObjectStorage.Enabled {
		return nil
	}
	if s.materialStore == nil || s.materialStoreErr != nil {
		return fmt.Errorf("object storage client is unavailable")
	}
	checker, ok := s.materialStore.(objectStorageReadiness)
	if !ok {
		return fmt.Errorf("object storage readiness check is unavailable")
	}

	s.storageReadyMu.Lock()
	defer s.storageReadyMu.Unlock()
	now := time.Now().UTC()
	cacheFor := 30 * time.Second
	if s.storageReadyErr != nil {
		cacheFor = 5 * time.Second
	}
	if !s.storageReadyAt.IsZero() && now.Sub(s.storageReadyAt) < cacheFor {
		return s.storageReadyErr
	}
	s.storageReadyErr = checker.CheckReady(ctx)
	s.storageReadyAt = now
	return s.storageReadyErr
}

func (s *Server) requireObjectStorageReady(c *gin.Context) bool {
	if err := s.objectStorageReady(c.Request.Context()); err != nil {
		failCode(c, http.StatusServiceUnavailable, apiErrorObjectStorageUnavailable, "object storage is unavailable")
		return false
	}
	return true
}

func (s *Server) ready(c *gin.Context) {
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		fail(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	if err := s.objectStorageReady(c.Request.Context()); err != nil {
		failCode(c, http.StatusServiceUnavailable, apiErrorObjectStorageUnavailable, "object storage unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pm-api", "time": time.Now().UTC()})
}

func (s *Server) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pm-api"})
}

func (s *Server) runtime(c *gin.Context) {
	localAllowed := s.cfg.AuthMode == "local" || s.cfg.AuthMode == "hybrid"
	ok(c, gin.H{
		"service":             "pm",
		"auth_mode":           s.cfg.AuthMode,
		"local_login_allowed": localAllowed,
		"login_methods":       loginMethods(s.cfg),
		"shared_jwt_enabled":  s.cfg.AuthMode != "local",
		"api_version":         "v1",
	})
}

func loginMethods(cfg config.Config) []string {
	methods := make([]string, 0, 5)
	if cfg.AuthMode == "local" || cfg.AuthMode == "hybrid" {
		methods = append(methods, "credentials")
	}
	if !cfg.LocalPreview() {
		methods = append(methods, "feishu")
	}
	if cfg.GoogleLogin.Enabled {
		methods = append(methods, "google")
	}
	if cfg.TelegramLogin.Enabled {
		methods = append(methods, "telegram")
	}
	if cfg.SSO.Enabled {
		methods = append(methods, "sso")
	}
	return methods
}

func (s *Server) mountCore(group *gin.RouterGroup) {
	group.GET("/public/runtime", s.runtime)
	group.GET("/public/modules", s.publicModules)
	group.POST("/auth/login", s.login)
	group.GET("/auth/feishu/start", s.feishuStart)
	group.GET("/auth/feishu/callback", s.feishuCallback)
	group.GET("/auth/sso/start", s.ssoStart)
	group.GET("/auth/sso/callback", s.ssoCallback)
	group.POST("/auth/session/exchange", s.exchangeSession)
	if s.mountLoginRoutes != nil {
		s.mountLoginRoutes(group)
	}

	authenticated := group.Group("")
	authenticated.Use(s.authenticate, s.enforceAccountAdmission, s.authorizeAccess)
	if s.mountAccountRoutes != nil {
		s.mountAccountRoutes(authenticated)
	}
	if s.mountBaselineDirectoryRoutes != nil {
		s.mountBaselineDirectoryRoutes(authenticated)
	}
	authenticated.GET("/auth/me", s.me)
	authenticated.POST("/auth/logout", s.logout)
	if s.mountAccessRoutes != nil {
		s.mountAccessRoutes(authenticated)
	}
	if s.mountMenuRoutes != nil {
		s.mountMenuRoutes(authenticated)
	}
	authenticated.GET("/users", s.listUsers)
	authenticated.POST("/users", s.createLocalUser)
	authenticated.PATCH("/users/:id/password", s.updateLocalPassword)
	// The assembled API mounts the identity module's workspace administrator.
	// Keep the legacy handlers only for minimal/test server compositions that do
	// not inject access routes, so baseline middleware routes remain exercisable
	// without registering duplicate Gin paths in production.
	if s.mountAccessRoutes == nil {
		workspaces := authenticated.Group("/workspaces")
		workspaces.Handle(http.MethodGet, "", s.listWorkspaces)
		workspaces.Handle(http.MethodPost, "", s.createWorkspace)
		workspaces.Handle(http.MethodGet, "/:id", s.getWorkspace)
		workspaces.Handle(http.MethodPatch, "/:id", s.patchWorkspace)
	}

	scoped := authenticated.Group("")
	scoped.Use(s.requireWorkspace)
	if s.mountWorkspaceDirectoryRoutes != nil {
		s.mountWorkspaceDirectoryRoutes(scoped)
	}
	scoped.GET("/teams", s.listTeams)
	scoped.POST("/teams", s.createTeam)
	scoped.GET("/teams/:id/members", s.listTeamMembers)
	scoped.GET("/projects", s.listCoreProjects)
}

func (s *Server) mountPlatform(group *gin.RouterGroup) {
	scoped := group.Group("")
	scoped.Use(s.authenticate, s.enforceAccountAdmission, s.authorizeAccess, s.requireWorkspace)
	// The PM action catalog controls access to the management portal. Tool
	// Runtime execution remains independently constrained by its exact Tool +
	// scope Grant and never derives execution authority from PM page access.
	if s.mountToolManagementRoutes != nil {
		s.mountToolManagementRoutes(scoped)
	}
	if s.mountDesignReviewRoutes != nil {
		s.mountDesignReviewRoutes(scoped)
	}
	if s.mountProjectAccessGuideRoutes != nil {
		s.mountProjectAccessGuideRoutes(scoped)
	}
	if s.mountDocumentRoutes != nil {
		s.mountDocumentRoutes(scoped)
	}
	if s.mountBitableRoutes != nil {
		s.mountBitableRoutes(scoped)
	}
	if s.mountResourceAccessRoutes != nil {
		s.mountResourceAccessRoutes(scoped)
	}
	if s.mountCollaborationRoutes != nil {
		s.mountCollaborationRoutes(scoped)
	}
	if s.mountReportRoutes != nil {
		s.mountReportRoutes(scoped)
	}
	if s.mountCodeContribRoutes != nil {
		s.mountCodeContribRoutes(scoped)
	}
	scoped.GET("/project-requirement-scope-options", s.scopeOptions)
	scoped.GET("/project-management-board", s.board)
	scoped.GET("/issue-kinds", s.listIssueKinds)
	scoped.GET("/issue-statuses", s.listIssueStatuses)
	scoped.GET("/issue-workflow-transitions", s.listIssueWorkflowTransitions)
	scoped.GET("/settings/work-item-layout", s.getTaskLayoutSettings)

	scoped.GET("/project-sets", s.listProjectSets)
	scoped.POST("/project-sets", s.createProjectSet)
	scoped.GET("/project-sets/:id/deletion-impact", s.getProjectSetDeletionImpact)
	scoped.GET("/project-sets/:id/quality-overview", s.projectQualityOverview)
	scoped.GET("/project-sets/:id/participants", s.listProjectParticipants)
	scoped.GET("/project-sets/:id", s.getProjectSet)
	scoped.PATCH("/project-sets/:id", s.patchProjectSet)
	scoped.DELETE("/project-sets/:id", s.deleteProjectSet)
	scoped.POST("/project-sets/:id/transfer-owner-new", s.transferProjectCreator)
	scoped.GET("/project-sets/:id/audit-logs", s.listAuditLogs)
	scoped.GET("/project-sets/:id/news", s.listProjectNews)
	scoped.POST("/project-sets/:id/news", s.createProjectNews)

	// Plan unit (project/version item) member management: owner +
	// participants are version-level facts.
	scoped.GET("/project-new/:id/members", s.getPlanUnitMembers)
	scoped.GET("/project-new/:id/member-options", s.getPlanUnitMemberOptions)
	scoped.GET("/project-new/:id/members/:userID/removal-impact", s.getPlanUnitMemberRemovalImpact)
	scoped.PUT("/project-new/:id/members", s.putPlanUnitMembers)

	scoped.GET("/project-new", s.listPlanUnits)
	scoped.POST("/project-new", s.createPlanUnit)
	scoped.GET("/project-new/:id/deletion-impact", s.getPlanUnitDeletionImpact)
	scoped.GET("/project-new/:id", s.getPlanUnit)
	scoped.PATCH("/project-new/:id", s.patchPlanUnit)
	scoped.DELETE("/project-new/:id", s.deletePlanUnit)
	scoped.POST("/project-new/:id/requirements-lock", s.lockPlanUnitRequirements)
	scoped.GET("/project-access/effective", s.getEffectiveProjectAccess)
	scoped.GET("/version-tasks-new/:id/delivery-node-labels", s.listDeliveryLabels)
	scoped.POST("/version-tasks-new/:id/delivery-node-labels", s.createDeliveryLabel)
	scoped.PATCH("/project-delivery-node-labels/:id", s.patchDeliveryLabel)
	scoped.GET("/project-delivery-node-labels/:id/deletion-impact", s.getDeliveryLabelDeletionImpact)
	scoped.DELETE("/project-delivery-node-labels/:id", s.deleteDeliveryLabel)

	scoped.GET("/product-requirements-new", s.listRequirements)
	scoped.POST("/product-requirements-new", s.createRequirement)
	scoped.GET("/product-requirements-new/:id/deletion-impact", s.getRequirementDeletionImpact)
	scoped.GET("/product-requirements-new/:id", s.getRequirement)
	scoped.PATCH("/product-requirements-new/:id", s.patchRequirement)
	scoped.DELETE("/product-requirements-new/:id", s.deleteRequirement)
	scoped.GET("/requirements", s.listRequirementPool)
	scoped.POST("/requirements", s.createRequirementPool)
	scoped.GET("/requirements/:id", s.getRequirementPool)
	scoped.GET("/requirements/:id/cover", s.getRequirementCover)
	scoped.PATCH("/requirements/:id", s.patchRequirementPool)
	scoped.PATCH("/requirements/:id/role-owners", s.patchRequirementRoleOwners)
	scoped.DELETE("/requirements/:id", s.deleteRequirement)
	scoped.POST("/requirements/batch-move", s.batchMoveRequirements)
	scoped.POST("/requirements/:id/schedule", s.scheduleRequirement)
	scoped.POST("/requirements/:id/feishu-document/validate", s.validateRequirementFeishuDocument)
	scoped.POST("/requirements/:id/design-assets/upload", s.uploadRequirementDesignAsset)
	scoped.GET("/requirements/:id/materials", s.listRequirementMaterials)
	scoped.POST("/requirements/:id/materials", s.uploadRequirementOwnedMaterial)
	scoped.DELETE("/requirements/:id/materials/:materialId", s.deleteRequirementMaterial)
	scoped.GET("/requirements/:id/materials/:materialId/download", s.downloadRequirementMaterial)
	scoped.GET("/pm/workflows/templates", s.listWorkflowTemplates)
	scoped.POST("/pm/workflows/templates", s.createWorkflowTemplate)
	scoped.GET("/pm/workflows/templates/:id", s.getWorkflowTemplate)
	scoped.PATCH("/pm/workflows/templates/:id", s.patchWorkflowTemplate)
	scoped.POST("/pm/workflows/templates/:id/validate", s.validateWorkflowTemplate)
	scoped.POST("/pm/workflows/templates/:id/validate-definition", s.validateWorkflowDefinition)
	scoped.POST("/pm/workflows/templates/:id/publish", s.publishWorkflowTemplate)
	scoped.POST("/pm/workflows/instances", s.startWorkflowInstance)
	scoped.GET("/pm/workflows/instances", s.listWorkflowInstances)
	scoped.GET("/pm/workflows/instances/:id", s.getWorkflowInstance)
	scoped.PATCH("/pm/workflows/instances/:id/node-plans", s.patchWorkflowNodePlans)
	scoped.GET("/pm/workflows/instances/:id/nodes/:nodeId/test-case-artifacts", s.listWorkflowTestCaseArtifacts)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/test-case-artifacts", s.uploadWorkflowTestCaseArtifact)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/test-case-artifacts/ai-generated", s.materializeWorkflowTestCaseArtifact)
	scoped.GET("/pm/workflows/test-case-artifacts/:artifactId/download", s.downloadWorkflowTestCaseArtifact)
	scoped.GET("/pm/workflows/test-case-materials/:requirementId/:materialId/download", s.downloadWorkflowTestCaseMaterial)
	scoped.GET("/pm/workflows/instances/:id/material-projections", s.listWorkflowMaterialProjections)
	scoped.GET("/pm/workflows/instances/:id/nodes/:nodeId/test-report-artifacts", s.listWorkflowTestReportArtifacts)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/test-report-artifacts", s.uploadWorkflowTestReportArtifact)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/test-report-artifacts/link", s.createWorkflowTestReportLink)
	scoped.GET("/pm/testing/items", s.listWorkflowTestLibrary)
	scoped.POST("/pm/testing/items", s.createTestingLibraryItem)
	scoped.GET("/pm/testing/version-reports", s.listVersionTestReports)
	scoped.POST("/pm/testing/version-reports/upload", s.uploadVersionTestReport)
	scoped.POST("/pm/testing/version-reports/link", s.linkVersionTestReport)
	scoped.GET("/pm/testing/reports/:id/download", s.downloadPlanUnitTestReport)
	scoped.GET("/pm/workflows/instances/:id/nodes/:nodeId/opinions", s.listWorkflowNodeOpinions)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/opinions", s.createWorkflowNodeOpinion)
	scoped.PATCH("/pm/workflows/instances/:id/nodes/:nodeId/opinions/:opinionId", s.updateWorkflowNodeOpinion)
	scoped.POST("/pm/workflows/instances/:id/nodes/:nodeId/opinion-images", s.uploadWorkflowNodeOpinionImage)
	scoped.GET("/pm/workflow-node-artifacts/:artifactId/download", s.downloadWorkflowNodeArtifact)
	scoped.POST("/pm/workflows/instances/:id/actions", s.actOnWorkflowInstance)
	scoped.GET("/ai/capabilities", s.listAICapabilities)
	scoped.GET("/ai/runs", s.listAIRuns)
	scoped.POST("/ai/runs", s.createAIRun)
	scoped.GET("/ai/runs/:id", s.getAIRun)
	scoped.GET("/ai/runs/:id/events", s.streamAIRunEvents)
	scoped.POST("/ai/runs/:id/apply", s.applyAIRun)
	scoped.POST("/ai/runs/:id/feedback", s.feedbackAIRun)
	scoped.POST("/ai/runs/:id/retry", s.retryAIRun)
	scoped.POST("/ai/runs/:id/cancel", s.cancelAIRun)

	scoped.GET("/scrum-tasks-new", s.listTasks)
	scoped.GET("/tasks", s.listWorkspaceTasks)
	scoped.POST("/tasks", s.createTask)
	scoped.POST("/tasks/batch-create", s.batchCreateTasks)
	scoped.GET("/tasks/client-options", s.taskClientOptions.List)
	scoped.POST("/tasks/client-options", s.taskClientOptions.Create)
	scoped.PATCH("/tasks/client-options/:id", s.taskClientOptions.Update)
	scoped.DELETE("/tasks/client-options/:id", s.taskClientOptions.Delete)
	scoped.GET("/tasks/stats", s.taskWorkspaceStats)
	scoped.POST("/tasks/batch-delete", s.batchDeleteTasks)
	scoped.POST("/tasks/materials/upload", s.uploadTaskRichTextMaterial)
	scoped.GET("/tasks/attachments/:attachmentId/download", s.downloadTaskAttachment)
	scoped.DELETE("/tasks/attachments/:attachmentId", s.deleteTaskAttachment)
	scoped.GET("/tasks/:id", s.getWorkspaceTask)
	scoped.PATCH("/tasks/:id", s.patchTask)
	scoped.PATCH("/tasks/:id/status", s.patchTask)
	scoped.DELETE("/tasks/:id", s.deleteTask)
	scoped.POST("/tasks/:id/children/batch", s.batchCreateTaskChildren)
	scoped.GET("/tasks/:id/deletion-impact", s.getTaskDeletionImpact)
	scoped.GET("/tasks/:id/related", s.listTaskRelated)
	scoped.GET("/tasks/:id/due-at-descendant-conflicts", s.listTaskDueAtDescendantConflicts)
	scoped.GET("/tasks/:id/attachments", s.listTaskAttachments)
	scoped.POST("/tasks/:id/attachments", s.uploadTaskAttachment)
	scoped.PUT("/tasks/:id/watch", s.putTaskWatch)
	scoped.GET("/tasks/:id/git-links", s.listTaskGitLinks)
	scoped.GET("/tasks/:id/activities", s.listTaskActivities)
	scoped.GET("/tasks/:id/comments", s.listTaskComments)
	scoped.POST("/tasks/:id/comments", s.createTaskComment)
	scoped.DELETE("/tasks/:id/comments/:cid", s.deleteTaskComment)
	scoped.DELETE("/tasks/comments/:cid", s.deleteTaskComment)
	s.mountTaskNativeOperationRoutes(scoped)
	s.mountTaskTestingRoutes(scoped)

	scoped.GET("/quality/bugs", s.listQualityBugs)
	scoped.GET("/quality/bugs/statistics", s.qualityBugStatistics)
	scoped.GET("/quality/bugs/dashboard", s.qualityBugDashboard)
	scoped.GET("/quality/scene-options/:kind", s.qualitySceneOptions.List)
	scoped.POST("/quality/scene-options/:kind", s.qualitySceneOptions.Create)
	scoped.DELETE("/quality/scene-options/:kind/:id", s.qualitySceneOptions.Delete)
	scoped.POST("/quality/bugs", s.createQualityBug)
	scoped.POST("/quality/bugs/batch-create", s.batchCreateQualityBugs)
	scoped.POST("/quality/bugs/batch-delete", s.batchDeleteQualityBugs)
	scoped.POST("/quality/bugs/batch-assignment", s.batchAssignQualityBugs)
	scoped.POST("/quality/bugs/materials/upload", s.uploadQualityBugMaterial)
	scoped.GET("/quality/bugs/:id/activities", s.listQualityBugActivities)
	scoped.GET("/quality/bugs/:id/comments", s.listQualityBugComments)
	scoped.POST("/quality/bugs/:id/comments", s.createQualityBugComment)
	scoped.DELETE("/quality/bugs/:id/comments/:cid", s.deleteQualityBugComment)
	scoped.GET("/quality/bugs/:id", s.getQualityBug)
	scoped.PATCH("/quality/bugs/:id", s.patchQualityBug)
	scoped.PATCH("/quality/bugs/:id/status", s.patchQualityBug)
	scoped.PATCH("/quality/bugs/:id/assignment", s.patchQualityBug)
	scoped.DELETE("/quality/bugs/:id", s.deleteQualityBug)
	scoped.GET("/workbench/todos/summary", s.workbenchTodoSummary)
	scoped.GET("/workbench/todos/feed", s.workbenchTodoFeed)
	scoped.GET("/executive-dashboard/bi-screen", s.executiveBIScreen)
	scoped.GET("/executive-dashboard/bi-screen/team-member-items", s.listBITeamMemberItems)
	scoped.PUT("/executive-dashboard/bi-screen/team-score-profile", s.putBITeamScoreProfile)
	scoped.GET("/executive-dashboard/bi-screen/team-metric-settings", s.getBITeamMetricSettings)
	scoped.PUT("/executive-dashboard/bi-screen/team-metric-settings", s.putBITeamMetricSettings)

}

func (s *Server) publicModules(c *gin.Context) {
	ok(c, gin.H{
		"modules": gin.H{
			"project_management": true, "task_library": true,
			"quality_management": true, "executive_dashboard": true,
		},
		"auth": gin.H{
			"local_login_allowed": s.cfg.AuthMode != "shared_jwt",
		},
	})
}
