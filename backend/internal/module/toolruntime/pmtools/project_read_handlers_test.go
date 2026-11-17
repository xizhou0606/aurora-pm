package pmtools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type projectReadFake struct {
	workspaceIDs []uint
	scopeUserIDs []uint
}

func (fake *projectReadFake) capture(workspaceID uint) {
	fake.workspaceIDs = append(fake.workspaceIDs, workspaceID)
}

func (fake *projectReadFake) GetProjectSet(_ context.Context, workspaceID, id uint) (project.ProjectSetSnapshot, error) {
	fake.capture(workspaceID)
	ownerID := uint(9)
	return project.ProjectSetSnapshot{ID: id, WorkspaceID: workspaceID, TeamID: 2, Name: "PM", Code: "PM", OwnerUserID: &ownerID}, nil
}

func (fake *projectReadFake) GetPlanUnit(_ context.Context, workspaceID, id uint) (project.PlanUnitSnapshot, error) {
	fake.capture(workspaceID)
	return project.PlanUnitSnapshot{ID: id, WorkspaceID: workspaceID, TeamID: 2, ProjectSetID: 11, Title: "V1"}, nil
}

func (fake *projectReadFake) ListPlanUnits(_ context.Context, workspaceID, projectSetID uint) ([]project.PlanUnitSnapshot, error) {
	fake.capture(workspaceID)
	ownerID := uint(9)
	return []project.PlanUnitSnapshot{{ID: 21, WorkspaceID: workspaceID, TeamID: 2, ProjectSetID: projectSetID, Title: "V1", OwnerUserID: &ownerID}}, nil
}

func (fake *projectReadFake) ListDeliveryLabels(_ context.Context, workspaceID, planUnitID uint) ([]project.DeliveryLabelSnapshot, error) {
	fake.capture(workspaceID)
	return []project.DeliveryLabelSnapshot{{ID: 31, WorkspaceID: workspaceID, ProjectSetID: 11, PlanUnitID: planUnitID, Name: "Ready"}}, nil
}

func (fake *projectReadFake) ListScopeOptions(_ context.Context, workspaceID, userID uint) (project.ScopeOptions, error) {
	fake.capture(workspaceID)
	fake.scopeUserIDs = append(fake.scopeUserIDs, userID)
	return project.ScopeOptions{
		ProjectSets: []project.ScopeProjectSet{{ID: 11, TeamID: 2, Name: "PM"}},
		PlanUnits:   []project.PlanUnitSnapshot{{ID: 21, WorkspaceID: workspaceID, TeamID: 2, ProjectSetID: 11, Title: "V1"}},
	}, nil
}

func (fake *projectReadFake) ListRequirements(_ context.Context, workspaceID uint, filter requirement.ListFilter) (requirement.ListPage, error) {
	fake.capture(workspaceID)
	planUnitID := uint(21)
	ownerID := uint(9)
	return requirement.ListPage{Items: []requirement.Snapshot{{
		ID: 41, WorkspaceID: workspaceID, TeamID: 2, ProjectSetID: 11, PlanUnitID: &planUnitID,
		Title: "Requirement", ProductOwnerUserIDs: []uint{ownerID}, DeveloperUserIDs: []uint{},
	}}, Total: 1}, nil
}

func (fake *projectReadFake) ListScopeRequirements(ctx context.Context, workspaceID uint, _ []uint) ([]requirement.Snapshot, error) {
	page, err := fake.ListRequirements(ctx, workspaceID, requirement.ListFilter{ProjectSetID: 11})
	return page.Items, err
}

func (fake *projectReadFake) ListTasks(_ context.Context, workspaceID uint, filter task.ListFilter) (task.ListResult, error) {
	fake.capture(workspaceID)
	assigneeID := uint(9)
	return task.ListResult{Items: []task.ListItem{{
		ID: 51, WorkspaceID: workspaceID, ProjectSetID: filter.ProjectSetID, TaskKey: "PM-51", Title: "Task", AssigneeID: &assigneeID,
	}}, Total: 1}, nil
}

