package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

const workflowCreatorPolicyDefinition = `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"review","type":"approval","name":"评审"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"review"},{"id":"e2","source":"review","target":"end","kind":"approve"}]}`

type workflowCreatorPolicyFixture struct {
	app     *testApp
	access  *identity.AccessService
	owner   identitygorm.User
	creator identitygorm.User
	other   identitygorm.User
}

func setupWorkflowCreatorPolicyFixture(t *testing.T) workflowCreatorPolicyFixture {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) { cfg.RBACMode = config.RBACModeEnforce })
	owner := app.createUser(t, 1, "workflow-policy-owner", "super_admin", "")
	creator := app.createUser(t, 2, "workflow-policy-creator", "user", "")
	other := app.createUser(t, 3, "workflow-policy-other", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "AI PM", OwnerUserID: &owner.ID}
	unit := projectgorm.PlanUnit{ID: 12, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID, Title: "V1.1", Type: "version"}
	for _, row := range []any{&workspace, &team, &project, &unit} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range []identitygorm.WorkspaceMember{
		{WorkspaceID: workspace.ID, UserID: creator.ID, Role: "member", Enabled: true},
		{WorkspaceID: workspace.ID, UserID: other.ID, Role: "member", Enabled: true},
	} {
		if err := app.db.Create(&member).Error; err != nil {
			t.Fatal(err)
		}
	}
	access := identity.NewAccessService(identitygorm.NewAccessRepository(app.db), nil)
	engine := NewWithDependencies(app.cfg, app.db, Dependencies{
		Authorizer: access, ProjectAccess: fakeAccessAllower{},
	})
	inventory := NewRouteInventory()
	inventory.Load(engine.Routes())
	access.SetServiceInterfaceValidator(inventory)
	for _, record := range accesscatalog.BootstrapActionRoutes() {
		if record.ActionCode != accesscatalog.ActionWorkflowsInstancesAct {
			continue
		}
		if _, err := access.CreateServiceInterface(context.Background(), identity.ServiceInterfaceInput{
			Code: record.InterfaceCode, ActionCode: string(record.ActionCode),
			HTTPMethod: record.HTTPMethod, RouteTemplate: record.RouteTemplate,
			Name: record.ActionName, Description: record.CopyKey, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	app.engine = engine
	now := time.Now().UTC()
	definition := workflowCreatorPolicyDefinition
	template := workflowgorm.WorkflowTemplate{
		ID: 51, WorkspaceID: workspace.ID, Name: "创建人审批流", SubjectType: "requirement",
		DraftDefinition: definition, CreatedByUserID: owner.ID, UpdatedByUserID: owner.ID,
	}
	version := workflowgorm.WorkflowTemplateVersion{
		ID: 61, WorkspaceID: workspace.ID, TemplateID: template.ID, Version: 1,
		Definition: definition, DefinitionHash: "policy-test-hash",
		PublishedByUser: owner.ID, PublishedAt: now,
	}
	if err := app.db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	return workflowCreatorPolicyFixture{app: app, access: access, owner: owner, creator: creator, other: other}
}

func (fix workflowCreatorPolicyFixture) seedRequirementWorkflow(t *testing.T, instanceID, requirementID, starterID uint) {
	t.Helper()
	now := time.Now().UTC()
	requirement := requirementgorm.Requirement{
		ID: requirementID, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
		Title: "创建人审批需求", Status: "dev",
	}
	instance := workflowgorm.WorkflowInstance{
		ID: instanceID, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "requirement", SubjectID: requirementID, Status: "running",
		CurrentNodeIDs: `["review"]`, NodePlansJSON: "{}",
		StartedByUserID: starterID, StartedAt: now, UpdatedAt: now,
	}
	token := workflowgorm.WorkflowToken{
		WorkspaceID: 1, InstanceID: instanceID, NodeID: "review", Status: "active",
		AssigneeUserID: &starterID, CreatedAt: now, UpdatedAt: now,
	}
	for _, row := range []any{&requirement, &instance, &token} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func (fix workflowCreatorPolicyFixture) seedPlanUnitWorkflow(t *testing.T, instanceID, starterID uint) {
	t.Helper()
	now := time.Now().UTC()
	instance := workflowgorm.WorkflowInstance{
		ID: instanceID, WorkspaceID: 1, TemplateID: 51, TemplateVersionID: 61,
		SubjectType: "plan_unit", SubjectID: 12, Status: "running",
		CurrentNodeIDs: `["review"]`, NodePlansJSON: "{}",
		StartedByUserID: starterID, StartedAt: now, UpdatedAt: now,
	}
	token := workflowgorm.WorkflowToken{
		WorkspaceID: 1, InstanceID: instanceID, NodeID: "review", Status: "active",
		AssigneeUserID: &starterID, CreatedAt: now, UpdatedAt: now,
	}
	for _, row := range []any{&instance, &token} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func (fix workflowCreatorPolicyFixture) approve(t *testing.T, instanceID uint, user identitygorm.User) *httptest.ResponseRecorder {
	t.Helper()
	return fix.app.request(t, http.MethodPost,
		"/api/v1/platform/pm/workflows/instances/"+uintString(instanceID)+"/actions",
		fix.app.token(t, user), 1,
		map[string]any{"action": "approve", "node_id": "review", "comment": "创建人推进"},
	)
}

func assertWorkflowInstanceStatus(t *testing.T, app *testApp, instanceID uint, expected string) {
	t.Helper()
	var instance workflowgorm.WorkflowInstance
	if err := app.db.First(&instance, instanceID).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != expected {
		t.Fatalf("instance %d status=%q want=%q", instanceID, instance.Status, expected)
	}
}

func TestRequirementWorkflowCreatorMayApproveWithoutActGrant(t *testing.T) {
	fix := setupWorkflowCreatorPolicyFixture(t)
	fix.seedRequirementWorkflow(t, 81, 71, fix.creator.ID)

	requireStatus(t, fix.approve(t, 81, fix.creator), http.StatusOK)

	assertWorkflowInstanceStatus(t, fix.app, 81, "completed")
	var approvals []workflowgorm.WorkflowApproval
	if err := fix.app.db.Where("workspace_id = ? AND instance_id = ?", 1, 81).Find(&approvals).Error; err != nil {
		t.Fatal(err)
	}
	if len(approvals) != 1 || approvals[0].ApproverUserID == nil || *approvals[0].ApproverUserID != fix.creator.ID {
		t.Fatalf("approvals=%+v", approvals)
	}
}

func TestRequirementWorkflowNonCreatorWithoutActGrantIsForbidden(t *testing.T) {
	fix := setupWorkflowCreatorPolicyFixture(t)
	fix.seedRequirementWorkflow(t, 82, 72, fix.creator.ID)

	requireStatus(t, fix.approve(t, 82, fix.other), http.StatusForbidden)

	assertWorkflowInstanceStatus(t, fix.app, 82, "running")
	var approvals int64
	if err := fix.app.db.Model(&workflowgorm.WorkflowApproval{}).
		Where("workspace_id = ? AND instance_id = ?", 1, 82).Count(&approvals).Error; err != nil {
		t.Fatal(err)
	}
	if approvals != 0 {
		t.Fatalf("approval rows=%d", approvals)
	}
}

func TestRequirementWorkflowActGrantStillAuthorizes(t *testing.T) {
	fix := setupWorkflowCreatorPolicyFixture(t)
	fix.seedRequirementWorkflow(t, 83, 73, fix.creator.ID)
	role, err := fix.access.CreateRole(context.Background(), identity.RoleInput{
		Code: "workflow-actor", Name: "Workflow actor", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceRoleActionCodes(
		context.Background(), fix.owner.ID, role.ID,
		[]string{string(accesscatalog.ActionWorkflowsInstancesAct)},
	); err != nil {
		t.Fatal(err)
	}
	if err := fix.access.ReplaceUserRoles(context.Background(), fix.other.ID, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}

	requireStatus(t, fix.approve(t, 83, fix.other), http.StatusOK)

	assertWorkflowInstanceStatus(t, fix.app, 83, "completed")
}

func TestPlanUnitWorkflowStarterCannotUseRequirementCreatorPolicy(t *testing.T) {
	fix := setupWorkflowCreatorPolicyFixture(t)
	fix.seedPlanUnitWorkflow(t, 84, fix.other.ID)

	requireStatus(t, fix.approve(t, 84, fix.other), http.StatusForbidden)

	assertWorkflowInstanceStatus(t, fix.app, 84, "running")
}
