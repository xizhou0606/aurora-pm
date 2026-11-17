package pmtools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

const (
	WorkflowOpinionList            = "workflow.opinion.list"
	WorkflowTestCaseArtifactList   = "workflow.test_case_artifact.list"
	WorkflowTestReportArtifactList = "workflow.test_report_artifact.list"
)

var workflowAuxReadHandlerNames = []string{
	WorkflowOpinionList,
	WorkflowTestCaseArtifactList,
	WorkflowTestReportArtifactList,
}

type WorkflowAuxQueries interface {
	ResolveNode(context.Context, uint, uint, string, workflow.AuxPurpose) (workflow.AuxNodeContext, error)
	ListOpinions(context.Context, uint, uint, string) (workflow.AuxNodeContext, []workflow.NodeOpinion, error)
}

type WorkflowMaterialQueries interface {
	List(uint, uint, string) ([]requirement.Material, error)
}

type WorkflowTestReportQueries interface {
	List(uint, quality.TestReportFilter) ([]quality.TestReport, error)
}

type WorkflowAuxReadDependencies struct {
	Projects     ProjectQueries
	Requirements RequirementQueries
	Workflow     WorkflowAuxQueries
	Materials    WorkflowMaterialQueries
	Reports      WorkflowTestReportQueries
}

type workflowAuxReadInput struct {
	ID     uint   `json:"id"`
	NodeID string `json:"nodeId"`
}

type workflowAuxListOutput[T any] struct {
	InstanceID  uint   `json:"instance_id"`
	NodeID      string `json:"node_id"`
	SubjectType string `json:"subject_type"`
	SubjectID   uint   `json:"subject_id"`
	Items       []T    `json:"items"`
	Total       int    `json:"total"`
}

type workflowTestCaseArtifact struct {
	ID             uint      `json:"id"`
	RequirementID  uint      `json:"requirement_id"`
	InstanceID     uint      `json:"instance_id"`
	NodeID         string    `json:"node_id"`
	Kind           string    `json:"kind"`
	Source         string    `json:"source"`
	Name           string    `json:"name"`
	ContentType    string    `json:"content_type"`
	Size           int64     `json:"size"`
	URL            string    `json:"url"`
	OriginURL      string    `json:"origin_url"`
	AIRunID        *uint     `json:"ai_run_id"`
	UploaderUserID uint      `json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type workflowTestReportArtifact struct {
	ID             uint      `json:"id"`
	PlanUnitID     uint      `json:"plan_unit_id"`
	ProjectSetID   uint      `json:"project_new_id"`
	InstanceID     uint      `json:"instance_id"`
	NodeID         string    `json:"node_id"`
	Kind           string    `json:"kind"`
	Source         string    `json:"source"`
	Name           string    `json:"name"`
	ContentType    string    `json:"content_type"`
	Size           int64     `json:"size"`
	URL            string    `json:"url"`
	UploaderUserID uint      `json:"uploader_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func WorkflowAuxReadHandlerNames() []string {
	return append([]string(nil), workflowAuxReadHandlerNames...)
}

