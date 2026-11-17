package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type AuxReadRepository struct{ db *gorm.DB }

func NewAuxReadRepository(db *gorm.DB) *AuxReadRepository { return &AuxReadRepository{db: db} }

func (repository *AuxReadRepository) ResolveAuxNode(
	ctx context.Context,
	workspaceID, instanceID uint,
	nodeID string,
	purpose workflow.AuxPurpose,
) (workflow.AuxNodeContext, error) {
	if repository == nil || repository.db == nil {
		return workflow.AuxNodeContext{}, workflow.ErrAuxReadInvalid
	}
	var instance WorkflowInstance
	err := repository.db.WithContext(ctx).Where("id = ? AND workspace_id = ?", instanceID, workspaceID).First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.AuxNodeContext{}, workflow.ErrWorkflowInstanceNotFound
	}
	if err != nil {
		return workflow.AuxNodeContext{}, err
	}
	if instance.SubjectID == 0 || (instance.SubjectType != "requirement" && instance.SubjectType != "plan_unit") {
		return workflow.AuxNodeContext{}, workflow.ErrWorkflowAuxDataInvalid
	}
	var version WorkflowTemplateVersion
	err = repository.db.WithContext(ctx).Where(
		"id = ? AND workspace_id = ?", instance.TemplateVersionID, instance.WorkspaceID,
	).First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.AuxNodeContext{}, workflow.ErrWorkflowAuxDataInvalid
	}
	if err != nil {
		return workflow.AuxNodeContext{}, err
	}
	node, err := resolveAuxDefinitionNode(version.Definition, strings.TrimSpace(nodeID))
	if err != nil {
		return workflow.AuxNodeContext{}, err
	}
	if !auxNodeSupports(node, purpose) {
		return workflow.AuxNodeContext{}, workflow.ErrWorkflowNodeUnsupported
	}
	return workflow.AuxNodeContext{
		InstanceID: instance.ID, WorkspaceID: instance.WorkspaceID,
		SubjectType: instance.SubjectType, SubjectID: instance.SubjectID,
		NodeID: node.ID, NodeType: node.Type, NodeName: node.Name,
	}, nil
}

func (repository *AuxReadRepository) ListNodeOpinions(
	ctx context.Context,
	workspaceID, instanceID uint,
	nodeID string,
) ([]workflow.NodeOpinion, error) {
	var rows []WorkflowNodeOpinion
	err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND instance_id = ? AND node_id = ?", workspaceID, instanceID, nodeID,
	).Order("id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]workflow.NodeOpinion, 0, len(rows))
	for _, row := range rows {
		if row.ID == 0 || row.WorkspaceID == 0 || row.InstanceID == 0 || strings.TrimSpace(row.NodeID) == "" ||
			row.AuthorUserID == 0 || strings.TrimSpace(row.Body) == "" || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
			return nil, workflow.ErrWorkflowAuxDataInvalid
		}
		items = append(items, workflow.NodeOpinion{
			ID: row.ID, WorkspaceID: row.WorkspaceID, InstanceID: row.InstanceID, NodeID: row.NodeID,
			AuthorUserID: row.AuthorUserID, Body: row.Body, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			Edited: row.UpdatedAt.After(row.CreatedAt),
		})
	}
	return items, nil
}

type auxDefinition struct {
	Nodes []auxDefinitionNode `json:"nodes"`
}

type auxDefinitionNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

func resolveAuxDefinitionNode(raw, nodeID string) (auxDefinitionNode, error) {
	var definition auxDefinition
	if json.Unmarshal([]byte(raw), &definition) != nil || len(definition.Nodes) == 0 {
		return auxDefinitionNode{}, workflow.ErrWorkflowAuxDataInvalid
	}
	seen := make(map[string]struct{}, len(definition.Nodes))
	var result auxDefinitionNode
	for _, node := range definition.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.Type = strings.TrimSpace(node.Type)
		node.Name = strings.TrimSpace(node.Name)
		if node.ID == "" || node.Type == "" || node.Name == "" {
			return auxDefinitionNode{}, workflow.ErrWorkflowAuxDataInvalid
		}
		if _, exists := seen[node.ID]; exists {
			return auxDefinitionNode{}, workflow.ErrWorkflowAuxDataInvalid
		}
		seen[node.ID] = struct{}{}
		if node.ID == nodeID {
			result = node
		}
	}
	if result.ID == "" {
		return auxDefinitionNode{}, workflow.ErrWorkflowNodeNotFound
	}
	return result, nil
}

func auxNodeSupports(node auxDefinitionNode, purpose workflow.AuxPurpose) bool {
	switch purpose {
	case workflow.AuxPurposeOpinion:
		if node.Type != "approval" {
			return false
		}
		marker := strings.ToLower(node.ID + " " + node.Name)
		if strings.Contains(node.Name, "预审") {
			return true
		}
		for _, token := range []string{"pre-review", "pre_review", "prereview", "preapproval", "pre-approval", "pre_approval"} {
			if strings.Contains(marker, token) {
				return true
			}
		}
		return false
	case workflow.AuxPurposeTestCase:
		configured, _ := node.Config["test_case_artifact"].(bool)
		return (node.Type == "human_task" || node.Type == "form") &&
			(configured || strings.Contains(node.Name, "测试用例") || strings.Contains(node.Name, "测试案例"))
	case workflow.AuxPurposeTestReport:
		configured, _ := node.Config["test_report_artifact"].(bool)
		return (node.Type == "human_task" || node.Type == "form") &&
			(configured || strings.Contains(node.Name, "测试验收") || strings.Contains(node.Name, "测试报告"))
	default:
		return false
	}
}

var _ workflow.AuxReadStore = (*AuxReadRepository)(nil)
