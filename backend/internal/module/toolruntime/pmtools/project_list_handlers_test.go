package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type projectListQueriesFake struct {
	workspaceID  uint
	filter       project.ProjectSetListFilter
	accessFilter project.AccessFilter
	projectID    uint
}

func (fake *projectListQueriesFake) ListAccessibleProjectSets(
	_ context.Context,
	workspaceID uint,
	filter project.ProjectSetAccessListFilter,
) (project.ProjectSetListPage, error) {
	fake.workspaceID, fake.filter = workspaceID, filter.ProjectSetListFilter
	fake.accessFilter = filter.Access
	return project.ProjectSetListPage{Items: []project.ProjectSetSnapshot{{ID: 11, WorkspaceID: workspaceID}}, Total: 3}, nil
}

func (fake *projectListQueriesFake) ListProjectSets(
	_ context.Context,
	workspaceID uint,
	filter project.ProjectSetListFilter,
) (project.ProjectSetListPage, error) {
	fake.workspaceID, fake.filter = workspaceID, filter
	return project.ProjectSetListPage{Items: []project.ProjectSetSnapshot{{ID: 11, WorkspaceID: workspaceID}}, Total: 3}, nil
}

func (fake *projectListQueriesFake) ListPlanUnits(
	_ context.Context,
	workspaceID uint,
	projectSetID uint,
) ([]project.PlanUnitSnapshot, error) {
	fake.workspaceID, fake.projectID = workspaceID, projectSetID
	return []project.PlanUnitSnapshot{{ID: 21, WorkspaceID: workspaceID, ProjectSetID: projectSetID}}, nil
}

func (fake *projectListQueriesFake) GetProjectSet(
	_ context.Context,
	workspaceID uint,
	projectSetID uint,
) (project.ProjectSetSnapshot, error) {
	fake.workspaceID, fake.projectID = workspaceID, projectSetID
	owner := uint(9)
	return project.ProjectSetSnapshot{ID: projectSetID, WorkspaceID: workspaceID, Name: "PM", OwnerUserID: &owner}, nil
}

func TestProjectListHandlersUseOnlyAuthorizedWorkspaceAndCanonicalFilters(t *testing.T) {
	fake := &projectListQueriesFake{}
	handlers, err := NewProjectListHandlers(ProjectListDependencies{Projects: fake})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handlers[ProjectSetList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"team_id":70,"q":"Alpha","limit":10,"offset":2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.workspaceID != 7 || fake.filter.TeamID != 70 || fake.filter.Query != "Alpha" || fake.filter.Limit != 10 || fake.filter.Offset != 2 {
		t.Fatalf("workspace=%d filter=%+v", fake.workspaceID, fake.filter)
	}
	var output projectSetListOutput
	if err := json.Unmarshal(raw, &output); err != nil || output.Total != 3 || len(output.Items) != 1 {
		t.Fatalf("output=%s err=%v", raw, err)
	}

	raw, err = handlers[ProjectPlanUnitList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"project_new_id":11}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.workspaceID != 7 || fake.projectID != 11 {
		t.Fatalf("workspace=%d project=%d", fake.workspaceID, fake.projectID)
	}
	var units listOutput[project.PlanUnitSnapshot]
	if err := json.Unmarshal(raw, &units); err != nil || units.Total != 1 || len(units.Items) != 1 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}

func TestProjectListHandlersRejectMissingScopeAndUnknownInputs(t *testing.T) {
	fake := &projectListQueriesFake{}
	handlers, err := NewProjectListHandlers(ProjectListDependencies{Projects: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = handlers[ProjectSetList].Invoke(context.Background(), toolruntime.ToolCall{Input: json.RawMessage(`{}`)})
	if !errors.Is(err, ErrReadHandlersInvalid) || fake.workspaceID != 0 {
		t.Fatalf("missing workspace err=%v workspace=%d", err, fake.workspaceID)
	}
	_, err = handlers[ProjectPlanUnitList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"project_new_id":11,"unexpected":true}`),
	})
	if !errors.Is(err, ErrReadHandlersInvalid) {
		t.Fatalf("unknown input err=%v", err)
	}
}

func TestProjectAccessHandlerUsesPrincipalAsInformationalActor(t *testing.T) {
	fake := &projectListQueriesFake{}
	handlers, err := NewProjectAccessHandlers(ProjectAccessDependencies{Projects: fake})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handlers[ProjectAccessEffective].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 9, WorkspaceID: 7},
		Input:         json.RawMessage(`{"project_set_id":11}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var output projectAccessEffectiveOutput
	if err := json.Unmarshal(raw, &output); err != nil || !output.Effective || !output.ProjectSet.ActorIsOwner || output.Actor.UserID != 9 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
	if output.AccessBasis.LegacyProjectPermissionUsed {
		t.Fatalf("legacy permission was reported: %+v", output.AccessBasis)
	}
}
