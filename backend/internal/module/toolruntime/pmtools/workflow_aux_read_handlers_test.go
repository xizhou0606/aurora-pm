package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

type workflowAuxHandlerFake struct {
	workspace     uint
	purpose       workflow.AuxPurpose
	parentHit     bool
	corruptParent bool
}

func (fake *workflowAuxHandlerFake) GetProjectSet(context.Context, uint, uint) (project.ProjectSetSnapshot, error) {
	return project.ProjectSetSnapshot{}, nil
}

func (fake *workflowAuxHandlerFake) GetPlanUnit(_ context.Context, workspaceID, id uint) (project.PlanUnitSnapshot, error) {
	fake.workspace, fake.parentHit = workspaceID, true
	if fake.corruptParent {
		workspaceID++
	}
	return project.PlanUnitSnapshot{ID: id, WorkspaceID: workspaceID, ProjectSetID: 11, Title: "v1", Status: "active"}, nil
}

func (fake *workflowAuxHandlerFake) GetRequirement(_ context.Context, workspaceID, id uint) (requirement.Snapshot, error) {
	fake.workspace, fake.parentHit = workspaceID, true
	if fake.corruptParent {
		workspaceID++
	}
	unitID := uint(42)
	return requirement.Snapshot{ID: id, WorkspaceID: workspaceID, ProjectSetID: 11, PlanUnitID: &unitID, Title: "需求", Priority: "normal", Status: "backlog"}, nil
}

func (fake *workflowAuxHandlerFake) ResolveNode(_ context.Context, workspaceID, instanceID uint, nodeID string, purpose workflow.AuxPurpose) (workflow.AuxNodeContext, error) {
	fake.workspace, fake.purpose = workspaceID, purpose
	subjectType, subjectID := "requirement", uint(41)
	if purpose == workflow.AuxPurposeTestReport {
		subjectType, subjectID = "plan_unit", 42
	}
	return workflow.AuxNodeContext{
		InstanceID: instanceID, WorkspaceID: workspaceID, SubjectType: subjectType, SubjectID: subjectID,
		NodeID: nodeID, NodeType: "human_task", NodeName: "辅助节点",
	}, nil
}

func (fake *workflowAuxHandlerFake) ListOpinions(_ context.Context, workspaceID, instanceID uint, nodeID string) (workflow.AuxNodeContext, []workflow.NodeOpinion, error) {
	fake.workspace = workspaceID
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	resolved := workflow.AuxNodeContext{
		InstanceID: instanceID, WorkspaceID: workspaceID, SubjectType: "requirement", SubjectID: 41,
		NodeID: nodeID, NodeType: "approval", NodeName: "需求预审",
	}
	return resolved, []workflow.NodeOpinion{{ID: 51, WorkspaceID: workspaceID, InstanceID: instanceID, NodeID: nodeID, AuthorUserID: 7, Body: "通过", CreatedAt: now, UpdatedAt: now}}, nil
}

type workflowMaterialFake struct{ workspace uint }

func (fake *workflowMaterialFake) List(workspaceID, requirementID uint, kind string) ([]requirement.Material, error) {
	fake.workspace = workspaceID
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	return []requirement.Material{{
		ID: 61, WorkspaceID: workspaceID, RequirementID: requirementID, Kind: kind,
		Source: requirement.MaterialSourceWorkflow, Name: "cases.csv", ContentType: "text/csv", Size: 10,
		ObjectKey: "object/cases.csv", URL: "https://objects.example/cases.csv", UploaderUserID: 7, CreatedAt: now,
	}}, nil
}

type workflowReportFake struct{ workspace uint }

func (fake *workflowReportFake) List(workspaceID uint, filter quality.TestReportFilter) ([]quality.TestReport, error) {
	fake.workspace = workspaceID
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	return []quality.TestReport{{
		ID: 71, WorkspaceID: workspaceID, PlanUnitID: filter.PlanUnitID, ProjectSetID: 11,
		Kind: quality.TestReportKindLink, Name: "报告", ContentType: "text/uri-list", URL: "https://qa.example/report",
		UploaderUserID: 7, CreatedAt: now,
	}}, nil
}