func (fake *projectReadFake) ListBugs(_ context.Context, workspaceID uint, filter quality.BugReadFilter) (quality.BugReadPage, error) {
	fake.capture(workspaceID)
	reporterID := uint(9)
	return quality.BugReadPage{Items: []quality.BugSnapshot{{
		ID: 61, WorkspaceID: workspaceID, ProjectSetID: filter.ProjectSetID, BugKey: "BUG-61", Title: "Bug", ReporterUserID: &reporterID,
	}}, Total: 1}, nil
}

func (fake *projectReadFake) ListProjectEvents(_ context.Context, workspaceID, projectSetID uint, _, _ int) (audit.ProjectEventPage, error) {
	fake.capture(workspaceID)
	return audit.ProjectEventPage{Items: []audit.EventSnapshot{{
		ID: 71, WorkspaceID: workspaceID, ProjectID: projectSetID, Resource: "project_set", Action: "updated", CreatedAt: time.Now(),
	}}, Total: 1}, nil
}

func (fake *projectReadFake) ListActiveUsers(context.Context) ([]identity.UserSummary, error) {
	return []identity.UserSummary{{ID: 9, Username: "owner", DisplayName: "Owner", Active: true}}, nil
}

type projectReadAuthorizer struct{}

func (projectReadAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(ProjectReadHandlerNames(), []string{"project:read", "workspace:read"})
}

func TestProjectReadHandlersInvokeThroughCanonicalApplication(t *testing.T) {
	fake := &projectReadFake{}
	handlers, err := NewProjectReadHandlers(ProjectReadDependencies{
		Projects: fake, Requirements: fake, Tasks: fake, Quality: fake, Audits: fake, Identities: fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: ProjectReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, projectReadAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]json.RawMessage{
		ProjectBoardGet:          json.RawMessage(`{"project_new_id":11}`),
		ProjectScopeOptionsList:  json.RawMessage(`{}`),
		ProjectSetAuditList:      json.RawMessage(`{"id":11,"limit":10,"offset":0}`),
		ProjectDeliveryLabelList: json.RawMessage(`{"id":21}`),
		ProjectRequirementList:   json.RawMessage(`{"project_new_id":11,"limit":10,"offset":0}`),
		ProjectParticipantList:   json.RawMessage(`{"id":11}`),
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 9, WorkspaceID: 7, GrantID: 8, ClientID: "project-read-test",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	for _, name := range ProjectReadHandlerNames() {
		result, invokeErr := application.Invoke(context.Background(), toolruntime.Invocation{
			Name: name, Input: inputs[name], Authorization: authorization,
		})
		if invokeErr != nil {
			t.Fatalf("invoke %s: %v", name, invokeErr)
		}
		if result.Name != name || len(result.Output) == 0 {
			t.Fatalf("invoke %s result=%+v", name, result)
		}
	}
	for _, workspaceID := range fake.workspaceIDs {
		if workspaceID != 7 {
			t.Fatalf("workspace leakage: got %d", workspaceID)
		}
	}
}

func TestProjectReadHandlersRejectUnknownBoardIncludeAndMissingWorkspace(t *testing.T) {
	fake := &projectReadFake{}
	handlers, err := NewProjectReadHandlers(ProjectReadDependencies{
		Projects: fake, Requirements: fake, Tasks: fake, Quality: fake, Audits: fake, Identities: fake,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handlers[ProjectBoardGet].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"project_new_id":11,"include":"secrets"}`),
	}); err == nil {
		t.Fatal("expected unsupported board include to fail")
	}
	if _, err := handlers[ProjectParticipantList].Invoke(context.Background(), toolruntime.ToolCall{
		Input: json.RawMessage(`{"id":11}`),
	}); err == nil {
		t.Fatal("expected missing server workspace to fail")
	}
}
