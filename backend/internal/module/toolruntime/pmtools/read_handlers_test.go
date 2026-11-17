package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type readQueriesFake struct {
	lastWorkspace uint
	lastTask      task.Lookup
}

type readAuthorizer struct{}

func (readAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(ReadHandlerNames(), []string{"project:read", "task:read", "workflow:read"})
}

func (fake *readQueriesFake) capture(workspaceID uint) error {
	fake.lastWorkspace = workspaceID
	if workspaceID != 7 {
		return errors.New("unexpected workspace")
	}
	return nil
}

func (fake *readQueriesFake) GetProjectSet(_ context.Context, workspaceID, id uint) (project.ProjectSetSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return project.ProjectSetSnapshot{}, err
	}
	return project.ProjectSetSnapshot{ID: id, WorkspaceID: workspaceID, Name: "PM", Status: "active"}, nil
}

func (fake *readQueriesFake) GetPlanUnit(_ context.Context, workspaceID, id uint) (project.PlanUnitSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return project.PlanUnitSnapshot{}, err
	}
	return project.PlanUnitSnapshot{ID: id, WorkspaceID: workspaceID, Title: "V1", Type: "version", Status: "planning"}, nil
}

func (fake *readQueriesFake) GetRequirement(_ context.Context, workspaceID, id uint) (requirement.Snapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return requirement.Snapshot{}, err
	}
	return requirement.Snapshot{ID: id, WorkspaceID: workspaceID, Title: "Requirement", Priority: "normal", Status: "backlog"}, nil
}

func (fake *readQueriesFake) GetTask(_ context.Context, workspaceID uint, lookup task.Lookup) (task.Snapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return task.Snapshot{}, err
	}
	fake.lastTask = lookup
	return task.Snapshot{ID: 44, WorkspaceID: workspaceID, TaskKey: lookup.Reference, Title: "Task", Children: []task.Reference{}}, nil
}

func (fake *readQueriesFake) ListTemplates(_ context.Context, workspaceID uint, filter workflow.TemplateFilter) ([]workflow.TemplateSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return nil, err
	}
	return []workflow.TemplateSnapshot{{ID: 5, WorkspaceID: workspaceID, SubjectType: filter.SubjectType}}, nil
}

func (fake *readQueriesFake) GetTemplate(_ context.Context, workspaceID, id uint) (workflow.TemplateSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return workflow.TemplateSnapshot{}, err
	}
	return workflow.TemplateSnapshot{ID: id, WorkspaceID: workspaceID, Definition: json.RawMessage(`{"nodes":[]}`)}, nil
}

func (fake *readQueriesFake) ListInstances(_ context.Context, workspaceID uint, filter workflow.InstanceFilter) ([]workflow.InstanceSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return nil, err
	}
	return []workflow.InstanceSnapshot{{
		ID: 6, WorkspaceID: workspaceID, SubjectType: filter.SubjectType, SubjectID: filter.SubjectID,
		AllowedActions: []string{}, CurrentNodeIDs: []string{}, NodePlans: json.RawMessage(`{}`),
	}}, nil
}

func (fake *readQueriesFake) GetInstance(_ context.Context, workspaceID, id uint) (workflow.InstanceSnapshot, error) {
	if err := fake.capture(workspaceID); err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	return workflow.InstanceSnapshot{
		ID: id, WorkspaceID: workspaceID, SubjectType: "requirement", SubjectID: 99,
		AllowedActions: []string{}, CurrentNodeIDs: []string{}, NodePlans: json.RawMessage(`{}`),
	}, nil
}

