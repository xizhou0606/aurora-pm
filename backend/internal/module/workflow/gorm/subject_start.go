package workflowgorm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type SubjectStartDependencies struct {
	AI          workflow.SubjectAIStarter
	Assignments workflow.SubjectAssignmentWriter
	Effects     workflow.ActionEffectWriter
}

type SubjectStartDependencyFactory func(*gorm.DB) (SubjectStartDependencies, error)

type SubjectStartRepository struct {
	db           *gorm.DB
	dependencies SubjectStartDependencyFactory
}

func NewSubjectStartRepository(db *gorm.DB, dependencies SubjectStartDependencyFactory) (*SubjectStartRepository, error) {
	if db == nil || dependencies == nil {
		return nil, workflow.ErrSubjectStartInvalid
	}
	return &SubjectStartRepository{db: db, dependencies: dependencies}, nil
}

func (repository *SubjectStartRepository) StartSubject(
	ctx context.Context,
	command workflow.SubjectStartCommand,
) (workflow.SubjectStartResult, error) {
	if replay, found, err := repository.replaySubjectStart(ctx, command); found || err != nil {
		return replay, err
	}
	var result workflow.SubjectStartResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ledger := ToolMutationApply{WorkspaceID: command.WorkspaceID, ApplyKey: command.ApplyKey,
			Operation: "subject.start", InputDigest: command.InputDigest, ResultJSON: ""}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		dependencies, err := repository.dependencies(tx)
		if err != nil || dependencies.AI == nil || dependencies.Assignments == nil || dependencies.Effects == nil {
			return workflow.ErrMutationUnavailable
		}
		template, version, definition, err := loadSubjectStartDefinition(tx, command)
		if err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&WorkflowInstance{}).Where(
			"workspace_id = ? AND subject_type = ? AND subject_id = ?",
			command.WorkspaceID, command.SubjectType, command.SubjectID,
		).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return workflow.ErrMutationConflict
		}
		now := time.Now().UTC()
		instance := WorkflowInstance{
			WorkspaceID: command.WorkspaceID, TemplateID: template.ID, TemplateVersionID: version.ID,
			SubjectType: command.SubjectType, SubjectID: command.SubjectID, Status: "running",
			CurrentNodeIDs: "[]", NodePlansJSON: "{}", StartedByUserID: command.PrincipalID, StartedAt: now,
		}
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		startID := subjectStartNodeID(definition)
		assignees := make(map[string]uint, len(definition.Nodes))
		for _, node := range definition.Nodes {
			if workflow.NeedsAssignee(node) {
				assignees[node.ID] = command.PrincipalID
			}
		}
		current, err := advanceAutomaticMutationNodes(ctx, tx, nil, instance, definition, []string{startID}, assignees)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			instance.Status, instance.CompletedAt = "completed", &now
		}
		encoded, _ := json.Marshal(current)
		updated := tx.Model(&WorkflowInstance{}).Where("id = ? AND workspace_id = ? AND current_node_ids = ?", instance.ID, instance.WorkspaceID, "[]").
			Updates(map[string]any{"current_node_ids": string(encoded), "status": instance.Status, "completed_at": instance.CompletedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return workflow.ErrMutationConflict
		}
		currentNodes := make([]workflow.DefinitionNode, 0, len(current))
		for _, nodeID := range current {
			node, found := definition.Node(nodeID)
			if !found {
				return workflow.ErrMutationConflict
			}
			currentNodes = append(currentNodes, node)
			if strings.HasPrefix(node.Type, "ai_") {
				runID, err := dependencies.AI.StartSubjectAI(ctx, workflow.SubjectAIStart{
					WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
					Capability: node.Capability, SubjectType: command.SubjectType, SubjectID: command.SubjectID,
					SubjectVersion: command.SubjectVersion, SubjectSnapshot: command.SubjectSnapshot,
				})
				if err != nil || runID == 0 {
					return workflow.ErrMutationUnavailable
				}
				run := WorkflowNodeRun{WorkspaceID: command.WorkspaceID, InstanceID: instance.ID,
					NodeID: node.ID, NodeType: node.Type, Status: "waiting", StartedAt: now}
				if err := tx.Create(&run).Error; err != nil {
					return err
				}
				payload, _ := json.Marshal(map[string]any{"ai_run_id": runID, "node_id": node.ID})
				job := WorkflowJob{WorkspaceID: command.WorkspaceID, InstanceID: instance.ID, NodeRunID: run.ID,
					Kind: "ai_run", Status: "pending", PayloadJSON: string(payload),
					IdempotencyKey: fmt.Sprintf("workflow:%d:%s:%d", instance.ID, node.ID, run.ID), AvailableAt: now}
				if err := tx.Create(&job).Error; err != nil {
					return err
				}
			}
			if err := dependencies.Assignments.RecordSubjectAssignment(ctx, workflow.SubjectAssignment{
				WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID,
				ProjectSetID: command.ProjectSetID, TeamID: command.TeamID, InstanceID: instance.ID,
				SubjectID: command.SubjectID, SubjectType: command.SubjectType,
				NodeID: node.ID, NodeName: node.Name, AssigneeUserID: command.PrincipalID, SubjectTitle: command.SubjectTitle,
			}); err != nil {
				return err
			}
		}
		if err := dependencies.Effects.RecordWorkflowAction(ctx, workflow.ActionEffect{
			WorkspaceID: command.WorkspaceID, PrincipalID: command.PrincipalID, ProjectSetID: command.ProjectSetID,
			InstanceID: instance.ID, Action: "started", Status: instance.Status, ApplyKey: command.ApplyKey,
		}); err != nil {
			return err
		}
		subjectStatus := workflow.SubjectStatusForCurrent(command.SubjectType, instance.Status, currentNodes)
		result = workflow.SubjectStartResult{InstanceID: instance.ID, Status: instance.Status,
			SubjectStatus: subjectStatus, CurrentNodeIDs: current}
		raw, _ := json.Marshal(result)
		completed := tx.Model(&ToolMutationApply{}).Where("id = ? AND result_json = ''", ledger.ID).Update("result_json", string(raw))
		if completed.Error != nil {
			return completed.Error
		}
		if completed.RowsAffected != 1 {
			return workflow.ErrMutationConflict
		}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if replay, found, replayErr := repository.replaySubjectStart(ctx, command); found || replayErr != nil {
		return replay, replayErr
	}
	return workflow.SubjectStartResult{}, normalizeWorkflowMutationError(err)
}

