package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type ReadRepository struct{ db *gorm.DB }

func NewReadRepository(db *gorm.DB) *ReadRepository { return &ReadRepository{db: db} }

func (repository *ReadRepository) ListTemplates(ctx context.Context, workspaceID uint, filter workflow.TemplateFilter) ([]workflow.TemplateSnapshot, error) {
	query := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if filter.SubjectType != "" {
		query = query.Where("subject_type = ?", filter.SubjectType)
	}
	var rows []WorkflowTemplate
	if err := query.Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]workflow.TemplateSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, templateSnapshot(row, false))
	}
	return items, nil
}

func (repository *ReadRepository) GetTemplate(ctx context.Context, workspaceID, id uint) (workflow.TemplateSnapshot, error) {
	var row WorkflowTemplate
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.TemplateSnapshot{}, workflow.ErrWorkflowTemplateNotFound
	}
	if err != nil {
		return workflow.TemplateSnapshot{}, err
	}
	definition, err := validObjectJSON(row.DraftDefinition, nil)
	if err != nil {
		return workflow.TemplateSnapshot{}, err
	}
	item := templateSnapshot(row, false)
	item.Definition = definition
	return item, nil
}

func (repository *ReadRepository) ListInstances(ctx context.Context, workspaceID uint, filter workflow.InstanceFilter) ([]workflow.InstanceSnapshot, error) {
	var rows []WorkflowInstance
	err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, filter.SubjectType, filter.SubjectID,
	).Order("id DESC").Limit(20).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]workflow.InstanceSnapshot, 0, len(rows))
	for _, row := range rows {
		item, buildErr := repository.instanceSnapshot(ctx, row, false)
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *ReadRepository) GetInstance(ctx context.Context, workspaceID, id uint) (workflow.InstanceSnapshot, error) {
	var row WorkflowInstance
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.InstanceSnapshot{}, workflow.ErrWorkflowInstanceNotFound
	}
	if err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	return repository.instanceSnapshot(ctx, row, true)
}

func (repository *ReadRepository) instanceSnapshot(ctx context.Context, row WorkflowInstance, details bool) (workflow.InstanceSnapshot, error) {
	item := workflow.InstanceSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TemplateID: row.TemplateID,
		TemplateVersionID: row.TemplateVersionID, SubjectType: row.SubjectType, SubjectID: row.SubjectID,
		Status: row.Status, CanAct: false, AllowedActions: []string{},
		StartedByUserID: row.StartedByUserID, StartedAt: row.StartedAt,
		CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt,
	}
	if err := json.Unmarshal([]byte(row.CurrentNodeIDs), &item.CurrentNodeIDs); err != nil {
		return workflow.InstanceSnapshot{}, workflow.ErrWorkflowDefinitionInvalid
	}
	if item.CurrentNodeIDs == nil {
		item.CurrentNodeIDs = []string{}
	}
	var err error
	item.NodePlans, err = validObjectJSON(row.NodePlansJSON, json.RawMessage(`{}`))
	if err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	var template WorkflowTemplate
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", row.WorkspaceID, row.TemplateID).First(&template).Error; err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	item.TemplateName = template.Name
	var version WorkflowTemplateVersion
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", row.WorkspaceID, row.TemplateVersionID).First(&version).Error; err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	item.TemplateVersion = version.Version
	item.PublishedDefinition, err = validObjectJSON(version.Definition, nil)
	if err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	if !details {
		return item, nil
	}
	var runs []WorkflowNodeRun
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND instance_id = ?", row.WorkspaceID, row.ID).Order("id").Find(&runs).Error; err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	item.NodeRuns = make([]workflow.NodeRun, 0, len(runs))
	for _, run := range runs {
		item.NodeRuns = append(item.NodeRuns, workflow.NodeRun{ID: run.ID, NodeID: run.NodeID, NodeType: run.NodeType, Status: run.Status, ErrorMessage: run.ErrorMessage, StartedAt: run.StartedAt, CompletedAt: run.CompletedAt})
	}
	var approvals []WorkflowApproval
	if err := repository.db.WithContext(ctx).Where("workspace_id = ? AND instance_id = ?", row.WorkspaceID, row.ID).Order("id").Find(&approvals).Error; err != nil {
		return workflow.InstanceSnapshot{}, err
	}
	item.Approvals = make([]workflow.Approval, 0, len(approvals))
	for _, approval := range approvals {
		item.Approvals = append(item.Approvals, workflow.Approval{ID: approval.ID, NodeRunID: approval.NodeRunID, Status: approval.Status, ApproverUserID: approval.ApproverUserID, Comment: approval.Comment, DecidedAt: approval.DecidedAt, CreatedAt: approval.CreatedAt})
	}
	return item, nil
}

func templateSnapshot(row WorkflowTemplate, includeDefinition bool) workflow.TemplateSnapshot {
	item := workflow.TemplateSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Name: row.Name, SubjectType: row.SubjectType,
		Description: row.Description, PublishedVersion: row.PublishedVersion,
		CreatedByUserID: row.CreatedByUserID, UpdatedByUserID: row.UpdatedByUserID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if includeDefinition {
		item.Definition = json.RawMessage(row.DraftDefinition)
	}
	return item
}

func validObjectJSON(value string, fallback json.RawMessage) (json.RawMessage, error) {
	if value == "" && fallback != nil {
		return append(json.RawMessage(nil), fallback...), nil
	}
	var object map[string]any
	if json.Unmarshal([]byte(value), &object) != nil || object == nil {
		return nil, workflow.ErrWorkflowDefinitionInvalid
	}
	return json.RawMessage(value), nil
}