type workflowAuxAuthorizer struct{}

func (workflowAuxAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant(WorkflowAuxReadHandlerNames(), []string{"workflow:read"})
}

func TestWorkflowAuxReadHandlersInvokeAllThreeThroughCanonicalApplication(t *testing.T) {
	workflowFake := &workflowAuxHandlerFake{}
	materials := &workflowMaterialFake{}
	reports := &workflowReportFake{}
	handlers, err := NewWorkflowAuxReadHandlers(WorkflowAuxReadDependencies{
		Projects: workflowFake, Requirements: workflowFake, Workflow: workflowFake, Materials: materials, Reports: reports,
	})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: WorkflowAuxReadHandlerNames()})
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, workflowAuxAuthorizer{}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 3, WorkspaceID: 7, GrantID: 2, ClientID: "workflow-test",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	inputs := map[string]json.RawMessage{
		WorkflowOpinionList:            json.RawMessage(`{"id":31,"nodeId":"pre-review"}`),
		WorkflowTestCaseArtifactList:   json.RawMessage(`{"id":31,"nodeId":"cases"}`),
		WorkflowTestReportArtifactList: json.RawMessage(`{"id":31,"nodeId":"report"}`),
	}
	for _, name := range WorkflowAuxReadHandlerNames() {
		result, invokeErr := application.Invoke(context.Background(), toolruntime.Invocation{Name: name, Input: inputs[name], Authorization: authorization})
		if invokeErr != nil {
			t.Fatalf("invoke %s: %v", name, invokeErr)
		}
		if result.Name != name || len(result.Output) == 0 {
			t.Fatalf("result %s=%+v", name, result)
		}
	}
	if workflowFake.workspace != 7 || materials.workspace != 7 || reports.workspace != 7 || !workflowFake.parentHit {
		t.Fatalf("workflow=%d materials=%d reports=%d parent=%v", workflowFake.workspace, materials.workspace, reports.workspace, workflowFake.parentHit)
	}
}

func TestWorkflowAuxReadHandlersRejectUnknownInputAndMissingWorkspace(t *testing.T) {
	workflowFake := &workflowAuxHandlerFake{}
	handlers, _ := NewWorkflowAuxReadHandlers(WorkflowAuxReadDependencies{
		Projects: workflowFake, Requirements: workflowFake, Workflow: workflowFake,
		Materials: &workflowMaterialFake{}, Reports: &workflowReportFake{},
	})
	if _, err := handlers[WorkflowOpinionList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"id":31,"nodeId":"review","unknown":true}`),
	}); err == nil {
		t.Fatal("expected unknown input rejection")
	}
	if _, err := handlers[WorkflowOpinionList].Invoke(context.Background(), toolruntime.ToolCall{Input: json.RawMessage(`{"id":31,"nodeId":"review"}`)}); !errors.Is(err, ErrReadHandlersInvalid) {
		t.Fatalf("missing workspace err=%v", err)
	}
}

func TestWorkflowAuxReadHandlersFailClosedForMismatchedSubjectParent(t *testing.T) {
	workflowFake := &workflowAuxHandlerFake{corruptParent: true}
	handlers, _ := NewWorkflowAuxReadHandlers(WorkflowAuxReadDependencies{
		Projects: workflowFake, Requirements: workflowFake, Workflow: workflowFake,
		Materials: &workflowMaterialFake{}, Reports: &workflowReportFake{},
	})
	_, err := handlers[WorkflowOpinionList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":31,"nodeId":"pre-review"}`),
	})
	if !errors.Is(err, workflow.ErrWorkflowAuxDataInvalid) {
		t.Fatalf("err=%v", err)
	}
}
