package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type biMetricDefinition struct {
	Nodes []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"nodes"`
}

type BIReadRepository struct{ db *gorm.DB }

func NewBIReadRepository(db *gorm.DB) *BIReadRepository { return &BIReadRepository{db: db} }

func (repository *BIReadRepository) ListBIReviewDecisions(ctx context.Context, workspaceID, templateID uint, nodeID string, requirementIDs []uint) ([]workflow.BIReviewDecision, error) {
	rows := []workflow.BIReviewDecision{}
	err := repository.db.WithContext(ctx).Table("workflow_approvals decisions").
		Select("decisions.id, instances.subject_id AS requirement_id, decisions.status, decisions.decided_at").
		Joins("JOIN workflow_node_runs runs ON runs.id = decisions.node_run_id AND runs.workspace_id = decisions.workspace_id").
		Joins("JOIN workflow_instances instances ON instances.id = decisions.instance_id AND instances.workspace_id = decisions.workspace_id").
		Where(`decisions.workspace_id = ? AND instances.subject_type = ? AND instances.template_id = ?
			AND instances.subject_id IN ? AND runs.node_id = ? AND runs.node_type = ?
			AND decisions.status IN ? AND decisions.decided_at IS NOT NULL`, workspaceID, "requirement", templateID,
			requirementIDs, nodeID, "approval", []string{"completed", "rejected"}).
		Order("decisions.decided_at ASC, decisions.id ASC").Scan(&rows).Error
	return rows, err
}

func (repository *BIReadRepository) ListBIMetricWorkflowOptions(ctx context.Context, workspaceID uint) ([]workflow.BIMetricWorkflowOption, error) {
	var templates []WorkflowTemplate
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND subject_type = ? AND published_version > ?", workspaceID, "requirement", 0).
		Order("name ASC, id ASC").Find(&templates).Error; err != nil {
		return nil, err
	}
	options := make([]workflow.BIMetricWorkflowOption, 0, len(templates))
	for _, template := range templates {
		var version WorkflowTemplateVersion
		if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND template_id = ? AND version = ?", workspaceID, template.ID, template.PublishedVersion).
			First(&version).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		var definition biMetricDefinition
		if json.Unmarshal([]byte(version.Definition), &definition) != nil {
			continue
		}
		nodes := []workflow.BIMetricApprovalNode{}
		for _, node := range definition.Nodes {
			if node.Type == "approval" && node.ID != "" && node.Name != "" {
				nodes = append(nodes, workflow.BIMetricApprovalNode{ID: node.ID, Name: node.Name})
			}
		}
		if len(nodes) > 0 {
			options = append(options, workflow.BIMetricWorkflowOption{TemplateID: template.ID, TemplateName: template.Name,
				PublishedVersion: version.Version, ApprovalNodes: nodes})
		}
	}
	return options, nil
}

var _ workflow.BIReadStore = (*BIReadRepository)(nil)