func NewWorkflowAuxReadHandlers(dependencies WorkflowAuxReadDependencies) (map[string]toolruntime.ToolHandler, error) {
	if dependencies.Projects == nil || dependencies.Requirements == nil || dependencies.Workflow == nil ||
		dependencies.Materials == nil || dependencies.Reports == nil {
		return nil, ErrReadHandlersInvalid
	}
	return map[string]toolruntime.ToolHandler{
		WorkflowOpinionList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowAuxInput(call.Input)
			if err != nil {
				return nil, err
			}
			resolved, items, err := dependencies.Workflow.ListOpinions(
				ctx, call.Authorization.WorkspaceID, input.ID, input.NodeID,
			)
			if err != nil {
				return nil, err
			}
			if err := validateWorkflowAuxSubject(ctx, call.Authorization.WorkspaceID, resolved, dependencies); err != nil {
				return nil, err
			}
			return workflowAuxOutput(resolved, items), nil
		}),
		WorkflowTestCaseArtifactList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowAuxInput(call.Input)
			if err != nil {
				return nil, err
			}
			resolved, err := dependencies.Workflow.ResolveNode(
				ctx, call.Authorization.WorkspaceID, input.ID, input.NodeID, workflow.AuxPurposeTestCase,
			)
			if err != nil {
				return nil, err
			}
			if resolved.SubjectType != "requirement" {
				return nil, workflow.ErrWorkflowNodeUnsupported
			}
			if err := validateWorkflowAuxSubject(ctx, call.Authorization.WorkspaceID, resolved, dependencies); err != nil {
				return nil, err
			}
			rows, err := dependencies.Materials.List(call.Authorization.WorkspaceID, resolved.SubjectID, requirement.MaterialKindTestCase)
			if err != nil {
				return nil, err
			}
			items := make([]workflowTestCaseArtifact, 0, len(rows))
			for _, row := range rows {
				if !validWorkflowTestCaseMaterial(row, call.Authorization.WorkspaceID, resolved.SubjectID) {
					return nil, workflow.ErrWorkflowAuxDataInvalid
				}
				items = append(items, workflowTestCaseArtifactFrom(row, resolved))
			}
			return workflowAuxOutput(resolved, items), nil
		}),
		WorkflowTestReportArtifactList: handler(func(ctx context.Context, call toolruntime.ToolCall) (any, error) {
			input, err := workflowAuxInput(call.Input)
			if err != nil {
				return nil, err
			}
			resolved, err := dependencies.Workflow.ResolveNode(
				ctx, call.Authorization.WorkspaceID, input.ID, input.NodeID, workflow.AuxPurposeTestReport,
			)
			if err != nil {
				return nil, err
			}
			planUnitID, err := workflowAuxPlanUnit(ctx, call.Authorization.WorkspaceID, resolved, dependencies)
			if err != nil {
				return nil, err
			}
			if planUnitID == 0 {
				return workflowAuxOutput(resolved, []workflowTestReportArtifact{}), nil
			}
			rows, err := dependencies.Reports.List(call.Authorization.WorkspaceID, quality.TestReportFilter{PlanUnitID: planUnitID})
			if err != nil {
				return nil, err
			}
			items := make([]workflowTestReportArtifact, 0, len(rows))
			for _, row := range rows {
				if !validWorkflowTestReport(row, call.Authorization.WorkspaceID, planUnitID) {
					return nil, workflow.ErrWorkflowAuxDataInvalid
				}
				items = append(items, workflowTestReportArtifactFrom(row, resolved))
			}
			return workflowAuxOutput(resolved, items), nil
		}),
	}, nil
}

func workflowAuxInput(raw json.RawMessage) (workflowAuxReadInput, error) {
	var input workflowAuxReadInput
	if err := decodeInput(raw, &input); err != nil || input.ID == 0 || strings.TrimSpace(input.NodeID) == "" {
		return workflowAuxReadInput{}, fmt.Errorf("%w: workflow instance id and nodeId are required", ErrReadHandlersInvalid)
	}
	input.NodeID = strings.TrimSpace(input.NodeID)
	return input, nil
}

func workflowAuxOutput[T any](resolved workflow.AuxNodeContext, items []T) workflowAuxListOutput[T] {
	if items == nil {
		items = []T{}
	}
	return workflowAuxListOutput[T]{
		InstanceID: resolved.InstanceID, NodeID: resolved.NodeID,
		SubjectType: resolved.SubjectType, SubjectID: resolved.SubjectID,
		Items: items, Total: len(items),
	}
}

func validateWorkflowAuxSubject(
	ctx context.Context,
	workspaceID uint,
	resolved workflow.AuxNodeContext,
	dependencies WorkflowAuxReadDependencies,
) error {
	switch resolved.SubjectType {
	case "requirement":
		item, err := dependencies.Requirements.GetRequirement(ctx, workspaceID, resolved.SubjectID)
		if err != nil {
			return err
		}
		if item.ID != resolved.SubjectID || item.WorkspaceID != workspaceID {
			return workflow.ErrWorkflowAuxDataInvalid
		}
		return nil
	case "plan_unit":
		item, err := dependencies.Projects.GetPlanUnit(ctx, workspaceID, resolved.SubjectID)
		if err != nil {
			return err
		}
		if item.ID != resolved.SubjectID || item.WorkspaceID != workspaceID {
			return workflow.ErrWorkflowAuxDataInvalid
		}
		return nil
	default:
		return workflow.ErrAuxReadInvalid
	}
}