func TestReadHandlersExposeOnlyActivationContract(t *testing.T) {
	fake := &readQueriesFake{}
	handlers, err := NewReadHandlers(ReadDependencies{Projects: fake, Requirements: fake, Tasks: fake, Workflows: fake})
	if err != nil {
		t.Fatal(err)
	}
	if len(handlers) != 8 {
		t.Fatalf("handlers=%d", len(handlers))
	}
	for _, name := range ReadHandlerNames() {
		if handlers[name] == nil {
			t.Fatalf("missing handler %q", name)
		}
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: ReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	active := make([]string, 0, 8)
	for _, definition := range catalog.Definitions() {
		if definition.Lifecycle == toolruntime.LifecycleActive {
			active = append(active, definition.Name)
		}
	}
	names := ReadHandlerNames()
	sort.Strings(active)
	sort.Strings(names)
	if !reflect.DeepEqual(active, names) {
		t.Fatalf("active=%v names=%v", active, ReadHandlerNames())
	}
}

func TestReadHandlersPassServerWorkspaceAndCanonicalInputs(t *testing.T) {
	fake := &readQueriesFake{}
	handlers, err := NewReadHandlers(ReadDependencies{Projects: fake, Requirements: fake, Tasks: fake, Workflows: fake})
	if err != nil {
		t.Fatal(err)
	}
	call := toolruntime.ToolCall{Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"task_ref":"PM-44","project_set_id":2,"project_new_id":3,"requirement_id":4}`)}
	raw, err := handlers[TaskGet].Invoke(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if fake.lastWorkspace != 7 || fake.lastTask.Reference != "PM-44" || fake.lastTask.PlanUnitID != 3 {
		t.Fatalf("workspace=%d lookup=%+v", fake.lastWorkspace, fake.lastTask)
	}
	var output task.Snapshot
	if err := json.Unmarshal(raw, &output); err != nil || output.WorkspaceID != 7 || output.Children == nil {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}

func TestReadHandlersRejectMissingServerWorkspace(t *testing.T) {
	fake := &readQueriesFake{}
	handlers, err := NewReadHandlers(ReadDependencies{Projects: fake, Requirements: fake, Tasks: fake, Workflows: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = handlers[ProjectSetGet].Invoke(context.Background(), toolruntime.ToolCall{Input: json.RawMessage(`{"id":1}`)})
	if !errors.Is(err, ErrReadHandlersInvalid) || fake.lastWorkspace != 0 {
		t.Fatalf("err=%v workspace=%d", err, fake.lastWorkspace)
	}
}

func TestWorkflowListOutputHasStableObjectEnvelope(t *testing.T) {
	fake := &readQueriesFake{}
	handlers, err := NewReadHandlers(ReadDependencies{Projects: fake, Requirements: fake, Tasks: fake, Workflows: fake})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handlers[WorkflowTemplateList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"subject_type":"requirement"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Items []workflow.TemplateSnapshot `json:"items"`
		Total int                         `json:"total"`
	}
	if err := json.Unmarshal(raw, &output); err != nil || output.Total != 1 || len(output.Items) != 1 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}

func TestActiveReadSliceInvokesThroughCanonicalApplication(t *testing.T) {
	fake := &readQueriesFake{}
	handlers, err := NewReadHandlers(ReadDependencies{Projects: fake, Requirements: fake, Tasks: fake, Workflows: fake})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: ReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, readAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 1, WorkspaceID: 7, GrantID: 2, ClientID: "test-client",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	inputs := map[string]json.RawMessage{
		ProjectSetGet:         json.RawMessage(`{"id":1}`),
		ProjectPlanUnitGet:    json.RawMessage(`{"id":2}`),
		ProjectRequirementGet: json.RawMessage(`{"id":3}`),
		TaskGet:               json.RawMessage(`{"id":44}`),
		WorkflowTemplateList:  json.RawMessage(`{"subject_type":"requirement"}`),
		WorkflowTemplateGet:   json.RawMessage(`{"id":5}`),
		WorkflowInstanceList:  json.RawMessage(`{"subject_type":"requirement","subject_id":99}`),
		WorkflowInstanceGet:   json.RawMessage(`{"id":6}`),
	}
	for _, name := range ReadHandlerNames() {
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
}