func loadSubjectStartDefinition(tx *gorm.DB, command workflow.SubjectStartCommand) (WorkflowTemplate, WorkflowTemplateVersion, workflow.Definition, error) {
	var template WorkflowTemplate
	if err := tx.Where("id = ? AND workspace_id = ? AND subject_type = ?", command.TemplateID, command.WorkspaceID, command.SubjectType).
		First(&template).Error; err != nil {
		return template, WorkflowTemplateVersion{}, workflow.Definition{}, normalizeWorkflowMutationError(err)
	}
	if template.PublishedVersion == 0 {
		return template, WorkflowTemplateVersion{}, workflow.Definition{}, workflow.ErrMutationConflict
	}
	var version WorkflowTemplateVersion
	if err := tx.Where("workspace_id = ? AND template_id = ? AND version = ?", command.WorkspaceID, template.ID, template.PublishedVersion).
		First(&version).Error; err != nil {
		return template, version, workflow.Definition{}, normalizeWorkflowMutationError(err)
	}
	definition, err := workflow.ParseDefinition(version.Definition)
	if err != nil || subjectStartNodeID(definition) == "" {
		return template, version, workflow.Definition{}, workflow.ErrMutationConflict
	}
	return template, version, definition, nil
}

func subjectStartNodeID(definition workflow.Definition) string {
	start, ends := "", 0
	for _, node := range definition.Nodes {
		switch node.Type {
		case "start":
			if start != "" {
				return ""
			}
			start = node.ID
		case "end":
			ends++
		}
	}
	if start == "" || ends != 1 || len(definition.Next(start, "complete")) == 0 {
		return ""
	}
	return start
}

func (repository *SubjectStartRepository) replaySubjectStart(
	ctx context.Context,
	command workflow.SubjectStartCommand,
) (workflow.SubjectStartResult, bool, error) {
	var ledger ToolMutationApply
	err := repository.db.WithContext(ctx).Where("apply_key = ?", command.ApplyKey).First(&ledger).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workflow.SubjectStartResult{}, false, nil
	}
	if err != nil {
		return workflow.SubjectStartResult{}, false, err
	}
	if ledger.WorkspaceID != command.WorkspaceID || ledger.Operation != "subject.start" || ledger.InputDigest != command.InputDigest || ledger.ResultJSON == "" {
		return workflow.SubjectStartResult{}, true, workflow.ErrMutationConflict
	}
	var result workflow.SubjectStartResult
	if json.Unmarshal([]byte(ledger.ResultJSON), &result) != nil || result.InstanceID == 0 {
		return workflow.SubjectStartResult{}, true, workflow.ErrMutationConflict
	}
	return result, true, nil
}

var _ workflow.SubjectStartStore = (*SubjectStartRepository)(nil)