func workflowAuxPlanUnit(
	ctx context.Context,
	workspaceID uint,
	resolved workflow.AuxNodeContext,
	dependencies WorkflowAuxReadDependencies,
) (uint, error) {
	switch resolved.SubjectType {
	case "plan_unit":
		unit, err := dependencies.Projects.GetPlanUnit(ctx, workspaceID, resolved.SubjectID)
		if err == nil && (unit.ID != resolved.SubjectID || unit.WorkspaceID != workspaceID) {
			return 0, workflow.ErrWorkflowAuxDataInvalid
		}
		return unit.ID, err
	case "requirement":
		item, err := dependencies.Requirements.GetRequirement(ctx, workspaceID, resolved.SubjectID)
		if err != nil {
			return 0, err
		}
		if item.ID != resolved.SubjectID || item.WorkspaceID != workspaceID {
			return 0, workflow.ErrWorkflowAuxDataInvalid
		}
		if item.PlanUnitID == nil || *item.PlanUnitID == 0 {
			return 0, nil
		}
		unit, err := dependencies.Projects.GetPlanUnit(ctx, workspaceID, *item.PlanUnitID)
		if err == nil && (unit.ID != *item.PlanUnitID || unit.WorkspaceID != workspaceID || unit.ProjectSetID != item.ProjectSetID) {
			return 0, workflow.ErrWorkflowAuxDataInvalid
		}
		return unit.ID, err
	default:
		return 0, workflow.ErrAuxReadInvalid
	}
}

func workflowTestCaseArtifactFrom(row requirement.Material, resolved workflow.AuxNodeContext) workflowTestCaseArtifact {
	kind := "uploaded"
	if row.Source == requirement.MaterialSourceAIGenerated {
		kind = "ai_generated"
	}
	url := row.URL
	if row.ObjectKey != "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(row.URL)), "http") {
		url = fmt.Sprintf("/api/v1/platform/requirements/%d/materials/%d/download", row.RequirementID, row.ID)
	}
	return workflowTestCaseArtifact{
		ID: row.ID, RequirementID: row.RequirementID, InstanceID: resolved.InstanceID, NodeID: resolved.NodeID,
		Kind: kind, Source: row.Source, Name: row.Name, ContentType: row.ContentType, Size: row.Size,
		URL: url, OriginURL: row.URL, AIRunID: row.AIRunID, UploaderUserID: row.UploaderUserID, CreatedAt: row.CreatedAt,
	}
}

func workflowTestReportArtifactFrom(row quality.TestReport, resolved workflow.AuxNodeContext) workflowTestReportArtifact {
	source := "file"
	url := fmt.Sprintf("/api/v1/platform/pm/testing/reports/%d/download", row.ID)
	if row.Kind == quality.TestReportKindLink {
		source, url = "link", row.URL
	}
	return workflowTestReportArtifact{
		ID: row.ID, PlanUnitID: row.PlanUnitID, ProjectSetID: row.ProjectSetID,
		InstanceID: resolved.InstanceID, NodeID: resolved.NodeID, Kind: "test_report", Source: source,
		Name: row.Name, ContentType: row.ContentType, Size: row.Size, URL: url,
		UploaderUserID: row.UploaderUserID, CreatedAt: row.CreatedAt,
	}
}

func validWorkflowTestCaseMaterial(row requirement.Material, workspaceID, requirementID uint) bool {
	if row.ID == 0 || row.WorkspaceID != workspaceID || row.RequirementID != requirementID ||
		row.Kind != requirement.MaterialKindTestCase || strings.TrimSpace(row.Name) == "" || row.Size < 0 ||
		row.UploaderUserID == 0 || row.CreatedAt.IsZero() || (strings.TrimSpace(row.ObjectKey) == "" && strings.TrimSpace(row.URL) == "") {
		return false
	}
	switch row.Source {
	case requirement.MaterialSourceRequirement, requirement.MaterialSourceWorkflow,
		requirement.MaterialSourceTestingLibrary, requirement.MaterialSourceAIGenerated:
		return true
	default:
		return false
	}
}

func validWorkflowTestReport(row quality.TestReport, workspaceID, planUnitID uint) bool {
	if row.ID == 0 || row.WorkspaceID != workspaceID || row.PlanUnitID != planUnitID || row.ProjectSetID == 0 ||
		strings.TrimSpace(row.Name) == "" || row.Size < 0 || row.UploaderUserID == 0 || row.CreatedAt.IsZero() {
		return false
	}
	switch row.Kind {
	case quality.TestReportKindFile:
		return strings.TrimSpace(row.ObjectKey) != ""
	case quality.TestReportKindLink:
		return quality.ValidReportURL(row.URL)
	default:
		return false
	}
}

var _ WorkflowAuxQueries = (*workflow.AuxReadService)(nil)
var _ WorkflowMaterialQueries = (*requirement.MaterialService)(nil)
var _ WorkflowTestReportQueries = (*quality.TestReportService)(nil)
